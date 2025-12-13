package unit

import (
	"testing"
)

// TODO (T032): Write unit tests for LRU eviction

func TestLRUTrackerAdd(t *testing.T) {
	// TODO: Test adding new plugin
	// TODO: Test updating existing plugin
	t.Skip("Not implemented")
}

func TestLRUTrackerGet(t *testing.T) {
	// TODO: Test getting existing plugin
	// TODO: Test getting non-existent plugin
	// TODO: Test that Get updates access order
	t.Skip("Not implemented")
}

func TestLRUTrackerGetOldest(t *testing.T) {
	// TODO: Test getting oldest plugin
	// TODO: Test with empty tracker
	// TODO: Test LRU order is correct
	t.Skip("Not implemented")
}

func TestLRUEviction(t *testing.T) {
	// TODO: Test evicting oldest plugin
	// TODO: Test eviction updates storage size
	// TODO: Test eviction removes files
	t.Skip("Not implemented")
}

func TestEvictUntilSpace(t *testing.T) {
	// TODO: Test evicting multiple plugins until space available
	// TODO: Test with enough space (no eviction needed)
	// TODO: Test with not enough plugins to evict
	t.Skip("Not implemented")
}
