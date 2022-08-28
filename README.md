# gstbgen

Gstbgen generates stub server code for external APIs and is primarily used for system analysis, load testing, and debugging.

# How it works

Gstbgen is used as an HTTP/HTTPS forward proxy of the system under test (System Unser Test: SUT) that interacts with external systems. It records requests and responses and generates Go code for the stub server that behaves as the external API. It can be edited as needed and easily used as a stub server.

The generated code contains a comment with the correspondence between the hostname of the external API and the port number that the generated server listens on.
By rewriting the hostname information of the external API used by the SUT to the address where the generated stub server is running, the SUT can be used without depending on the external API.

This would be useful for load testing and debugging the SUT.

# Installation

```
$ go install github.com/teru01/gstbgen@latest
```

# Usage

```
$ ./gstbgen
```
