package main

import (
	"bytes"
	"io"
	"net/http"
	"sync"
)

type Flow struct {
	ID       string
	Request  http.Request
	Response http.Response
}

type Flowsx struct {
	Flows map[string]Flow
	mutex sync.Mutex
}

func (f *Flowsx) add(flow Flow) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.Flows[flow.ID] = flow
}

func duplicateReadCloser(rc io.ReadCloser) (original io.ReadCloser, duplicated io.ReadCloser) {
	var b bytes.Buffer
	original = teeReadCloser(rc, &b)
	return original, io.NopCloser(&b)
}

func teeReadCloser(rc io.ReadCloser, w io.Writer) Body {
	n := io.TeeReader(rc, w)
	return Body{
		c: rc,
		b: n,
	}
}

type Body struct {
	c io.Closer
	b io.Reader
}

func (body Body) Close() error {
	return body.c.Close()
}

func (body Body) Read(p []byte) (n int, err error) {
	return body.b.Read(p)
}
