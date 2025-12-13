package storage

import "time"

// CachedPlugin represents a cached Jenkins plugin file with metadata
// TODO (T011): Implement complete CachedPlugin struct with all metadata fields
type CachedPlugin struct {
	Name               string    `json:"name"`
	Version            string    `json:"version"`
	Extension          string    `json:"extension"`           // "hpi" or "jpi"
	FilePath           string    `json:"file_path"`           // Absolute path to cached file
	FileSize           int64     `json:"file_size"`           // File size in bytes
	ChecksumSHA256     string    `json:"checksum_sha256"`     // SHA-256 hash
	DownloadTimestamp  time.Time `json:"download_timestamp"`  // When first cached
	LastAccessTime     time.Time `json:"last_access_time"`    // Most recent access (for LRU)
	UpstreamURL        string    `json:"upstream_url"`        // Original Jenkins update center URL
}

// Key returns a unique key for this plugin version
func (p *CachedPlugin) Key() string {
	return p.Name + ":" + p.Version
}

// UpdateAccessTime updates the last access time to now (for LRU tracking)
// TODO (T033): Implement access time update logic
func (p *CachedPlugin) UpdateAccessTime() {
	p.LastAccessTime = time.Now()
}

// IsStale checks if the plugin exceeds configured TTL
// TODO (T046): Implement TTL checking logic
func (p *CachedPlugin) IsStale(ttlHours int) bool {
	if ttlHours == 0 {
		return false // No TTL, never stale
	}
	// TODO: Check if age exceeds TTL
	return false
}
