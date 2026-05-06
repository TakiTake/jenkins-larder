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
	"os"
	"path/filepath"
	"time"
)

func main() {
	outdir := flag.String("out", ".", "Output directory for key and certificate files")
	keyfile := flag.String("key", "tls.key", "Private key filename")
	certfile := flag.String("cert", "tls.crt", "Certificate filename")
	flag.Parse()

	key, cert, err := generateKeyAndCert()
	if err != nil {
		log.Fatalf("failed to generate key and cert: %v", err)
	}

	keyPath := filepath.Join(*outdir, *keyfile)
	certPath := filepath.Join(*outdir, *certfile)

	if err := writeKey(key, keyPath); err != nil {
		log.Fatalf("failed to write key: %v", err)
	}
	if err := writeCert(cert, certPath); err != nil {
		log.Fatalf("failed to write cert: %v", err)
	}

	fmt.Printf("RSA key pair generated successfully:\n")
	fmt.Printf("  Private key: %s\n", keyPath)
	fmt.Printf("  Certificate: %s\n", certPath)
	fmt.Printf("\nCreate Kubernetes secret with:\n")
	fmt.Printf("  kubectl create secret generic larder-signing-key -n larder \\\n")
	fmt.Printf("    --from-file=tls.key=%s \\\n", keyPath)
	fmt.Printf("    --from-file=tls.crt=%s\n", certPath)
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
			CommonName: "larder-signing",
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

func writeKey(key *rsa.PrivateKey, path string) error {
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	keyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer f.Close()

	if err := pem.Encode(f, keyPEM); err != nil {
		return fmt.Errorf("failed to encode key: %w", err)
	}

	return os.Chmod(path, 0600)
}

func writeCert(cert *x509.Certificate, path string) error {
	certPEM := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %w", err)
	}
	defer f.Close()

	if err := pem.Encode(f, certPEM); err != nil {
		return fmt.Errorf("failed to encode cert: %w", err)
	}

	return os.Chmod(path, 0644)
}
