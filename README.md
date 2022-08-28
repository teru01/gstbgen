# gstbgen

gstbgen generates stub server code of external APIs and is primarily used for system analysis, load testing, and debugging.

# Why gstbgen?

Stub servers are often created to avoid overloading external APIs in load tests or for analysis and debugging and so on.

If the system under test(SUT) is complex and depends on many external APIs, or if it is legacy and not well documented, creating stubs to mimic the external APIs can be a daunting task.

gstbgen semi-automates such tasks and helps create stub servers from collected requests and responses.

A similar tool is [mock-server](https://www.mock-server.com/#what-is-mockserver), but it is written in Java and the generated code is also in Java.
gstbgen is a tool for people who love Go and want to run it lightly in a single binary.

# How it works

gstbgen is used as an HTTP/HTTPS forward proxy of the SUT that interacts with external systems. It records requests and responses and generates Go code for the stub server that behaves as the external API. It can be edited as needed and easily used as a stub server.

The generated code contains a comment with the correspondence between the hostname of the external API and the port number that the generated server listens on.
By rewriting the hostname information of the external API used by the SUT to the address where the generated stub server is running, the SUT can be used without depending on the external API.

This would be useful for load testing and debugging.

# Installation

Use prebuild releases.

```
$ curl -LO https://github.com/teru01/gstbgen/releases/download/v0.1.0/gstbgen_0.1.0_[OS]_[ARCH].tar.gz
$ tar -xvf gstbgen_0.1.0_[OS]_[ARCH].tar.gz
$ mv gstbgen /usr/local/bin
```

or you can build on your own.

```
$ go install github.com/teru01/gstbgen@latest
```

# Usage

```
$ ./gstbgen -h
NAME:
   gstbgen - Stub generator for system analysis written in Go.

USAGE:
   gstbgen [global options] command [command options] [arguments...]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --cert value                     certificate path
   --debug, -d                      enable debug log (default: false)
   --help, -h                       show help (default: false)
   --host value, -H value           listening host (default: "0.0.0.0")
   --key value                      certificate key path
   --mockBeginPort value, -m value  begin port of generated mock server (default: 8080)
   --out value, -o value            generated stub server code path(default: stdout)
   --port value, -p value           listening port (default: 8888)
```

```
$ ./gstbgen
```

gstbgen listens on port 8888 by default.
You need to set `http_proxy` environment variable on the SUT to the listening address of gstbgen (for example gstbgen is running on 10.0.0.10).

```
http_proxy=10.0.0.10:8888

# example: Amazon ECS task definition
"environment": [
    {
        "name": "http_proxy",
        "value": "10.0.0.10:8888"
    }
]
```

Then, gstbgen starts to record requests and responses of the SUT and external APIs.

```
$ ./gstbgen 
4:49PM INF listening on 0.0.0.0:8888
4:49PM INF GET http://sheepex.co.jp:8080/api/foo
4:50PM INF GET http://example.com:8080/api/bar?name=hoge
```

When gstbgen exits, it generates the stub server code like following (excerpt).

```
package main

import (
        "encoding/json"
        "fmt"
        "io"
        "log"
        "net/http"
        "net/url"
        "os"
        "os/signal"
        "syscall"
)

func main() {
        func() {
                mux := http.NewServeMux()
                mux.HandleFunc("/api/bar", func(rw http.ResponseWriter, r *http.Request) {
                        if r.Method == "GET" {
                                if q, _ := stringifyUrlValues(r.URL.Query()); q == "{\"name\":[\"hoge\"]}" {
                                        body, _ := stringify(r.Body)
                                        if body == "" {
                                                rw.WriteHeader(200)
                                                fmt.Fprint(rw, "{\"code\":\"112233\",\"name\":\"foobar\"}")
                                                return
                                        }
                                }
                        }
                })
                port := 8080
                server := http.Server{
                        Addr:    "0.0.0.0:" + fmt.Sprint(port),
                        Handler: enableLogRequest(mux, port),
                }
                fmt.Printf("Listening on %v\n", server.Addr)
                go server.ListenAndServe()
        }()
        func() {
                mux := http.NewServeMux()
                mux.HandleFunc("/api/foo", func(rw http.ResponseWriter, r *http.Request) {
                        if r.Method == "GET" {
                                if q, _ := stringifyUrlValues(r.URL.Query()); q == "{}" {
                                        body, _ := stringify(r.Body)
                                        if body == "" {
                                                rw.Header().Set("X-Foo", "bar")
                                                rw.WriteHeader(200)
                                                fmt.Fprint(rw, "{\"color\":\"black\",\"price\":\"1200\"}")
                                                return
                                        }
                                }
                        }
                })
                port := 8081
                server := http.Server{
                        Addr:    "0.0.0.0:" + fmt.Sprint(port),
                        Handler: enableLogRequest(mux, port),
                }
                fmt.Printf("Listening on %v\n", server.Addr)
                go server.ListenAndServe()
        }()
        sig := make(chan os.Signal, 1)
        signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
        <-sig
}

/*
map of external API to mock server listen port
example.com:8080: 8080
sheepex.co.jp:8080: 8081
*/
```

This code can be run as a stub server.

```
$ go run main.go
Listening on 0.0.0.0:8080
Listening on 0.0.0.0:8081
```

At the end of the code, you can find the comment that show correspondence between the external APIs and the port that the stub server listens on.

So you need to rewrite the addresses of external APIs on the SUT (for example the stub server running on 10.10.10.111).

```
EXAMPLE_API_HOST=10.10.10.111:8080
SHEEP_API_HOST=10.10.10.111:8081
```

That's it! You can use the stub server instead of the external APIs.

## HTTPS

If the SUT uses HTTPS, the certificate and key path must be passed when to start gstbgen.

```
$ ./gstbgen --cert ./cert.crt --key ./cert.key
```

On the SUT, you need to set the `https_proxy` environment variable and trust the `cert.crt` that you used when started gstbgen.

```
https_proxy=10.0.0.10:8888
```

On Linux, the certificate trust settings should be different for each distribution, for example, do this

```
cat cert.crt >> /etc/ssl/certs/ca-certificates.crt
```

The generated stub server then uses `server.ListenAndServeTLS`. Prepare appropriate certificates and keys in `cert.pem` and `key.pem` (you can use the ones from earlier)

```
go server.ListenAndServeTLS("cert.pem", "key.pem")
```

# LICENSE

MIT
