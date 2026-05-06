package unit

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yourorg/jenkins-larder/src/storage"
)

func TestCalculateSHA256(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.bin")

	content := []byte("hello world")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Calculate expected hash
	h := sha256.Sum256(content)
	expected := hex.EncodeToString(h[:])

	got, err := storage.CalculateSHA256(filePath)
	if err != nil {
		t.Fatalf("CalculateSHA256() error: %v", err)
	}
	if got != expected {
		t.Errorf("CalculateSHA256() = %q, want %q", got, expected)
	}
}

func TestCalculateSHA256_FileNotFound(t *testing.T) {
	_, err := storage.CalculateSHA256("/nonexistent/file")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestValidateChecksum(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.bin")

	content := []byte("test data for checksum")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256(content)
	correctHash := hex.EncodeToString(h[:])

	t.Run("matching checksum", func(t *testing.T) {
		if err := storage.ValidateChecksum(filePath, correctHash); err != nil {
			t.Errorf("expected no error for correct checksum, got: %v", err)
		}
	})

	t.Run("mismatched checksum", func(t *testing.T) {
		err := storage.ValidateChecksum(filePath, "0000000000000000000000000000000000000000000000000000000000000000")
		if err == nil {
			t.Fatal("expected error for incorrect checksum")
		}
		if !strings.Contains(err.Error(), "checksum mismatch") {
			t.Errorf("expected 'checksum mismatch' in error, got: %v", err)
		}
	})
}

func TestCalculateChecksumFromReader(t *testing.T) {
	content := "streaming content"
	reader := strings.NewReader(content)

	h := sha256.Sum256([]byte(content))
	expected := hex.EncodeToString(h[:])

	got, err := storage.CalculateChecksumFromReader(reader)
	if err != nil {
		t.Fatalf("CalculateChecksumFromReader() error: %v", err)
	}
	if got != expected {
		t.Errorf("CalculateChecksumFromReader() = %q, want %q", got, expected)
	}
}
