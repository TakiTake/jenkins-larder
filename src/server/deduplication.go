package server

import (
	"context"

	"golang.org/x/sync/singleflight"
)

// DeduplicationManager handles concurrent request deduplication
type DeduplicationManager struct {
	group singleflight.Group
}

// NewDeduplicationManager creates a new deduplication manager
func NewDeduplicationManager() *DeduplicationManager {
	return &DeduplicationManager{}
}

// Do executes and returns the results of the given function,
// making sure that only one execution is in-flight for a given key.
// If a duplicate comes in, the duplicate caller waits for the
// original to complete and receives the same results.
func (d *DeduplicationManager) Do(ctx context.Context, key string, fn func() (interface{}, error)) (interface{}, error) {
	// TODO: Implement singleflight.Do with context support
	// TODO: Handle context cancellation

	result, err, _ := d.group.Do(key, fn)
	return result, err
}

// Forget removes a key from the deduplication manager
func (d *DeduplicationManager) Forget(key string) {
	d.group.Forget(key)
}
