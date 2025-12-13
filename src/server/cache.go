package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/yourorg/jenkins-larder/src/config"
	"github.com/yourorg/jenkins-larder/src/storage"
	"github.com/yourorg/jenkins-larder/src/upstream"
)

// CacheService coordinates storage, LRU tracking, and upstream downloads
type CacheService struct {
	storage      *storage.Storage
	lru          *storage.LRUTracker
	upstream     *upstream.Client
	dedup        *DeduplicationManager
	config       *config.Config
}

// NewCacheService creates a new cache service
func NewCacheService(cfg *config.Config) (*CacheService, error) {
	// Initialize storage
	stor, err := storage.NewStorage(cfg.Storage.Path, cfg.Storage.LimitBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize LRU tracker (use 10000 as max items)
	lruTracker, err := storage.NewLRUTracker(10000)
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

	// Initialize upstream client
	upstreamClient := upstream.NewClient(cfg.Upstream.URL, cfg.Upstream.TimeoutSeconds)

	// Initialize deduplication manager
	dedupManager := NewDeduplicationManager()

	return &CacheService{
		storage:  stor,
		lru:      lruTracker,
		upstream: upstreamClient,
		dedup:    dedupManager,
		config:   cfg,
	}, nil
}

// GetPlugin retrieves a plugin, downloading from upstream if necessary
func (c *CacheService) GetPlugin(ctx context.Context, name, version, extension string) (*storage.CachedPlugin, io.ReadCloser, error) {
	key := fmt.Sprintf("%s:%s", name, version)

	// Check LRU cache first
	if plugin, found := c.lru.Get(key); found {
		// Verify file still exists on disk
		if _, err := os.Stat(plugin.FilePath); err == nil {
			// Update access time
			plugin.UpdateAccessTime()
			c.lru.Add(plugin)

			// Save updated metadata
			storage.SaveMetadata(plugin, c.config.Storage.Path)

			// Open file for reading
			file, err := os.Open(plugin.FilePath)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to open cached plugin: %w", err)
			}

			slog.Info("Cache HIT", "plugin", name, "version", version)
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
		return nil, nil, err
	}

	plugin := result.(*storage.CachedPlugin)

	// Open file for reading
	file, err := os.Open(plugin.FilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open downloaded plugin: %w", err)
	}

	return plugin, file, nil
}

// downloadAndCache downloads a plugin from upstream and caches it
func (c *CacheService) downloadAndCache(ctx context.Context, name, version, extension string) (*storage.CachedPlugin, error) {
	slog.Info("Cache MISS - downloading from upstream", "plugin", name, "version", version)

	// Download from upstream
	body, err := c.upstream.DownloadPlugin(ctx, name, version, extension)
	if err != nil {
		return nil, fmt.Errorf("failed to download from upstream: %w", err)
	}
	defer body.Close()

	// Prepare plugin metadata
	plugin := &storage.CachedPlugin{
		Name:              name,
		Version:           version,
		Extension:         extension,
		DownloadTimestamp: time.Now(),
		LastAccessTime:    time.Now(),
		UpstreamURL:       c.upstream.PluginURL(name, version, extension),
	}

	// Calculate file path
	plugin.FilePath = c.storage.PluginPath(name, version, extension)

	// Create directory
	if err := os.MkdirAll(filepath.Dir(plugin.FilePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Write to temporary file first
	tmpPath := plugin.FilePath + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Copy data and calculate size
	written, err := io.Copy(tmpFile, body)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to write plugin: %w", err)
	}
	tmpFile.Close()

	plugin.FileSize = written

	// Calculate checksum
	checksum, err := storage.CalculateSHA256(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}
	plugin.ChecksumSHA256 = checksum

	// Ensure we have space (evict if needed)
	if err := c.ensureSpace(plugin.FileSize); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to ensure storage space: %w", err)
	}

	// Move temp file to final location
	if err := os.Rename(tmpPath, plugin.FilePath); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to move plugin to final location: %w", err)
	}

	// Save metadata
	if err := storage.SaveMetadata(plugin, c.config.Storage.Path); err != nil {
		slog.Warn("Failed to save metadata", "error", err)
	}

	// Add to LRU tracker
	c.lru.Add(plugin)

	slog.Info("Plugin cached successfully",
		"plugin", name,
		"version", version,
		"size_bytes", plugin.FileSize,
		"checksum", plugin.ChecksumSHA256[:16]+"...",
	)

	return plugin, nil
}

// ensureSpace ensures there's enough space for a new plugin by evicting if necessary
func (c *CacheService) ensureSpace(requiredBytes int64) error {
	// Get current storage size
	currentSize, err := c.storage.GetCurrentSize()
	if err != nil {
		return fmt.Errorf("failed to get current storage size: %w", err)
	}

	// Check if we need to evict
	for currentSize+requiredBytes > c.config.Storage.LimitBytes {
		// Get oldest plugin
		oldest, found := c.lru.GetOldest()
		if !found {
			return fmt.Errorf("storage full but no plugins to evict")
		}

		// Evict the plugin
		slog.Info("Evicting plugin due to storage limit",
			"plugin", oldest.Name,
			"version", oldest.Version,
			"size_bytes", oldest.FileSize,
		)

		// Delete file
		if err := os.Remove(oldest.FilePath); err != nil && !os.IsNotExist(err) {
			slog.Warn("Failed to delete plugin file", "error", err)
		}

		// Delete metadata
		metadataPath := filepath.Join(c.config.Storage.Path, "metadata", oldest.Name, oldest.Version+".json")
		if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("Failed to delete metadata", "error", err)
		}

		// Remove from LRU
		c.lru.Remove(oldest.Key())

		// Recalculate current size
		currentSize -= oldest.FileSize
	}

	return nil
}
