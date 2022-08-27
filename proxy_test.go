package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

type EndServer struct {
	m      map[RequestContent]ResponseContent
	server *httptest.Server
}

type RequestContent struct {
	Path        string
	Method      string
	QueryString string
	ReqBody     string
}

type ResponseContent struct {
	StatusCode int
	Header     http.Header
	Body       string
}

func (e *EndServer) request(c *http.Client, r *http.Request) (*http.Response, error) {
	url := e.server.URL + r.URL.Path
	fmt.Println(url)
	req, err := http.NewRequest(r.Method, url, r.Body)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

func NewEndServer(flows []Flow) EndServer {
	mux := http.NewServeMux()
	for _, flow := range flows {
		mux.HandleFunc(flow.Request.URL.Path, func(rw http.ResponseWriter, r *http.Request) {
			for k, val := range flow.Response.Header {
				for _, v := range val {
					rw.Header().Set(k, v)
				}
			}
			rw.WriteHeader(flow.Response.StatusCode)
			response, _ := stringify(flow.Response.Body)
			fmt.Fprint(rw, response)
		})
	}
	return EndServer{
		server: httptest.NewServer(mux),
	}
}

func TestProxy(t *testing.T) {
	flows := []Flow{
		{
			Request: http.Request{
				Method: "GET",
				URL:    &url.URL{Path: "/"},
			},
			Response: http.Response{
				StatusCode: 200,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(bytes.NewReader([]byte(`{"foo": "bar"}`))),
			},
		},
		{
			Request: http.Request{
				Method: "POST",
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
		},
	}

	es := NewEndServer(flows)
	pserver := httptest.NewServer(NewProxy())
	url, _ := url.Parse(pserver.URL)
	c := http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(url),
		},
	}
	for _, flow := range flows {
		resp, err := es.request(&c, &flow.Request)
		assert.NoError(t, err)
		r, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "hello", string(r))
	}
}
