package unit

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/yourorg/jenkins-larder/src/storage"
)

func TestCalculateSHA256NonexistentFile(t *testing.T) {
	_, err := storage.CalculateSHA256("/nonexistent/file")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestValidateChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.txt"
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	err := storage.ValidateChecksum(path, "wrong-checksum")
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestValidateChecksumNonexistent(t *testing.T) {
	err := storage.ValidateChecksum("/nonexistent/file", "abc")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestCalculateChecksumFromReaderError(t *testing.T) {
	r := &failReader{}
	_, err := storage.CalculateChecksumFromReader(r)
	if err == nil {
		t.Fatal("expected error from failing reader")
	}
}

func TestCalculateChecksumFromReaderSuccess(t *testing.T) {
	r := strings.NewReader("hello world")
	checksum, err := storage.CalculateChecksumFromReader(r)
	if err != nil {
		t.Fatal(err)
	}
	if checksum == "" {
		t.Error("expected non-empty checksum")
	}
}

type failReader struct{}

func (e *failReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}
