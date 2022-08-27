package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"os"

	"github.com/elazarl/goproxy"
	"github.com/urfave/cli/v2"
)

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
