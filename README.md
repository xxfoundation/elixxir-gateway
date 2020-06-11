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

# Path to where the IDF is saved. This is used by the wrapper management script.
idfPath: "gatewayIDF.json"

# The path to the private key associated with the self-signed TLS certificate.
keyPath: "gateway.cmix.rip.key"

# The path to the self-signed TLS certificate for Gateway. Expects PEM format.
certPath: "gateway.cmix.rip.crt"

# The path to the self-signed TLS certificate for Server. Expects PEM format.
serverCertPath: "cmix.rip.crt"

# The path to the self-signed TLS certificate for the Permissioning server. Expects PEM format.
permissioningCertPath: "permissioning.cmix.rip.crt"
```

## Command line flags

Note that the `--config` flag, if left blank, will by default look for
`gateway.yaml` in the user's home directory first and secon din `/etc/`.

| Long flag | Short flag | Effect |
|---|---|---|
|--certPath string| |Path to the self-signed TLS certificate for Gateway. Expects PEM format. Required field.|
|--config string|-c|Path to load the Gateway configuration file from.|
|--help|-h|help for gateway|
|--idfPath string| |Path to where the IDF is saved. This is used by the wrapper management script. (default "/home/User/.xxnetwork/idf.json")|
|--keyPath string| |Path to the private key associated with the self-signed TLS certificate. Required field.|
|--log string| |Path where log file will be saved. (default "/home/User/.xxnetwork/cmix-gateway.log")|
|--logLevel uint|-l|Level of debugging to print (0 = info, 1 = debug, >1 = trace).|
|--messageTimeout duration| |Period in which the message cleanup function executes. Recommended period is on the order of a minute. (default 1m0s)|
|--nodeAddress string| |Public IP address of the Node associated with this Gateway. Required field.|
|--permissioningCertPath string| |Path to the self-signed TLS certificate for the Permissioning server. Expects PEM format. Required field.|
|--port int|-p|Port for Gateway to listen on. Gateway must be the only listener on this port. Required field. (default -1)|
|--serverCertPath string| |Path to the self-signed TLS certificate for Server. Expects PEM format. Required field.|

### Generate version information

To generate version information, including versions of dependencies, before building or running, run this command:

`$ go generate cmd/version.go`
