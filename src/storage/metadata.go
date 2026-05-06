package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TODO (T012): Implement metadata JSON marshaling

// SaveMetadata writes plugin metadata to a JSON file
func SaveMetadata(plugin *CachedPlugin, storageDir string) error {
	metadataPath := MetadataPath(plugin, storageDir)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// TODO: Marshal plugin to JSON
	data, err := json.MarshalIndent(plugin, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// TODO: Write to file
	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// LoadMetadata reads plugin metadata from a JSON file
func LoadMetadata(name, version, storageDir string) (*CachedPlugin, error) {
	metadataPath := filepath.Join(storageDir, "metadata", name, version+".json")

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var plugin CachedPlugin
	if err := json.Unmarshal(data, &plugin); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &plugin, nil
}

// MetadataPath returns the path to the metadata JSON file for a plugin
func MetadataPath(plugin *CachedPlugin, storageDir string) string {
	// metadata/{name}/{version}.json
	return filepath.Join(storageDir, "metadata", plugin.Name, plugin.Version+".json")
}

// ListMetadata returns all cached plugin metadata
func ListMetadata(storageDir string) ([]*CachedPlugin, error) {
	metadataDir := filepath.Join(storageDir, "metadata")
	var plugins []*CachedPlugin

	err := filepath.Walk(metadataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".json" {
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read metadata file %s: %w", path, err)
			}

			var plugin CachedPlugin
			if err := json.Unmarshal(data, &plugin); err != nil {
				return fmt.Errorf("failed to unmarshal metadata %s: %w", path, err)
			}

			plugins = append(plugins, &plugin)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return plugins, nil
}
