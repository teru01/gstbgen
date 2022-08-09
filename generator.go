package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
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
			log.Println(err)
		}

		reqBodyString, err := stringify(flow.Request.Body)
		if err != nil {
			// 失敗しても最低限のコード生成は可能なので続行する
			log.Println(err)
		}

		respBodyString, err := stringify(flow.Response.Body)
		if err != nil {
			// 失敗しても最低限のコード生成は可能なので続行する
			log.Println(err)
		}

		delete(flow.Response.Header, "Date")
		delete(flow.Response.Header, "Content-Type")
		delete(flow.Response.Header, "Content-Length")
		delete(flow.Response.Header, "Server")
		delete(flow.Response.Header, "Connection")
		delete(flow.Response.Header, "Keep-Alive")

		var host, path, method, qs, req, res SyntaxNode
		var found bool
		hostString := fmt.Sprintf("%s:%d", flow.Request.Host, port)
		if host, found = root.children()[hostString]; !found {
			host = &Host{
				Value:    hostString,
				Children: make(map[string]SyntaxNode),
			}
			root.addChild(host)
		}
		if path, found = host.children()[flow.Request.URL.Path]; !found {
			path = &Path{
				Value:    flow.Request.URL.Path,
				Children: make(map[string]SyntaxNode),
			}
			host.addChild(path)
		}
		if method, found = path.children()[flow.Request.Method]; !found {
			method = &Method{
				Value:    flow.Request.Method,
				Children: make(map[string]SyntaxNode),
			}
			path.addChild(method)
		}
		if qs, found = method.children()[queryString]; !found {
			qs = &QueryParameter{
				Value:    queryString,
				Children: make(map[string]SyntaxNode),
			}
			method.addChild(qs)
		}
		if req, found = qs.children()[reqBodyString]; !found {
			req = &ReqBody{
				Value:    reqBodyString,
				Children: make(map[string]SyntaxNode),
			}
			qs.addChild(req)
		}
		if res, found = req.children()[respBodyString]; !found {
			res = &RespBody{
				Value:      respBodyString,
				Children:   make(map[string]SyntaxNode),
				StatusCode: flow.Response.StatusCode,
				Header:     flow.Response.Header,
			}
			req.addChild(res)
		}
	}
	// hostsList := make([]SyntaxNode, 0, len(hosts))
	// for _, host := range hosts {
	// 	h := host
	// 	hostsList = append(hostsList, &h)
	// }
	// root.Children = hostsList
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
		fmt.Println("stringify", string(body))
		return string(body), err
	}
	if j, err := json.Marshal(bm); err != nil {
		fmt.Println("stringify", string(body))
		return string(body), err
	} else {
		fmt.Println("stringify", string(j))
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
	var codes []jen.Code
	codes = append(codes, generateStringify()...)
	codes = append(codes, jen.Line())
	codes = append(codes, generateStringifyUrlValues()...)
	codes = append(codes, jen.Line())
	codes = append(codes, generateServerFuncs(root, true, true)...)
	s := jen.Statement(codes)
	return &s
}

func generateServerFuncs(node SyntaxNode, isFirst, isLast bool) []jen.Code {
	fmt.Println("in gen", node.value())
	var codes []jen.Code
	children := node.children()
	if len(children) == 0 {
		return node.render(nil, isFirst, isLast)
	}
	childrenList := make([]SyntaxNode, 0, len(children))
	for _, child := range children {
		childrenList = append(childrenList, child)
	}
	sort.Slice(childrenList, func(i, j int) bool {
		return childrenList[i].value() < childrenList[j].value()
	})
	if _, ok := node.(*ReqBody); ok {
		fmt.Println("value is reqbody", node.value())
		fmt.Println("value is reqbody, childer len: ", len(children))
	}
	for i, child := range childrenList {
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
