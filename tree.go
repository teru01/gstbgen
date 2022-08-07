package main

import (
	"fmt"

	"github.com/dave/jennifer/jen"
)

type SyntaxNode interface {
	render(childCodes *[]jen.Code, isFirst, isLast bool) []jen.Code
	children() []SyntaxNode
	value() string
}

var (
	root            = new(Root)
	hosts           = make(map[string]Host)
	paths           = make(map[string]Path)
	methods         = make(map[string]Method)
	queryParameters = make(map[string]QueryParameter)
	reqBodies       = make(map[string]ReqBody)
	respBodies      = make(map[string]RespBody)
	mockServerPort  = 8080
)

type Node struct {
	Value    string
	Children []SyntaxNode
}

type Root Node
type Host Node
type Path Node
type Method Node
type QueryParameter Node
type ReqBody Node

func mergeChild(parent SyntaxNode, child SyntaxNode) []SyntaxNode {
	children := parent.children()
	for _, c := range children {
		if c.value() == child.value() {
			return children
		}
	}
	return append(children, child)
}

type RespBody struct {
	Value    string
	Children []SyntaxNode
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

func (h *Root) children() []SyntaxNode {
	return h.Children
}

func (h *Root) value() string {
	return h.Value
}

func (h *Host) render(childCodes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	var codes []jen.Code
	codes = append(codes, jen.Id("mux").Op(":=").Qual("net/http", "NewServeMux").Call())
	codes = append(codes, *childCodes...)
	codes = append(codes,
		jen.Id("server").Op(":=").Qual("net/http", "Server").Values(jen.Dict{
			jen.Lit("Addr"):    jen.Lit(fmt.Sprintf("0.0.0.0:%d", mockServerPort)),
			jen.Lit("Handler"): jen.Id("mux"),
		}),
		jen.Go().Id("server").Dot("ListenAndServe").Call(),
	)
	// return codes
	mockServerPort++
	return []jen.Code{
		jen.Func().Params().Block(
			codes...,
		).Call(),
	}
}

func (h *Host) children() []SyntaxNode {
	return h.Children
}

func (h *Host) value() string {
	return h.Value
}

func (h *ReqBody) render(childCodes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	return []jen.Code{
		jen.If(jen.Id("body").Op("==").Lit(h.value())).Block(*childCodes...),
	}
}

func (h *ReqBody) children() []SyntaxNode {
	return h.Children
}

func (h *ReqBody) value() string {
	return h.Value
}

func (h *RespBody) render(childCodes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	return []jen.Code{
		// jen.Qual("fmt", "Fprintf").Call(jen.Id("rw"), jen.Lit("%s"), jen.Id("b")),
		// TODO
		jen.Id("rw").Dot("WriteHeader").Call(jen.Qual("net/http", "StatusOK")),
		jen.Return(),
	}
}

func (h *RespBody) children() []SyntaxNode {
	return h.Children
}

func (h *RespBody) value() string {
	return h.Value
}

func (h *QueryParameter) render(childCodes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	return []jen.Code{
		jen.If(jen.List(jen.Id("q"), jen.Id("_")).Op(":=").Id("stringifyUrlValues").Call(jen.Id("r").Dot("URL").Dot("Query").Call()), jen.Id("q").Op("==").Lit(h.value())).Block(
			func() []jen.Code {
				var codes []jen.Code
				if isFirst {
					codes = append(codes,
						jen.List(jen.Id("body"), jen.Err()).Op(":=").Id("stringify").Call(jen.Id("r").Dot("Body")),
						jen.If(jen.Err().Op("!=").Nil()).Block(
							jen.Id("rw").Dot("WriteHeader").Call(jen.Qual("net/http", "StatusBadRequest")),
							jen.Return(),
						),
					)
				}
				codes = append(codes, *childCodes...)
				return codes
			}()...,
		),
	}
}

func (h *QueryParameter) children() []SyntaxNode {
	return h.Children
}

func (h *QueryParameter) value() string {
	return h.Value
}

func (h *Method) render(childCodes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	return []jen.Code{
		jen.If(jen.Id("r").Dot("Method").Op("==").Lit(h.value())).Block(*childCodes...),
	}
}

func (h *Method) children() []SyntaxNode {
	return h.Children
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

func (h *Path) children() []SyntaxNode {
	return h.Children
}

func (h *Path) value() string {
	return h.Value
}
