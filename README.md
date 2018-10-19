# privategrity/gateway

[![pipeline status](https://gitlab.com/privategrity/gateway/badges/master/pipeline.svg)](https://gitlab.com/privategrity/gateway/commits/master)
[![coverage report](https://gitlab.com/privategrity/gateway/badges/master/coverage.svg)](https://gitlab.com/privategrity/gateway/commits/master)

## Purpose

Gateways are go-betweens for the servers and clients. They retain messages that
have gone through the network for clients to fetch at their leisure, and send
batches of unprocessed messages to the server team that will process them.

Gateways are likely to acquire additional functions in the future, including
load balancing and DDoS protection, and connecting to more than one node at
a time.

## How to run locally

Tests: ` $ go test ./...`

Gateway (example options): ` $ go run main.go --config gateway.yaml`

## Example configuration file

Note: YAML prohibits the use of tabs because whitespace has meaning.

```yaml
# This is really useful for tracing the sending/receiving of individual messages
# Especially if you're testing clients and making sure that they're actually sending messages to the right people
# Recommend turning off for production, because of the space that verbose logs take up on disk
verbose: true
# Log to a different log file if you like
log: "gateway.log"
# This the port the gateway will listen on
# To limit traffic to localhost only, use "localhost:port" instead of ":port", which listens for all incoming traffic
# Clients connecting to this gateway must specify the gateway's IP address and this port
GatewayAddress: ":8443"
# This would be for four nodes on the local network
cMixNodes:
 - "localhost:50000"
 - "localhost:50001"
 - "localhost:50002"
 - "localhost:50003"
# The gateway gets and puts message batches from/to this node from the cMixNodes list
# This index is zero-based
GatewayNodeIndex: 0
# In the current implementation, messages waiting for individual users get
# deleted between this amount of time and twice this amount of time
MessageTimeout: 15600
# We've assumed that the gateway uses the same batch size as the nodes do
# We plan to deprecate this option
batchSize: 27
```

## Command line flags

| Long flag | Short flag | Effect |
|---|---|---|
|--help|-h|Shows a help message|
|--verbose|-v|Log more things to help debugging|
|--version|-V|Print full version information|

### Generate version information

To generate version information, including versions of dependencies, before building or running, run this command:

` $ go generate cmd/version.go`
