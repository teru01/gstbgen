package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

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
		Action: Start,
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatalf("%+v\n", err)
	}
}

func Start(c *cli.Context) error {
	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().DoFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		flowID := uuid.New().String()
		flows.Add(Flow{
			ID:      flowID,
			Request: *r,
		})
		ctx.UserData = flowID
		_, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
		}
		log.Printf("%s %s\n", r.Method, r.URL.String())
		return r, nil
	})

	proxy.OnResponse().DoFunc(func(r *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		flowID := ctx.UserData.(string)
		flows.AddResponse(flowID, r)
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

	pressQ := make(chan struct{})
	if err := OpenTTY(pressQ); err != nil {
		return fmt.Errorf("OpenTTY: %w", err)
	}
	<-pressQ
	if err := svc.Shutdown(context.Background()); err != nil {
		log.Println(err)
	}
	for _, flow := range flows.Flows {
		if flow.Response != nil {
			log.Printf("%s -> %s\n", flow.Request.URL.String(), flow.Response.Status)
		} else {
			log.Printf("%s -> ERR\n", flow.Request.URL.String())
		}
	}
	<-quit
	return nil
}
