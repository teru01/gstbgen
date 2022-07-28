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

type outerAPI struct {
	o         OuterAPIMapKey
	responses []OuterAPIResponse
}

func generate(outerAPI OuterAPI) *jen.Statement {
	sorted := sortedOuterAPI(outerAPI)
	return jen.Func().Id("main").Params().Block(
		generateMain(sorted)...,
	)
}

func generateMain(oa []outerAPI) []jen.Code {
	var codes []jen.Code
	for i, api := range oa {
		codes = append(codes, jen.Id(fmt.Sprintf("srv%d", i)).Op(":=").Func().Params().Qual("net/http", "Server").Block(
			jen.Id("mux").Op(":=").Qual("net/http", "NewServeMux").Call(),
			jen.Id("mux").Dot("HandleFunc").Call(jen.Lit(api.o.Path), jen.Func().Params(jen.Id("rw").Qual("net/http", "ResponseWriter"), jen.Id("r").Add(jen.Op("*")).Qual("net/http", "Request")).Block(
				jen.Id("rw").Dot("Header").Dot("Set").Call(jen.Lit("Content-Type"), jen.Lit("application/json")),
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

func sortedOuterAPI(o OuterAPI) []outerAPI {
	l := make([]outerAPI, 0, len(o))
	for key, v := range o {
		l = append(l, outerAPI{key, v})
	}
	sort.Slice(l, func(i, j int) bool {
		if l[i].o.HostKey.Domain == l[j].o.HostKey.Domain {
			if l[i].o.HostKey.Port == l[j].o.HostKey.Port {
				if l[i].o.Path == l[j].o.Path {
					if l[i].o.ReqValue.Method == l[j].o.ReqValue.Method {
						return l[i].o.ReqValue.QueryJSON < l[j].o.ReqValue.QueryJSON
					}
					return l[i].o.ReqValue.Method < l[j].o.ReqValue.Method
				}
				return l[i].o.Path < l[j].o.Path
			}
			return l[i].o.HostKey.Port < l[j].o.HostKey.Port
		}
		return l[i].o.HostKey.Domain < l[j].o.HostKey.Domain
	})
	return l
}
