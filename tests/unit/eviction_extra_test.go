package unit

import (
	"testing"

	"github.com/yourorg/jenkins-larder/src/storage"
)

func TestEvictEmptyTracker(t *testing.T) {
	dir := t.TempDir()
	s, err := storage.NewStorage(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}

	tracker, err := storage.NewLRUTracker(100)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Evict(tracker)
	if err == nil {
		t.Fatal("expected error when evicting from empty tracker")
	}
}

func TestEvictMissingFile(t *testing.T) {
	dir := t.TempDir()
	s, err := storage.NewStorage(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}

	tracker, err := storage.NewLRUTracker(100)
	if err != nil {
		t.Fatal(err)
	}

	// Add a plugin with a non-existent file path
	plugin := &storage.CachedPlugin{
		Name:     "ghost",
		Version:  "1.0",
		FilePath: "/nonexistent/path/ghost.hpi",
	}
	tracker.Add(plugin)

	_, err = s.Evict(tracker)
	if err == nil {
		t.Fatal("expected error when evicting plugin with missing file")
	}
}

func TestEvictUntilSpaceEmptyTracker(t *testing.T) {
	dir := t.TempDir()
	s, err := storage.NewStorage(dir, 10) // tiny limit
	if err != nil {
		t.Fatal(err)
	}

	tracker, err := storage.NewLRUTracker(100)
	if err != nil {
		t.Fatal(err)
	}

	// No plugins to evict, but requesting space larger than limit
	err = s.EvictUntilSpace(100, tracker)
	if err == nil {
		t.Fatal("expected error when no plugins to evict")
	}
}
