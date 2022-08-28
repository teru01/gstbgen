package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/dave/jennifer/jen"
	"github.com/elazarl/goproxy"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

type GenProxy struct {
	proxy *goproxy.ProxyHttpServer
	flows *Flowsx
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
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Value:   8888,
				Usage:   "listening port",
			},
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Value:   false,
				Usage:   "enable debug log",
			},
			&cli.IntFlag{
				Name:    "mockBeginPort",
				Aliases: []string{"m"},
				Value:   8080,
				Usage:   "begin port of generated mock server",
			},
			&cli.StringFlag{
				Name:  "cert",
				Usage: "certificate path",
			},
			&cli.StringFlag{
				Name:  "key",
				Usage: "certificate key path",
			},
			&cli.StringFlag{
				Name:    "out",
				Aliases: []string{"o"},
				Usage:   "generated stub server code path(default: stdout)",
			},
		},
		Name:   "gstbgen",
		Usage:  "Stub generator for system analysis written in Go.",
		Action: start,
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Error().Err(err).Msgf("%+v", err)
	}
}

func start(c *cli.Context) error {
	initLog(c)
	mockServerPort = c.Int("mockBeginPort")
	proxy := NewGenProxy()
	if c.String("cert") != "" && c.String("key") != "" {
		enableHttpsProxy(c, proxy.Proxy())
	}
	svc := http.Server{
		Addr:    c.String("host") + ":" + c.String("port"),
		Handler: proxy.Proxy(),
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
		return fmt.Errorf("failed to shutdown: %w", err)
	}
	root, err := createExternalAPITree(proxy.Flows())
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	stmt := generate(root)
	if stmt != nil {
		f := jen.NewFile("main")
		f.Add(stmt)
		var buf bytes.Buffer
		if err := f.Render(&buf); err != nil {
			return fmt.Errorf("faield to render: %w", err)
		}
		if c.String("out") == "" {
			fmt.Println(buf.String())
		} else {
			out, err := os.OpenFile(c.String("out"), os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			if _, err := io.Copy(out, &buf); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
		}
	}
	<-quit
	return nil
}

func initLog(c *cli.Context) {
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		panic(err)
	}
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().In(jst)
	}
	if c != nil && c.Bool("debug") {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
}

func NewGenProxy() *GenProxy {
	proxy := goproxy.NewProxyHttpServer()
	flows := &Flowsx{
		Flows: make(map[string]Flow),
		mutex: sync.Mutex{},
	}
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
		ctx.UserData = Flow{
			ID:      flowID,
			Request: request,
		}
		return r, nil
	})

	proxy.OnResponse().DoFunc(func(r *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		flow := ctx.UserData.(Flow)
		var respBody io.ReadCloser
		if r == nil {
			log.Warn().Msgf("%s %s", ctx.Req.Method, ctx.Req.URL.String())
			return r
		} else {
			r.Body, respBody = duplicateReadCloser(r.Body)
		}
		response := http.Response{
			StatusCode: r.StatusCode,
			Header:     r.Header,
			Body:       respBody,
		}
		flow.Response = response
		flows.add(flow)
		log.Info().Msgf("%s %s", r.Request.Method, r.Request.URL.String())
		return r
	})
	return &GenProxy{
		proxy: proxy,
		flows: flows,
	}
}

func (p *GenProxy) Proxy() *goproxy.ProxyHttpServer {
	return p.proxy
}

func (p *GenProxy) Flows() map[string]Flow {
	return p.flows.Flows
}
