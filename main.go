package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/elazarl/goproxy"
	"github.com/google/uuid"
	"github.com/urfave/cli/v2"
)

var flows = &Flowsx{
	Flows: make(map[string]Flow),
	mutex: sync.Mutex{},
}

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "host",
				Aliases: []string{"H"},
				Value:   "0.0.0.0",
				Usage:   "listening host",
			},
			&cli.Int64Flag{
				Name:    "port",
				Aliases: []string{"p"},
				Value:   8080,
				Usage:   "listening port",
			},
		},
		Name:   "gprogen",
		Usage:  "Go proxy and stub generator for load test",
		Action: start,
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatalf("%+v\n", err)
	}
}

func start(c *cli.Context) error {
	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().DoFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		var b bytes.Buffer
		flowID := uuid.New().String()

		r.Body = io.NopCloser(io.TeeReader(r.Body, &b))
		request := http.Request{
			URL:    r.URL,
			Host:   r.Host,
			Method: r.Method,
			Body:   io.NopCloser(&b),
		}

		flows.add(Flow{
			ID:      flowID,
			Request: request,
		})
		ctx.UserData = flowID
		log.Printf("%s %s\n", r.Method, r.URL.String())
		return r, nil
	})

	proxy.OnResponse().DoFunc(func(r *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		var b bytes.Buffer
		var response http.Response
		flowID := ctx.UserData.(string)

		r.Body = io.NopCloser(io.TeeReader(r.Body, &b))
		response.StatusCode = r.StatusCode
		response.Header = r.Header
		response.Body = io.NopCloser(&b)
		flows.addResponse(flowID, response)
		return r
	})

	svc := http.Server{
		Addr:    c.String("host") + ":" + c.String("port"),
		Handler: proxy,
	}
	quit := make(chan struct{})
	go func(svc *http.Server, quit chan struct{}) {
		log.Println("listening on", svc.Addr)
		if err := svc.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
		quit <- struct{}{}
	}(&svc, quit)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-shutdown
	if err := svc.Shutdown(context.Background()); err != nil {
		log.Println(err)
	}
	root, err := createExternalAPITree(flows.Flows)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	stmt := generate(root)
	fmt.Printf("%#v", stmt)
	<-quit
	return nil
}
