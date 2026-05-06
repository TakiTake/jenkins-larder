package unit

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yourorg/jenkins-larder/src/admin"
)

// mockCacheManager implements admin.CacheManager for testing
type mockCacheManager struct {
	invalidateErr error
	statsResult   *admin.CacheStats
	statsErr      error
	healthResult  map[string]string
}

func (m *mockCacheManager) InvalidatePlugin(name, version string) error {
	return m.invalidateErr
}

func (m *mockCacheManager) GetStats() (*admin.CacheStats, error) {
	return m.statsResult, m.statsErr
}

func (m *mockCacheManager) IsHealthy() map[string]string {
	return m.healthResult
}

func TestInvalidateCacheHandler(t *testing.T) {
	t.Run("rejects non-POST", func(t *testing.T) {
		h := admin.NewAdminHandler(&mockCacheManager{})
		req := httptest.NewRequest("GET", "/admin/cache/invalidate", nil)
		rr := httptest.NewRecorder()
		h.InvalidateCacheHandler(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", rr.Code)
		}
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		h := admin.NewAdminHandler(&mockCacheManager{})
		req := httptest.NewRequest("POST", "/admin/cache/invalidate", strings.NewReader("{bad"))
		rr := httptest.NewRecorder()
		h.InvalidateCacheHandler(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("rejects missing name", func(t *testing.T) {
		h := admin.NewAdminHandler(&mockCacheManager{})
		req := httptest.NewRequest("POST", "/admin/cache/invalidate", strings.NewReader(`{"version":"1.0"}`))
		rr := httptest.NewRecorder()
		h.InvalidateCacheHandler(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("rejects missing version", func(t *testing.T) {
		h := admin.NewAdminHandler(&mockCacheManager{})
		req := httptest.NewRequest("POST", "/admin/cache/invalidate", strings.NewReader(`{"name":"git"}`))
		rr := httptest.NewRecorder()
		h.InvalidateCacheHandler(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("returns 404 when plugin not found", func(t *testing.T) {
		h := admin.NewAdminHandler(&mockCacheManager{invalidateErr: errors.New("not found")})
		req := httptest.NewRequest("POST", "/admin/cache/invalidate", strings.NewReader(`{"name":"git","version":"1.0"}`))
		rr := httptest.NewRecorder()
		h.InvalidateCacheHandler(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("returns 200 on success", func(t *testing.T) {
		h := admin.NewAdminHandler(&mockCacheManager{})
		req := httptest.NewRequest("POST", "/admin/cache/invalidate", strings.NewReader(`{"name":"git","version":"1.0"}`))
		rr := httptest.NewRecorder()
		h.InvalidateCacheHandler(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rr.Code)
		}
	})
}

func TestCacheStatsHandler(t *testing.T) {
	t.Run("rejects non-GET", func(t *testing.T) {
		h := admin.NewAdminHandler(&mockCacheManager{})
		req := httptest.NewRequest("POST", "/admin/cache/stats", nil)
		rr := httptest.NewRecorder()
		h.CacheStatsHandler(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", rr.Code)
		}
	})

	t.Run("returns 500 on stats error", func(t *testing.T) {
		h := admin.NewAdminHandler(&mockCacheManager{statsErr: errors.New("storage error")})
		req := httptest.NewRequest("GET", "/admin/cache/stats", nil)
		rr := httptest.NewRecorder()
		h.CacheStatsHandler(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rr.Code)
		}
	})

	t.Run("returns stats on success", func(t *testing.T) {
		stats := &admin.CacheStats{
			TotalPlugins:       5,
			TotalSizeBytes:     1024,
			LimitBytes:         10240,
			UtilizationPercent: 10.0,
		}
		h := admin.NewAdminHandler(&mockCacheManager{statsResult: stats})
		req := httptest.NewRequest("GET", "/admin/cache/stats", nil)
		rr := httptest.NewRecorder()
		h.CacheStatsHandler(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rr.Code)
		}

		var result admin.CacheStats
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if result.TotalPlugins != 5 {
			t.Errorf("TotalPlugins = %d, want 5", result.TotalPlugins)
		}
	})
}

func TestHealthCheckHandler(t *testing.T) {
	t.Run("rejects non-GET", func(t *testing.T) {
		h := admin.NewAdminHandler(&mockCacheManager{healthResult: map[string]string{"storage": "ok", "upstream": "ok"}})
		req := httptest.NewRequest("POST", "/admin/health", nil)
		rr := httptest.NewRecorder()
		h.HealthCheckHandler(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", rr.Code)
		}
	})

	t.Run("returns healthy status", func(t *testing.T) {
		h := admin.NewAdminHandler(&mockCacheManager{healthResult: map[string]string{"storage": "ok", "upstream": "ok"}})
		req := httptest.NewRequest("GET", "/admin/health", nil)
		rr := httptest.NewRecorder()
		h.HealthCheckHandler(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rr.Code)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if result["status"] != "healthy" {
			t.Errorf("status = %v, want healthy", result["status"])
		}
	})

	t.Run("returns unhealthy with 503", func(t *testing.T) {
		h := admin.NewAdminHandler(&mockCacheManager{healthResult: map[string]string{"storage": "error: disk full", "upstream": "ok"}})
		req := httptest.NewRequest("GET", "/admin/health", nil)
		rr := httptest.NewRecorder()
		h.HealthCheckHandler(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", rr.Code)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if result["status"] != "unhealthy" {
			t.Errorf("status = %v, want unhealthy", result["status"])
		}
	})
}
