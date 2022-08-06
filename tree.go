package main

import "github.com/dave/jennifer/jen"

type SyntaxNode interface {
	render(codes *[]jen.Code) []jen.Code
	children() []SyntaxNode
	value() string
}

var (
	root         *Root
	hostKeys     = make(map[string]HostKey)
	paths        = make(map[string]Path)
	methods      = make(map[string]Method)
	queryStrings = make(map[string]QueryString)
	reqBodies    = make(map[string]ReqBody)
	respBodies   = make(map[string]RespBody)
)

type Node struct {
	Value    string
	Children []SyntaxNode
}

type Root Node
type HostKey Node
type Path Node
type Method Node
type QueryString Node
type ReqBody Node

func addChild(parent SyntaxNode, child SyntaxNode) []SyntaxNode {
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

func (h *Root) render(codes *[]jen.Code) []jen.Code {
	return []jen.Code{}
}

func (h *Root) children() []SyntaxNode {
	return h.Children
}

func (h *Root) value() string {
	return h.Value
}

func (h *HostKey) render(codes *[]jen.Code) []jen.Code {
	return []jen.Code{}
}

func (h *HostKey) children() []SyntaxNode {
	return h.Children
}

func (h *HostKey) value() string {
	return h.Value
}
