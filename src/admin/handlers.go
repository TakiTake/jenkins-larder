package admin

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// CacheManager defines the interface for cache operations needed by admin handlers
type CacheManager interface {
	InvalidatePlugin(name, version string) error
	GetStats() (*CacheStats, error)
	IsHealthy() map[string]string
}

// CacheStats mirrors server.CacheStats for decoupling
type CacheStats struct {
	TotalPlugins       int     `json:"total_plugins"`
	TotalSizeBytes     int64   `json:"total_size_bytes"`
	LimitBytes         int64   `json:"limit_bytes"`
	UtilizationPercent float64 `json:"utilization_percent"`
}

// InvalidateRequest represents a cache invalidation request body
type InvalidateRequest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// AdminHandler handles administrative operations
type AdminHandler struct {
	cache CacheManager
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(cache CacheManager) *AdminHandler {
	return &AdminHandler{cache: cache}
}

// InvalidateCacheHandler handles cache invalidation requests
// POST /admin/cache/invalidate
func (h *AdminHandler) InvalidateCacheHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req InvalidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "plugin name is required", http.StatusBadRequest)
		return
	}

	if req.Version == "" {
		http.Error(w, "plugin version is required", http.StatusBadRequest)
		return
	}

	if err := h.cache.InvalidatePlugin(req.Name, req.Version); err != nil {
		slog.Warn("Invalidation failed", "plugin", req.Name, "version", req.Version, "error", err)
		http.Error(w, "Plugin not found in cache", http.StatusNotFound)
		return
	}

	slog.Info("Cache invalidated", "plugin", req.Name, "version", req.Version, "remote_addr", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "invalidated",
		"name":    req.Name,
		"version": req.Version,
	}); err != nil {
		slog.Error("Failed to write invalidation response", "error", err)
	}
}

// CacheStatsHandler returns cache statistics
// GET /admin/cache/stats
func (h *AdminHandler) CacheStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := h.cache.GetStats()
	if err != nil {
		slog.Error("Failed to get cache stats", "error", err)
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		slog.Error("Failed to write stats response", "error", err)
	}
}

// HealthCheckHandler returns health status
// GET /admin/health
func (h *AdminHandler) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	components := h.cache.IsHealthy()

	status := "healthy"
	for _, v := range components {
		if v != "ok" {
			status = "unhealthy"
			break
		}
	}

	health := map[string]interface{}{
		"status":  status,
		"storage": components["storage"],
		"upstream": components["upstream"],
	}

	w.Header().Set("Content-Type", "application/json")
	if status != "healthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	if err := json.NewEncoder(w).Encode(health); err != nil {
		slog.Error("Failed to write health response", "error", err)
	}
}
