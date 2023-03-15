package main

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path"
	"testing"
)

//Check correct status is returned when doing a bad request
func TestGenerateCertificate(t *testing.T) {
	var cleanUp func()
	exePath, cleanUp = getStorageLocation(t)
	defer cleanUp()
	name := "127.0.0.1"
	generateCert(name, certFilename, keyFilename)
	certPem, err := os.ReadFile(path.Join(exePath, certFilename))
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certPem) {
		t.Fatal("Failed to parse certificate")
	}

	block, _ := pem.Decode(certPem)
	if block == nil {
		t.Fatal("Failed to parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	opts := x509.VerifyOptions{
		Roots:         roots,
		DNSName:       name,
		Intermediates: x509.NewCertPool(),
	}

	if _, err := cert.Verify(opts); err != nil {
		t.Fatal(err)
	}
}
