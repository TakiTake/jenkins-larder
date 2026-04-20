package contract

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// T042: Contract tests for admin invalidation API endpoint

func TestAdminInvalidateCacheContract(t *testing.T) {
	srv, _, _ := newTestServer(t)
	adminTS := httptest.NewServer(srv.AdminHandler())
	defer adminTS.Close()

	// Prime cache first via plugin handler
	pluginTS := httptest.NewServer(srv.PluginHandler())
	defer pluginTS.Close()

	resp, err := http.Get(pluginTS.URL + "/download/plugins/admin-test/1.0.0/admin-test.hpi")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to prime cache: %d", resp.StatusCode)
	}

	t.Run("POST with valid plugin name and version", func(t *testing.T) {
		body := `{"name":"admin-test","version":"1.0.0"}`
		resp, err := http.Post(adminTS.URL+"/admin/cache/invalidate", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if result["status"] != "invalidated" {
			t.Errorf("status = %v, want invalidated", result["status"])
		}
	})

	t.Run("POST with missing plugin name", func(t *testing.T) {
		body := `{"version":"1.0.0"}`
		resp, err := http.Post(adminTS.URL+"/admin/cache/invalidate", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})

	t.Run("POST with non-existent plugin returns 404", func(t *testing.T) {
		body := `{"name":"nonexistent","version":"9.9.9"}`
		resp, err := http.Post(adminTS.URL+"/admin/cache/invalidate", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("GET method not allowed", func(t *testing.T) {
		resp, err := http.Get(adminTS.URL + "/admin/cache/invalidate")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", resp.StatusCode)
		}
	})
}

func TestAdminCacheStatsContract(t *testing.T) {
	srv, _, _ := newTestServer(t)
	adminTS := httptest.NewServer(srv.AdminHandler())
	defer adminTS.Close()

	resp, err := http.Get(adminTS.URL + "/admin/cache/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify required fields
	requiredFields := []string{"total_plugins", "total_size_bytes", "limit_bytes", "utilization_percent"}
	for _, field := range requiredFields {
		if _, ok := stats[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

func TestAdminHealthCheckContract(t *testing.T) {
	srv, _, _ := newTestServer(t)
	adminTS := httptest.NewServer(srv.AdminHandler())
	defer adminTS.Close()

	t.Run("healthy state", func(t *testing.T) {
		resp, err := http.Get(adminTS.URL + "/admin/health")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		ct := resp.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}

		var health map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if health["status"] != "healthy" {
			t.Errorf("status = %v, want healthy", health["status"])
		}
	})
}
