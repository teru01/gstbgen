package main

import (
	"bytes"
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
	o, err := CreateOuterAPIMap(flows)
	assert.NoError(t, err)
	generate(o)
}
