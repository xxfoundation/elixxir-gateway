# Active Users Script

### Description

The active users script is a lightweight tool for inferring the number of active clients that are
running on cMix over time. It uses bloom filter data stored in gateway databases in order
to provide that approximation. Additionally, it uploads these data to CloudWatch so that
it can be aggregated from many gateways in order to show a more realistic approximation in
a decentralized network.

### Installation

> __Note:__ This guide assumes a standard gateway installation as prescribed by the node handbook.
> The service file and script inputs can be modified for alternate installations as needed
> but will not be covered here.
>
> Run `./active-users.py --help` for more information on these options.

1. Prepare the service file in a text editor of your choice.
   There are four relevant items you must ensure are set correctly:
   1. `User=elixxir` - The username provided here should match the user that runs the gateway wrapper. Can be
      determined by running `ls -l /opt/xxnetwork` and examining the output, for example.
   2. `--pass` - Provide your gateway database password. Can be extracted from `/opt/xxnetwork/config/gateway.yaml`
      under the `dbPassword` field.
   3. `--aws-key` - AWS access credentials for CloudWatch. Can be extracted from
      `/opt/xxnetwork/xxnetwork-gateway.service` under the `--s3-access-key` field.
   4. `--aws-secret` - AWS access credentials for CloudWatch. Can be extracted from
      `/opt/xxnetwork/xxnetwork-gateway.service` under the `--s3-secret` field.
2. Install required Python dependencies by running `pip3 install psycopg2-binary`
3. Stage the script. Place `active-users.py` at `/opt/xxnetwork/active-users.py` on your gateway machine, matching
   the `ExecStart` path to the script provided in the service file.
4. Make the script executable by running `chmod +x /opt/xxnetwork/active-users.py`.
   1. Additionally, verify the user provided to the service file is the owner of the script.
      If not, you can change it via `chown elixxir:elixxir /opt/xxnetwork/active-users.py`, for example.
5. Stage the service file. Take the completed service file from __Step 1__ and place it at
   `/etc/systemd/system/active-users.service`. This will require root permissions.
   1. For example, `sudo nano /etc/systemd/system/active-users.service`, paste, and save.
6. Enable the service by running `sudo systemctl enable active-users`
7. Start the service by running `sudo systemctl restart active-users`
8. Verify the active users script is running correctly. This can be accomplished in a variety of ways:
   1. Run `systemctl status active-users`. The resulting page should indicate the service is `active (running)`.
   2. Check the log directory by running `ls /opt/xxnetwork/log`. Both the `active-users.log` and `active-users.csv`
      files should be present.
9. If these checks fail and you were specifically petitioned to run this tool, please get in touch with the team.
   Otherwise, the lightweight script will continue to run in the background unless stopped via
   `sudo systemctl stop active-users`
