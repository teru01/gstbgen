package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/dave/jennifer/jen"
)

func createExternalAPITree(flows map[string]Flow) (SyntaxNode, error) {
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

		queryString, err := stringifyUrlValues(flow.Request.URL.Query())
		if err != nil {
			// 失敗しても最低限のコード生成は可能なので続行する
			queryString = ""
		}

		reqBodyString, err := stringify(flow.Request.Body)
		if err != nil {
			// 失敗しても最低限のコード生成は可能なので続行する
			reqBodyString = ""
		}

		respBodyString, err := stringify(flow.Response.Body)
		if err != nil {
			// 失敗しても最低限のコード生成は可能なので続行する
			respBodyString = ""
		}

		delete(flow.Response.Header, "Date")
		delete(flow.Response.Header, "Content-Type")
		delete(flow.Response.Header, "Content-Length")
		delete(flow.Response.Header, "Server")
		delete(flow.Response.Header, "Connection")
		delete(flow.Response.Header, "Keep-Alive")

		respBody := RespBody{
			Value: respBodyString,
		}
		respBodies[respBodyString] = respBody

		reqBody := ReqBody{
			Value: reqBodyString,
			Children: func() []SyntaxNode {
				if r, found := reqBodies[reqBodyString]; found {
					return mergeChild(&r, &respBody)
				}
				return []SyntaxNode{&respBody}
			}(),
		}
		reqBodies[reqBodyString] = reqBody

		queryParameter := QueryParameter{
			Value: queryString,
			Children: func() []SyntaxNode {
				if q, found := queryParameters[queryString]; found {
					return mergeChild(&q, &reqBody)
				}
				return []SyntaxNode{&reqBody}
			}(),
		}
		queryParameters[queryString] = queryParameter

		method := Method{
			Value: flow.Request.Method,
			Children: func() []SyntaxNode {
				if m, found := methods[flow.Request.Method]; found {
					return mergeChild(&m, &queryParameter)
				}
				return []SyntaxNode{&queryParameter}
			}(),
		}
		methods[method.Value] = method

		path := Path{
			Value: flow.Request.URL.Path,
			Children: func() []SyntaxNode {
				if p, found := paths[flow.Request.URL.Path]; found {
					return mergeChild(&p, &method)
				}
				return []SyntaxNode{&method}
			}(),
		}
		paths[path.Value] = path

		hostString := fmt.Sprintf("%s:%d", flow.Request.Host, port)
		host := Host{
			Value: hostString,
			Children: func() []SyntaxNode {
				if h, found := hosts[hostString]; found {
					return mergeChild(&h, &path)
				}
				return []SyntaxNode{&path}
			}(),
		}
		hosts[hostString] = host
	}
	hostsList := make([]SyntaxNode, 0, len(hosts))
	for _, host := range hosts {
		hostsList = append(hostsList, &host)
	}
	root.Children = hostsList
	return root, nil
}

// JSONに変換できるものはJSON文字列にする
// できないものはそのまま文字列にして返す
func stringify(r io.ReadCloser) (string, error) {
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

func stringifyUrlValues(m url.Values) (string, error) {
	query, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(query), nil
}

func generate(root SyntaxNode) *jen.Statement {
	return jen.Func().Id("main").Params().Block(
		append(generateServerFuncs(root, true, true), generateSignalHandler()...)...,
	)
}

func generateServerFuncs(node SyntaxNode, isFirst, isLast bool) []jen.Code {
	var codes []jen.Code
	children := node.children()
	if len(children) == 0 {
		return node.render(nil, isFirst, isLast)
	}
	for i, child := range children {
		var isFirst, isLast bool
		if i == 0 {
			isFirst = true
		}
		if i == len(children)-1 {
			isLast = true
		}
		codes = append(codes, generateServerFuncs(child, isFirst, isLast)...)
	}
	return node.render(&codes, isFirst, isLast)
}

// func generateServer(apis []ExternalAPI) []jen.Code {
// 	var codes, codesInSameMux, codesInSameHandler []jen.Code
// 	var prevApi ExternalAPI
// 	for i, api := range apis {
// 		if prevApi.key.HostKey.Domain == api.key.HostKey.Domain && prevApi.key.HostKey.Port == api.key.HostKey.Port {
// 			if prevApi.key.Path == api.key.Path {
// 				if prevApi.key.ReqValue.Method == api.key.ReqValue.Method {
// 					if prevApi.key.ReqValue.QueryJSON == api.key.ReqValue.QueryJSON {
// 					}
// 				}
// 				// codesInSameHandler = append(codesInSameHandler, api.render(&codesInSameHandler)...)
// 			}
// 			// handlerfunc生成
// 			hf := jen.Id("mux").Dot("HandleFunc").Call(jen.Lit(api.key.Path), jen.Func().Params(jen.Id("rw").Qual("net/http", "ResponseWriter"), jen.Id("r").Add(jen.Op("*")).Qual("net/http", "Request")).Block(
// 				codesInSameHandler...,
// 			))
// 			codesInSameMux = append(codesInSameMux, hf)
// 		} else {
// 			codes = append(codes, jen.Func().Params().Block(
// 				jen.Id("mux").Op(":=").Qual("net/http", "NewServeMux").Call(),
// 				jen.Id("server").Op(":=").Qual("net/http", "Server").Values(jen.Dict{
// 					jen.Lit("Addr"):    jen.Lit(fmt.Sprintf("0.0.0.0:%d", nextPort(i))),
// 					jen.Lit("Handler"): jen.Id("mux"),
// 				}),
// 				jen.Go().Id("server").Dot("ListenAndServe").Call(),
// 			).Call())
// 		}
// 		prevApi = api
// 	}
// 	return codes
// }

func generateSignalHandler() []jen.Code {
	return []jen.Code{
		jen.Id("sig").Op(":=").Make(jen.Chan().Qual("os", "Signal")),
		jen.Qual("os/signal", "Notify").Call(jen.Id("sig"), jen.Qual("syscall", "SIGINT"), jen.Qual("syscall", "SIGTERM"), jen.Qual("syscall", "SIGQUIT")),
		jen.Id("<-sig"),
	}
}

func nextPort(i int) int {
	// TODO: フラグでポート範囲選択
	return 8080 + i
}
