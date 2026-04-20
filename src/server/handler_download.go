package server

import (
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/yourorg/jenkins-larder/src/metrics"
	"github.com/yourorg/jenkins-larder/src/upstream"
)

// DownloadHandler handles plugin download requests
type DownloadHandler struct {
	cache *CacheService
}

// NewDownloadHandler creates a new download handler
func NewDownloadHandler(cache *CacheService) *DownloadHandler {
	return &DownloadHandler{
		cache: cache,
	}
}

// ServeHTTP handles the plugin download request
// Pattern: /download/plugins/{name}/{version}/{name}.{extension}
func (h *DownloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse plugin name, version, extension from URL
	name, version, extension, err := upstream.ParsePluginURL(r.URL.Path)
	if err != nil {
		slog.Warn("Invalid plugin URL", "path", r.URL.Path, "error", err)
		http.Error(w, "Invalid plugin URL", http.StatusBadRequest)
		return
	}

	slog.Info("Plugin download request",
		"plugin", name,
		"version", version,
		"extension", extension,
		"remote_addr", r.RemoteAddr,
	)

	// Get plugin from cache (or download if needed)
	start := time.Now()
	plugin, file, err := h.cache.GetPlugin(r.Context(), name, version, extension)
	if err != nil {
		slog.Error("Failed to get plugin", "plugin", name, "version", version, "error", err)
		http.Error(w, "Failed to retrieve plugin", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	duration := time.Since(start).Seconds()

	// Determine source for duration metric
	source := "upstream"
	if duration < 0.1 {
		source = "cache"
	}
	metrics.DownloadDuration.WithLabelValues(source).Observe(duration)

	// Set response headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename="+name+"."+extension)
	w.Header().Set("X-Checksum-SHA256", plugin.ChecksumSHA256)

	// Stream file to response
	if _, err := io.Copy(w, file); err != nil {
		slog.Error("Failed to stream plugin", "plugin", name, "version", version, "error", err)
		return
	}

	slog.Info("Plugin download completed",
		"plugin", name,
		"version", version,
		"size_bytes", plugin.FileSize,
	)
}

