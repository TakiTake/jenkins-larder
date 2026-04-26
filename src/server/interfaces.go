package server

import (
	"context"
	"io"
	"os"

	"github.com/yourorg/jenkins-larder/src/storage"
)

// PluginStore abstracts the storage layer for CacheService.
type PluginStore interface {
	PluginPath(name, version, extension string) string
	GetCurrentSize() (int64, error)
	WritePlugin(body io.Reader, name, version, ext string) (filePath string, written int64, checksum string, err error)
	RemovePlugin(filePath string) error
	RemoveMetadata(name, version string) error
	OpenPlugin(filePath string) (io.ReadCloser, error)
	PluginExists(filePath string) (os.FileInfo, error)
	CheckHealth() error
	SavePluginMetadata(plugin *storage.CachedPlugin) error
	LoadPluginMetadata(name, version string) (*storage.CachedPlugin, error)
}

// LRUCache abstracts LRU tracking for CacheService.
type LRUCache interface {
	Add(plugin *storage.CachedPlugin)
	Get(key string) (*storage.CachedPlugin, bool)
	GetOldest() (*storage.CachedPlugin, bool)
	Remove(key string)
	Len() int
}

// UpstreamClient abstracts the upstream Jenkins update center.
type UpstreamClient interface {
	DownloadPlugin(ctx context.Context, name, version, extension string) (io.ReadCloser, error)
	PluginURL(name, version, extension string) string
}

// Deduplicator abstracts concurrent request deduplication.
type Deduplicator interface {
	Do(ctx context.Context, key string, fn func() (interface{}, error)) (interface{}, error)
}
