package storage

import (
	"fmt"
	"io"
	"log/slog"
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

// WritePlugin writes plugin data from a reader to disk atomically (tmp + rename).
// Returns the final file path, bytes written, and SHA-256 checksum.
func (s *Storage) WritePlugin(body io.Reader, name, version, extension string) (string, int64, string, error) {
	filePath := s.PluginPath(name, version, extension)

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return "", 0, "", fmt.Errorf("failed to create plugin directory: %w", err)
	}

	tmpPath := filePath + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return "", 0, "", fmt.Errorf("failed to create temp file: %w", err)
	}

	written, err := io.Copy(tmpFile, body)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", 0, "", fmt.Errorf("failed to write plugin: %w", err)
	}
	tmpFile.Close()

	checksum, err := CalculateSHA256(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return "", 0, "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return "", 0, "", fmt.Errorf("failed to move plugin to final location: %w", err)
	}

	return filePath, written, checksum, nil
}

// RemovePlugin removes a plugin file from disk. Returns nil if the file does not exist.
func (s *Storage) RemovePlugin(filePath string) error {
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plugin file: %w", err)
	}
	return nil
}

// RemoveMetadata removes a plugin's metadata JSON file. Best-effort; logs warning on failure.
func (s *Storage) RemoveMetadata(name, version string) error {
	metadataPath := filepath.Join(s.BaseDir, "metadata", name, version+".json")
	if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
		slog.Warn("Failed to delete metadata", "path", metadataPath, "error", err)
	}
	return nil
}

// OpenPlugin opens a cached plugin file for reading.
func (s *Storage) OpenPlugin(filePath string) (io.ReadCloser, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin file: %w", err)
	}
	return file, nil
}

// PluginExists checks whether a plugin file exists on disk and returns its FileInfo.
func (s *Storage) PluginExists(filePath string) (os.FileInfo, error) {
	return os.Stat(filePath)
}

// CheckHealth verifies the storage directory is writable by writing and removing a probe file.
func (s *Storage) CheckHealth() error {
	testPath := filepath.Join(s.BaseDir, ".health-check")
	if err := os.WriteFile(testPath, []byte("ok"), 0644); err != nil {
		return err
	}
	os.Remove(testPath)
	return nil
}

// SavePluginMetadata writes plugin metadata to a JSON file.
func (s *Storage) SavePluginMetadata(plugin *CachedPlugin) error {
	return SaveMetadata(plugin, s.BaseDir)
}

// LoadPluginMetadata reads plugin metadata from a JSON file.
func (s *Storage) LoadPluginMetadata(name, version string) (*CachedPlugin, error) {
	return LoadMetadata(name, version, s.BaseDir)
}
