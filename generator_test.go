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
			Body:   io.NopCloser(bytes.NewReader([]byte(`{"token": "abc"}`))),
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
					body, _ := stringify(r.Body)
					if body == "{\"token\":\"abc\"}" {
						rw.WriteHeader(200)
						fmt.Fprint(rw, "{\"foo\":\"bar\"}")
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
		mux.HandleFunc("/hoge", func(rw http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				if q, _ := stringifyUrlValues(r.URL.Query()); q == "{}" {
					body, _ := stringify(r.Body)
					if body == "" {
						rw.Header().Set("X-Foo", "foo")
						rw.WriteHeader(200)
						fmt.Fprint(rw, "{\"foo\":\"bar\"}")
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

func TestGenerateEnableLogRequest(t *testing.T) {
	generated := jen.Statement(generateEnableLogRequest())
	assert.Equal(t,
		`func enableLogRequest(handler http.Handler, port int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
		log.Printf("%s %s %v %s \n", r.RemoteAddr, r.Method, port, r.URL)
	})
}`, fmt.Sprintf("%#v", &generated))
}
