package storage

import (
	"fmt"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// TODO (T029): Implement LRU tracking for cache eviction

// LRUTracker manages LRU tracking for cached plugins
type LRUTracker struct {
	cache *lru.Cache[string, *CachedPlugin]
	mu    sync.RWMutex
}

// NewLRUTracker creates a new LRU tracker
func NewLRUTracker(size int) (*LRUTracker, error) {
	cache, err := lru.New[string, *CachedPlugin](size)
	if err != nil {
		return nil, fmt.Errorf("failed to create LRU cache: %w", err)
	}

	return &LRUTracker{
		cache: cache,
	}, nil
}

// Add adds or updates a plugin in the LRU tracker
func (t *LRUTracker) Add(plugin *CachedPlugin) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cache.Add(plugin.Key(), plugin)
}

// Get retrieves a plugin and marks it as recently used
func (t *LRUTracker) Get(key string) (*CachedPlugin, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.cache.Get(key)
}

// GetOldest returns the least recently used plugin
func (t *LRUTracker) GetOldest() (*CachedPlugin, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Get all keys and find the oldest by access time
	var oldest *CachedPlugin
	var oldestTime time.Time

	keys := t.cache.Keys()
	if len(keys) == 0 {
		return nil, false
	}

	for _, key := range keys {
		if plugin, ok := t.cache.Peek(key); ok {
			if oldest == nil || plugin.LastAccessTime.Before(oldestTime) {
				oldest = plugin
				oldestTime = plugin.LastAccessTime
			}
		}
	}

	return oldest, oldest != nil
}

// Remove removes a plugin from the LRU tracker
func (t *LRUTracker) Remove(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cache.Remove(key)
}

// Len returns the number of tracked plugins
func (t *LRUTracker) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.cache.Len()
}
