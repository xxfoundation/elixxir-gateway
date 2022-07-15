#!/usr/bin/env python3

"""
This script can be used to scrape local Gateway database
to output a CSV containing information about ephemeralId
usage in the network. From these data, you can infer
the number of active users in the network.

To run, install dependencies:
    pip3 install psycopg2-binary
And provide DB connection info as program arguments, if needed.
"""

import argparse
import csv
import datetime
import logging as log
import sys
import time
import boto3
import botocore
from botocore.config import Config
import threading
import os
import psycopg2

# Static keys used for reading and storing state to the database
last_run_state_key = "ActiveUsersEpoch"
period_state_key = "Period"
# Determines number of epochs to check for each run per period
check_ranges = [1, 12, 48]
# Maximum number of historical epochs that can be checked
# 21 days * 24 hours * 2 epochs per hour
# Should probably be modified to rely on the period value
max_historical_epochs = 21 * 24 * 2


def main():
    # Process input variables and program arguments
    args = get_args()
    log.info("Running with configuration: {}".format(args))
    db_host = args['host']
    db_port = args['port']
    db_name = args['db']
    db_user = args['user']
    db_pass = args['pass']
    output_path = args['output']
    s3_access_key_id = args["aws_key"]
    s3_access_key_secret = args["aws_secret"]
    s3_region = args["aws_region"]
    id_file = args["id_file"]
    cloudwatch_log_group = args["cloudwatch_group"]

    conn, csv_file = None, None
    try:
        # Set up output file
        csv_file = open(output_path, "a")
        csv_writer = csv.writer(csv_file, delimiter=',',
                                quotechar='"', quoting=csv.QUOTE_MINIMAL)

        # Set up cloudwatch logging
        cw_log_thread = start_cw_logger(cloudwatch_log_group, args['log'],  id_file, s3_region,
                                                                   s3_access_key_id, s3_access_key_secret)

        # Set up database connection
        conn = get_conn(db_host, db_port, db_name, db_user, db_pass)
        # Obtain period from GW database
        period = get_state(conn, period_state_key)
        # Convert database time (in nanoseconds) to seconds
        period = period / 1e9

        # Get epoch of last run
        last_run_epoch = get_state(conn, last_run_state_key)
        if last_run_epoch is None:
            # If no database time, start as far back as possible
            current_time = time.mktime(datetime.datetime.now().timetuple())
            current_epoch = current_time / period
            last_run_epoch = current_epoch - max_historical_epochs
        # Force into integer space
        last_run_epoch = int(last_run_epoch)

        epoch_to_run = last_run_epoch + 1
        while True:
            # If current epoch is less than last_run_epoch, wait for next epoch
            current_time = time.mktime(datetime.datetime.now().timetuple())
            current_epoch = int(current_time / period)
            if current_epoch <= epoch_to_run:
                next_epoch_start_time = (epoch_to_run + 1) * period
                wait_time = next_epoch_start_time - current_time
                log.info(f"Waiting {wait_time}s for epoch {epoch_to_run} to finish...")
                time.sleep(wait_time)

            # Obtain unique_users count for each range in check_ranges
            end_timestamp = epoch_to_run * period + period
            log.info(f"Fetching information for epoch {epoch_to_run}...")
            for epoch_range in check_ranges:
                real_epoch_range = epoch_range - 1  # Subtract one to avoid double-counting the current
                unique_users = count_in_epoch_range(conn,
                                                    epoch_to_run - real_epoch_range,
                                                    epoch_to_run)

                # Output format: epoch,numEpochs,endTimestamp,numUnique
                output = [epoch_to_run, epoch_range,
                          datetime.datetime.fromtimestamp(end_timestamp), unique_users]
                log.debug(f"Results: {output}")

                # Write output to file
                csv_writer.writerow(output)

            # Update state to declare current run as completed
            upsert_state(conn, last_run_state_key, epoch_to_run)
            # Increment for next loop
            epoch_to_run += 1

    except Exception as e:
        log.fatal(f"Unhandled exception occurred: {e}", exc_info=True)
        if csv_file:
            csv_file.close()
        if conn:
            conn.close()
        cw_log_thread.join()
        sys.exit(1)


def start_cw_logger(cloudwatch_log_group, log_file_path, id_path, region, access_key_id, access_key_secret):
    """
    start_cw_logger is a blocking function which starts the thread to log to cloudwatch.
    This requires a blocking function so we can ensure that if a log file is present,
    it is opened before logging resumes.  This prevents lines from being omitted in cloudwatch.

    :param cloudwatch_log_group: log group name for cloudwatch logging
    :param log_file_path: Path to the log file
    :param id_path: path to node's id file
    :param region: AWS region
    :param access_key_id: aws access key
    :param access_key_secret: aws secret key
    """
    # If there is already a log file, open it here so we don't lose records
    log_file = None

    # Configure boto retries
    config = Config(
        retries=dict(
            max_attempts=50
        )
    )

    # Setup cloudwatch logs client
    client = boto3.client('logs', region_name=region,
                          aws_access_key_id=access_key_id,
                          aws_secret_access_key=access_key_secret,
                          config=config)

    # Open the log file read-only and pin its current size if it already exists
    if os.path.isfile(log_file_path):
        log_file = open(log_file_path, 'r')
        log_file.seek(0, os.SEEK_END)

    # Start the log backup service
    thr = threading.Thread(target=cloudwatch_log,
                           args=(cloudwatch_log_group, log_file_path,
                                 id_path, log_file, client))
    thr.start()
    return thr


def cloudwatch_log(cloudwatch_log_group, log_file_path, id_path, log_file, client):
    """
    cloudwatch_log is intended to run in a thread.  It will monitor the file at
    log_file_path and send the logs to cloudwatch.  Note: if the node lacks a
    stream, one will be created for it, named by node ID.

    :param client: cloudwatch client for this logging thread
    :param log_file: log file for this logging thread
    :param cloudwatch_log_group: log group name for cloudwatch logging
    :param log_file_path: Path to the log file
    :param id_path: path to node's id file
    """
    global read_node_id
    # Constants
    megabyte = 1048576  # Size of one megabyte in bytes
    max_size = 100 * megabyte  # Maximum log file size before truncation
    push_frequency = 3  # frequency of pushes to cloudwatch, in seconds
    max_send_size = megabyte

    # Event buffer and storage
    event_buffer = ""  # Incomplete data not yet added to log_events for push to cloudwatch
    log_events = []  # Buffer of events from log not yet sent to cloudwatch
    events_size = 0

    log_file, client, log_stream_name, upload_sequence_token, init_err = init(log_file_path, id_path,
                                                                              cloudwatch_log_group, log_file, client)
    if init_err:
        log.error("Failed to init cloudwatch logging for {}: {}".format(cloudwatch_log_group, init_err))
        return

    log.info("Starting cloudwatch logging for {}...".format(cloudwatch_log_group))

    last_push_time = time.time()
    last_line_time = time.time()
    while True:
        event_buffer, log_events, events_size, last_line_time = process_line(log_file, event_buffer, log_events,
                                                                             events_size, last_line_time)

        # Check if we should send events to cloudwatch
        log_event_size = 26
        is_over_max_size = len(event_buffer.encode(encoding='utf-8')) + log_event_size + events_size > max_send_size
        is_time_to_push = time.time() - last_push_time > push_frequency

        if (is_over_max_size or is_time_to_push) and len(log_events) > 0:
            # Send to cloudwatch, then reset events, size and push time
            upload_sequence_token, ok = send(client, upload_sequence_token,
                                             log_events, log_stream_name, cloudwatch_log_group)
            if ok:
                events_size = 0
                log_events = []
                last_push_time = time.time()

        # Clear the log file if it has exceeded maximum size
        log_size = os.path.getsize(log_file_path)
        log.debug("Current log {} size: {}".format(log_file_path, log_size))
        if log_size > max_size:
            # Close the old log file
            log.warning("Log {} has reached maximum size: {}. Clearing...".format(log_file_path, log_size))
            log_file.close()
            # Overwrite the log with an empty file and reopen
            log_file = open(log_file_path, "w+")
            log.info("Log {} has been cleared. New Size: {}".format(
                log_file_path, os.path.getsize(log_file_path)))


def init(log_file_path, id_path, cloudwatch_log_group, log_file, client):
    """
    Initialize client for cloudwatch logging
    :param client: cloudwatch client for this logging thread
    :param log_file_path: path to log output
    :param id_path: path to id file
    :param cloudwatch_log_group: cloudwatch log group name
    :param log_file: log file to read lines from
    :return log_file, client, log_stream_name, upload_sequence_token:
    """
    global read_node_id
    upload_sequence_token = ""

    # If the log file does not already exist, wait for it
    if not log_file:
        while not os.path.isfile(log_file_path):
            time.sleep(1)
        # Open the newly-created log file as read-only
        log_file = open(log_file_path, 'r')

    # Define prefix for log stream - should be ID based on file
    if read_node_id:
        log_prefix = read_node_id
    # Read node ID from the file
    else:
        log.info("Waiting for ID file...")
        while not os.path.exists(id_path):
            time.sleep(1)
        log_prefix = get_node_id(id_path)

    log_name = os.path.basename(log_file_path)  # Name of the log file
    log_stream_name = "{}-{}".format(log_prefix, log_name)  # Stream name should be {ID}-{node/gateway}.log

    try:
        # Determine if stream exists.  If not, make one.
        streams = client.describe_log_streams(logGroupName=cloudwatch_log_group,
                                              logStreamNamePrefix=log_stream_name)['logStreams']

        if len(streams) == 0:
            # Create a log stream on the fly if ours does not exist
            client.create_log_stream(logGroupName=cloudwatch_log_group, logStreamName=log_stream_name)
        else:
            # If our log stream exists, we need to get the sequence token from this call to start sending to it again
            for s in streams:
                if log_stream_name == s['logStreamName'] and 'uploadSequenceToken' in s.keys():
                    upload_sequence_token = s['uploadSequenceToken']

    except Exception as e:
        return None, None, None, None, e

    return log_file, client, log_stream_name, upload_sequence_token, None


def process_line(log_file, event_buffer, log_events, events_size, last_line_time):
    """
    Accepts current buffer and log events from main loop
    Processes one line of input, either adding an event or adding it to the buffer
    New events are marked by a string in log_starters, or are separated by more than 0.5 seconds
    :param log_file: file to read line from
    :param last_line_time: Timestamp when last log line was read
    :param events_size: message size for log events per aws docs
    :param event_buffer: string buffer of concatenated lines that make up a single event
    :param log_events: current array of events
    :return:
    """
    # using these to deliniate the start of an event
    log_starters = ["INFO", "WARN", "DEBUG", "ERROR", "FATAL", "TRACE"]

    # This controls how long we should wait after a line before assuming it's the end of an event
    force_event_time = 1
    maximum_event_size = 260000

    # Get a line and mark the time it's read
    line = log_file.readline()
    line_time = int(round(time.time() * 1000))  # Timestamp for this line

    # Check the potential size, if over max, we should force a new event
    potential_buffer = event_buffer + line
    is_event_too_big = len(potential_buffer.encode(encoding='utf-8')) > maximum_event_size

    if not line:
        # if it's been more than force_event_time since last line, push buffer to events
        is_new_line = time.time() - last_line_time > force_event_time and event_buffer != ""
        time.sleep(0.5)
    else:
        # Reset last line time
        last_line_time = time.time()
        # If a new event is starting, push buffer to events
        is_new_line = line.split(' ')[0] in log_starters and event_buffer != ""

    if is_new_line or is_event_too_big:
        # Push the buffer into events
        size = len(event_buffer.encode(encoding='utf-8'))
        log_events.append({'timestamp': line_time, 'message': event_buffer})
        event_buffer = ""
        events_size += (size + 26)  # Increment buffer size by message len + 26 (per aws documentation)

    if line:
        if len(line.encode(encoding='utf-8')) > maximum_event_size:
            line = line[:maximum_event_size-1]
        # Push line on to the buffer
        event_buffer += line

    return event_buffer, log_events, events_size, last_line_time


def send(client, upload_sequence_token, log_events, log_stream_name, cloudwatch_log_group):
    """
    send is a helper function for cloudwatch_log, used to push a batch of events to the proper stream
    :param log_events: Log events to be sent to cloudwatch
    :param log_stream_name: Name of cloudwatch log stream
    :param cloudwatch_log_group: Name of cloudwatch log group
    :param client: cloudwatch logs client
    :param upload_sequence_token: sequence token for log stream
    :return: new sequence token
    """
    ok = True
    if len(log_events) == 0:
        return upload_sequence_token

    try:
        if upload_sequence_token == "":
            # for the first message in a stream, there is no sequence token
            resp = client.put_log_events(logGroupName=cloudwatch_log_group,
                                         logStreamName=log_stream_name,
                                         logEvents=log_events)
        else:
            resp = client.put_log_events(logGroupName=cloudwatch_log_group,
                                         logStreamName=log_stream_name,
                                         logEvents=log_events,
                                         sequenceToken=upload_sequence_token)
        upload_sequence_token = resp['nextSequenceToken']  # set the next sequence token

        # IF anything was rejected, log as warning
        if 'rejectedLogEventsInfo' in resp.keys():
            log.warning("Some log events were rejected:")
            log.warning(resp['rejectedLogEventsInfo'])

    except client.exceptions.InvalidSequenceTokenException as e:
        ok = False
        log.warning(f"Boto3 invalidSequenceTokenException encountered: {e}")
        upload_sequence_token = e.response['Error']['Message'].split()[-1]
    except botocore.exceptions.ClientError as e:
        ok = False
        log.error("Boto3 client error encountered: %s" % e)
    except Exception as e:
        ok = False
        log.error(e)
    finally:
        # Always return upload sequence token - dropping this causes lots of errors
        return upload_sequence_token, ok


def count_in_epoch_range(conn, start, end):
    """
    Returns count of unique ClientBloomFilter.RecipientId for given Epoch
    :param conn: database connection object
    :param start: first epoch to check
    :param end: last epoch to check
    :return: int
    """
    cur = conn.cursor()
    select_command = f"SELECT COUNT(DISTINCT recipient_id)  FROM client_bloom_filters" \
                     f" WHERE epoch BETWEEN {start} AND {end};"
    try:
        cur.execute(select_command)
        log.debug(cur.query)
        conn.commit()
    except Exception as e:
        log.error(f"Failed to count in epoch range: {cur.query}")
        cur.close()
        raise e

    res = cur.fetchone()
    cur.close()
    return int(res[0])


def upsert_state(conn, key, value):
    """
    Update state value in the database
    :param conn: database connection object
    :param key: key to input value at
    :param value: value to input into state
    :return:
    """
    cur = conn.cursor()
    update_command = "INSERT INTO states (key, value) VALUES (%s, %s) " \
                     "ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;"
    try:
        cur.execute(update_command, (key, str(value),))
        log.debug(cur.query)
        conn.commit()
    except Exception as e:
        log.error(f"Failed to set state value: {cur.query}")
        raise e
    finally:
        cur.close()


def get_state(conn, key):
    """
    Get last run timestamp from the database
    :param conn: database connection object
    :param key: state key to look up
    :return: int(result) or None
    """
    cur = conn.cursor()
    select_command = "SELECT value FROM states WHERE key = %s;"
    try:
        cur.execute(select_command, (key,))
        log.debug(cur.query)
    except Exception as e:
        log.error(f"Failed to get state value from db: {cur.query}")
        cur.close()
        raise e

    res = cur.fetchone()
    cur.close()
    if res is None:
        log.info("No state value found!")
        return None
    return int(res[0])


def get_conn(host, port, db, user, pw):
    """
    Create a database connection object for use in the rest of the script
    :param host: Hostname for database connection
    :param port: port for database connection
    :param db: database name
    :param user: database user
    :param pw: database password
    :return: connection object for the database
    """
    conn_str = "dbname={} user={} password={} host={} port={}".format(db, user, pw, host, port)
    try:
        conn = psycopg2.connect(conn_str)
    except Exception as e:
        log.error(f"Failed to get database connection: {conn_str}")
        raise e
    log.info("Connected to {}@{}:{}/{}".format(user, host, port, db))
    return conn


def get_args():
    """
    get_args controls the argparse usage for the script. It sets up and parses
    arguments and returns them in dict format
    """
    parser = argparse.ArgumentParser(description="Options for active users monitoring script")
    parser.add_argument("--verbose", action="store_true",
                        help="Print debug logs", default=False)
    parser.add_argument("--log", type=str,
                        help="Path to output log information",
                        default="active_users.log")
    parser.add_argument("--output", type=str,
                        help="Path to output results in CSV format",
                        default="active_users.csv")
    parser.add_argument("-a", "--host", metavar="host", type=str,
                        help="Database server host for attempted connection",
                        default="localhost")
    parser.add_argument("-p", "--port", type=int,
                        help="Port for database connection",
                        default=5432)
    parser.add_argument("-d", "--db", type=str,
                        help="Database name",
                        default="cmix_gateway")
    parser.add_argument("-U", "--user", type=str,
                        help="Username for connecting to database",
                        default="cmix")
    parser.add_argument("--pass", type=str,
                        help="DB password")
    parser.add_argument("--aws-key", type=str, required=True,
                        help="aws access key")
    parser.add_argument("--aws-secret", type=str, required=True,
                        help="aws access key secret")
    parser.add_argument("--aws-region", type=str, required=True,
                        help="aws region")
    parser.add_argument("--id-file", type=str, required=True,
                        help="Id file path")
    parser.add_argument("--cloudwatch-group", type=str, required=True,
                        help="Cloudwatch log group")

    args = vars(parser.parse_args())
    log.basicConfig(format='[%(levelname)s] %(asctime)s: %(message)s',
                    level=log.DEBUG if args['verbose'] else log.INFO,
                    datefmt='%d-%b-%y %H:%M:%S',
                    filename=args["log"])
    return args


if __name__ == "__main__":
    main()
