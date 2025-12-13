package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// TODO (T014): Implement storage initialization

// Storage manages the filesystem-based plugin cache
type Storage struct {
	BaseDir    string
	LimitBytes int64
}

// NewStorage creates a new Storage instance
func NewStorage(baseDir string, limitBytes int64) (*Storage, error) {
	s := &Storage{
		BaseDir:    baseDir,
		LimitBytes: limitBytes,
	}

	// TODO: Initialize storage directory
	if err := s.Initialize(); err != nil {
		return nil, err
	}

	return s, nil
}

// Initialize creates the storage directory structure
func (s *Storage) Initialize() error {
	// Create base directory
	if err := os.MkdirAll(s.BaseDir, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Create subdirectories
	subdirs := []string{"plugins", "metadata"}
	for _, subdir := range subdirs {
		path := filepath.Join(s.BaseDir, subdir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", subdir, err)
		}
	}

	// TODO: Validate directory is writable
	// TODO: Check available disk space

	return nil
}

// PluginPath returns the filesystem path for a plugin file
func (s *Storage) PluginPath(name, version, extension string) string {
	// plugins/{name}/{version}/{name}.{extension}
	filename := fmt.Sprintf("%s.%s", name, extension)
	return filepath.Join(s.BaseDir, "plugins", name, version, filename)
}

// GetCurrentSize returns the current total size of cached plugins
func (s *Storage) GetCurrentSize() (int64, error) {
	pluginsDir := filepath.Join(s.BaseDir, "plugins")
	var totalSize int64

	err := filepath.Walk(pluginsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to calculate storage size: %w", err)
	}

	return totalSize, nil
}

// EnsureSpace ensures there's enough space for a new plugin
// TODO (T031): Implement LRU eviction when approaching storage limit
func (s *Storage) EnsureSpace(requiredBytes int64) error {
	// TODO: Check if space available
	// TODO: If not, evict LRU plugins until space available
	return nil
}
