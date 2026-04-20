package contract

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// readBody is a test helper that reads and returns the response body, failing the test on error.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	return string(body)
}

// drainBody is a test helper that reads and discards the response body, failing the test on error.
func drainBody(t *testing.T, resp *http.Response) {
	t.Helper()
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("failed to drain response body: %v", err)
	}
	resp.Body.Close()
}

// T052: Contract test for Prometheus metrics endpoint format

func TestMetricsEndpointContract(t *testing.T) {
	srv, _, _ := newTestServer(t)
	metricsTS := httptest.NewServer(srv.MetricsHandler())
	defer metricsTS.Close()

	resp, err := http.Get(metricsTS.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	content := readBody(t, resp)

	// Verify Prometheus text format (contains HELP and TYPE lines)
	if !strings.Contains(content, "# HELP") {
		t.Error("expected Prometheus text format with HELP lines")
	}
	if !strings.Contains(content, "# TYPE") {
		t.Error("expected Prometheus text format with TYPE lines")
	}
}

func TestMetricsDownloadCounterContract(t *testing.T) {
	srv, _, _ := newTestServer(t)

	pluginTS := httptest.NewServer(srv.PluginHandler())
	defer pluginTS.Close()

	metricsTS := httptest.NewServer(srv.MetricsHandler())
	defer metricsTS.Close()

	// Generate a download to create metric data
	resp, err := http.Get(pluginTS.URL + "/download/plugins/metrics-test/1.0.0/metrics-test.hpi")
	if err != nil {
		t.Fatal(err)
	}
	drainBody(t, resp)

	// Fetch metrics
	resp, err = http.Get(metricsTS.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	content := readBody(t, resp)

	// Verify download counter exists with labels
	if !strings.Contains(content, "jenkins_larder_plugin_downloads_total") {
		t.Error("expected jenkins_larder_plugin_downloads_total metric")
	}

	// Verify cache miss counter incremented
	if !strings.Contains(content, "jenkins_larder_cache_misses_total") {
		t.Error("expected jenkins_larder_cache_misses_total metric")
	}
}

func TestMetricsCacheHitRatioContract(t *testing.T) {
	srv, _, _ := newTestServer(t)

	pluginTS := httptest.NewServer(srv.PluginHandler())
	defer pluginTS.Close()

	metricsTS := httptest.NewServer(srv.MetricsHandler())
	defer metricsTS.Close()

	url := pluginTS.URL + "/download/plugins/hit-ratio-test/1.0.0/hit-ratio-test.hpi"

	// Cache miss
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	drainBody(t, resp)

	// Cache hit
	resp, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	drainBody(t, resp)

	// Fetch metrics
	resp, err = http.Get(metricsTS.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	content := readBody(t, resp)

	// Both hits and misses should be tracked
	if !strings.Contains(content, "jenkins_larder_cache_hits_total") {
		t.Error("expected jenkins_larder_cache_hits_total metric")
	}
	if !strings.Contains(content, "jenkins_larder_cache_misses_total") {
		t.Error("expected jenkins_larder_cache_misses_total metric")
	}
}

func TestMetricsStorageUsageContract(t *testing.T) {
	srv, _, _ := newTestServer(t)

	metricsTS := httptest.NewServer(srv.MetricsHandler())
	defer metricsTS.Close()

	resp, err := http.Get(metricsTS.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	content := readBody(t, resp)

	if !strings.Contains(content, "jenkins_larder_storage_usage_bytes") {
		t.Error("expected jenkins_larder_storage_usage_bytes metric")
	}
	if !strings.Contains(content, "jenkins_larder_storage_limit_bytes") {
		t.Error("expected jenkins_larder_storage_limit_bytes metric")
	}
}

func TestMetricsDurationHistogramContract(t *testing.T) {
	srv, _, _ := newTestServer(t)

	pluginTS := httptest.NewServer(srv.PluginHandler())
	defer pluginTS.Close()

	metricsTS := httptest.NewServer(srv.MetricsHandler())
	defer metricsTS.Close()

	// Generate download
	resp, err := http.Get(pluginTS.URL + "/download/plugins/duration-test/1.0.0/duration-test.hpi")
	if err != nil {
		t.Fatal(err)
	}
	drainBody(t, resp)

	// Fetch metrics
	resp, err = http.Get(metricsTS.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	content := readBody(t, resp)

	if !strings.Contains(content, "jenkins_larder_download_duration_seconds") {
		t.Error("expected jenkins_larder_download_duration_seconds metric")
	}
}

func TestMetricsEvictionCounterContract(t *testing.T) {
	srv, _, _ := newTestServer(t)

	metricsTS := httptest.NewServer(srv.MetricsHandler())
	defer metricsTS.Close()

	resp, err := http.Get(metricsTS.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	content := readBody(t, resp)

	// Eviction metric should be registered even if zero
	if !strings.Contains(content, "jenkins_larder_evictions_total") {
		t.Error("expected jenkins_larder_evictions_total metric to be registered")
	}
}

func TestMetricsBandwidthSavedContract(t *testing.T) {
	srv, _, _ := newTestServer(t)

	pluginTS := httptest.NewServer(srv.PluginHandler())
	defer pluginTS.Close()

	metricsTS := httptest.NewServer(srv.MetricsHandler())
	defer metricsTS.Close()

	url := pluginTS.URL + "/download/plugins/bw-test/1.0.0/bw-test.hpi"

	// Cache miss then cache hit
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	drainBody(t, resp)

	resp, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	drainBody(t, resp)

	// Fetch metrics
	resp, err = http.Get(metricsTS.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	content := readBody(t, resp)

	if !strings.Contains(content, "jenkins_larder_bandwidth_saved_bytes_total") {
		t.Error("expected jenkins_larder_bandwidth_saved_bytes_total metric")
	}
}
