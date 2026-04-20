package unit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yourorg/jenkins-larder/src/storage"
)

func TestCachedPluginKey(t *testing.T) {
	plugin := &storage.CachedPlugin{Name: "git", Version: "4.11.0"}
	got := plugin.Key()
	if got != "git:4.11.0" {
		t.Errorf("Key() = %q, want %q", got, "git:4.11.0")
	}
}

func TestUpdateAccessTime(t *testing.T) {
	plugin := &storage.CachedPlugin{
		Name:           "git",
		Version:        "4.11.0",
		LastAccessTime: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	before := time.Now()
	plugin.UpdateAccessTime()
	after := time.Now()

	if plugin.LastAccessTime.Before(before) || plugin.LastAccessTime.After(after) {
		t.Errorf("UpdateAccessTime() set time outside expected range")
	}
}

func TestIsStale(t *testing.T) {
	t.Run("ttl=0 never stale", func(t *testing.T) {
		plugin := &storage.CachedPlugin{
			DownloadTimestamp: time.Now().Add(-24 * 365 * time.Hour),
		}
		if plugin.IsStale(0) {
			t.Error("expected not stale when TTL is 0")
		}
	})

	t.Run("plugin within TTL is not stale", func(t *testing.T) {
		plugin := &storage.CachedPlugin{
			DownloadTimestamp: time.Now().Add(-1 * time.Hour),
		}
		if plugin.IsStale(24) {
			t.Error("expected not stale when within TTL")
		}
	})

	t.Run("plugin exceeding TTL is stale", func(t *testing.T) {
		plugin := &storage.CachedPlugin{
			DownloadTimestamp: time.Now().Add(-48 * time.Hour),
		}
		if !plugin.IsStale(24) {
			t.Error("expected stale when exceeding TTL")
		}
	})
}

func TestMetadataSaveLoad(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().Truncate(time.Second)

	original := &storage.CachedPlugin{
		Name:              "git",
		Version:           "4.11.0",
		Extension:         "hpi",
		FilePath:          "/var/cache/plugins/git/4.11.0/git.hpi",
		FileSize:          12345678,
		ChecksumSHA256:    "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		DownloadTimestamp:  now,
		LastAccessTime:    now,
		UpstreamURL:       "https://updates.jenkins.io/download/plugins/git/4.11.0/git.hpi",
	}

	// Save
	if err := storage.SaveMetadata(original, dir); err != nil {
		t.Fatalf("SaveMetadata() error: %v", err)
	}

	// Verify file exists
	metaPath := filepath.Join(dir, "metadata", "git", "4.11.0.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Fatal("metadata file not created")
	}

	// Load
	loaded, err := storage.LoadMetadata("git", "4.11.0", dir)
	if err != nil {
		t.Fatalf("LoadMetadata() error: %v", err)
	}

	// Compare
	if loaded.Name != original.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, original.Name)
	}
	if loaded.Version != original.Version {
		t.Errorf("Version = %q, want %q", loaded.Version, original.Version)
	}
	if loaded.Extension != original.Extension {
		t.Errorf("Extension = %q, want %q", loaded.Extension, original.Extension)
	}
	if loaded.FileSize != original.FileSize {
		t.Errorf("FileSize = %d, want %d", loaded.FileSize, original.FileSize)
	}
	if loaded.ChecksumSHA256 != original.ChecksumSHA256 {
		t.Errorf("ChecksumSHA256 = %q, want %q", loaded.ChecksumSHA256, original.ChecksumSHA256)
	}
	if loaded.Key() != original.Key() {
		t.Errorf("Key() = %q, want %q", loaded.Key(), original.Key())
	}
}

func TestMetadataLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := storage.LoadMetadata("nonexistent", "1.0", dir)
	if err == nil {
		t.Fatal("expected error loading nonexistent metadata")
	}
}

func TestStorageInitialize(t *testing.T) {
	dir := t.TempDir()
	storageDir := filepath.Join(dir, "cache")

	s, err := storage.NewStorage(storageDir, 1024*1024)
	if err != nil {
		t.Fatalf("NewStorage() error: %v", err)
	}

	// Verify subdirectories created
	for _, subdir := range []string{"plugins", "metadata"} {
		path := filepath.Join(storageDir, subdir)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			t.Errorf("directory %s not created", subdir)
		} else if !info.IsDir() {
			t.Errorf("%s is not a directory", subdir)
		}
	}

	_ = s
}

func TestStoragePluginPath(t *testing.T) {
	s := &storage.Storage{BaseDir: "/var/cache", LimitBytes: 1024}
	got := s.PluginPath("git", "4.11.0", "hpi")
	want := filepath.Join("/var/cache", "plugins", "git", "4.11.0", "git.hpi")
	if got != want {
		t.Errorf("PluginPath() = %q, want %q", got, want)
	}
}

func TestStorageGetCurrentSize(t *testing.T) {
	dir := t.TempDir()
	s, err := storage.NewStorage(dir, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	// Empty storage
	size, err := s.GetCurrentSize()
	if err != nil {
		t.Fatalf("GetCurrentSize() error: %v", err)
	}
	if size != 0 {
		t.Errorf("expected 0 for empty storage, got %d", size)
	}

	// Add a file
	pluginDir := filepath.Join(dir, "plugins", "test", "1.0")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	data := []byte("test plugin content")
	if err := os.WriteFile(filepath.Join(pluginDir, "test.hpi"), data, 0644); err != nil {
		t.Fatal(err)
	}

	size, err = s.GetCurrentSize()
	if err != nil {
		t.Fatalf("GetCurrentSize() error: %v", err)
	}
	if size != int64(len(data)) {
		t.Errorf("expected %d, got %d", len(data), size)
	}
}
