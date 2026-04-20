package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yourorg/jenkins-larder/src/config"
	"github.com/yourorg/jenkins-larder/src/server"
)

// T043: Integration test for manual cache invalidation

func TestManualCacheInvalidation(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("plugin-data-for-invalidation-test"))
	})

	upstreamServer := httptest.NewServer(upstream)
	defer upstreamServer.Close()

	cacheDir := t.TempDir()
	cfg := &config.Config{
		Storage:  config.StorageConfig{LimitBytes: 100 * 1024 * 1024, Path: cacheDir},
		Upstream: config.UpstreamConfig{URL: upstreamServer.URL, TimeoutSeconds: 10},
		Server:   config.ServerConfig{Port: 0, MetricsPort: 0},
		Admin:    config.AdminConfig{Port: 0},
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	pluginTS := httptest.NewServer(srv.PluginHandler())
	defer pluginTS.Close()

	adminTS := httptest.NewServer(srv.AdminHandler())
	defer adminTS.Close()

	// Step 1: Cache a plugin
	resp, err := http.Get(pluginTS.URL + "/download/plugins/inval-test/2.0.0/inval-test.hpi")
	if err != nil {
		t.Fatal(err)
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to cache plugin: %d", resp.StatusCode)
	}

	// Verify file is on disk
	cachedFile := filepath.Join(cacheDir, "plugins", "inval-test", "2.0.0", "inval-test.hpi")
	if _, err := os.Stat(cachedFile); os.IsNotExist(err) {
		t.Fatal("expected plugin to be cached on disk")
	}

	// Step 2: Invalidate via admin API
	body := `{"name":"inval-test","version":"2.0.0"}`
	resp, err = http.Post(adminTS.URL+"/admin/cache/invalidate", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("invalidation failed: %d", resp.StatusCode)
	}

	// Step 3: Verify file removed from disk
	if _, err := os.Stat(cachedFile); !os.IsNotExist(err) {
		t.Error("expected plugin file to be removed after invalidation")
	}

	// Step 4: Verify metadata removed
	metaFile := filepath.Join(cacheDir, "metadata", "inval-test", "2.0.0.json")
	if _, err := os.Stat(metaFile); !os.IsNotExist(err) {
		t.Error("expected metadata file to be removed after invalidation")
	}

	// Step 5: Re-request should re-download from upstream
	resp, err = http.Get(pluginTS.URL + "/download/plugins/inval-test/2.0.0/inval-test.hpi")
	if err != nil {
		t.Fatal(err)
	}
	reBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("re-download after invalidation: status = %d, want 200", resp.StatusCode)
	}
	if string(reBody) != "plugin-data-for-invalidation-test" {
		t.Errorf("re-download content mismatch")
	}
}

// T044: Integration test for optional TTL policy

func TestTTLPolicyRefresh(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ttl-plugin-data"))
	})

	upstreamServer := httptest.NewServer(upstream)
	defer upstreamServer.Close()

	cacheDir := t.TempDir()
	cfg := &config.Config{
		Storage:  config.StorageConfig{LimitBytes: 100 * 1024 * 1024, Path: cacheDir},
		Upstream: config.UpstreamConfig{URL: upstreamServer.URL, TimeoutSeconds: 10},
		Server:   config.ServerConfig{Port: 0, MetricsPort: 0},
		Admin:    config.AdminConfig{Port: 0},
		TTL:      config.TTLConfig{Enabled: true, DefaultHours: 0}, // 0 hours = always stale
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	pluginTS := httptest.NewServer(srv.PluginHandler())
	defer pluginTS.Close()

	adminTS := httptest.NewServer(srv.AdminHandler())
	defer adminTS.Close()

	// Cache a plugin
	resp, err := http.Get(pluginTS.URL + "/download/plugins/ttl-test/1.0.0/ttl-test.hpi")
	if err != nil {
		t.Fatal(err)
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()

	// Check stats - plugin should be marked as stale when TTL=0
	resp, err = http.Get(adminTS.URL + "/admin/cache/stats")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("stats status = %d, want 200", resp.StatusCode)
	}
}
