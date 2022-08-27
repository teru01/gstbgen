package main

import (
	"fmt"
	"net/http"

	"github.com/dave/jennifer/jen"
	"github.com/rs/zerolog/log"
)

type SyntaxNode interface {
	render(childCodes *[]jen.Code, isFirst, isLast bool) []jen.Code
	children() map[string]SyntaxNode
	addChild(child SyntaxNode)
	value() string
}

var (
	root = &Root{
		Value:    "",
		Children: make(map[string]SyntaxNode),
	}
	mockServerPort             = 8080
	externalAPIToMockServerMap = make(map[string]int)
)

type Node struct {
	Value    string
	Children map[string]SyntaxNode
}

type Root Node
type Host Node
type Path Node
type Method Node
type QueryParameter Node
type ReqBody Node
type RespBody struct {
	Header     http.Header
	StatusCode int
	Value      string
	Children   map[string]SyntaxNode
}

func (h *Root) render(childCodes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	var codes []jen.Code
	codes = append(codes, *childCodes...)
	codes = append(codes, generateSignalHandler()...)
	return []jen.Code{
		jen.Func().Id("main").Params().Block(
			codes...,
		),
	}
}

func (h *Root) children() map[string]SyntaxNode {
	return h.Children
}

func (h *Root) addChild(child SyntaxNode) {
	h.Children[child.value()] = child
}

func (h *Root) value() string {
	return h.Value
}

func (h *Host) render(childCodes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	var codes []jen.Code
	listenAddr := fmt.Sprintf("0.0.0.0:%d", mockServerPort)
	codes = append(codes, jen.Id("mux").Op(":=").Qual("net/http", "NewServeMux").Call())
	codes = append(codes, *childCodes...)
	codes = append(codes,
		jen.Id("server").Op(":=").Qual("net/http", "Server").Values(jen.Dict{
			jen.Id("Addr"):    jen.Lit(listenAddr),
			jen.Id("Handler"): jen.Id("mux"),
		}),
		jen.Go().Id("server").Dot("ListenAndServe").Call(),
	)
	externalAPIToMockServerMap[h.value()] = mockServerPort
	mockServerPort++
	return []jen.Code{
		jen.Func().Params().Block(
			codes...,
		).Call(),
	}
}

func (h *Host) children() map[string]SyntaxNode {
	return h.Children
}

func (h *Host) addChild(child SyntaxNode) {
	h.Children[child.value()] = child
}

func (h *Host) value() string {
	return h.Value
}

func (h *ReqBody) render(childCodes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	return []jen.Code{
		jen.If(jen.Id("body").Op("==").Lit(h.value())).Block(*childCodes...),
	}
}

func (h *ReqBody) children() map[string]SyntaxNode {
	return h.Children
}

func (h *ReqBody) addChild(child SyntaxNode) {
	h.Children[child.value()] = child
}

func (h *ReqBody) value() string {
	return h.Value
}

func (r *RespBody) render(childCodes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	var codes []jen.Code
	for k, vv := range r.Header {
		for _, v := range vv {
			codes = append(codes, jen.Id("rw").Dot("Header").Call().Dot("Set").Call(jen.Lit(k), jen.Lit(v)))
		}
	}
	return append(codes, []jen.Code{
		jen.Id("rw").Dot("WriteHeader").Call(jen.Lit(r.StatusCode)),
		jen.Qual("fmt", "Fprint").Call(jen.Id("rw"), jen.Lit(r.Value)),
		jen.Return(),
	}...)
}

func (h *RespBody) children() map[string]SyntaxNode {
	return h.Children
}

func (h *RespBody) addChild(child SyntaxNode) {
	log.Error().Msg("not supported")
}

func (h *RespBody) value() string {
	return createResponseKey(h)
}

func (h *QueryParameter) render(childCodes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	return []jen.Code{
		jen.If(jen.List(jen.Id("q"), jen.Id("_")).Op(":=").Id("stringifyUrlValues").Call(jen.Id("r").Dot("URL").Dot("Query").Call()), jen.Id("q").Op("==").Lit(h.value())).Block(
			func() []jen.Code {
				var codes []jen.Code
				if isFirst {
					codes = append(codes,
						jen.List(jen.Id("body"), jen.Id("_")).Op(":=").Id("stringify").Call(jen.Id("r").Dot("Body")),
					)
				}
				codes = append(codes, *childCodes...)
				return codes
			}()...,
		),
	}
}

func (h *QueryParameter) children() map[string]SyntaxNode {
	return h.Children
}

func (h *QueryParameter) addChild(child SyntaxNode) {
	h.Children[child.value()] = child
}

func (h *QueryParameter) value() string {
	return h.Value
}

func (h *Method) render(childCodes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	return []jen.Code{
		jen.If(jen.Id("r").Dot("Method").Op("==").Lit(h.value())).Block(*childCodes...),
	}
}

func (h *Method) children() map[string]SyntaxNode {
	return h.Children
}

func (h *Method) addChild(child SyntaxNode) {
	h.Children[child.value()] = child
}

func (h *Method) value() string {
	return h.Value
}

func (h *Path) render(childCodes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	return []jen.Code{
		jen.Id("mux").Dot("HandleFunc").Call(jen.Lit(h.value()), jen.Func().Params(jen.Id("rw").Qual("net/http", "ResponseWriter"), jen.Id("r").Add(jen.Op("*")).Qual("net/http", "Request")).Block(
			*childCodes...,
		)),
	}
}

func (h *Path) children() map[string]SyntaxNode {
	return h.Children
}

func (h *Path) addChild(child SyntaxNode) {
	h.Children[child.value()] = child
}

func (h *Path) value() string {
	return h.Value
}

func createResponseKey(r *RespBody) string {
	header, err := stringifyHeader(r.Header)
	if err != nil {
		// 失敗しても最低限のコード生成は可能なので続行する
		log.Error().Err(err)
	}
	return fmt.Sprintf("%d-%s-%s", r.StatusCode, header, r.Value)
}
