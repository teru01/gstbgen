package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

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
			&cli.IntFlag{
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
	mockServerPort = c.Int("mockBeginPort")

	proxy := goproxy.NewProxyHttpServer()
	if c.String("cert") != "" && c.String("key") != "" {
		enableHttpsProxy(c, proxy)
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
		fmt.Printf("%#v\n", stmt)
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

func createCertificate(certificateReader, keyReader io.Reader) (tls.Certificate, error) {
	var c tls.Certificate
	cert, err := io.ReadAll(certificateReader)
	if err != nil {
		return c, fmt.Errorf("failed to read cert: %w", err)
	}
	k, err := io.ReadAll(keyReader)
	if err != nil {
		return c, fmt.Errorf("failed to read key: %w", err)
	}
	return tls.X509KeyPair(cert, k)
}

func enableHttpsProxy(c *cli.Context, proxy *goproxy.ProxyHttpServer) error {
	certFile, err := os.Open(c.String("cert"))
	if err != nil {
		return fmt.Errorf("failed to open cert file: %w", err)
	}
	keyFile, err := os.Open(c.String("key"))
	if err != nil {
		return fmt.Errorf("failed to open key file: %w", err)
	}
	certificate, err := createCertificate(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}
	customConnectAction := &goproxy.ConnectAction{
		Action:    goproxy.ConnectMitm,
		TLSConfig: goproxy.TLSConfigFromCA(&certificate),
	}
	httpsHandler := func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		return customConnectAction, host
	}
	proxy.OnRequest().HandleConnect(goproxy.FuncHttpsHandler(httpsHandler))
	return nil
}
