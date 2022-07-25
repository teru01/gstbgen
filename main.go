package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/elazarl/goproxy"
	"github.com/google/uuid"
	"github.com/urfave/cli/v2"
)

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

var flow = make(map[string]string)

func Start(c *cli.Context) error {
	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().DoFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		flowID := uuid.NewString()
		flow[flowID] = r.URL.String()
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
		fmt.Println(flowID)
		fmt.Println(flow[flowID])
		// flow[flowID] = flow[flowID] + " -> " + r.Request.URL.String()
		log.Printf("%s %s\n", r.Request.Method, r.Request.URL.String())
		return r
	})

	svc := http.Server{
		Addr:    c.String("host") + ":" + c.String("port"),
		Handler: proxy,
	}
	quit := make(chan struct{})
	go func(svc http.Server, quit chan struct{}) {
		log.Println("listening on", svc.Addr)
		if err := svc.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
		close(quit)
	}(svc, quit)

	pressQ := make(chan struct{})
	if err := OpenTTY(pressQ); err != nil {
		return fmt.Errorf("OpenTTY: %w", err)
	}
	<-pressQ
	if err := svc.Shutdown(context.Background()); err != nil {
		log.Println(err)
		for id, v := range flow {
			log.Println(id, v)
		}
	}
	<-quit
	return nil
}
