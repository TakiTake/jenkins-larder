package contract

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/yourorg/jenkins-larder/src/config"
	"github.com/yourorg/jenkins-larder/src/server"
)

func TestEvictionUsesActualDiskSize(t *testing.T) {
	// Verify that after eviction, the size is re-read from disk
	// rather than relying on metadata-based subtraction.
	//
	// We sneak an extra file into the plugins dir. With the old code
	// (currentSize -= oldest.FileSize), the loop wouldn't know about it.
	// With the fix (re-read from disk), the extra file is accounted for
	// and additional eviction happens if needed.
	pluginSize := 100
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := make([]byte, pluginSize)
		if _, err := w.Write(data); err != nil {
			return
		}
	}))
	defer upstream.Close()

	cacheDir := t.TempDir()
	keyPath, certPath := createTestRSAKeys(t)

	// Limit allows 2 plugins but not 2 plugins + sneaky file + third plugin
	cfg := &config.Config{
		Storage:  config.StorageConfig{LimitBytes: 350, Path: cacheDir},
		Upstream: config.UpstreamConfig{URL: upstream.URL, TimeoutSeconds: 10},
		Server:   config.ServerConfig{Port: 0, MetricsPort: 0},
		Admin:    config.AdminConfig{Port: 0},
	}
	cfg.Larder.RSA.KeyPath = keyPath
	cfg.Larder.RSA.CertPath = certPath
	cfg.Larder.UpdateCenter.BaseURL = "http://localhost:8080"
	cfg.Larder.UpdateCenter.TTLSeconds = 3600

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pluginTS := httptest.NewServer(srv.PluginHandler())
	defer pluginTS.Close()

	// Download two plugins
	for _, name := range []string{"acc-a", "acc-b"} {
		url := pluginTS.URL + "/download/plugins/" + name + "/1.0/" + name + ".hpi"
		resp, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.ReadAll(resp.Body); err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("download %s failed: %d", name, resp.StatusCode)
		}
	}

	// Sneak an extra file into the plugins directory
	sneakyDir := filepath.Join(cacheDir, "plugins", "sneaky")
	if err := os.MkdirAll(sneakyDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sneakyDir, "extra.dat"), make([]byte, 150), 0644); err != nil {
		t.Fatal(err)
	}

	// Download a third plugin — eviction must account for the sneaky file
	resp, err := http.Get(pluginTS.URL + "/download/plugins/acc-c/1.0/acc-c.hpi")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("download acc-c status = %d, want 200", resp.StatusCode)
	}

	// Both old plugins should have been evicted to make room
	// (the sneaky file eats into the budget)
	for _, name := range []string{"acc-a", "acc-b"} {
		evictedFile := filepath.Join(cacheDir, "plugins", name, "1.0", name+".hpi")
		if _, err := os.Stat(evictedFile); !os.IsNotExist(err) {
			t.Errorf("expected %s to be evicted", name)
		}
	}
}

func TestEvictionAbortsOnFileRemoveError(t *testing.T) {
	// When os.Remove fails on the plugin file (not IsNotExist),
	// ensureSpace should return an error, and the download should fail.
	pluginSize := 100
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := make([]byte, pluginSize)
		if _, err := w.Write(data); err != nil {
			return
		}
	}))
	defer upstream.Close()

	cacheDir := t.TempDir()
	keyPath, certPath := createTestRSAKeys(t)

	cfg := &config.Config{
		Storage:  config.StorageConfig{LimitBytes: 200, Path: cacheDir},
		Upstream: config.UpstreamConfig{URL: upstream.URL, TimeoutSeconds: 10},
		Server:   config.ServerConfig{Port: 0, MetricsPort: 0},
		Admin:    config.AdminConfig{Port: 0},
	}
	cfg.Larder.RSA.KeyPath = keyPath
	cfg.Larder.RSA.CertPath = certPath
	cfg.Larder.UpdateCenter.BaseURL = "http://localhost:8080"
	cfg.Larder.UpdateCenter.TTLSeconds = 3600

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pluginTS := httptest.NewServer(srv.PluginHandler())
	defer pluginTS.Close()

	// Download first plugin
	resp, err := http.Get(pluginTS.URL + "/download/plugins/rm-err/1.0/rm-err.hpi")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first download failed: %d", resp.StatusCode)
	}

	// Make the plugin file's parent directory read-only so os.Remove fails
	pluginDir := filepath.Join(cacheDir, "plugins", "rm-err", "1.0")
	if err := os.Chmod(pluginDir, 0555); err != nil {
		t.Fatal(err)
	}
	// Restore permissions on cleanup so t.TempDir() can remove it
	t.Cleanup(func() {
		if err := os.Chmod(pluginDir, 0755); err != nil {
			t.Logf("cleanup chmod: %v", err)
		}
	})

	// Download second plugin ��� should trigger eviction of first,
	// but os.Remove will fail due to read-only directory
	resp2, err := http.Get(pluginTS.URL + "/download/plugins/rm-err2/1.0/rm-err2.hpi")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(resp2.Body); err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	resp2.Body.Close()

	// Should fail because eviction couldn't delete the file
	if resp2.StatusCode == http.StatusOK {
		t.Error("expected failure when eviction cannot delete file, got 200")
	}
}
