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

Note: YAML prohibits the use of tabs because whitespace has meaning.

```yaml
# Level of debugging to print. 0 = info, 1 = debug, >1 = trace
logLevel: 1

#Path where logs will be printed.
log: "gateway.log"

# Port for the Gateway to listen on. Gateway must be the only listener on this port.
port: 8443

# The public IP address and port of the Node associated with this Gateway.
nodeAddress: "0.0.0.128:11420"

# Period in which the message cleanup function executes. All users who message buffer have exceeded the 
# maximum size will get their messages deleted. Recommended period is on the order of a minute to an hour.
messageTimeout: "60s"

# (default "./gateway-logs/gatewayIDF.json")
idfPath: "/opt/xxnetwork/gateway-logs/gatewayIDF.json"
​
# Path to the private key associated with the self-signed TLS certificate.
# Required field.
keyPath: "/opt/xxnetwork/creds/gateway_key.key"
​
# Path to the self-signed TLS certificate for Gateway. Expects PEM format.
# Required field.
certPath: "/opt/xxnetwork/creds/gateway_cert.crt"
​
# Path to the self-signed TLS certificate for Server. Expects PEM format.
# Required field.
serverCertPath: "/opt/xxnetwork/creds/node_cert.crt"
​
# Path to the self-signed TLS certificate for the Permissioning server. Expects
# PEM format. Required field.
permissioningCertPath: "/opt/xxnetwork/creds/permissioning_cert.crt"
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
      --certPath string                Path to the self-signed TLS certificate for Gateway. Expects PEM format. Required field.
  -c, --config string                  Path to load the Gateway configuration file from.
  -h, --help                           help for gateway
      --idfPath string                 Path to where the IDF is saved. This is used by the wrapper management script. (default "/home/carback1/.xxnetwork/idf.json")
      --keyPath string                 Path to the private key associated with the self-signed TLS certificate. Required field.
      --listeningAddress string        Local IP address of the Gateway used for internal listening. (default "0.0.0.0")
      --log string                     Path where log file will be saved. (default "/home/carback1/.xxnetwork/cmix-gateway.log")
  -l, --logLevel uint                  Level of debugging to print (0 = info, 1 = debug, >1 = trace).
      --messageTimeout duration        Period in which the message cleanup function executes. Recommended period is on the order of a minute. (default 1m0s)
      --nodeAddress string             Public IP address of the Node associated with this Gateway. Required field.
      --permissioningCertPath string   Path to the self-signed TLS certificate for the Permissioning server. Expects PEM format. Required field.
  -p, --port int                       Port for Gateway to listen on. Gateway must be the only listener on this port. Required field. (default -1)
      --serverCertPath string          Path to the self-signed TLS certificate for Server. Expects PEM format. Required field.

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

`rateLimiting` handles rate limits from clients.

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
