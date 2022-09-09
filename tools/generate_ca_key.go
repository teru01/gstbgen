package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"os"
	"time"
)

func main() {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalln(err)
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalln(err)
	}
	notBefore := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	notAfter := time.Now().AddDate(10, 0, 0)

	template := x509.Certificate{
		SerialNumber: serial,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		Issuer: pkix.Name{
			Organization: []string{"gstbgen"},
			CommonName:   "gstbgen",
		},
		IsCA:        true,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}

	certB, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, &priv)
	if err != nil {
		log.Fatalln(err)
	}
	var certPem, keyPem bytes.Buffer
	if err := pem.Encode(&certPem, &pem.Block{Type: "CERTIFICATE", Bytes: certB}); err != nil {
		log.Fatalln(err)
	}

	marshaledKey := x509.MarshalPKCS1PrivateKey(priv)
	if err := pem.Encode(&keyPem, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: marshaledKey}); err != nil {
		log.Fatalln(err)
	}
	if err := os.WriteFile("gstbgen.crt", certPem.Bytes(), 0400); err != nil {
		log.Fatalln(err)
	}
	if err := os.WriteFile("gstbgen.key", keyPem.Bytes(), 0400); err != nil {
		log.Fatalln(err)
	}
}
