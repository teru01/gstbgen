package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/urfave/cli/v2"
)

func createRootCAInfo(rootCertificateReader, rootKeyReader io.Reader) (*x509.Certificate, *rsa.PrivateKey, error) {
	rootCertB, err := io.ReadAll(rootCertificateReader)
	if err != nil {
		return nil, nil, fmt.Errorf(": %w", err)
	}
	rootCertBlock, _ := pem.Decode(rootCertB)
	if err != nil {
		return nil, nil, fmt.Errorf(": %w", err)
	}
	rootCertificate, err := x509.ParseCertificate(rootCertBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf(": %w", err)
	}
	k, err := io.ReadAll(rootKeyReader)
	if err != nil {
		return nil, nil, fmt.Errorf(": %w", err)
	}
	rootKBlock, _ := pem.Decode(k)
	rootKey, err := x509.ParsePKCS1PrivateKey(rootKBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf(": %w", err)
	}
	return rootCertificate, rootKey, nil
}

func createCertificate(host string, rootCertificate *x509.Certificate, rootKey *rsa.PrivateKey) (tls.Certificate, error) {
	var c tls.Certificate
	serial, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return c, fmt.Errorf(": %w", err)
	}

	notBefore := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	notAfter := time.Now().AddDate(2, 0, 0)
	cnHosts := strings.Split(host, ":")
	template := x509.Certificate{
		SerialNumber: serial,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		Issuer: pkix.Name{
			Country:      []string{"JP", "US"},
			Organization: []string{"gstbgenCA"},
			CommonName:   "gstbgenCA",
		},
		Subject: pkix.Name{
			Country:      []string{"JP", "US"},
			Organization: []string{"gstbgen"},
			CommonName:   cnHosts[0],
		},
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  false,
		AuthorityKeyId:        rootCertificate.SubjectKeyId,
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return c, fmt.Errorf(": %w", err)
	}
	certB, err := x509.CreateCertificate(rand.Reader, &template, rootCertificate, &priv.PublicKey, rootKey)
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
	fmt.Println(string(certPemB))
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
	rootCert, rootKey, err := createRootCAInfo(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("failed to create rootca info: %w", err)
	}
	httpsHandler := func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		certificate, err := createCertificate(host, rootCert, rootKey)
		if err != nil {
			fmt.Println(err)
			// return fmt.Errorf("failed to create certificate: %w", err)
		}
		customConnectAction := &goproxy.ConnectAction{
			Action:    goproxy.ConnectMitm,
			TLSConfig: goproxy.TLSConfigFromCA(&certificate),
		}
		return customConnectAction, host
	}
	proxy.OnRequest().HandleConnect(goproxy.FuncHttpsHandler(httpsHandler))
	return nil
}
