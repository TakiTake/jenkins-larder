package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yourorg/jenkins-larder/src/storage"
)

func TestListMetadata(t *testing.T) {
	t.Run("empty metadata dir", func(t *testing.T) {
		dir := t.TempDir()
		// Create metadata dir so Walk succeeds
		if err := os.MkdirAll(filepath.Join(dir, "metadata"), 0755); err != nil {
			t.Fatal(err)
		}

		plugins, err := storage.ListMetadata(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plugins) != 0 {
			t.Errorf("expected 0 plugins, got %d", len(plugins))
		}
	})

	t.Run("loads multiple plugins", func(t *testing.T) {
		dir := t.TempDir()

		for _, name := range []string{"git", "credentials"} {
			plugin := &storage.CachedPlugin{
				Name:      name,
				Version:   "1.0",
				Extension: "hpi",
			}
			if err := storage.SaveMetadata(plugin, dir); err != nil {
				t.Fatal(err)
			}
		}

		plugins, err := storage.ListMetadata(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plugins) != 2 {
			t.Errorf("expected 2 plugins, got %d", len(plugins))
		}
	})

	t.Run("skips non-json files", func(t *testing.T) {
		dir := t.TempDir()
		metaDir := filepath.Join(dir, "metadata", "test")
		if err := os.MkdirAll(metaDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Write a non-JSON file
		if err := os.WriteFile(filepath.Join(metaDir, "readme.txt"), []byte("not json"), 0644); err != nil {
			t.Fatal(err)
		}
		// Write a valid JSON metadata file
		plugin := &storage.CachedPlugin{Name: "test", Version: "1.0", Extension: "hpi"}
		if err := storage.SaveMetadata(plugin, dir); err != nil {
			t.Fatal(err)
		}

		plugins, err := storage.ListMetadata(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plugins) != 1 {
			t.Errorf("expected 1 plugin, got %d", len(plugins))
		}
	})

	t.Run("error on corrupt json", func(t *testing.T) {
		dir := t.TempDir()
		metaDir := filepath.Join(dir, "metadata", "bad")
		if err := os.MkdirAll(metaDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(metaDir, "1.0.json"), []byte("{invalid"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := storage.ListMetadata(dir)
		if err == nil {
			t.Fatal("expected error for corrupt JSON")
		}
	})

	t.Run("error when metadata dir missing", func(t *testing.T) {
		dir := t.TempDir()
		// Don't create metadata dir
		_, err := storage.ListMetadata(dir)
		if err == nil {
			t.Fatal("expected error when metadata dir missing")
		}
	})
}

func TestLoadMetadataCorrupt(t *testing.T) {
	dir := t.TempDir()
	metaDir := filepath.Join(dir, "metadata", "bad")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, "1.0.json"), []byte("{invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := storage.LoadMetadata("bad", "1.0", dir)
	if err == nil {
		t.Fatal("expected error for corrupt JSON metadata")
	}
}

func TestMetadataPath(t *testing.T) {
	plugin := &storage.CachedPlugin{Name: "git", Version: "4.11.0"}
	got := storage.MetadataPath(plugin, "/var/cache")
	want := filepath.Join("/var/cache", "metadata", "git", "4.11.0.json")
	if got != want {
		t.Errorf("MetadataPath() = %q, want %q", got, want)
	}
}
