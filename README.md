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

Jenkins Larder implements the same URL pattern as Jenkins Update Center:
```
/download/plugins/{name}/{version}/{file}
```

When Jenkins requests a plugin, the larder:
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
# Unit tests
go test ./...

# Integration tests
go test -tags=integration ./tests/integration/

# Contract tests
go test ./tests/contract/
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
