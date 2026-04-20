package unit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yourorg/jenkins-larder/src/storage"
)

func TestLRUTrackerAdd(t *testing.T) {
	tracker, err := storage.NewLRUTracker(100)
	if err != nil {
		t.Fatal(err)
	}

	plugin := &storage.CachedPlugin{Name: "git", Version: "4.11.0", LastAccessTime: time.Now()}
	tracker.Add(plugin)

	if tracker.Len() != 1 {
		t.Errorf("Len() = %d, want 1", tracker.Len())
	}

	// Update existing
	plugin.FileSize = 999
	tracker.Add(plugin)
	if tracker.Len() != 1 {
		t.Errorf("Len() after update = %d, want 1", tracker.Len())
	}

	got, found := tracker.Get("git:4.11.0")
	if !found {
		t.Fatal("expected to find plugin after update")
	}
	if got.FileSize != 999 {
		t.Errorf("FileSize = %d, want 999", got.FileSize)
	}
}

func TestLRUTrackerGet(t *testing.T) {
	tracker, err := storage.NewLRUTracker(100)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("existing plugin", func(t *testing.T) {
		plugin := &storage.CachedPlugin{Name: "git", Version: "4.11.0", LastAccessTime: time.Now()}
		tracker.Add(plugin)

		got, found := tracker.Get("git:4.11.0")
		if !found {
			t.Fatal("expected to find plugin")
		}
		if got.Name != "git" {
			t.Errorf("Name = %q, want %q", got.Name, "git")
		}
	})

	t.Run("nonexistent plugin", func(t *testing.T) {
		_, found := tracker.Get("nonexistent:1.0")
		if found {
			t.Error("expected not found for nonexistent plugin")
		}
	})
}

func TestLRUTrackerGetOldest(t *testing.T) {
	t.Run("empty tracker", func(t *testing.T) {
		tracker, _ := storage.NewLRUTracker(100)
		_, found := tracker.GetOldest()
		if found {
			t.Error("expected not found on empty tracker")
		}
	})

	t.Run("returns oldest by access time", func(t *testing.T) {
		tracker, _ := storage.NewLRUTracker(100)

		old := &storage.CachedPlugin{
			Name: "old-plugin", Version: "1.0",
			LastAccessTime: time.Now().Add(-2 * time.Hour),
		}
		recent := &storage.CachedPlugin{
			Name: "recent-plugin", Version: "1.0",
			LastAccessTime: time.Now().Add(-1 * time.Hour),
		}
		newest := &storage.CachedPlugin{
			Name: "newest-plugin", Version: "1.0",
			LastAccessTime: time.Now(),
		}

		tracker.Add(old)
		tracker.Add(recent)
		tracker.Add(newest)

		oldest, found := tracker.GetOldest()
		if !found {
			t.Fatal("expected to find oldest")
		}
		if oldest.Name != "old-plugin" {
			t.Errorf("oldest = %q, want %q", oldest.Name, "old-plugin")
		}
	})
}

func TestLRUTrackerRemove(t *testing.T) {
	tracker, _ := storage.NewLRUTracker(100)
	plugin := &storage.CachedPlugin{Name: "git", Version: "4.11.0", LastAccessTime: time.Now()}
	tracker.Add(plugin)

	tracker.Remove("git:4.11.0")

	if tracker.Len() != 0 {
		t.Errorf("Len() = %d after remove, want 0", tracker.Len())
	}
	_, found := tracker.Get("git:4.11.0")
	if found {
		t.Error("expected not found after remove")
	}
}

func TestEvictRemovesFiles(t *testing.T) {
	dir := t.TempDir()
	s, err := storage.NewStorage(dir, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	tracker, _ := storage.NewLRUTracker(100)

	// Create a plugin file and metadata
	pluginDir := filepath.Join(dir, "plugins", "git", "4.11.0")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	pluginFile := filepath.Join(pluginDir, "git.hpi")
	if err := os.WriteFile(pluginFile, []byte("plugin data"), 0644); err != nil {
		t.Fatal(err)
	}

	plugin := &storage.CachedPlugin{
		Name:           "git",
		Version:        "4.11.0",
		Extension:      "hpi",
		FilePath:       pluginFile,
		FileSize:       11,
		LastAccessTime: time.Now(),
	}

	// Save metadata
	if err := storage.SaveMetadata(plugin, dir); err != nil {
		t.Fatal(err)
	}

	tracker.Add(plugin)

	// Evict
	evicted, err := s.Evict(tracker)
	if err != nil {
		t.Fatalf("Evict() error: %v", err)
	}

	if evicted.Name != "git" {
		t.Errorf("evicted plugin = %q, want %q", evicted.Name, "git")
	}

	// Verify file removed
	if _, err := os.Stat(pluginFile); !os.IsNotExist(err) {
		t.Error("expected plugin file to be deleted")
	}

	// Verify removed from tracker
	if tracker.Len() != 0 {
		t.Errorf("tracker Len() = %d, want 0", tracker.Len())
	}
}

func TestEvictUntilSpace(t *testing.T) {
	dir := t.TempDir()
	// Storage limit of 100 bytes
	s, err := storage.NewStorage(dir, 100)
	if err != nil {
		t.Fatal(err)
	}

	tracker, _ := storage.NewLRUTracker(100)

	// Add two 40-byte plugins
	for i, name := range []string{"plugin-a", "plugin-b"} {
		pluginDir := filepath.Join(dir, "plugins", name, "1.0")
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			t.Fatal(err)
		}
		data := make([]byte, 40)
		pluginFile := filepath.Join(pluginDir, name+".hpi")
		if err := os.WriteFile(pluginFile, data, 0644); err != nil {
			t.Fatal(err)
		}

		plugin := &storage.CachedPlugin{
			Name:           name,
			Version:        "1.0",
			Extension:      "hpi",
			FilePath:       pluginFile,
			FileSize:       40,
			LastAccessTime: time.Now().Add(time.Duration(i) * time.Hour),
		}
		if err := storage.SaveMetadata(plugin, dir); err != nil {
			t.Fatalf("failed to save metadata for %s: %v", plugin.Name, err)
		}
		tracker.Add(plugin)
	}

	// Need space for 30 bytes -> total would be 110 > 100, must evict at least one
	if err := s.EvictUntilSpace(30, tracker); err != nil {
		t.Fatalf("EvictUntilSpace() error: %v", err)
	}

	// Should have evicted the oldest plugin
	if tracker.Len() != 1 {
		t.Errorf("tracker Len() = %d, want 1", tracker.Len())
	}
}
