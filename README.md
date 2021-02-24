# elixxir/gateway

[![pipeline status](https://gitlab.com/elixxir/gateway/badges/master/pipeline.svg)](https://gitlab.com/elixxir/gateway/commits/master)
[![coverage report](https://gitlab.com/elixxir/gateway/badges/master/coverage.svg)](https://gitlab.com/elixxir/gateway/commits/master)

## Purpose

Gateways are go-betweens for the servers and clients. They retain messages that
have gone through the network for clients to fetch at their leisure, and send
batches of unprocessed messages to the server team that will process them.

Gateways are likely to acquire additional functions in the future, including
load balancing and DDoS protection, and connecting to more than one node at
a time.

## Running a Gateway

To run the gateway:

```
go run main.go --config [configuration-file]
```

## Example configuration file

The Gateway configuration file must be named `gateway.yaml` and be located in
one of the following directories:
1. `$HOME/.xxnetwork/`
2. `/opt/xxnetwork/`
3. `/etc/xxnetwork/`

Gateway searches for the YAML file in that order and uses the first occurance
found.

Note: YAML prohibits the use of tabs because whitespace has meaning.

```yaml
# Level of debugging to print (0 = info, 1 = debug, >1 = trace). (default 0)
logLevel: 1

# Path where log file will be saved. (default "./gateway-logs/gateway.log")
log: "/opt/xxnetwork/gateway-logs/gateway.log"

# Port for Gateway to listen on. Gateway must be the only listener on this port.
# Required field.
port: 22840

# Local IP address of the Gateway, used for internal listening. Expects an IPv4
# address without a port. (default "0.0.0.0")
listeningAddress: ""

# The public IPv4 address of the Gateway, as reported to the network, to use
# instead of dynamically looking up Gateway's own IP address. If a port is not
# included, then the port flag is used instead.

# The public IPv4 address of the Gateway, as reported to the network, to be
# used instead of dynamically looking up Gateway's own IP address. If a port
# is not included, then the port from the port flag is used instead.
overridePublicIP: ""

# The IP address of the Node that the Gateway communicates with. Expects an IPv4
# address with a port. Required field.
nodeAddress: "0.0.0.128:11420"

# Period in which the message cleanup function executes. All users who message
# buffer have exceeded the maximum size will get their messages deleted.
# Recommended period is on the order of a minute to an hour. (default 1m0s)
messageTimeout: "1m0s"

# Path to where the IDF is saved. This is used by the wrapper management script.
# (default "./gateway-logs/gatewayIDF.json")
idfPath: "/opt/xxnetwork/gateway-logs/gatewayIDF.json"

# Path to the private key associated with the self-signed TLS certificate.
# Required field.
keyPath: "/opt/xxnetwork/creds/gateway_key.key"

# Path to the self-signed TLS certificate for Gateway. Expects PEM format.
# Required field.
certPath: "/opt/xxnetwork/creds/gateway_cert.crt"

# Path to the self-signed TLS certificate for Server. Expects PEM format.
# Required field.
serverCertPath: "/opt/xxnetwork/creds/node_cert.crt"

# Path to the self-signed TLS certificate for the Permissioning server. Expects
# PEM format. Required field.
permissioningCertPath: "/opt/xxnetwork/creds/permissioning_cert.crt"

# How long messages, rounds and bloom filters remain in storage 
# before being cleaned from storage. 
# Valid time units are "h"
# Defaults to 1 week (168 hours) if not set
retentionPeriod: "168h"

# How often the periodic storage tracker checks for items older 
# than the retention Period value.
# Valid time units are "s", "m", "h". 
# Defaults to 5 minutes if not set
cleanupInterval: "5m"

# Database connection information
dbUsername: "cmix"
dbPassword: ""
dbName: "cmix_gateway"
dbAddress: ""

# Flags for our gossip protocol

# How long a message record should last in the buffer
BufferExpirationTime: "1m0s"

# Frequency with which to check the buffer.
# Should be long, since the thread takes a lock each time it checks the buffer
MonitorThreadFrequency: "3m0s" 

# Flags for rate limiting communications
ratelimiting:
    # The capacity of buckets in the map
    capacity: 5
    # The leak rate is calculated by LeakedTokens / LeakDuration
    # It is the rate that the bucket leaks tokens at [tokens/ns]
    leakedTokens: 3
    leakDuration: 1ms
    # Duration between polls for stale buckets
    pollDuration: 0m10s
    # Max time of inactivity before removal
    bucketMaxAge: 0m3s
```

## Command line flags

The command line flags for the server can be generated `--help` as follows:


```
$ go run main.go --help
The cMix gateways coordinate communications between servers and clients

Usage:
  gateway [flags]
  gateway [command]

Available Commands:
  generate    Generates version and dependency information for the xx network binary
  help        Help about any command
  version     Print the version and dependency information for the xx network binary

Flags:
      --bucketMaxAge duration             Max time of inactivity before removal (default 10s)
      --bufferExpiration duration         How long a message record should last in the buffer (default 5m0s)
      --capacity uint32                   Amount of buckets to keep track of for rate limiting communications (default 20)
      --certPath string                   Path to the self-signed TLS certificate for Gateway. Expects PEM format.
                                          Required field.
  -c, --config string                     Path to load the Gateway configuration file from. If not set, this file must
                                          be named gateway.yaml and must be located in ~/.xxnetwork/, /opt/xxnetwork,
                                          or /etc/xxnetwork.
      --enableGossip                      Feature flag for in progress gossip functionality
  -h, --help                              help for gateway
      --idfPath string                    Path to where the IDF is saved. This is used by the wrapper management script.
                                          (default "./gateway-logs/gatewayIDF.json")
      --keyPath string                    Path to the private key associated with the self-signed TLS certificate.
                                          Required field.
      --kr int                            Amount of rounds to keep track of in kr (default 1024)
      --leakDuration duration             Used to calculate the leak rate (default 1ms)
      --leakedTokens uint32               Used to calculate the leak rate (default 3)
      --listeningAddress string           Local IP address of the Gateway used for internal listening. (default "0.0.0.0")
      --log string                        Path where log file will be saved. (default "./gateway-logs/gateway.log")
  -l, --logLevel uint                     Level of debugging to print (0 = info, 1 = debug, >1 = trace).
      --messageTimeout duration           Period in which the message cleanup function executes. All users who message
                                          buffer have exceeded the maximum size will get their messages deleted.
                                          Recommended period is on the order of a minute to an hour. (default 1m0s)
      --monitorThreadFrequency duration   Frequency with which to check the gossip's buffer. (default 2m30s)
      --nodeAddress string                The IP address of the Node that the Gateway communicates with. Required field.
      --overridePublicIP string           The public IPv4 address of the Gateway, as reported to the network, to use
                                          instead of dynamically looking up Gateway's own IP address. If a port is not
                                          included, then the port flag is used instead.
      --permissioningCertPath string      Path to the self-signed TLS certificate for the Permissioning server. Expects
                                          PEM format. Required field.
      --pollDuration duration             Duration between polls for stale buckets (default 10s)
  -p, --port int                          Port for Gateway to listen on. Gateway must be the only listener on this port.
                                          Required field. (default -1)
      --serverCertPath string             Path to the self-signed TLS certificate for Server. Expects PEM format.
                                          Required field.

Use "gateway [command] --help" for more information about a command.


Use "gateway [command] --help" for more information about a command.
```

All of those flags, except `--config`, override values in the configuration
file.

The `version` subcommand prints the version:


```
$ go run main.go version
Elixxir Gateway v1.1.0 -- 426617f Fix MessageTimeout, change localAddress to listeningAddress and mark hidden, and change example nodeAddress

Dependencies:

module gitlab.com/elixxir/gateway

go 1.13
...
```

The `generate` subcommand is used for updating version information (see the
next section).

## Updating Version Info
```
$ go run main.go generate
$ mv version_vars.go cmd
```

## Project Structure


`cmd` handles command line flags and all gateway logic.

`notifications` handles notification logic use to push alerts to clients.

`storage` contains the database and ram-based storage implementations.

## Compiling the Binary

To compile a binary that will run the server on your platform,
you will need to run one of the commands in the following sections.
The `.gitlab-ci.yml` file also contains cross build instructions
for all of these platforms.


### Linux

```
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags '-w -s' -o gateway main.go
```

### Windows

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags '-w -s' -o gateway main.go
```

or

```
GOOS=windows GOARCH=386 CGO_ENABLED=0 go build -ldflags '-w -s' -o gateway main.go
```

for a 32 bit version.

### Mac OSX

```
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags '-w -s' -o gateway main.go
```
