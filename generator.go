package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/dave/jennifer/jen"
)

type HostKey struct {
	Domain string
	Port   int
}

type ReqValue struct {
	Method    string
	QueryJSON string
}

type ExternalAPIMapKey struct {
	HostKey  HostKey
	Path     string
	ReqValue ReqValue
}

type ExternalAPIResponse struct {
	Header     http.Header
	StatusCode int
	Body       []byte
}

type ExternalAPI struct {
	key       ExternalAPIMapKey
	responses []ExternalAPIResponse
}

type ExternalAPIMap map[ExternalAPIMapKey][]ExternalAPIResponse

func createExternalAPIMap(flows map[string]Flow) (ExternalAPIMap, error) {
	externalAPI := ExternalAPIMap{}
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
			return externalAPI, err
		}

		o := ExternalAPIMapKey{
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

		externalAPI[o] = append(externalAPI[o], ExternalAPIResponse{
			Header: flow.Response.Header,
		})
	}
	return externalAPI, nil
}

func generate(externalAPI ExternalAPIMap) *jen.Statement {
	sorted := sortExternalAPI(externalAPI)
	return jen.Func().Id("main").Params().Block(
		generateMain(sorted)...,
	)
}

func generateMain(oa []ExternalAPI) []jen.Code {
	var codes []jen.Code
	for i, api := range oa {
		codes = append(codes, jen.Id(fmt.Sprintf("srv%d", i)).Op(":=").Func().Params().Qual("net/http", "Server").Block(
			jen.Id("mux").Op(":=").Qual("net/http", "NewServeMux").Call(),
			jen.Id("mux").Dot("HandleFunc").Call(jen.Lit(api.key.Path), jen.Func().Params(jen.Id("rw").Qual("net/http", "ResponseWriter"), jen.Id("r").Add(jen.Op("*")).Qual("net/http", "Request")).Block(
				jen.Id("rw").Dot("Header").Dot("Set").Call(jen.Lit("Content-Type"), jen.Lit("application/json")),
				jen.Id("rw").Dot("WriteHeader").Call(jen.Qual("net/http", "StatusOK")),
				// TODO
			)),
			jen.Id("server").Op(":=").Qual("net/http", "Server").Values(jen.Dict{
				jen.Lit("Addr"):    jen.Lit(":8080"),
				jen.Lit("Handler"): jen.Id("mux"),
			}),
			jen.Go().Id("server").Dot("ListenAndServe").Call(),
			jen.Return(jen.Id("server")),
		).Call())
	}
	return codes
}

func sortExternalAPI(o ExternalAPIMap) []ExternalAPI {
	l := make([]ExternalAPI, 0, len(o))
	for key, v := range o {
		l = append(l, ExternalAPI{key, v})
	}
	sort.Slice(l, func(i, j int) bool {
		if l[i].key.HostKey.Domain == l[j].key.HostKey.Domain {
			if l[i].key.HostKey.Port == l[j].key.HostKey.Port {
				if l[i].key.Path == l[j].key.Path {
					if l[i].key.ReqValue.Method == l[j].key.ReqValue.Method {
						return l[i].key.ReqValue.QueryJSON < l[j].key.ReqValue.QueryJSON
					}
					return l[i].key.ReqValue.Method < l[j].key.ReqValue.Method
				}
				return l[i].key.Path < l[j].key.Path
			}
			return l[i].key.HostKey.Port < l[j].key.HostKey.Port
		}
		return l[i].key.HostKey.Domain < l[j].key.HostKey.Domain
	})
	return l
}
