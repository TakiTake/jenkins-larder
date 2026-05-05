# Jenkins Larder 🏰

**A well-stocked larder for your Jenkins plugins**

A high-performance caching proxy that stores Jenkins plugins locally, reducing backup size and shortening initialization time. Like a butler's larder keeps provisions ready, Jenkins Larder keeps your plugins ready to serve.

## Features

- **Plugin Caching**: Mirrors Jenkins plugin downloads with LRU eviction
- **Storage Management**: Configurable storage limits with automatic eviction
- **Cache Invalidation**: Admin API for manual cache management
- **Observability**: Prometheus metrics for monitoring cache performance
- **High Performance**: Request deduplication and streaming responses

## Architecture

Jenkins Larder exposes two kinds of endpoints on the same port:

```
GET /update-center.json[?version=<jenkins-version>]
GET /download/plugins/{name}/{version}/{file}
```

### Cache Policy

**Plugin files** are cached by plugin name and version only:

```
plugins/git/4.11.0/git.hpi   ← single copy regardless of Jenkins version
```

A plugin binary for a given version is identical no matter which Jenkins instance requests it, so one cached copy serves all consumers.

**`update-center.json`** is cached per Jenkins version:

```
ucCache[""]        ← no ?version= param (latest)
ucCache["2.492.3"] ← ?version=2.492.3
ucCache["2.491.0"] ← ?version=2.491.0
```

Older Jenkins instances append `?version=<jenkins-version>` when fetching the update center. The upstream redirects this to a version-specific catalog (e.g. `/dynamic-stable-2.492.3/update-center.json`) whose plugin list differs from the latest. Larder forwards the parameter, caches the response under that version key, and re-signs the JSON with its own RSA key so Jenkins accepts it.

Each cache entry has an independent TTL (default 1 hour). Plugin download URLs inside the JSON are rewritten from `updates.jenkins.io` to Larder's own address before signing.

### Request Flow

When Jenkins requests a plugin:
1. Checks local provisions (cache)
2. Serves from stock if available (cache hit)
3. Fetches from upstream if not stocked (cache miss)
4. Applies LRU eviction when storage is full

## Quick Start

### Prerequisites

- Go 1.21+
- Kubernetes cluster (for deployment)

### Local Development

```bash
# Build
make build

# Run
make run

# Or manually
go build -o bin/larder ./cmd/larder
CONFIG_PATH=config/default.yaml ./bin/larder
```

### Kubernetes Deployment

See [quickstart guide](specs/001-jenkins-plugin-cache/quickstart.md) for complete deployment instructions.

```bash
# Apply Kubernetes manifests
kubectl apply -f deployments/kubernetes/
```

### Configure Jenkins

Set the update center download URL:
```bash
export JENKINS_UC_DOWNLOAD=http://jenkins-larder.default.svc.cluster.local:8080
```

## Configuration

Edit `config/default.yaml`:

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
```

## API Endpoints

### Plugin Download
```
GET /download/plugins/{name}/{version}/{file}
```

### Admin API
```
POST /admin/cache/invalidate    # Invalidate cache
GET  /admin/cache/stats         # Cache statistics
GET  /admin/health              # Health check
```

### Metrics
```
GET /metrics                    # Prometheus metrics
```

Key metrics:
- `jenkins_larder_cache_hits_total` / `jenkins_larder_cache_misses_total` — Hit rate
- `jenkins_larder_storage_usage_bytes` / `jenkins_larder_storage_limit_bytes` — Storage utilization
- `jenkins_larder_download_duration_seconds` — Latency histogram by source (cache/upstream)
- `jenkins_larder_bandwidth_saved_bytes_total` — Bandwidth saved by cache hits
- `jenkins_larder_evictions_total` — Eviction count by reason
- `jenkins_larder_cached_plugins_total` — Current number of cached plugins

## Development

### Project Structure

```
jenkins-larder/
├── cmd/larder/          # Application entry point
├── src/
│   ├── config/         # Configuration loading
│   ├── storage/        # Cache storage and LRU
│   ├── upstream/       # Upstream client
│   ├── server/         # HTTP handlers
│   ├── admin/          # Admin API
│   └── metrics/        # Prometheus metrics
├── tests/              # Test suites
├── config/             # Default configuration
└── deployments/        # Kubernetes manifests
```

### Testing

```bash
make test              # All tests
make test-unit         # Unit tests only
make test-integration  # Integration tests (requires -tags=integration)
make test-contract     # Contract tests
make lint              # golangci-lint
make fmt               # go fmt
```

## Implementation Tasks

See [tasks.md](specs/001-jenkins-plugin-cache/tasks.md) for the complete task breakdown (76 tasks across 6 phases).

## Documentation

- [Feature Specification](specs/001-jenkins-plugin-cache/spec.md)
- [Implementation Plan](specs/001-jenkins-plugin-cache/plan.md)
- [API Contracts](specs/001-jenkins-plugin-cache/contracts/mirror-api.yaml)
- [Deployment Guide](specs/001-jenkins-plugin-cache/quickstart.md)
- [Project Constitution](.specify/memory/constitution.md)

## License

TODO: Add license information
