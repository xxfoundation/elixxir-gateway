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
# Level of debugging to print (0 = info, 1 = debug, >1 = trace). (Default info)
logLevel: 1

# Path where log file will be saved. (Default "log/gateway.log")
log: "/opt/xxnetwork/log/gateway.log"

# Port for Gateway to listen on. Gateway must be the only listener on this port.
# (Required)
port: 22840

# Local IP address of the Gateway, used for internal listening. Expects an IPv4
# address without a port. (Default "0.0.0.0")
listeningAddress: ""

# The public IPv4 address of the Gateway, as reported to the network. When not
# set, external IP address lookup services are used to set this value. If a
# port is not included, then the port from the port flag is used instead.
overridePublicIP: ""

# The IP address of the machine running cMix that the Gateway communicates with.
# Expects an IPv4 address with a port. (Required)
cmixAddress: "0.0.0.0:11420"

# Path to where the identity file (IDF) is saved. The IDF stores the Gateway's
# network identity. This is used by the wrapper management script. (Required)
idfPath: "/opt/xxnetwork/cred/gateway-IDF.json"

# Path to the private key associated with the self-signed TLS certificate.
# (Required)
keyPath: "/opt/xxnetwork/cred/gateway-key.key"

# Path to the self-signed TLS certificate for Gateway. Expects PEM format.
# (Required)
certPath: "/opt/xxnetwork/cred/gateway-cert.crt"

# Path to the self-signed TLS certificate for cMix. Expects PEM format.
# (Required)
cmixCertPath: "/opt/xxnetwork/cred/cmix-cert.crt"

# Path to the self-signed TLS certificate for the Scheduling server. Expects
# PEM format. (Required)
schedulingCertPath: "/opt/xxnetwork/cred/scheduling-cert.crt"

# Database connection information. (Required)
dbName: "cmix_gateway"
dbAddress: "0.0.0.0:5432"
dbUsername: "cmix"
dbPassword: ""

# Flags listed below should be left as their defaults unless you know what you
# are doing.

# How often the periodic storage tracker checks for items older than the
# retention period value. Expects duration in "s", "m", "h". (Defaults to 5
# minutes)
cleanupInterval: 5m

# Flags for gossip protocol

# How long a message record should last in the gossip buffer if it arrives
# before the Gateway starts handling the gossip. (Default 300s)
bufferExpiration: 300s

# Frequency with which to check the gossip buffer. Should be long, since the
# thread takes a lock each time it checks the buffer. (Default 150s)
monitorThreadFrequency: 150s

# Flags for rate limiting communications

# The capacity of rate limiting buckets in the map. (Default 20)
capacity: 20

# The rate that the rate limiting bucket leaks tokens at [tokens/ns]. (Default 3)
leakedTokens: 3

# How often the number of leaked tokens is leaked from the bucket. (Default 1ms)
leakDuration: 1ms

# How often inactive buckets are removed. (Default 10s)
pollDuration: 10s

# The max age of a bucket without activity before it is removed. (Default 10s)
bucketMaxAge: 10s

# time.Duration used to calculate lower bound of when to replace TLS cert (default 30 days)
replaceHttpsCertBuffer: 720h

# time.Duration used to calculate upper bound of when to replace TLS cert (default 7 days)
maxCertReplaceRange: 168h
```

## Command line flags

The command line flags for the server can be generated `--help` as follows:


```
% go run main.go --help
The cMix gateways coordinate communications between servers and clients

Usage:
  gateway [flags]
  gateway [command]

Available Commands:
  autocert    automatic cert request test command
  generate    Generates version and dependency information for the xx network binary
  help        Help about any command
  version     Print the version and dependency information for the xx network binary

Flags:
      --bucketMaxAge duration             The max age of a bucket without activity before it is removed. (default 10s)
      --bufferExpiration duration         How long a message record should last in the gossip buffer if it arrives before the Gateway starts handling the gossip. (default 5m0s)
      --capacity uint32                   The capacity of rate-limiting buckets in the map. (default 20)
      --certPath string                   Path to the self-signed TLS certificate for Gateway. Expects PEM format. (Required)
      --cmixAddress string                The IP address of the machine running cMix that the Gateway communicates with. Expects an IPv4 address with a port. (Required)
      --cmixCertPath string               Path to the self-signed TLS certificate for cMix. Expects PEM format. (Required)
  -c, --config string                     Path to load the Gateway configuration file from. (Required)
      --enableGossip                      Feature flag for in progress gossip functionality
  -h, --help                              help for gateway
      --idfPath string                    Path to where the identity file (IDF) is saved. The IDF stores the Gateway's Node's network identity. This is used by the wrapper management script. (Required)
      --keyPath string                    Path to the private key associated with the self-signed TLS certificate. (Required)
      --kr int                            Amount of rounds to keep track of in kr (default 1024)
      --leakDuration duration             How often the number of leaked tokens is leaked from the bucket. (default 1ms)
      --leakedTokens uint32               The rate that the rate limiting bucket leaks tokens at [tokens/ns]. (default 3)
      --log string                        Path where log file will be saved. (default "log/gateway.log")
  -l, --logLevel uint                     Level of debugging to print (0 = info, 1 = debug, >1 = trace).
      --monitorThreadFrequency duration   Frequency with which to check the gossip buffer. (default 2m30s)
      --pollDuration duration             How often inactive buckets are removed. (default 10s)
  -p, --port int                          Port for Gateway to listen on.Gateway must be the only listener on this port. (Required) (default -1)
      --profile-cpu string                Enable cpu profiling to this file
      --schedulingCertPath string         Path to the self-signed TLS certificate for the Scheduling server. Expects PEM format. (Required)

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

The `autocert` subcommand should not be used. It allows you to make a
ZeroSSL certificate request and returns DNS Settings which can only be
set by the xx network technical team. This subcommand is used for testing only.

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
