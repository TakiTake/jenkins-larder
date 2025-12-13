# Research: Jenkins Plugin Cache Mirror

**Date**: 2025-12-13
**Purpose**: Resolve technical unknowns from Technical Context section

## Research Questions

1. **Language/Version Selection**: Which language provides optimal balance of performance, simplicity, and HTTP server ecosystem?
2. **Primary Dependencies**: What HTTP server framework and libraries minimize dependencies while meeting requirements?
3. **Testing Framework**: Language-appropriate testing framework with good concurrency and integration test support?

---

## Decision 1: Programming Language

**Decision**: Go 1.21+

**Rationale**:
- **Performance**: Compiled language with excellent HTTP server performance, easily meets <500ms p95 requirement
- **Concurrency**: Built-in goroutines handle FR-006 (concurrent requests) naturally without complex threading
- **HTTP ecosystem**: Standard library `net/http` is production-ready, no heavy framework needed
- **Simplicity**: Single binary deployment perfect for Kubernetes containers
- **LRU implementation**: Multiple mature LRU cache libraries available (e.g., `groupcache/lru`, `hashicorp/golang-lru`)
- **Prometheus integration**: Official `prometheus/client_golang` library well-maintained
- **Operational fit**: Go is common in Kubernetes ecosystems, familiar to platform teams

**Alternatives Considered**:
- **Python 3.11+**: Simpler syntax but GIL limits concurrency, heavier container image, slower response times
- **Rust 1.75+**: Excellent performance but steeper learning curve, longer compile times, more complexity than needed
- **Node.js**: Good HTTP performance but less suitable for CPU-bound LRU eviction logic, higher memory overhead

---

## Decision 2: Primary Dependencies

**Decision**:
- **HTTP Server**: Go standard library `net/http` (zero external dependency)
- **Prometheus Client**: `prometheus/client_golang` v1.17+ (official client)
- **LRU Cache**: `hashicorp/golang-lru/v2` (widely used, well-tested)
- **HTTP Client**: Go standard library `net/http` with custom Transport for connection pooling
- **Checksum**: Go standard library `crypto/sha256` (Jenkins uses SHA-256 for plugin checksums)
- **Configuration**: `gopkg.in/yaml.v3` (minimal YAML parser)
- **Structured Logging**: `log/slog` (Go 1.21+ standard library)

**Rationale**:
- Minimizes external dependencies (constitution principle III)
- All chosen libraries are industry-standard with active maintenance
- Standard library covers most needs (HTTP, crypto, logging)
- Total external dependencies: 3 (Prometheus client, LRU, YAML parser)

**Alternatives Considered**:
- **Gin/Echo frameworks**: Unnecessary overhead for simple HTTP mirroring, violates simplicity principle
- **Custom LRU implementation**: Reinventing wheel, `hashicorp/golang-lru` is battle-tested with 4k+ stars
- **Logrus/Zap logging**: `slog` in stdlib (Go 1.21+) provides structured logging without external dependency

---

## Decision 3: Testing Framework

**Decision**:
- **Unit Tests**: Go standard library `testing` package
- **Integration Tests**: `testing` + `net/http/httptest` for HTTP server testing
- **Contract Tests**: `testing` + real HTTP requests to upstream Jenkins update center (read-only)
- **Performance Tests**: `testing` with benchmarks (`-bench` flag) to establish <500ms baseline

**Rationale**:
- Go's standard `testing` package supports all test types needed
- `httptest` provides HTTP server/client mocking for integration tests
- No external test framework needed (minimizes dependencies)
- Built-in benchmark support for performance regression testing (constitution principle II)

**Alternatives Considered**:
- **Ginkgo/Gomega**: BDD-style testing adds complexity without clear benefit for this use case
- **Testify**: Assertion library adds dependency; standard `testing` assertions sufficient

---

## Architecture Patterns

### HTTP Request Flow

```
Jenkins Container → Mirror Server → Upstream Jenkins Update Center
                         ↓
                   Local Storage (LRU)
```

1. **Cache Hit**: Mirror serves file from disk (<500ms target)
2. **Cache Miss**: Mirror fetches from upstream, stores locally, serves to Jenkins
3. **Concurrent Requests**: First request downloads, others wait via `sync.Singleflight` pattern

### LRU Eviction Strategy

- Track last access time in metadata file alongside each plugin
- When storage limit reached, evict least recently accessed plugins
- Use `hashicorp/golang-lru` for in-memory LRU index of plugin metadata
- Actual files stored on disk, LRU cache holds metadata pointers

### Concurrency Model

- One goroutine per HTTP request (standard Go HTTP server)
- `sync.Singleflight` deduplicates concurrent requests for same uncached plugin
- Mutex-protected LRU cache updates to prevent race conditions

---

## Jenkins Update Center URL Patterns

Research of Jenkins plugin download URLs:

**Standard Pattern**:
```
https://updates.jenkins.io/download/plugins/<plugin-name>/<version>/<plugin-name>.hpi
```

**Example**:
```
https://updates.jenkins.io/download/plugins/git/5.0.0/git.hpi
```

**Checksum Pattern**:
```
https://updates.jenkins.io/download/plugins/<plugin-name>/<version>/<plugin-name>.hpi.sha256
```

**Mirror Implementation**:
- Parse incoming requests matching `/download/plugins/{name}/{version}/{file}`
- Validate `name`, `version`, and `file` to prevent path traversal
- Forward upstream if not cached, cache response, serve to client

---

## Deployment Considerations

### Kubernetes Resources

- **Service**: ClusterIP exposing port 8080 (HTTP) and 9090 (metrics)
- **Deployment**: Single replica initially (can scale horizontally with shared PersistentVolume)
- **PersistentVolumeClaim**: 10GB default (admin configurable via ConfigMap)
- **ConfigMap**: Storage limit, optional TTL policies, upstream URL
- **ServiceMonitor**: Prometheus Operator CRD for automatic metrics scraping

### Configuration

```yaml
# Default configuration
storage:
  limit: 10GB
  path: /var/cache/jenkins-plugins

upstream:
  url: https://updates.jenkins.io
  timeout: 60s

server:
  port: 8080
  metrics_port: 9090

admin:
  enabled: true
  port: 8081
```

---

## Performance Analysis

### Expected Response Times

- **Cached plugin (10MB)**: ~50-100ms (disk read + HTTP overhead)
- **Uncached plugin (10MB)**: ~5-30s (upstream download time)
- **50 concurrent requests**: Go HTTP server handles easily, limited by disk I/O and network

### Storage Estimates

- **Average plugin size**: ~5-10MB
- **10GB limit**: ~1000-2000 plugins
- **Typical Jenkins setup**: 50-100 plugins
- **Multiple versions**: 10GB accommodates 10-20 versions per plugin

### Bandwidth Savings

- **Initial cache fill**: High upstream bandwidth (one-time per plugin version)
- **After warmup**: 80% hit rate → 80% reduction in upstream bandwidth
- **50 Jenkins instances**: Without mirror: 50 × 500MB = 25GB. With mirror: 500MB + (20% × 25GB) = 5.5GB

---

## Security Considerations

### Input Validation

- **Plugin name**: Alphanumeric + hyphens only, max length 100 chars
- **Version**: Semver pattern (e.g., `5.0.0`), max length 20 chars
- **File extension**: Must be `.hpi` or `.jpi` or `.hpi.sha256` / `.jpi.sha256`
- Reject any path traversal attempts (`../`, absolute paths)

### Checksum Validation

- Download `.sha256` file from upstream alongside plugin
- Verify downloaded plugin matches checksum before storing
- Re-validate on serve if file modified time changed (detects corruption)

### Rate Limiting

- Not required for internal K8s traffic (trusted network)
- If exposed externally, add rate limiting per IP/Jenkins instance ID

---

## Open Questions for Implementation

1. **Shared storage in K8s**: Use ReadWriteMany PVC for horizontal scaling, or single replica with ReadWriteOnce?
   - **Recommendation**: Start with ReadWriteOnce + single replica (simpler), add RWX if horizontal scaling needed

2. **Graceful shutdown**: How long to wait for in-flight downloads before shutting down pod?
   - **Recommendation**: 60-second grace period, cancel downloads > 60s old on shutdown

3. **Metrics granularity**: Per-plugin metrics or aggregate only?
   - **Recommendation**: Aggregate metrics (hit rate, bandwidth) + top 10 most-requested plugins to limit cardinality
