package contract

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yourorg/jenkins-larder/src/config"
	"github.com/yourorg/jenkins-larder/src/server"
)

func TestCacheServeStaleOnUpstreamFailure(t *testing.T) {
	var upstreamAvailable atomic.Bool
	upstreamAvailable.Store(true)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !upstreamAvailable.Load() {
			http.Error(w, "upstream down", http.StatusBadGateway)
			return
		}
		if _, err := w.Write([]byte("stale-test-data")); err != nil {
			return
		}
	}))
	defer upstream.Close()

	cacheDir := t.TempDir()
	cfg := &config.Config{
		Storage:  config.StorageConfig{LimitBytes: 100 * 1024 * 1024, Path: cacheDir},
		Upstream: config.UpstreamConfig{URL: upstream.URL, TimeoutSeconds: 10},
		Server:   config.ServerConfig{Port: 0, MetricsPort: 0},
		Admin:    config.AdminConfig{Port: 0},
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pluginTS := httptest.NewServer(srv.PluginHandler())
	defer pluginTS.Close()

	// Cache the plugin
	resp, err := http.Get(pluginTS.URL + "/download/plugins/stale-cache/1.0/stale-cache.hpi")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initial download failed: %d", resp.StatusCode)
	}

	// Now invalidate from LRU (simulate eviction from index) by using admin API
	adminTS := httptest.NewServer(srv.AdminHandler())
	defer adminTS.Close()
	invResp, err := http.Post(adminTS.URL+"/admin/cache/invalidate", "application/json",
		strings.NewReader(`{"name":"stale-cache","version":"1.0"}`))
	if err != nil {
		t.Fatal(err)
	}
	invResp.Body.Close()

	// Re-create the plugin file on disk (simulate file existing without LRU entry)
	pluginDir := filepath.Join(cacheDir, "plugins", "stale-cache", "1.0")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "stale-cache.hpi"), body, 0644); err != nil {
		t.Fatal(err)
	}

	// Take upstream down
	upstreamAvailable.Store(false)

	// Request should serve stale from disk
	resp2, err := http.Get(pluginTS.URL + "/download/plugins/stale-cache/1.0/stale-cache.hpi")
	if err != nil {
		t.Fatal(err)
	}
	body2, err := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("stale serve status = %d, want 200", resp2.StatusCode)
	}
	if string(body2) != string(body) {
		t.Error("stale serve returned different content")
	}
}

func TestCacheUpstreamFailureNoStale(t *testing.T) {
	// Upstream always fails
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer upstream.Close()

	cacheDir := t.TempDir()
	cfg := &config.Config{
		Storage:  config.StorageConfig{LimitBytes: 100 * 1024 * 1024, Path: cacheDir},
		Upstream: config.UpstreamConfig{URL: upstream.URL, TimeoutSeconds: 10},
		Server:   config.ServerConfig{Port: 0, MetricsPort: 0},
		Admin:    config.AdminConfig{Port: 0},
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pluginTS := httptest.NewServer(srv.PluginHandler())
	defer pluginTS.Close()

	// Request uncached plugin with upstream down
	resp, err := http.Get(pluginTS.URL + "/download/plugins/no-cache/1.0/no-cache.hpi")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Error("expected failure when upstream down and no cache")
	}
}

func TestCacheEnsureSpaceEviction(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Each plugin is 50 bytes
		data := make([]byte, 50)
		if _, err := w.Write(data); err != nil {
			return
		}
	}))
	defer upstream.Close()

	cacheDir := t.TempDir()
	cfg := &config.Config{
		Storage:  config.StorageConfig{LimitBytes: 120, Path: cacheDir}, // Room for ~2 plugins
		Upstream: config.UpstreamConfig{URL: upstream.URL, TimeoutSeconds: 10},
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

	// Download 3 plugins - should trigger eviction
	for _, name := range []string{"evict-a", "evict-b", "evict-c"} {
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

	// First plugin should be evicted
	evictedFile := filepath.Join(cacheDir, "plugins", "evict-a", "1.0", "evict-a.hpi")
	if _, err := os.Stat(evictedFile); !os.IsNotExist(err) {
		t.Error("expected evict-a to be evicted")
	}

	// Check stats to verify eviction happened
	statsResp, err := http.Get(adminTS.URL + "/admin/cache/stats")
	if err != nil {
		t.Fatal(err)
	}
	var stats map[string]interface{}
	if err := json.NewDecoder(statsResp.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode stats: %v", err)
	}
	statsResp.Body.Close()
}

func TestCacheGetStatsAndHealth(t *testing.T) {
	srv, _, _ := newTestServer(t)
	adminTS := httptest.NewServer(srv.AdminHandler())
	defer adminTS.Close()
	pluginTS := httptest.NewServer(srv.PluginHandler())
	defer pluginTS.Close()

	// Download a plugin to have stats
	resp, err := http.Get(pluginTS.URL + "/download/plugins/stats-test/1.0/stats-test.hpi")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	resp.Body.Close()

	// Get stats
	statsResp, err := http.Get(adminTS.URL + "/admin/cache/stats")
	if err != nil {
		t.Fatal(err)
	}
	var stats admin_stats
	if err := json.NewDecoder(statsResp.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	statsResp.Body.Close()

	if stats.TotalPlugins < 1 {
		t.Errorf("total_plugins = %d, want >= 1", stats.TotalPlugins)
	}
	if stats.LimitBytes <= 0 {
		t.Error("limit_bytes should be > 0")
	}

	// Health check
	healthResp, err := http.Get(adminTS.URL + "/admin/health")
	if err != nil {
		t.Fatal(err)
	}
	var health map[string]interface{}
	if err := json.NewDecoder(healthResp.Body).Decode(&health); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	healthResp.Body.Close()

	if health["status"] != "healthy" {
		t.Errorf("health status = %v, want healthy", health["status"])
	}
}

type admin_stats struct {
	TotalPlugins       int     `json:"total_plugins"`
	TotalSizeBytes     int64   `json:"total_size_bytes"`
	LimitBytes         int64   `json:"limit_bytes"`
	UtilizationPercent float64 `json:"utilization_percent"`
}

func TestCacheLRUEntryButFileDeleted(t *testing.T) {
	srv, _, cacheDir := newTestServer(t)
	pluginTS := httptest.NewServer(srv.PluginHandler())
	defer pluginTS.Close()

	// Download a plugin
	resp, err := http.Get(pluginTS.URL + "/download/plugins/file-del/1.0/file-del.hpi")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	resp.Body.Close()

	// Delete the cached file behind the cache's back
	cachedFile := filepath.Join(cacheDir, "plugins", "file-del", "1.0", "file-del.hpi")
	if err := os.Remove(cachedFile); err != nil {
		t.Fatal(err)
	}

	// Re-request — should re-download from upstream (LRU entry exists but file gone)
	resp2, err := http.Get(pluginTS.URL + "/download/plugins/file-del/1.0/file-del.hpi")
	if err != nil {
		t.Fatal(err)
	}
	body2, err := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("re-download status = %d, want 200", resp2.StatusCode)
	}
	if len(body2) == 0 {
		t.Error("expected non-empty response")
	}
}

func TestCacheInvalidPluginURL(t *testing.T) {
	srv, _, _ := newTestServer(t)
	pluginTS := httptest.NewServer(srv.PluginHandler())
	defer pluginTS.Close()

	resp, err := http.Get(pluginTS.URL + "/download/plugins/bad-url")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCacheServeStaleWithMetadata(t *testing.T) {
	// Test the path where metadata IS available for stale serve
	var upstreamAvailable atomic.Bool
	upstreamAvailable.Store(true)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !upstreamAvailable.Load() {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		if _, err := w.Write([]byte("stale-with-meta")); err != nil {
			return
		}
	}))
	defer upstream.Close()

	cacheDir := t.TempDir()
	cfg := &config.Config{
		Storage:  config.StorageConfig{LimitBytes: 100 * 1024 * 1024, Path: cacheDir},
		Upstream: config.UpstreamConfig{URL: upstream.URL, TimeoutSeconds: 10},
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

	// Cache the plugin (this creates both the file and metadata)
	resp, err := http.Get(pluginTS.URL + "/download/plugins/stale-meta/1.0/stale-meta.hpi")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	resp.Body.Close()

	// Invalidate from LRU only (remove LRU entry but keep file + metadata on disk)
	invResp, err := http.Post(adminTS.URL+"/admin/cache/invalidate", "application/json",
		strings.NewReader(`{"name":"stale-meta","version":"1.0"}`))
	if err != nil {
		t.Fatal(err)
	}
	invResp.Body.Close()

	// Re-create the file (invalidation deletes it)
	pluginDir := filepath.Join(cacheDir, "plugins", "stale-meta", "1.0")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "stale-meta.hpi"), []byte("stale-with-meta"), 0644); err != nil {
		t.Fatal(err)
	}

	// Take upstream down
	upstreamAvailable.Store(false)

	// This should serve stale and find the metadata file
	resp2, err := http.Get(pluginTS.URL + "/download/plugins/stale-meta/1.0/stale-meta.hpi")
	if err != nil {
		t.Fatal(err)
	}
	body2, err := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp2.StatusCode)
	}
	if string(body2) != "stale-with-meta" {
		t.Errorf("body = %q, want %q", string(body2), "stale-with-meta")
	}
}
