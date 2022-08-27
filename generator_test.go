package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/stretchr/testify/assert"
)

func TestGenerateMain(t *testing.T) {
	initLog(nil)
	flows := map[string]Flow{}
	flows["1"] = Flow{
		Request: http.Request{
			Method: "GET",
			Host:   "localhost:8080",
			URL:    &url.URL{Path: "/"},
		},
		Response: http.Response{
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
		Response: http.Response{
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
	stmt := jen.Statement(generateServerFuncs(o, true, true))
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
						rw.WriteHeader(200)
						fmt.Fprint(rw, "{\"foo\":\"bar\"}")
						return
					}
				}
			}
		})
		server := http.Server{
			Addr:    "0.0.0.0:8080",
			Handler: mux,
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
						rw.Header().Set("X-Foo", "foo")
						rw.WriteHeader(200)
						fmt.Fprint(rw, "{\"foo\":\"bar\"}")
						return
					}
				}
			}
		})
		server := http.Server{
			Addr:    "0.0.0.0:8081",
			Handler: mux,
		}
		go server.ListenAndServe()
	}()
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-sig
}`, fmt.Sprintf("%#v", &stmt))
}

func TestStringify(t *testing.T) {
	generated := jen.Statement(generateStringify())
	assert.Equal(t,
		`func stringify(r io.ReadCloser) (string, error) {
	if r == nil {
		return "", nil
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	defer r.Close()
	bm := make(map[string]interface{})
	if err := json.Unmarshal(body, &bm); err != nil {
		return string(body), err
	}
	if j, err := json.Marshal(bm); err != nil {
		return string(body), err
	} else {
		return string(j), nil
	}
} 
`, fmt.Sprintf("%#v", &generated))
}

func TestGenerateStringifyUrlValues(t *testing.T) {
	generated := jen.Statement(generateStringifyUrlValues())
	assert.Equal(t,
		`func stringifyUrlValues(m url.Values) (string, error) {
	query, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(query), nil
} 
`, fmt.Sprintf("%#v", &generated))
}
