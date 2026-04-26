package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/yourorg/jenkins-larder/src/admin"
	"github.com/yourorg/jenkins-larder/src/config"
	"github.com/yourorg/jenkins-larder/src/metrics"
	"github.com/yourorg/jenkins-larder/src/storage"
	"github.com/yourorg/jenkins-larder/src/upstream"
)

// CacheService coordinates storage, LRU tracking, and upstream downloads
type CacheService struct {
	store    PluginStore
	lru      LRUCache
	upstream UpstreamClient
	dedup    Deduplicator
	config   *config.Config
}

const maxLRUItems = 10000

func pluginKey(name, version string) string {
	return name + ":" + version
}

// NewCacheService creates a new cache service with concrete dependencies
func NewCacheService(cfg *config.Config) (*CacheService, error) {
	stor, err := storage.NewStorage(cfg.Storage.Path, cfg.Storage.LimitBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	lruTracker, err := storage.NewLRUTracker(maxLRUItems)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LRU tracker: %w", err)
	}

	// Load existing plugins into LRU tracker
	plugins, err := storage.ListMetadata(cfg.Storage.Path)
	if err == nil {
		for _, plugin := range plugins {
			lruTracker.Add(plugin)
		}
		slog.Info("Loaded cached plugins into LRU", "count", len(plugins))
	}

	upstreamClient := upstream.NewClient(cfg.Upstream.URL, cfg.Upstream.TimeoutSeconds)
	dedupManager := NewDeduplicationManager()

	return NewCacheServiceWithDeps(cfg, stor, lruTracker, upstreamClient, dedupManager), nil
}

// NewCacheServiceWithDeps creates a CacheService with injected dependencies for testing
func NewCacheServiceWithDeps(cfg *config.Config, store PluginStore, lru LRUCache, up UpstreamClient, dedup Deduplicator) *CacheService {
	cs := &CacheService{
		store:    store,
		lru:      lru,
		upstream: up,
		dedup:    dedup,
		config:   cfg,
	}

	metrics.StorageLimitBytes.Set(float64(cfg.Storage.LimitBytes))
	metrics.UpdateCachedPlugins(lru.Len())
	cs.updateStorageMetrics()

	return cs
}

// updateStorageMetrics refreshes the storage usage gauge
func (c *CacheService) updateStorageMetrics() {
	currentSize, err := c.store.GetCurrentSize()
	if err == nil {
		metrics.UpdateStorageMetrics(currentSize, c.config.Storage.LimitBytes)
		metrics.UpdateCachedPlugins(c.lru.Len())
	}
}

// GetPlugin retrieves a plugin, downloading from upstream if necessary
func (c *CacheService) GetPlugin(ctx context.Context, name, version, extension string) (*storage.CachedPlugin, io.ReadCloser, error) {
	key := pluginKey(name, version)

	// Check LRU cache first
	if plugin, found := c.lru.Get(key); found {
		// Verify file still exists on disk
		if _, err := c.store.PluginExists(plugin.FilePath); err == nil {
			plugin.UpdateAccessTime()
			c.lru.Add(plugin)

			if err := c.store.SavePluginMetadata(plugin); err != nil {
				slog.Warn("Failed to save updated metadata", "plugin", name, "version", version, "error", err)
			}

			file, err := c.store.OpenPlugin(plugin.FilePath)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to open cached plugin: %w", err)
			}

			slog.Info("Cache HIT", "plugin", name, "version", version)
			metrics.RecordDownload(name, version, "cache")
			metrics.RecordBandwidthSaved(plugin.FileSize)
			return plugin, file, nil
		}

		// File deleted, remove from LRU
		c.lru.Remove(key)
	}

	// Cache miss - use singleflight to deduplicate concurrent requests
	result, err := c.dedup.Do(ctx, key, func() (interface{}, error) {
		return c.downloadAndCache(ctx, name, version, extension)
	})

	if err != nil {
		metrics.RecordUpstreamError("download_failed")
		// Try to serve stale cached file from disk if upstream failed
		plugin, file, staleErr := c.serveStaleFromDisk(name, version, extension)
		if staleErr == nil {
			slog.Warn("Serving stale cached plugin due to upstream failure",
				"plugin", name, "version", version, "upstream_error", err)
			metrics.RecordDownload(name, version, "stale")
			return plugin, file, nil
		}
		return nil, nil, err
	}

	plugin := result.(*storage.CachedPlugin)

	file, err := c.store.OpenPlugin(plugin.FilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open downloaded plugin: %w", err)
	}

	return plugin, file, nil
}

// serveStaleFromDisk attempts to serve a previously cached plugin file from disk
// even if it's no longer in the LRU index (e.g., after restart or eviction from index).
func (c *CacheService) serveStaleFromDisk(name, version, extension string) (*storage.CachedPlugin, io.ReadCloser, error) {
	filePath := c.store.PluginPath(name, version, extension)
	info, err := c.store.PluginExists(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("no stale cache available: %w", err)
	}

	plugin, err := c.store.LoadPluginMetadata(name, version)
	if err != nil {
		// Reconstruct minimal metadata from the file on disk
		checksum, checksumErr := storage.CalculateSHA256(filePath)
		if checksumErr != nil {
			slog.Warn("Failed to calculate checksum for stale cache", "path", filePath, "error", checksumErr)
		}
		plugin = &storage.CachedPlugin{
			Name:           name,
			Version:        version,
			Extension:      extension,
			FilePath:       filePath,
			FileSize:       info.Size(),
			ChecksumSHA256: checksum,
		}
	}

	file, err := c.store.OpenPlugin(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open stale cached file: %w", err)
	}

	return plugin, file, nil
}

// downloadAndCache downloads a plugin from upstream and caches it
func (c *CacheService) downloadAndCache(ctx context.Context, name, version, extension string) (*storage.CachedPlugin, error) {
	slog.Info("Cache MISS - downloading from upstream", "plugin", name, "version", version)

	body, err := c.upstream.DownloadPlugin(ctx, name, version, extension)
	if err != nil {
		return nil, fmt.Errorf("failed to download from upstream: %w", err)
	}
	defer body.Close()

	// Ensure we have space (evict if needed) before writing
	// Use a size estimate of 0 since we don't know the size yet;
	// the real check happens after writing when we know the actual size
	filePath, written, checksum, err := c.store.WritePlugin(body, name, version, extension)
	if err != nil {
		return nil, err
	}

	plugin := &storage.CachedPlugin{
		Name:              name,
		Version:           version,
		Extension:         extension,
		FilePath:          filePath,
		FileSize:          written,
		ChecksumSHA256:    checksum,
		DownloadTimestamp:  time.Now(),
		LastAccessTime:    time.Now(),
		UpstreamURL:       c.upstream.PluginURL(name, version, extension),
	}

	// Ensure we have space (evict if needed)
	if err := c.ensureSpace(plugin.FileSize); err != nil {
		if removeErr := c.store.RemovePlugin(filePath); removeErr != nil {
			slog.Warn("Failed to clean up plugin after space error", "path", filePath, "error", removeErr)
		}
		return nil, fmt.Errorf("failed to ensure storage space: %w", err)
	}

	if err := c.store.SavePluginMetadata(plugin); err != nil {
		slog.Warn("Failed to save metadata", "error", err)
	}

	c.lru.Add(plugin)

	metrics.RecordDownload(name, version, "upstream")
	c.updateStorageMetrics()

	slog.Info("Plugin cached successfully",
		"plugin", name,
		"version", version,
		"size_bytes", plugin.FileSize,
		"checksum", plugin.ChecksumSHA256[:16]+"...",
	)

	return plugin, nil
}

// InvalidatePlugin removes a specific plugin from the cache
func (c *CacheService) InvalidatePlugin(name, version string) error {
	key := pluginKey(name, version)

	plugin, found := c.lru.Get(key)
	if !found {
		return fmt.Errorf("plugin not found in cache: %s:%s", name, version)
	}

	if err := c.store.RemovePlugin(plugin.FilePath); err != nil {
		return fmt.Errorf("failed to delete plugin file: %w", err)
	}

	if err := c.store.RemoveMetadata(plugin.Name, plugin.Version); err != nil {
		slog.Warn("Failed to delete metadata during invalidation", "error", err)
	}
	c.lru.Remove(key)

	slog.Info("Plugin invalidated", "plugin", name, "version", version)
	return nil
}

// GetStats returns current cache statistics
func (c *CacheService) GetStats() (*admin.CacheStats, error) {
	currentSize, err := c.store.GetCurrentSize()
	if err != nil {
		return nil, fmt.Errorf("failed to get storage size: %w", err)
	}

	utilization := 0.0
	if c.config.Storage.LimitBytes > 0 {
		utilization = float64(currentSize) / float64(c.config.Storage.LimitBytes) * 100
	}

	return &admin.CacheStats{
		TotalPlugins:       c.lru.Len(),
		TotalSizeBytes:     currentSize,
		LimitBytes:         c.config.Storage.LimitBytes,
		UtilizationPercent: utilization,
	}, nil
}

// IsHealthy checks if the cache service is operational
func (c *CacheService) IsHealthy() map[string]string {
	health := map[string]string{
		"storage":  "ok",
		"upstream": "ok",
	}

	if err := c.store.CheckHealth(); err != nil {
		health["storage"] = "error: " + err.Error()
	}

	return health
}

// ensureSpace ensures there's enough space for a new plugin by evicting if necessary
func (c *CacheService) ensureSpace(requiredBytes int64) error {
	currentSize, err := c.store.GetCurrentSize()
	if err != nil {
		return fmt.Errorf("failed to get current storage size: %w", err)
	}

	for currentSize+requiredBytes > c.config.Storage.LimitBytes {
		oldest, found := c.lru.GetOldest()
		if !found {
			return fmt.Errorf("storage full but no plugins to evict")
		}

		slog.Info("Evicting plugin due to storage limit",
			"plugin", oldest.Name,
			"version", oldest.Version,
			"size_bytes", oldest.FileSize,
		)

		if err := c.store.RemovePlugin(oldest.FilePath); err != nil {
			return fmt.Errorf("failed to evict plugin file %s: %w", oldest.FilePath, err)
		}

		if err := c.store.RemoveMetadata(oldest.Name, oldest.Version); err != nil {
			slog.Warn("Failed to delete metadata during eviction", "error", err)
		}
		c.lru.Remove(oldest.Key())
		metrics.RecordEviction("storage_limit")

		// Re-read actual disk usage to keep accounting accurate
		currentSize, err = c.store.GetCurrentSize()
		if err != nil {
			return fmt.Errorf("failed to recalculate storage size after eviction: %w", err)
		}
	}

	c.updateStorageMetrics()
	return nil
}
