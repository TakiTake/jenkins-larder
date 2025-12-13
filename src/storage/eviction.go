package storage

import (
	"fmt"
	"log/slog"
	"os"
)

// TODO (T031): Implement LRU eviction when approaching storage limit

// Evict removes the least recently used plugin from storage
func (s *Storage) Evict(lruTracker *LRUTracker) (*CachedPlugin, error) {
	// Get oldest plugin from LRU tracker
	oldest, found := lruTracker.GetOldest()
	if !found {
		return nil, fmt.Errorf("no plugins to evict")
	}

	slog.Info("Evicting plugin",
		"name", oldest.Name,
		"version", oldest.Version,
		"size_bytes", oldest.FileSize,
		"last_access", oldest.LastAccessTime,
	)

	// TODO: Delete plugin file
	if err := os.Remove(oldest.FilePath); err != nil {
		return nil, fmt.Errorf("failed to delete plugin file: %w", err)
	}

	// TODO: Delete metadata file
	metadataPath := MetadataPath(oldest, s.BaseDir)
	if err := os.Remove(metadataPath); err != nil {
		slog.Warn("Failed to delete metadata file",
			"path", metadataPath,
			"error", err,
		)
	}

	// Remove from LRU tracker
	lruTracker.Remove(oldest.Key())

	return oldest, nil
}

// EvictUntilSpace evicts plugins until required space is available
func (s *Storage) EvictUntilSpace(requiredBytes int64, lruTracker *LRUTracker) error {
	for {
		currentSize, err := s.GetCurrentSize()
		if err != nil {
			return fmt.Errorf("failed to get current size: %w", err)
		}

		available := s.LimitBytes - currentSize
		if available >= requiredBytes {
			return nil // Enough space available
		}

		// Need to evict
		evicted, err := s.Evict(lruTracker)
		if err != nil {
			return fmt.Errorf("failed to evict plugin: %w", err)
		}

		slog.Info("Evicted plugin to free space",
			"name", evicted.Name,
			"version", evicted.Version,
			"freed_bytes", evicted.FileSize,
		)
	}
}
