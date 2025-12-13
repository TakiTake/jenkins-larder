package server

import (
	"io"
	"log/slog"
	"net/http"

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
	plugin, file, err := h.cache.GetPlugin(r.Context(), name, version, extension)
	if err != nil {
		slog.Error("Failed to get plugin", "plugin", name, "version", version, "error", err)
		http.Error(w, "Failed to retrieve plugin", http.StatusInternalServerError)
		return
	}
	defer file.Close()

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

// serveCachedPlugin serves a plugin from local cache
// TODO (T026): Implement serving from cache
func (h *DownloadHandler) serveCachedPlugin(w http.ResponseWriter, r *http.Request, name, version, extension string) {
	// TODO: Get plugin from storage
	// TODO: Update access time
	// TODO: Set Content-Type header
	// TODO: Set Content-Length header
	// TODO: Stream file to response
}

// downloadAndCachePlugin downloads from upstream and caches
// TODO (T035): Implement download and caching
func (h *DownloadHandler) downloadAndCachePlugin(w http.ResponseWriter, r *http.Request, name, version, extension string) error {
	// TODO: Download from upstream
	// TODO: Validate checksum
	// TODO: Save to storage
	// TODO: Update metadata
	// TODO: Stream to response while caching
	return nil
}
