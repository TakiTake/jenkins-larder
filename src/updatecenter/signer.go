package updatecenter

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Signer signs the update center JSON using RSA keys.
type Signer struct {
	Key  *rsa.PrivateKey
	Cert *x509.Certificate
}

// NewSigner creates a new signer with the given key and certificate.
func NewSigner(key *rsa.PrivateKey, cert *x509.Certificate) *Signer {
	return &Signer{
		Key:  key,
		Cert: cert,
	}
}

// Sign adds a cryptographic signature to the update center JSON.
// It removes any existing signature, computes digests, signs them,
// and adds a new signature block.
func (s *Signer) Sign(uc map[string]interface{}) error {
	// Remove any existing signature
	delete(uc, "signature")

	// Marshal to JSON to get the bytes we'll sign
	jsonBytes, err := json.Marshal(uc)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Compute SHA-1 digest
	sha1Hash := sha1.Sum(jsonBytes)

	// Compute SHA-512 digest
	sha512Hash := sha512.Sum512(jsonBytes)

	// Sign the SHA-1 digest
	sha1Sig, err := rsa.SignPKCS1v15(rand.Reader, s.Key, crypto.SHA1, sha1Hash[:])
	if err != nil {
		return fmt.Errorf("failed to sign SHA-1 digest: %w", err)
	}

	// Sign the SHA-512 digest
	sha512Sig, err := rsa.SignPKCS1v15(rand.Reader, s.Key, crypto.SHA512, sha512Hash[:])
	if err != nil {
		return fmt.Errorf("failed to sign SHA-512 digest: %w", err)
	}

	// Encode certificate to base64
	certB64 := base64.StdEncoding.EncodeToString(s.Cert.Raw)

	// Create the signature block
	signature := map[string]interface{}{
		"certificates": []string{certB64},
		"correct_digest": base64.StdEncoding.EncodeToString(sha1Hash[:]),
		"correct_digest512": hex.EncodeToString(sha512Hash[:]),
		"correct_signature": base64.StdEncoding.EncodeToString(sha1Sig),
		"correct_signature512": hex.EncodeToString(sha512Sig),
	}

	// Add signature back to the update center
	uc["signature"] = signature

	return nil
}
