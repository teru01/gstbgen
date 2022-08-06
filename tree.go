package main

import "github.com/dave/jennifer/jen"

type SyntaxNode interface {
	render(codes *[]jen.Code, isFirst, isLast bool) []jen.Code
	children() []SyntaxNode
	value() string
}

var (
	root            *Root
	hosts           = make(map[string]Host)
	paths           = make(map[string]Path)
	methods         = make(map[string]Method)
	queryParameters = make(map[string]QueryParameter)
	reqBodies       = make(map[string]ReqBody)
	respBodies      = make(map[string]RespBody)
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

func (h *Root) render(codes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	return []jen.Code{}
}

func (h *Root) children() []SyntaxNode {
	return h.Children
}

func (h *Root) value() string {
	return h.Value
}

func (h *Host) render(codes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	return []jen.Code{}
}

func (h *Host) children() []SyntaxNode {
	return h.Children
}

func (h *Host) value() string {
	return h.Value
}

func (h *ReqBody) render(codes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	// jen.If(jen.Id("rw").Dot("Header").Call(jen.Lit("Content-Type")).Op("==").Lit("application/json")).Block(
	return []jen.Code{}
}

func (h *ReqBody) children() []SyntaxNode {
	return h.Children
}

func (h *ReqBody) value() string {
	return h.Value
}

func (h *RespBody) render(codes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	return []jen.Code{
		jen.Id("rw").Dot("WriteHeader").Call(jen.Qual("net/http", "StatusOK")),
	}
}

func (h *RespBody) children() []SyntaxNode {
	return h.Children
}

func (h *RespBody) value() string {
	return h.Value
}

func (h *QueryParameter) render(codes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	return []jen.Code{}
}

func (h *QueryParameter) children() []SyntaxNode {
	return h.Children
}

func (h *QueryParameter) value() string {
	return h.Value
}

func (h *Method) render(codes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	return []jen.Code{}
}

func (h *Method) children() []SyntaxNode {
	return h.Children
}

func (h *Method) value() string {
	return h.Value
}

func (h *Path) render(codes *[]jen.Code, isFirst, isLast bool) []jen.Code {
	return []jen.Code{}
}

func (h *Path) children() []SyntaxNode {
	return h.Children
}

func (h *Path) value() string {
	return h.Value
}
