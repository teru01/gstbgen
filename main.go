package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/elazarl/goproxy"
	"github.com/mattn/go-tty"
	"github.com/urfave/cli/v2"
)

var quitChan = make(chan struct{})

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
		log.Printf("%s %s %s\n", r.Method, r.URL.String(), r.Proto)
		return r, nil
	})

	proxy.OnResponse().DoFunc(func(r *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		log.Printf("%s %s %s\n", r.Request.Method, r.Request.URL.String(), r.Request.Proto)
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
	}
	<-quit
	return nil
}

func OpenTTY(pressQ chan struct{}) error {
	t, err := tty.Open()
	if err != nil {
		return fmt.Errorf("failed to open tty: %w", err)
	}
	go func(t *tty.TTY) {
		defer t.Close()
		for {
			r, err := t.ReadRune()
			if err != nil {
				log.Println(err)
			}
			if r == 'q' {
				close(pressQ)
				break
			}
			log.Printf("Press q to quit (pressed %v).\n", string(r))
		}
	}(t)
	return nil
}
