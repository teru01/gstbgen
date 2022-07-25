package main

import (
	"net/http"
	"sync"
)

type Flow struct {
	ID       string
	Request  http.Request
	Response *http.Response
}

type Flowsx struct {
	Flows map[string]Flow
	mutex sync.Mutex
}

func (f *Flowsx) Add(flow Flow) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.Flows[flow.ID] = flow
}

func (f *Flowsx) AddResponse(flowID string, response *http.Response) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.Flows[flowID] = Flow{
		ID:       flowID,
		Request:  f.Flows[flowID].Request,
		Response: response,
	}
}
