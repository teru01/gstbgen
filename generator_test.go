package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateMain(t *testing.T) {
	flows := map[string]Flow{}
	flows["1"] = Flow{
		Request: http.Request{
			Method: "GET",
			Host:   "localhost:8080",
			URL:    &url.URL{Path: "/"},
		},
		Response: &http.Response{
			StatusCode: 200,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(bytes.NewReader([]byte(`{"foo": "bar"}`))),
		},
	}
	flows["2"] = Flow{
		Request: http.Request{
			Method: "GET",
			Host:   "localhost:8081",
			URL:    &url.URL{Path: "/hoge"},
		},
		Response: &http.Response{
			StatusCode: 200,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"X-Foo":        []string{"foo"},
			},
			Body: io.NopCloser(bytes.NewReader([]byte(`{"foo": "bar"}`))),
		},
	}
	o, err := createExternalAPITree(flows)
	assert.NoError(t, err)
	stmt := generate(o)
	assert.Equal(t,
		`func main() {
	func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				if q, _ := stringifyUrlValues(r.URL.Query()); q == "{}" {
					body, err := stringify(r.Body)
					if err != nil {
						rw.WriteHeader(http.StatusBadRequest)
						return
					}
					if body == "" {
						rw.WriteHeader(http.StatusOK)
						return
					}
				}
			}
		})
		server := http.Server{
			"Addr":    "0.0.0.0:8080",
			"Handler": mux,
		}
		go server.ListenAndServe()
	}()
	func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/hoge", func(rw http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				if q, _ := stringifyUrlValues(r.URL.Query()); q == "{}" {
					body, err := stringify(r.Body)
					if err != nil {
						rw.WriteHeader(http.StatusBadRequest)
						return
					}
					if body == "" {
						rw.WriteHeader(http.StatusOK)
						return
					}
				}
			}
		})
		server := http.Server{
			"Addr":    "0.0.0.0:8081",
			"Handler": mux,
		}
		go server.ListenAndServe()
	}()
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-sig
}`, fmt.Sprintf("%#v", stmt))
}

// func TestGenerateServerFuncs(t *testing.T) {
// 	root := new(Root)
// 	host := Host{
// 		Value: "localhost:8080",
// 		Children: func() []SyntaxNode {
// 			if h, found := hosts["localhost:8080"]; found {
// 				return mergeChild(&h, nil)
// 			}
// 			return []SyntaxNode{&path}
// 		}(),
// 	}
// 	hostsList := make([]SyntaxNode, 0, len(hosts))
// 	for _, host := range hosts {
// 		hostsList = append(hostsList, &host)
// 	}
// 	root.Children = hostsList
// 	assert.Equal(t, generateServerFuncs(root, true, true), []jen.Code{})
// }
