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

    conn, csv_file = None, None
    try:
        # Set up output file
        csv_file = open(output_path, "a")
        csv_writer = csv.writer(csv_file, delimiter=',',
                                quotechar='"', quoting=csv.QUOTE_MINIMAL)

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
        sys.exit(1)


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

    args = vars(parser.parse_args())
    log.basicConfig(format='[%(levelname)s] %(asctime)s: %(message)s',
                    level=log.DEBUG if args['verbose'] else log.INFO,
                    datefmt='%d-%b-%y %H:%M:%S',
                    filename=args["log"])
    return args


if __name__ == "__main__":
    main()
