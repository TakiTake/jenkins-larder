package contract

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yourorg/jenkins-larder/src/config"
	"github.com/yourorg/jenkins-larder/src/server"
)

// newTestServer creates a test server with a mock upstream.
// Returns the cache server, mock upstream server, and cache dir.
func newTestServer(t *testing.T, opts ...func(*config.Config)) (*server.Server, *httptest.Server, string) {
	t.Helper()

	// Mock upstream that serves plugin files
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve a fake plugin file
		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write([]byte("fake-plugin-content-" + r.URL.Path)); err != nil {
			return
		}
	}))
	t.Cleanup(upstream.Close)

	cacheDir := t.TempDir()
	keyPath, certPath := createTestRSAKeys(t)

	cfg := &config.Config{
		Storage: config.StorageConfig{
			LimitBytes: 100 * 1024 * 1024, // 100MB
			Path:       cacheDir,
		},
		Upstream: config.UpstreamConfig{
			URL:            upstream.URL,
			TimeoutSeconds: 10,
		},
		Server: config.ServerConfig{
			Port:        0, // will use httptest
			MetricsPort: 0,
		},
		Admin: config.AdminConfig{Port: 0},
	}
	cfg.Larder.RSA.KeyPath = keyPath
	cfg.Larder.RSA.CertPath = certPath
	cfg.Larder.UpdateCenter.BaseURL = "http://localhost:8080"
	cfg.Larder.UpdateCenter.TTLSeconds = 3600

	for _, opt := range opts {
		opt(cfg)
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	return srv, upstream, cacheDir
}

func createTestRSAKeys(t *testing.T) (keyPath, certPath string) {
	t.Helper()

	tmpDir := t.TempDir()
	keyPath = filepath.Join(tmpDir, "test.key")
	certPath = filepath.Join(tmpDir, "test.crt")

	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	// Create a self-signed certificate
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	// Write private key
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	keyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(keyPEM), 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	// Write certificate
	certPEM := &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(certPEM), 0644); err != nil {
		t.Fatalf("failed to write cert file: %v", err)
	}

	return keyPath, certPath
}

func TestPluginDownloadContract(t *testing.T) {
	srv, _, _ := newTestServer(t)

	handler := srv.PluginHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/download/plugins/git/4.11.0/git.hpi")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Content-Type header
	ct := resp.Header.Get("Content-Type")
	if ct != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", ct)
	}

	// X-Checksum-SHA256 header present
	checksum := resp.Header.Get("X-Checksum-SHA256")
	if checksum == "" {
		t.Error("expected X-Checksum-SHA256 header")
	}
	if len(checksum) != 64 {
		t.Errorf("X-Checksum-SHA256 length = %d, want 64", len(checksum))
	}

	// Body is non-empty
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if len(body) == 0 {
		t.Error("expected non-empty response body")
	}
}

func TestPluginDownloadNotFoundContract(t *testing.T) {
	// Upstream returns 404
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer upstream.Close()

	cacheDir := t.TempDir()
	keyPath, certPath := createTestRSAKeys(t)

	cfg := &config.Config{
		Storage:  config.StorageConfig{LimitBytes: 100 * 1024 * 1024, Path: cacheDir},
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

	ts := httptest.NewServer(srv.PluginHandler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/download/plugins/nonexistent/1.0.0/nonexistent.hpi")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestPluginDownloadURLPatternContract(t *testing.T) {
	srv, _, _ := newTestServer(t)
	handler := srv.PluginHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	tests := []struct {
		path       string
		wantStatus int
	}{
		{"/download/plugins/git/4.11.0/git.hpi", http.StatusOK},
		{"/download/plugins/credentials/2.6.1/credentials.jpi", http.StatusOK},
		{"/download/plugins/git/4.11.0/git", http.StatusBadRequest},     // missing extension
		{"/download/plugins/git/4.11.0/wrong.hpi", http.StatusBadRequest}, // filename mismatch
		{"/other/path", http.StatusNotFound},                                   // non-matching path returns 404
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			resp, err := http.Get(ts.URL + tt.path)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestPluginDownloadCacheHitContract(t *testing.T) {
	srv, _, cacheDir := newTestServer(t)
	handler := srv.PluginHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	url := ts.URL + "/download/plugins/git/4.11.0/git.hpi"

	// First request (cache miss)
	resp1, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	body1, err := io.ReadAll(resp1.Body)
	resp1.Body.Close()
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	// Verify file cached on disk
	cachedFile := filepath.Join(cacheDir, "plugins", "git", "4.11.0", "git.hpi")
	if _, err := os.Stat(cachedFile); os.IsNotExist(err) {
		t.Fatal("expected plugin file to be cached on disk")
	}

	// Second request (cache hit) - should return same content
	resp2, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	body2, err := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("cache hit status = %d, want 200", resp2.StatusCode)
	}
	if string(body1) != string(body2) {
		t.Error("cache hit returned different content than cache miss")
	}

	// Both should have checksum header
	if resp1.Header.Get("X-Checksum-SHA256") != resp2.Header.Get("X-Checksum-SHA256") {
		t.Error("checksum headers differ between cache miss and hit")
	}
}

func TestPluginDownloadCacheHitPerformance(t *testing.T) {
	srv, _, _ := newTestServer(t)
	handler := srv.PluginHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	url := ts.URL + "/download/plugins/perf-test/1.0.0/perf-test.hpi"

	// Prime cache
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	resp.Body.Close()

	// Measure cache hit latency
	start := time.Now()
	resp, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	resp.Body.Close()
	duration := time.Since(start)

	// Cache hit should be under 500ms (spec SC-006)
	if duration > 500*time.Millisecond {
		t.Errorf("cache hit took %v, want < 500ms", duration)
	}

	fmt.Printf("Cache hit latency: %v\n", duration)
}
