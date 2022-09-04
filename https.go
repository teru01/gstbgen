package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"

	"github.com/elazarl/goproxy"
	"github.com/urfave/cli/v2"
)

func createCertificate(rootCertificateReader, rootKeyReader io.Reader) (tls.Certificate, error) {
	var c tls.Certificate
	rootCertB, err := io.ReadAll(rootCertificateReader)
	if err != nil {
		return c, fmt.Errorf(": %w", err)
	}
	rootCertBlock, _ := pem.Decode(rootCertB)
	if err != nil {
		return c, fmt.Errorf(": %w", err)
	}
	rootCertificate, err := x509.ParseCertificate(rootCertBlock.Bytes)
	if err != nil {
		return c, fmt.Errorf(": %w", err)
	}
	k, err := io.ReadAll(rootKeyReader)
	if err != nil {
		return c, fmt.Errorf("failed to read key: %w", err)
	}
	rootKBlock, _ := pem.Decode(k)
	rootKey, err := x509.ParsePKCS1PrivateKey(rootKBlock.Bytes)
	if err != nil {
		return c, fmt.Errorf("failed to read key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return c, fmt.Errorf(": %w", err)
	}
	template := x509.Certificate{
		SerialNumber: serial,
		Issuer: pkix.Name{
			Country:      []string{"JP", "US"},
			Organization: []string{"gstbgenCA"},
			CommonName:   "gstbgenCA",
		},
		Subject: pkix.Name{
			Country:      []string{"JP", "US"},
			Organization: []string{"gstbgen"},
			CommonName:   "gstbgen",
		},
		AuthorityKeyId: rootCertificate.SubjectKeyId,
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return c, fmt.Errorf(": %w", err)
	}
	certB, err := x509.CreateCertificate(rand.Reader, &template, rootCertificate, pub, rootKey)
	if err != nil {
		return c, fmt.Errorf(": %w", err)
	}
	var certPem, keyPem bytes.Buffer
	if err := pem.Encode(&certPem, &pem.Block{Type: "CERTIFICATE", Bytes: certB}); err != nil {
		return c, fmt.Errorf(": %w", err)
	}

	marshaledKey, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return c, fmt.Errorf(": %w", err)
	}
	if err := pem.Encode(&keyPem, &pem.Block{Type: "PRIVATE KEY", Bytes: marshaledKey}); err != nil {
		return c, fmt.Errorf(": %w", err)
	}

	certPemB, err := io.ReadAll(&certPem)
	if err != nil {
		return c, fmt.Errorf(": %w", err)
	}
	keyPemB, err := io.ReadAll(&keyPem)
	if err != nil {
		return c, fmt.Errorf(": %w", err)
	}
	return tls.X509KeyPair(certPemB, keyPemB)
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
	defer keyFile.Close()
	defer certFile.Close()
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
