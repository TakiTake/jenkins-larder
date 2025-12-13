# Data Model: Jenkins Plugin Cache Mirror

**Date**: 2025-12-13
**Purpose**: Define data structures, storage schema, and relationships

---

## Entity: CachedPlugin

Represents a mirrored Jenkins plugin file stored locally.

### Attributes

| Field | Type | Required | Description | Validation Rules |
|-------|------|----------|-------------|-----------------|
| `name` | string | Yes | Plugin identifier (e.g., "git") | Alphanumeric + hyphens, max 100 chars |
| `version` | string | Yes | Semantic version (e.g., "5.0.0") | Semver pattern, max 20 chars |
| `extension` | string | Yes | File extension | Must be "hpi" or "jpi" |
| `file_path` | string | Yes | Absolute path to cached file | Within storage directory |
| `file_size` | int64 | Yes | File size in bytes | > 0 |
| `checksum_sha256` | string | Yes | SHA-256 hash of plugin file | 64 hex characters |
| `download_timestamp` | time.Time | Yes | When plugin was first cached | ISO 8601 format |
| `last_access_time` | time.Time | Yes | Most recent access (for LRU) | ISO 8601 format, updated on each serve |
| `upstream_url` | string | Yes | Original Jenkins update center URL | Valid HTTP URL |

### Storage Format

**Filesystem Layout**:
```
/var/cache/jenkins-plugins/
├── git/
│   ├── 5.0.0/
│   │   ├── git.hpi          # Actual plugin file
│   │   └── metadata.json    # CachedPlugin metadata
│   └── 5.1.0/
│       ├── git.hpi
│       └── metadata.json
├── workflow-aggregator/
│   └── 2.7/
│       ├── workflow-aggregator.hpi
│       └── metadata.json
└── .index.json              # LRU index (in-memory, persisted on shutdown)
```

**metadata.json Example**:
```json
{
  "name": "git",
  "version": "5.0.0",
  "extension": "hpi",
  "file_path": "/var/cache/jenkins-plugins/git/5.0.0/git.hpi",
  "file_size": 10485760,
  "checksum_sha256": "abc123...",
  "download_timestamp": "2025-12-13T10:30:00Z",
  "last_access_time": "2025-12-13T14:25:30Z",
  "upstream_url": "https://updates.jenkins.io/download/plugins/git/5.0.0/git.hpi"
}
```

### State Transitions

```
[Non-existent]
    ↓ (HTTP request arrives)
[Downloading]
    ↓ (download complete + checksum valid)
[Cached]
    ↓ (served to client)
[Cached, LRU updated]
    ↓ (storage limit reached, this plugin is LRU victim)
[Deleted]
```

**States**:
1. **Non-existent**: Plugin version not in cache
2. **Downloading**: Actively fetching from upstream (other requests wait)
3. **Cached**: Successfully stored with validated checksum
4. **Deleted**: Evicted by LRU or manual invalidation

### Relationships

- **One Plugin Name → Many Versions**: Each plugin can have multiple cached versions
- **LRU Index → Cached Plugins**: In-memory index tracks access times for eviction decisions

---

## Entity: MirrorRequest

Represents an incoming HTTP request for a plugin. This is an ephemeral entity (not persisted), used for logging and metrics.

### Attributes

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `request_id` | string | Yes | Unique ID for tracing (UUID) |
| `plugin_name` | string | Yes | Requested plugin name |
| `plugin_version` | string | Yes | Requested plugin version |
| `client_ip` | string | No | Jenkins instance IP (for optional rate limiting) |
| `timestamp` | time.Time | Yes | Request arrival time |
| `cache_hit` | bool | Yes | True if served from cache, false if downloaded |
| `response_time_ms` | int | Yes | Total response time in milliseconds |
| `bytes_served` | int64 | Yes | Response body size |
| `error` | string | No | Error message if request failed |

### Usage

- Logged to structured logs for debugging
- Aggregated into Prometheus metrics (hit/miss counters, latency histograms)
- Not persisted to disk (ephemeral)

---

## Entity: MirrorMetrics

Aggregated statistics exposed via Prometheus. These are derived from MirrorRequest events and CachedPlugin state.

### Prometheus Metrics

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `mirror_requests_total` | Counter | `status={hit,miss,error}` | Total requests by outcome |
| `mirror_request_duration_seconds` | Histogram | `status={hit,miss}` | Request latency distribution (p50/p95/p99) |
| `mirror_storage_bytes_used` | Gauge | - | Current storage usage in bytes |
| `mirror_storage_bytes_limit` | Gauge | - | Configured storage limit |
| `mirror_cached_plugins_total` | Gauge | - | Number of cached plugin versions |
| `mirror_bandwidth_saved_bytes_total` | Counter | - | Total bytes saved by cache hits |
| `mirror_evictions_total` | Counter | `reason={lru,manual}` | Plugin evictions count |
| `mirror_upstream_failures_total` | Counter | - | Failed upstream requests |
| `mirror_checksum_failures_total` | Counter | - | Checksum validation failures |

### Calculation Examples

- **Hit Rate**: `mirror_requests_total{status="hit"} / mirror_requests_total`
- **Bandwidth Saved**: `sum(bytes_served) where cache_hit=true`
- **Storage Usage**: `sum(file_size) for all CachedPlugin`

---

## Entity: Configuration

Application configuration loaded from YAML or environment variables.

### Attributes

| Field | Type | Default | Description | Validation |
|-------|------|---------|-------------|------------|
| `storage.limit_bytes` | int64 | 10737418240 (10GB) | Max storage size | > 0 |
| `storage.path` | string | `/var/cache/jenkins-plugins` | Storage directory | Must exist and be writable |
| `upstream.url` | string | `https://updates.jenkins.io` | Jenkins update center URL | Valid HTTP URL |
| `upstream.timeout_seconds` | int | 60 | Upstream request timeout | 10-300 |
| `server.port` | int | 8080 | HTTP server port | 1-65535 |
| `server.metrics_port` | int | 9090 | Prometheus metrics port | 1-65535 |
| `admin.port` | int | 8081 | Admin API port | 1-65535 |
| `ttl.enabled` | bool | false | Enable optional TTL policies | - |
| `ttl.default_hours` | int | 0 | Default TTL (0 = infinite) | >= 0 |

### Configuration File Example

```yaml
storage:
  limit_bytes: 10737418240  # 10GB
  path: /var/cache/jenkins-plugins

upstream:
  url: https://updates.jenkins.io
  timeout_seconds: 60

server:
  port: 8080
  metrics_port: 9090

admin:
  port: 8081

ttl:
  enabled: false
  default_hours: 0  # No automatic TTL
```

---

## Concurrency Model

### Deduplication Strategy

Use Go's `sync.Singleflight` to deduplicate concurrent requests for the same uncached plugin:

```go
// Pseudo-code
type DownloadManager struct {
    group singleflight.Group
}

func (dm *DownloadManager) GetPlugin(name, version string) ([]byte, error) {
    key := fmt.Sprintf("%s:%s", name, version)

    // Only one goroutine downloads, others wait
    result, err, shared := dm.group.Do(key, func() (interface{}, error) {
        return downloadFromUpstream(name, version)
    })

    // shared=true means this request waited for another to download
    return result.([]byte), err
}
```

### LRU Cache Thread Safety

- In-memory LRU index protected by mutex (or use thread-safe `hashicorp/golang-lru`)
- Filesystem operations (read/write plugin files) synchronized via file locks if needed
- Metadata updates atomic via temp file + rename pattern

---

## Validation Rules Summary

| Field | Rule |
|-------|------|
| Plugin Name | `^[a-zA-Z0-9-]{1,100}$` |
| Plugin Version | `^\d+\.\d+(\.\d+)?(-[a-zA-Z0-9]+)?$` (semver) |
| File Extension | Must be `hpi` or `jpi` |
| File Path | Must be within configured storage.path, no path traversal |
| Checksum | Exactly 64 hexadecimal characters (SHA-256) |
| Storage Limit | Must be positive integer, at least 1GB recommended |
| Ports | Valid range 1-65535, non-conflicting |

---

## Error Scenarios

| Scenario | Behavior | HTTP Status | Metrics Impact |
|----------|----------|-------------|----------------|
| Plugin not in cache, upstream fails | Return 502 Bad Gateway | 502 | `mirror_upstream_failures_total++` |
| Plugin not in cache, download timeout | Return 504 Gateway Timeout | 504 | `mirror_upstream_failures_total++` |
| Checksum validation fails | Delete partial download, return 500 | 500 | `mirror_checksum_failures_total++` |
| Storage full, cannot evict (all recently accessed) | Return 507 Insufficient Storage | 507 | - |
| Invalid plugin name/version | Return 400 Bad Request | 400 | `mirror_requests_total{status="error"}++` |
| Upstream returns 404 | Forward 404 to client | 404 | `mirror_requests_total{status="error"}++` |
| Stale plugin, upstream unreachable | Serve stale with warning log | 200 | `mirror_requests_total{status="hit"}++` |

---

## Data Persistence Strategy

### What is Persisted

- **Plugin Files**: Stored on disk in `/var/cache/jenkins-plugins/`
- **Metadata JSON**: One per plugin version, alongside `.hpi` file
- **LRU Index**: Persisted to `.index.json` on graceful shutdown, loaded on startup

### What is Ephemeral

- **MirrorRequest**: Logged but not stored
- **In-memory LRU**: Rebuilt from metadata files on startup if `.index.json` missing
- **Prometheus Metrics**: Prometheus scrapes and stores, mirror server resets on restart

### Backup Considerations

- Mirror storage is cache, not authoritative source (can be regenerated)
- PersistentVolume backup not critical but helpful for faster restarts
- Configuration should be in ConfigMap/version control, not just in PV
