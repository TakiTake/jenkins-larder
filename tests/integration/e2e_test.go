//go:build integration

package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yourorg/jenkins-larder/src/config"
	"github.com/yourorg/jenkins-larder/src/server"
)

// T073: End-to-end test exercising all three server endpoints

func TestEndToEnd(t *testing.T) {
	// Set up upstream that serves realistic plugin data
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		// Simulate a small plugin binary
		data := make([]byte, 1024)
		for i := range data {
			data[i] = byte(i % 256)
		}
		if _, err := w.Write(data); err != nil {
			return
		}
	})

	upstreamServer := httptest.NewServer(upstream)
	defer upstreamServer.Close()

	cacheDir := t.TempDir()
	cfg := &config.Config{
		Storage:  config.StorageConfig{LimitBytes: 10 * 1024 * 1024, Path: cacheDir},
		Upstream: config.UpstreamConfig{URL: upstreamServer.URL, TimeoutSeconds: 30},
		Server:   config.ServerConfig{Port: 0, MetricsPort: 0},
		Admin:    config.AdminConfig{Port: 0},
		TTL:      config.TTLConfig{Enabled: false},
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	pluginTS := httptest.NewServer(srv.PluginHandler())
	defer pluginTS.Close()

	adminTS := httptest.NewServer(srv.AdminHandler())
	defer adminTS.Close()

	metricsTS := httptest.NewServer(srv.MetricsHandler())
	defer metricsTS.Close()

	// 1. Health check
	t.Run("health check", func(t *testing.T) {
		resp, err := http.Get(adminTS.URL + "/admin/health")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("health check: status %d", resp.StatusCode)
		}

		var health map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
			t.Fatalf("failed to decode health response: %v", err)
		}
		if health["status"] != "healthy" {
			t.Errorf("health status = %v, want healthy", health["status"])
		}
	})

	// 2. Download plugin (cache miss)
	t.Run("cache miss download", func(t *testing.T) {
		resp, err := http.Get(pluginTS.URL + "/download/plugins/git/5.0.0/git.hpi")
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("download: status %d", resp.StatusCode)
		}
		if len(body) != 1024 {
			t.Errorf("body size = %d, want 1024", len(body))
		}
		if resp.Header.Get("X-Checksum-SHA256") == "" {
			t.Error("missing X-Checksum-SHA256 header")
		}
	})

	// 3. Download same plugin (cache hit)
	t.Run("cache hit download", func(t *testing.T) {
		resp, err := http.Get(pluginTS.URL + "/download/plugins/git/5.0.0/git.hpi")
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("cache hit: status %d", resp.StatusCode)
		}
		if len(body) != 1024 {
			t.Errorf("cache hit body size = %d, want 1024", len(body))
		}
	})

	// 4. Check cache stats
	t.Run("cache stats", func(t *testing.T) {
		resp, err := http.Get(adminTS.URL + "/admin/cache/stats")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		var stats map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			t.Fatalf("failed to decode stats response: %v", err)
		}

		if stats["total_plugins"].(float64) < 1 {
			t.Errorf("total_plugins = %v, want >= 1", stats["total_plugins"])
		}
		if stats["total_size_bytes"].(float64) < 1024 {
			t.Errorf("total_size_bytes = %v, want >= 1024", stats["total_size_bytes"])
		}
	})

	// 5. Check Prometheus metrics
	t.Run("prometheus metrics", func(t *testing.T) {
		resp, err := http.Get(metricsTS.URL + "/metrics")
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("failed to read metrics body: %v", err)
		}
		content := string(body)

		expected := []string{
			"jenkins_larder_plugin_downloads_total",
			"jenkins_larder_cache_hits_total",
			"jenkins_larder_cache_misses_total",
			"jenkins_larder_storage_usage_bytes",
			"jenkins_larder_storage_limit_bytes",
			"jenkins_larder_download_duration_seconds",
			"jenkins_larder_cached_plugins_total",
			"jenkins_larder_bandwidth_saved_bytes_total",
		}
		for _, metric := range expected {
			if !strings.Contains(content, metric) {
				t.Errorf("missing metric: %s", metric)
			}
		}
	})

	// 6. Invalidate cached plugin
	t.Run("invalidate plugin", func(t *testing.T) {
		body := `{"name":"git","version":"5.0.0"}`
		resp, err := http.Post(adminTS.URL+"/admin/cache/invalidate", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("invalidation: status %d", resp.StatusCode)
		}
	})

	// 7. Verify re-download after invalidation
	t.Run("re-download after invalidation", func(t *testing.T) {
		resp, err := http.Get(pluginTS.URL + "/download/plugins/git/5.0.0/git.hpi")
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("re-download: status %d", resp.StatusCode)
		}
		if len(body) != 1024 {
			t.Errorf("re-download body size = %d, want 1024", len(body))
		}
	})
}
