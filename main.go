package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/elazarl/goproxy"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Value:   false,
				Usage:   "enable debug log",
			},
		},
		Name:   "gprogen",
		Usage:  "Go proxy and stub generator for load test",
		Action: start,
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Error().Err(err).Msgf("%+v", err)
	}
}

func start(c *cli.Context) error {
	initLog(c)
	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().DoFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		flowID := uuid.New().String()
		var reqBody io.ReadCloser
		r.Body, reqBody = duplicateReadCloser(r.Body)
		request := http.Request{
			URL:    r.URL,
			Host:   r.Host,
			Method: r.Method,
			Body:   reqBody,
		}
		flows.add(Flow{
			ID:      flowID,
			Request: request,
		})
		ctx.UserData = flowID
		log.Info().Msgf("%s %s", r.Method, r.URL.String())
		return r, nil
	})

	proxy.OnResponse().DoFunc(func(r *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		flowID := ctx.UserData.(string)
		var respBody io.ReadCloser
		r.Body, respBody = duplicateReadCloser(r.Body)
		response := http.Response{
			StatusCode: r.StatusCode,
			Header:     r.Header,
			Body:       respBody,
		}
		flows.addResponse(flowID, response)
		return r
	})

	svc := http.Server{
		Addr:    c.String("host") + ":" + c.String("port"),
		Handler: proxy,
	}
	quit := make(chan struct{})
	go func(svc *http.Server, quit chan struct{}) {
		log.Info().Msgf("listening on %v", svc.Addr)
		if err := svc.ListenAndServe(); err != http.ErrServerClosed {
			log.Error().Err(err)
		}
		quit <- struct{}{}
	}(&svc, quit)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-shutdown
	if err := svc.Shutdown(context.Background()); err != nil {
		log.Error().Err(err)
	}
	root, err := createExternalAPITree(flows.Flows)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	stmt := generate(root)
	if stmt != nil {
		fmt.Printf("%#v", stmt)
	}
	<-quit
	return nil
}

func initLog(c *cli.Context) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	if c != nil && c.Bool("debug") {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
}
