package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
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
	"golang.org/x/net/idna"
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

func createIntermediateCertificate(rootCertificate *x509.Certificate, rootKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf(": %w", err)
	}

	notBefore := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	notAfter := time.Now().AddDate(2, 0, 0)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf(": %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serial,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		Issuer: pkix.Name{
			Country:            []string{"JP", "US"},
			Organization:       []string{"mitmproxy"},
			OrganizationalUnit: []string{"mitmproxy"},
			CommonName:         "mitmproxy",
		},
		Subject: pkix.Name{
			Country:      []string{"JP", "US"},
			Organization: []string{"gstbgenMidCA"},
		},
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		AuthorityKeyId:        rootCertificate.SubjectKeyId,
		SubjectKeyId:          bigIntHash(priv.N),
	}

	certB, err := x509.CreateCertificate(rand.Reader, &template, rootCertificate, &priv.PublicKey, rootKey)
	if err != nil {
		return nil, nil, fmt.Errorf(": %w", err)
	}

	cert, err := x509.ParseCertificate(certB)
	if err != nil {
		return nil, nil, fmt.Errorf(": %w", err)
	}
	return cert, priv, nil
}

func createCertificate(host string, rootCertificate *x509.Certificate, rootKey *rsa.PrivateKey) (tls.Certificate, error) {
	var c tls.Certificate
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return c, fmt.Errorf(": %w", err)
	}

	notBefore := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	notAfter := time.Now().AddDate(2, 0, 0)

	cnHosts := strings.Split(host, ":")
	hostName := cnHosts[0]

	hostName, err = idna.ToASCII(hostName)
	if err != nil {
		return c, err
	}
	hostName = strings.ToLower(hostName)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return c, fmt.Errorf(": %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serial,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		Issuer: pkix.Name{
			Organization:       []string{"mitmproxy"},
			OrganizationalUnit: []string{"mitmproxy"},
			CommonName:         "mitmproxy",
		},
		Subject: pkix.Name{
			CommonName: "*." + hostName,
		},
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:       x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		AuthorityKeyId: rootCertificate.SubjectKeyId,
		DNSNames:       []string{hostName, "*." + hostName},
		SubjectKeyId:   bigIntHash(priv.N),
	}

	certB, err := x509.CreateCertificate(rand.Reader, &template, rootCertificate, &priv.PublicKey, rootKey)
	if err != nil {
		return c, fmt.Errorf(": %w", err)
	}
	var certPem, keyPem bytes.Buffer
	if err := pem.Encode(&certPem, &pem.Block{Type: "CERTIFICATE", Bytes: certB}); err != nil {
		return c, fmt.Errorf(": %w", err)
	}

	marshaledKey := x509.MarshalPKCS1PrivateKey(priv)
	// if err != nil {
	// 	return c, fmt.Errorf(": %w", err)
	// }
	if err := pem.Encode(&keyPem, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: marshaledKey}); err != nil {
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
	fmt.Println(string(keyPemB))
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

	// rootCA, err := tls.LoadX509KeyPair(c.String("cert"), c.String("key"))
	// if err != nil {
	// 	return fmt.Errorf(": %w", err)
	// }

	rootCert, rootKey, err := createRootCAInfo(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("failed to create rootca info: %w", err)
	}

	// if rootCA.Leaf, err = x509.ParseCertificate(rootCA.Certificate[0]); err != nil {
	// 	return err
	// }

	// cert, key, err := createIntermediateCertificate(rootCert, rootKey)
	// if err != nil {
	// 	return fmt.Errorf("failed to create rootca info: %w", err)
	// }

	httpsHandler := func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		// certificate, err := createCertificate(host, cert, key)
		certificate, err := createCertificate(host, rootCert, rootKey)
		if err != nil {
			fmt.Println(err)
			return nil, ""
		}

		if err != nil {
			fmt.Println(err)
			// return fmt.Errorf("failed to create certificate: %w", err)
		}
		customConnectAction := &goproxy.ConnectAction{
			Action: goproxy.ConnectMitm,
			TLSConfig: func(host string, ctx *goproxy.ProxyCtx) (*tls.Config, error) {
				var config tls.Config
				config.Certificates = append(config.Certificates, certificate)
				return &config, nil
			},
		}
		return customConnectAction, host
	}
	proxy.OnRequest().HandleConnect(goproxy.FuncHttpsHandler(httpsHandler))
	return nil
}

func bigIntHash(n *big.Int) []byte {
	h := sha1.New()
	h.Write(n.Bytes())
	return h.Sum(nil)
}
