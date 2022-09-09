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
	req, err := http.NewRequest(r.Method, e.server.URL+r.URL.Path, r.Body)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

func NewEndServer(flows []FlowTest) EndServer {
	mux := http.NewServeMux()
	for _, flow := range flows {
		flow := flow
		mux.HandleFunc(flow.Request.URL.Path, func(rw http.ResponseWriter, r *http.Request) {
			for k, val := range flow.Response.Header {
				for _, v := range val {
					rw.Header().Set(k, v)
				}
			}
			rw.WriteHeader(flow.Response.StatusCode)
			duplicateReadCloser(flow.Response.Body)
			response, _ := stringify(flow.Response.Body)
			fmt.Fprint(rw, response)
		})
	}
	return EndServer{
		server: httptest.NewServer(mux),
	}
}

type FlowTest struct {
	Flow
	RespBody io.ReadCloser
}

func TestProxy(t *testing.T) {
	testFlows := createFlows()
	es := NewEndServer(testFlows)
	p := NewGenProxy()
	pserver := httptest.NewServer(p.Proxy())
	url, _ := url.Parse(pserver.URL)
	c := http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(url),
		},
	}
	for _, flow := range testFlows {
		resp, err := es.request(&c, &flow.Request)
		assert.NoError(t, err)
		actual, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		expected, err := io.ReadAll(flow.RespBody)
		assert.NoError(t, err)
		assert.Equal(t, string(expected), string(actual))
	}
	capturedFlows := p.Flows()
	assert.Equal(t, len(testFlows), len(capturedFlows))
}

func createFlows() []FlowTest {
	original, dup := duplicateReadCloser(io.NopCloser(bytes.NewReader([]byte(`{"foo":"bar"}`))))
	original2, dup2 := duplicateReadCloser(io.NopCloser(bytes.NewReader([]byte(`{"a":"b","hoge":"foo"}`))))
	flows := []FlowTest{
		{
			Flow: Flow{
				Request: http.Request{
					Method: "GET",
					URL:    &url.URL{Path: "/"},
				},
				Response: http.Response{
					StatusCode: 200,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: original,
				},
			},
			RespBody: dup,
		},
		{
			Flow: Flow{
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
					Body: original2,
				},
			},
			RespBody: dup2,
		},
	}
	return flows
}
