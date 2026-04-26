package unit

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yourorg/jenkins-larder/src/config"
	"github.com/yourorg/jenkins-larder/src/server"
	"github.com/yourorg/jenkins-larder/src/storage"
)

// --- Mock implementations ---

type mockStore struct {
	pluginPathFn        func(name, version, ext string) string
	getCurrentSizeFn    func() (int64, error)
	writePluginFn       func(body io.Reader, name, version, ext string) (string, int64, string, error)
	removePluginFn      func(filePath string) error
	removeMetadataFn    func(name, version string) error
	openPluginFn        func(filePath string) (io.ReadCloser, error)
	pluginExistsFn      func(filePath string) (os.FileInfo, error)
	checkHealthFn       func() error
	savePluginMetaFn    func(plugin *storage.CachedPlugin) error
	loadPluginMetaFn    func(name, version string) (*storage.CachedPlugin, error)
}

func (m *mockStore) PluginPath(name, version, ext string) string {
	if m.pluginPathFn != nil {
		return m.pluginPathFn(name, version, ext)
	}
	return "/mock/plugins/" + name + "/" + version + "/" + name + "." + ext
}
func (m *mockStore) GetCurrentSize() (int64, error) {
	if m.getCurrentSizeFn != nil {
		return m.getCurrentSizeFn()
	}
	return 0, nil
}
func (m *mockStore) WritePlugin(body io.Reader, name, version, ext string) (string, int64, string, error) {
	if m.writePluginFn != nil {
		return m.writePluginFn(body, name, version, ext)
	}
	return "/mock/plugins/" + name + "/" + version + "/" + name + "." + ext, 100, "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", nil
}
func (m *mockStore) RemovePlugin(filePath string) error {
	if m.removePluginFn != nil {
		return m.removePluginFn(filePath)
	}
	return nil
}
func (m *mockStore) RemoveMetadata(name, version string) error {
	if m.removeMetadataFn != nil {
		return m.removeMetadataFn(name, version)
	}
	return nil
}
func (m *mockStore) OpenPlugin(filePath string) (io.ReadCloser, error) {
	if m.openPluginFn != nil {
		return m.openPluginFn(filePath)
	}
	return io.NopCloser(strings.NewReader("plugin-data")), nil
}
func (m *mockStore) PluginExists(filePath string) (os.FileInfo, error) {
	if m.pluginExistsFn != nil {
		return m.pluginExistsFn(filePath)
	}
	return nil, os.ErrNotExist
}
func (m *mockStore) CheckHealth() error {
	if m.checkHealthFn != nil {
		return m.checkHealthFn()
	}
	return nil
}
func (m *mockStore) SavePluginMetadata(plugin *storage.CachedPlugin) error {
	if m.savePluginMetaFn != nil {
		return m.savePluginMetaFn(plugin)
	}
	return nil
}
func (m *mockStore) LoadPluginMetadata(name, version string) (*storage.CachedPlugin, error) {
	if m.loadPluginMetaFn != nil {
		return m.loadPluginMetaFn(name, version)
	}
	return nil, errors.New("not found")
}

type mockLRU struct {
	plugins map[string]*storage.CachedPlugin
	order   []string // insertion order for GetOldest
}

func newMockLRU() *mockLRU {
	return &mockLRU{plugins: make(map[string]*storage.CachedPlugin)}
}
func (m *mockLRU) Add(p *storage.CachedPlugin) {
	key := p.Key()
	if _, exists := m.plugins[key]; !exists {
		m.order = append(m.order, key)
	}
	m.plugins[key] = p
}
func (m *mockLRU) Get(key string) (*storage.CachedPlugin, bool) {
	p, ok := m.plugins[key]
	return p, ok
}
func (m *mockLRU) GetOldest() (*storage.CachedPlugin, bool) {
	if len(m.order) == 0 {
		return nil, false
	}
	return m.plugins[m.order[0]], true
}
func (m *mockLRU) Remove(key string) {
	delete(m.plugins, key)
	for i, k := range m.order {
		if k == key {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
}
func (m *mockLRU) Len() int { return len(m.plugins) }

type mockUpstream struct {
	downloadFn func(ctx context.Context, name, version, ext string) (io.ReadCloser, error)
	pluginURLFn func(name, version, ext string) string
}

func (m *mockUpstream) DownloadPlugin(ctx context.Context, name, version, ext string) (io.ReadCloser, error) {
	if m.downloadFn != nil {
		return m.downloadFn(ctx, name, version, ext)
	}
	return io.NopCloser(strings.NewReader("upstream-data")), nil
}
func (m *mockUpstream) PluginURL(name, version, ext string) string {
	if m.pluginURLFn != nil {
		return m.pluginURLFn(name, version, ext)
	}
	return "https://upstream/" + name + "/" + version + "/" + name + "." + ext
}

type mockDedup struct{}

func (m *mockDedup) Do(ctx context.Context, key string, fn func() (interface{}, error)) (interface{}, error) {
	return fn()
}

func newTestCacheService(store *mockStore, lru *mockLRU, up *mockUpstream) *server.CacheService {
	cfg := &config.Config{
		Storage: config.StorageConfig{LimitBytes: 1000, Path: "/mock"},
	}
	return server.NewCacheServiceWithDeps(cfg, store, lru, up, &mockDedup{})
}

// --- Tests ---

func TestGetPlugin_CacheHit(t *testing.T) {
	store := &mockStore{
		pluginExistsFn: func(filePath string) (os.FileInfo, error) {
			return nil, nil // file exists
		},
	}
	lru := newMockLRU()
	lru.Add(&storage.CachedPlugin{
		Name: "git", Version: "4.11.0", Extension: "hpi",
		FilePath: "/mock/plugins/git/4.11.0/git.hpi",
		FileSize: 100,
	})

	cs := newTestCacheService(store, lru, &mockUpstream{})

	plugin, file, err := cs.GetPlugin(context.Background(), "git", "4.11.0", "hpi")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if plugin.Name != "git" {
		t.Errorf("Name = %q, want git", plugin.Name)
	}
}

func TestGetPlugin_CacheHit_FileDeleted(t *testing.T) {
	store := &mockStore{
		pluginExistsFn: func(filePath string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		},
	}
	lru := newMockLRU()
	lru.Add(&storage.CachedPlugin{
		Name: "git", Version: "4.11.0", Extension: "hpi",
		FilePath: "/mock/plugins/git/4.11.0/git.hpi",
	})
	up := &mockUpstream{}

	cs := newTestCacheService(store, lru, up)

	plugin, file, err := cs.GetPlugin(context.Background(), "git", "4.11.0", "hpi")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	// Should have re-downloaded
	if plugin.Name != "git" {
		t.Errorf("Name = %q, want git", plugin.Name)
	}
	// LRU should have the new entry
	if lru.Len() != 1 {
		t.Errorf("LRU Len = %d, want 1", lru.Len())
	}
}

func TestGetPlugin_CacheMiss_DownloadSuccess(t *testing.T) {
	store := &mockStore{}
	lru := newMockLRU()
	up := &mockUpstream{}

	cs := newTestCacheService(store, lru, up)

	plugin, file, err := cs.GetPlugin(context.Background(), "git", "4.11.0", "hpi")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if plugin.Name != "git" {
		t.Errorf("Name = %q, want git", plugin.Name)
	}
	if lru.Len() != 1 {
		t.Errorf("LRU Len = %d, want 1", lru.Len())
	}
}

func TestGetPlugin_UpstreamFails_StaleAvailable(t *testing.T) {
	store := &mockStore{
		pluginExistsFn: func(filePath string) (os.FileInfo, error) {
			return fakeFileInfo{size: 50}, nil
		},
		openPluginFn: func(filePath string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("stale-data")), nil
		},
	}
	lru := newMockLRU()
	up := &mockUpstream{
		downloadFn: func(ctx context.Context, name, version, ext string) (io.ReadCloser, error) {
			return nil, errors.New("upstream down")
		},
	}

	cs := newTestCacheService(store, lru, up)

	plugin, file, err := cs.GetPlugin(context.Background(), "git", "4.11.0", "hpi")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if plugin.FileSize != 50 {
		t.Errorf("FileSize = %d, want 50 (stale)", plugin.FileSize)
	}
}

func TestGetPlugin_UpstreamFails_NoStale(t *testing.T) {
	store := &mockStore{} // PluginExists returns ErrNotExist by default
	lru := newMockLRU()
	up := &mockUpstream{
		downloadFn: func(ctx context.Context, name, version, ext string) (io.ReadCloser, error) {
			return nil, errors.New("upstream down")
		},
	}

	cs := newTestCacheService(store, lru, up)

	_, _, err := cs.GetPlugin(context.Background(), "git", "4.11.0", "hpi")
	if err == nil {
		t.Fatal("expected error when upstream down and no stale")
	}
}

func TestDownloadAndCache_WritePluginError(t *testing.T) {
	store := &mockStore{
		writePluginFn: func(body io.Reader, name, version, ext string) (string, int64, string, error) {
			return "", 0, "", errors.New("disk full")
		},
	}
	lru := newMockLRU()
	up := &mockUpstream{}

	cs := newTestCacheService(store, lru, up)

	_, _, err := cs.GetPlugin(context.Background(), "git", "4.11.0", "hpi")
	if err == nil {
		t.Fatal("expected error on WritePlugin failure")
	}
	if lru.Len() != 0 {
		t.Error("plugin should not be in LRU after write failure")
	}
}

func TestDownloadAndCache_EnsureSpaceError(t *testing.T) {
	callCount := 0
	store := &mockStore{
		getCurrentSizeFn: func() (int64, error) {
			callCount++
			if callCount <= 1 {
				return 0, nil // initial metrics call
			}
			return 999, nil // over limit for ensureSpace
		},
	}
	lru := newMockLRU()
	up := &mockUpstream{}

	cfg := &config.Config{
		Storage: config.StorageConfig{LimitBytes: 50, Path: "/mock"},
	}
	cs := server.NewCacheServiceWithDeps(cfg, store, lru, up, &mockDedup{})

	_, _, err := cs.GetPlugin(context.Background(), "git", "4.11.0", "hpi")
	if err == nil {
		t.Fatal("expected error when storage full")
	}
}

func TestEnsureSpace_EvictsOldest(t *testing.T) {
	evictionHappened := false
	store := &mockStore{
		getCurrentSizeFn: func() (int64, error) {
			if evictionHappened {
				return 0, nil // after eviction, plenty of space
			}
			return 950, nil // 950 + 100 > 1000 limit
		},
		removePluginFn: func(filePath string) error {
			evictionHappened = true
			return nil
		},
	}
	lru := newMockLRU()
	lru.Add(&storage.CachedPlugin{
		Name: "old", Version: "1.0", FilePath: "/mock/old.hpi", FileSize: 500,
		LastAccessTime: time.Now().Add(-1 * time.Hour),
	})
	up := &mockUpstream{}

	cfg := &config.Config{
		Storage: config.StorageConfig{LimitBytes: 1000, Path: "/mock"},
	}
	cs := server.NewCacheServiceWithDeps(cfg, store, lru, up, &mockDedup{})

	// Download triggers ensureSpace with 100 bytes needed + 900 current = over 1000 limit
	plugin, file, err := cs.GetPlugin(context.Background(), "new", "1.0", "hpi")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if !evictionHappened {
		t.Error("expected eviction to happen")
	}
	if plugin.Name != "new" {
		t.Errorf("Name = %q, want new", plugin.Name)
	}
}

func TestEnsureSpace_RemovePluginError(t *testing.T) {
	store := &mockStore{
		getCurrentSizeFn: func() (int64, error) {
			return 950, nil // 950 + 100 > 1000
		},
		removePluginFn: func(filePath string) error {
			return errors.New("permission denied")
		},
	}
	lru := newMockLRU()
	lru.Add(&storage.CachedPlugin{
		Name: "old", Version: "1.0", FilePath: "/mock/old.hpi", FileSize: 500,
	})
	up := &mockUpstream{}

	cfg := &config.Config{
		Storage: config.StorageConfig{LimitBytes: 1000, Path: "/mock"},
	}
	cs := server.NewCacheServiceWithDeps(cfg, store, lru, up, &mockDedup{})

	_, _, err := cs.GetPlugin(context.Background(), "new", "1.0", "hpi")
	if err == nil {
		t.Fatal("expected error when eviction fails")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("error = %v, want permission denied", err)
	}
}

func TestEnsureSpace_NoPluginsToEvict(t *testing.T) {
	store := &mockStore{
		getCurrentSizeFn: func() (int64, error) {
			return 950, nil // 950 + 100 > 1000
		},
	}
	lru := newMockLRU() // empty
	up := &mockUpstream{}

	cfg := &config.Config{
		Storage: config.StorageConfig{LimitBytes: 1000, Path: "/mock"},
	}
	cs := server.NewCacheServiceWithDeps(cfg, store, lru, up, &mockDedup{})

	_, _, err := cs.GetPlugin(context.Background(), "new", "1.0", "hpi")
	if err == nil {
		t.Fatal("expected error when no plugins to evict")
	}
}

func TestInvalidatePlugin_Success(t *testing.T) {
	removed := false
	store := &mockStore{
		removePluginFn: func(filePath string) error {
			removed = true
			return nil
		},
	}
	lru := newMockLRU()
	lru.Add(&storage.CachedPlugin{
		Name: "git", Version: "4.11.0", FilePath: "/mock/git.hpi",
	})

	cs := newTestCacheService(store, lru, &mockUpstream{})

	if err := cs.InvalidatePlugin("git", "4.11.0"); err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Error("expected plugin file to be removed")
	}
	if lru.Len() != 0 {
		t.Error("expected LRU to be empty after invalidation")
	}
}

func TestInvalidatePlugin_NotFound(t *testing.T) {
	cs := newTestCacheService(&mockStore{}, newMockLRU(), &mockUpstream{})

	err := cs.InvalidatePlugin("nonexistent", "1.0")
	if err == nil {
		t.Fatal("expected error for nonexistent plugin")
	}
}

func TestIsHealthy_StorageOk(t *testing.T) {
	store := &mockStore{
		checkHealthFn: func() error { return nil },
	}
	cs := newTestCacheService(store, newMockLRU(), &mockUpstream{})

	health := cs.IsHealthy()
	if health["storage"] != "ok" {
		t.Errorf("storage = %q, want ok", health["storage"])
	}
}

func TestIsHealthy_StorageFails(t *testing.T) {
	store := &mockStore{
		checkHealthFn: func() error { return errors.New("disk error") },
	}
	cs := newTestCacheService(store, newMockLRU(), &mockUpstream{})

	health := cs.IsHealthy()
	if !strings.Contains(health["storage"], "disk error") {
		t.Errorf("storage = %q, want error containing 'disk error'", health["storage"])
	}
}

func TestGetStats(t *testing.T) {
	store := &mockStore{
		getCurrentSizeFn: func() (int64, error) { return 500, nil },
	}
	lru := newMockLRU()
	lru.Add(&storage.CachedPlugin{Name: "a", Version: "1.0"})
	lru.Add(&storage.CachedPlugin{Name: "b", Version: "1.0"})

	cs := newTestCacheService(store, lru, &mockUpstream{})

	stats, err := cs.GetStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalPlugins != 2 {
		t.Errorf("TotalPlugins = %d, want 2", stats.TotalPlugins)
	}
	if stats.TotalSizeBytes != 500 {
		t.Errorf("TotalSizeBytes = %d, want 500", stats.TotalSizeBytes)
	}
	if stats.UtilizationPercent != 50.0 {
		t.Errorf("Utilization = %f, want 50.0", stats.UtilizationPercent)
	}
}

func TestGetStats_StorageError(t *testing.T) {
	store := &mockStore{
		getCurrentSizeFn: func() (int64, error) { return 0, errors.New("storage error") },
	}
	cs := newTestCacheService(store, newMockLRU(), &mockUpstream{})

	_, err := cs.GetStats()
	if err == nil {
		t.Fatal("expected error")
	}
}

// fakeFileInfo implements os.FileInfo for testing
type fakeFileInfo struct {
	size int64
}

func (f fakeFileInfo) Name() string      { return "fake" }
func (f fakeFileInfo) Size() int64       { return f.size }
func (f fakeFileInfo) Mode() os.FileMode { return 0644 }
func (f fakeFileInfo) ModTime() time.Time { return time.Now() }
func (f fakeFileInfo) IsDir() bool       { return false }
func (f fakeFileInfo) Sys() interface{}  { return nil }
