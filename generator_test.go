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
			Host:   "localhost:8080",
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
	o, err := createExternalAPIMap(flows)
	assert.NoError(t, err)
	stmt := generate(o)
	assert.Equal(t, fmt.Sprintf("%#v", stmt),
		`func main() {
	func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
			rw.Header.Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusOK)
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
			rw.Header.Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusOK)
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
}`)
}
