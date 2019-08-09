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

## How to run locally

First, make sure dependencies are installed into the vendor folder by running
`glide up`. Then, in the project directory, run `go run main.go`.

If what you're working on requires you to change other repos, you can remove
the other repo from the vendor folder and Go's build tools will look for those
packages in your Go path instead. Knowing which dependencies to remove can be
really helpful if you're changing a lot of repos at once.

If glide isn't working and you don't know why, try removing glide.lock and
~/.glide to brutally cleanse the cache.

To run tests: ` $ go test ./...`

## Example configuration file

Note: YAML prohibits the use of tabs because whitespace has meaning.

```yaml
# Used for debugging
verbose: True

# Output log file
log: "gateway.log"

# The cMix nodes in the network
cMixNodes:
 - "0.0.0.0:11420"
 - "0.0.0.0:11421"
 - "0.0.0.0:11422"

# The listening port of this gateway
Port: 8443

# The number of seconds a message should remain in the globals before being
# deleted from the user's message queue
MessageTimeout: 60

# === REQUIRED FOR ENABLING TLS ===
# Path to the gateway private key file
keyPath: "gateway.cmix.rip.key"
# Path to the gateway certificate file
certPath: "gateway.cmix.rip.crt"
# Path to the gateway certificate file
serverCertPath: "cmix.rip.crt"

### Anything below this line is to be deprecated ###

# Number of nodes in the cMix Network

# Batch size of the cMix Network (to be deprecated)
batchSize: 2
```

## Command line flags

| Long flag | Short flag | Effect |
|---|---|---|
|--help|-h|Shows a help message|
|--verbose|-v|Log more things to help debugging|
|--version|-V|Print full version information|
|--config|-c|Specify alternate path to configuration file|
|--index|-i|Index of the node to connect to from the list of nodes|
|--port|-p|Port for the gateway to listen on|
|--disablePermissioning|None|Disables interaction with the Permissioning Server|

### Generate version information

To generate version information, including versions of dependencies, before building or running, run this command:

`$ go generate cmd/version.go`
