package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// TODO (T015): Implement checksum validation

// CalculateSHA256 computes the SHA-256 hash of a file
func CalculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ValidateChecksum verifies a file's SHA-256 hash matches the expected value
func ValidateChecksum(filePath, expectedSHA256 string) error {
	actual, err := CalculateSHA256(filePath)
	if err != nil {
		return err
	}

	if actual != expectedSHA256 {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA256, actual)
	}

	return nil
}

// CalculateChecksumFromReader computes SHA-256 hash from an io.Reader
// TODO (T034): Implement streaming checksum calculation during download
func CalculateChecksumFromReader(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", fmt.Errorf("failed to hash stream: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
