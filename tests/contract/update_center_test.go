package contract

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yourorg/jenkins-larder/src/config"
	"github.com/yourorg/jenkins-larder/src/server"
)

// newUCUpstream serves JSONP that echoes the ?version= query param so tests
// can verify Larder forwards it to the upstream.
func newUCUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/update-center.json" {
			http.NotFound(w, r)
			return
		}
		version := r.URL.Query().Get("version")
		// Embed the version so tests can assert which upstream path was used.
		body := fmt.Sprintf(
			"updateCenter.post(\n{\"id\":\"default\",\"connectionCheckUrl\":\"http://example.com\",\"plugins\":{},\"core\":{\"buildDate\":\"Jan 1, 2024\",\"name\":\"core\",\"sha1\":\"aaa\",\"url\":\"http://example.com/jenkins.war\",\"version\":%q},\"signature\":{},\"updateCenterVersion\":\"1\",\"requestedVersion\":%q}\n);",
			"2.400", version,
		)
		w.Header().Set("Content-Type", "text/javascript")
		if _, err := fmt.Fprint(w, body); err != nil {
			t.Errorf("upstream write: %v", err)
		}
	}))
	t.Cleanup(upstream.Close)
	return upstream
}

// newUCServer creates a Larder server backed by the given upstream.
func newUCServer(t *testing.T, upstreamURL string) *httptest.Server {
	t.Helper()
	keyPath, certPath := createTestRSAKeys(t)
	cfg := &config.Config{
		Storage: config.StorageConfig{
			LimitBytes: 100 * 1024 * 1024,
			Path:       t.TempDir(),
		},
		Upstream: config.UpstreamConfig{
			URL:            upstreamURL,
			TimeoutSeconds: 10,
		},
		Server: config.ServerConfig{Port: 0, MetricsPort: 0},
		Admin:  config.AdminConfig{Port: 0},
	}
	cfg.Larder.RSA.KeyPath = keyPath
	cfg.Larder.RSA.CertPath = certPath
	cfg.Larder.UpdateCenter.BaseURL = "http://localhost:8080"
	cfg.Larder.UpdateCenter.TTLSeconds = 3600

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	ts := httptest.NewServer(srv.PluginHandler())
	t.Cleanup(ts.Close)
	return ts
}

func TestUpdateCenterNoVersionContract(t *testing.T) {
	upstream := newUCUpstream(t)
	ts := newUCServer(t, upstream.URL)

	resp, err := http.Get(ts.URL + "/update-center.json")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("Content-Type = %q, want text/javascript", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.HasPrefix(string(body), "updateCenter.post(") {
		t.Errorf("body missing JSONP wrapper: %s", truncate(string(body), 100))
	}
}

func TestUpdateCenterVersionParamForwardedContract(t *testing.T) {
	upstream := newUCUpstream(t)
	ts := newUCServer(t, upstream.URL)

	resp, err := http.Get(ts.URL + "/update-center.json?version=2.492.3")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	// Upstream echoes the requested version in "requestedVersion"; confirm it arrived.
	if !strings.Contains(string(body), "2.492.3") {
		t.Errorf("version param not forwarded to upstream; body: %s", truncate(string(body), 200))
	}
}

func TestUpdateCenterVersionCacheIsolationContract(t *testing.T) {
	upstream := newUCUpstream(t)
	ts := newUCServer(t, upstream.URL)

	fetch := func(url string) string {
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return string(body)
	}

	noVer := fetch(ts.URL + "/update-center.json")
	v492 := fetch(ts.URL + "/update-center.json?version=2.492.3")
	v491 := fetch(ts.URL + "/update-center.json?version=2.491.0")

	// Each response must reflect its own version, not bleed into others.
	if strings.Contains(noVer, "2.492.3") || strings.Contains(noVer, "2.491.0") {
		t.Error("no-version response contains a versioned string")
	}
	if !strings.Contains(v492, "2.492.3") {
		t.Error("version=2.492.3 response is missing 2.492.3")
	}
	if !strings.Contains(v491, "2.491.0") {
		t.Error("version=2.491.0 response is missing 2.491.0")
	}
	if strings.Contains(v491, "2.492.3") {
		t.Error("version=2.491.0 response contains 2.492.3 (cache bleed)")
	}

	// Subsequent requests must return cached content (identical bytes).
	if fetch(ts.URL+"/update-center.json?version=2.492.3") != v492 {
		t.Error("second fetch of version=2.492.3 returned different content")
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
