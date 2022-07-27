package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type HostKey struct {
	Domain string
	Port   int
}

type ReqValue struct {
	Method    string
	QueryJSON string
}

type OuterAPIMapKey struct {
	HostKey  HostKey
	Path     string
	ReqValue ReqValue
}

type OuterAPIResponse struct {
	Header     http.Header
	StatusCode int
	Body       []byte
}

type OuterAPI map[OuterAPIMapKey][]OuterAPIResponse

func CreateOuterAPIMap(flows map[string]Flow) (OuterAPI, error) {
	outerAPI := OuterAPI{}
	for _, flow := range flows {
		var port int
		hostport := strings.Split(flow.Request.Host, ":")
		if len(hostport) == 1 {
			if flow.Request.URL.Scheme == "https" {
				port = 443
			} else {
				port = 80
			}
		} else {
			port, _ = strconv.Atoi(hostport[1])
		}

		query, err := json.Marshal(flow.Request.URL.Query())
		if err != nil {
			return outerAPI, err
		}

		o := OuterAPIMapKey{
			HostKey: HostKey{
				Domain: flow.Request.Host,
				Port:   port,
			},
			Path: flow.Request.URL.Path,
			ReqValue: ReqValue{
				Method:    flow.Request.Method,
				QueryJSON: string(query),
			},
		}

		delete(flow.Response.Header, "Date")
		delete(flow.Response.Header, "Content-Type")
		delete(flow.Response.Header, "Content-Length")
		delete(flow.Response.Header, "Server")
		delete(flow.Response.Header, "Connection")
		delete(flow.Response.Header, "Keep-Alive")

		outerAPI[o] = append(outerAPI[o], OuterAPIResponse{
			Header: flow.Response.Header,
		})
	}
	return outerAPI, nil
}

func Generate(o OuterAPI) error {
	return nil
}
