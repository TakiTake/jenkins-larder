package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"time"
)

func main() {
	name := flag.String("name", "larder-signing-key", "Secret name")
	namespace := flag.String("namespace", "larder", "Kubernetes namespace")
	flag.Parse()

	key, cert, err := generateKeyAndCert()
	if err != nil {
		log.Fatalf("failed to generate key and cert: %v", err)
	}

	keyPEM, certPEM, err := encodePEM(key, cert)
	if err != nil {
		log.Fatalf("failed to encode PEM: %v", err)
	}

	manifest := generateSecret(*name, *namespace, keyPEM, certPEM)
	fmt.Print(manifest)
}

func generateKeyAndCert() (*rsa.PrivateKey, *x509.Certificate, error) {
	// Generate 4096-bit RSA key
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Create self-signed certificate
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "larder-agent",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10 years
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return key, cert, nil
}

func encodePEM(key *rsa.PrivateKey, cert *x509.Certificate) (string, string, error) {
	// Encode private key to PEM
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	})

	// Encode certificate to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})

	return string(keyPEM), string(certPEM), nil
}

func generateSecret(name, namespace, keyPEM, certPEM string) string {
	// Escape YAML special characters in the PEM content
	escapedKeyPEM := escapeYAML(keyPEM)
	escapedCertPEM := escapeYAML(certPEM)

	manifest := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
type: Opaque
stringData:
  tls.key: |
%s
  tls.crt: |
%s
`, name, namespace, escapedKeyPEM, escapedCertPEM)

	return manifest
}

func escapeYAML(s string) string {
	// Indent each line by 4 spaces for YAML literal block scalar
	result := ""
	for _, line := range splitLines(s) {
		result += "    " + line + "\n"
	}
	return result
}

func splitLines(s string) []string {
	var lines []string
	var current string
	for _, ch := range s {
		if ch == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
