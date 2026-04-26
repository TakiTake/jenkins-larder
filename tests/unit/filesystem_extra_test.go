package unit

import (
	"testing"

	"github.com/yourorg/jenkins-larder/src/storage"
)

func TestStorageEnsureSpace(t *testing.T) {
	dir := t.TempDir()
	s, err := storage.NewStorage(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}

	// EnsureSpace is a stub that always returns nil
	if err := s.EnsureSpace(100); err != nil {
		t.Errorf("EnsureSpace() error = %v, want nil", err)
	}
}

func TestNewLRUTrackerError(t *testing.T) {
	// 0 capacity should error
	_, err := storage.NewLRUTracker(0)
	if err == nil {
		t.Fatal("expected error for 0 capacity")
	}
}
