package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
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
		h := host
		hostsList = append(hostsList, &h)
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
	s := jen.Statement(generateServerFuncs(root, true, true))
	return &s
}

func generateServerFuncs(node SyntaxNode, isFirst, isLast bool) []jen.Code {
	var codes []jen.Code
	children := node.children()
	if len(children) == 0 {
		return node.render(nil, isFirst, isLast)
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].value() < children[j].value()
	})
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

func generateSignalHandler() []jen.Code {
	return []jen.Code{
		jen.Id("sig").Op(":=").Make(jen.Chan().Qual("os", "Signal")),
		jen.Qual("os/signal", "Notify").Call(jen.Id("sig"), jen.Qual("syscall", "SIGINT"), jen.Qual("syscall", "SIGTERM"), jen.Qual("syscall", "SIGQUIT")),
		jen.Id("<-sig"),
	}
}

func generateStringify() []jen.Code {
	return []jen.Code{
		jen.Func().Id("stringify").Params(jen.Id("r").Qual("io", "ReadCloser")).Parens(jen.List(jen.String(), jen.Error())).Block(
			jen.If(jen.Id("r").Op("==").Nil()).Block(
				jen.Return(jen.Lit(""), jen.Nil()),
			),
			jen.List(jen.Id("body"), jen.Err()).Op(":=").Qual("io", "ReadAll").Call(jen.Id("r")),
			jen.If(jen.Err().Op("!=").Nil()).Block(
				jen.Return(jen.Lit(""), jen.Err()),
			),
			jen.Defer().Id("r").Dot("Close").Call(),
			jen.Id("bm").Op(":=").Make(jen.Map(jen.String()).Interface()),
			jen.If(jen.Err().Op(":=").Qual("encoding/json", "Unmarshal").Call(jen.Id("body"), jen.Op("&").Id("bm")), jen.Err().Op("!=").Nil()).Block(
				jen.Return(jen.Id("string").Call(jen.Id("body")), jen.Err()),
			),
			jen.If(jen.List(jen.Id("j"), jen.Err()).Op(":=").Qual("encoding/json", "Marshal").Call(jen.Id("bm")), jen.Err().Op("!=").Nil()).Block(
				jen.Return(jen.Id("string").Call(jen.Id("body")), jen.Err()),
			).Else().Block(
				jen.Return(jen.Id("string").Call(jen.Id("j")), jen.Nil()),
			),
		),
	}
}

func generateStringifyUrlValues() []jen.Code {
	return []jen.Code{
		jen.Func().Id("stringifyUrlValues").Params(jen.Id("m").Qual("net/url", "Values")).Parens(jen.List(jen.String(), jen.Error())).Block(
			jen.List(jen.Id("query"), jen.Err()).Op(":=").Qual("encoding/json", "Marshal").Call(jen.Id("m")),
			jen.If(jen.Err().Op("!=").Nil()).Block(
				jen.Return(jen.Lit(""), jen.Err()),
			),
			jen.Return(jen.Id("string").Call(jen.Id("query")), jen.Nil()),
		),
	}
}
