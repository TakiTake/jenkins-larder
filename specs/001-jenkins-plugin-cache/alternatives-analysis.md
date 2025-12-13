# Alternative Implementation Analysis

**Date**: 2025-12-13
**Purpose**: Evaluate OSS web server alternatives vs custom Go implementation

## User Request

> "Could you investigate capability of the open source web servers? For example, Nginx + Lua extension. I'd like to evaluate if developing web server by ourself is the best choice."

---

## Alternatives Evaluated

### Option 1: Nginx + OpenResty (Lua Extension)

**Description**: Use Nginx as the HTTP server with OpenResty (Lua scripting) for cache logic.

**Architecture**:
```
Nginx (HTTP Server)
  ↓
OpenResty/Lua (Cache Logic)
  ↓
Filesystem Storage
  ↓
Upstream Jenkins Update Center
```

**Pros**:
- ✅ **Proven HTTP server**: Nginx handles 10K+ concurrent connections efficiently
- ✅ **No development needed**: Nginx is production-ready, battle-tested
- ✅ **Lua scripting**: OpenResty provides full Lua environment for custom cache logic
- ✅ **Built-in caching**: `proxy_cache` module handles basic HTTP caching
- ✅ **Low resource usage**: Nginx is extremely memory-efficient
- ✅ **Community support**: Massive ecosystem, extensive documentation

**Cons**:
- ❌ **LRU eviction complexity**: `proxy_cache` uses simple size limits, custom LRU requires Lua + shared dict
- ❌ **Prometheus metrics**: Requires nginx-lua-prometheus module (external dependency)
- ❌ **Checksum validation**: Must implement in Lua, no native support
- ❌ **Concurrent request deduplication**: Lua shared dict locking is complex for singleflight pattern
- ❌ **Admin API**: Must implement separate Lua endpoints for invalidation
- ❌ **Configuration complexity**: Nginx config + Lua code split across multiple files
- ❌ **Debugging**: Lua errors harder to debug than compiled Go code
- ❌ **Testing**: Integration tests require full Nginx deployment

**Implementation Complexity**: **Medium-High**

**Example Nginx Config** (simplified):
```nginx
http {
    lua_shared_dict cache_metadata 10m;
    lua_package_path "/etc/nginx/lua/?.lua;;";

    upstream jenkins_upstream {
        server updates.jenkins.io:443;
    }

    server {
        listen 8080;

        location ~ ^/download/plugins/(.+)/(.+)/(.+)$ {
            set $plugin_name $1;
            set $plugin_version $2;
            set $plugin_file $3;

            access_by_lua_block {
                -- Validate plugin name/version
                -- Check LRU cache
                -- Update access time
            }

            proxy_cache jenkins_cache;
            proxy_cache_valid 200 365d;  # No TTL, cache indefinitely
            proxy_cache_key "$plugin_name:$plugin_version:$plugin_file";
            proxy_pass https://jenkins_upstream;

            header_filter_by_lua_block {
                -- Validate checksum
                -- Update Prometheus metrics
            }
        }

        location /admin/cache/invalidate {
            content_by_lua_block {
                -- Admin API implementation
            }
        }
    }
}
```

**Lua Code Complexity**:
- LRU tracking: ~100-150 lines of Lua
- Checksum validation: ~50 lines
- Prometheus metrics: ~80 lines (using external library)
- Admin API: ~60 lines
- **Total custom code**: ~300-350 lines Lua + Nginx config

---

### Option 2: Apache Traffic Server (ATS)

**Description**: Use Apache Traffic Server, a caching HTTP proxy designed for CDN use cases.

**Architecture**:
```
Apache Traffic Server (Caching Proxy)
  ↓
Upstream Jenkins Update Center
```

**Pros**:
- ✅ **Purpose-built for caching**: Designed as a caching proxy
- ✅ **LRU eviction built-in**: Native cache eviction policies
- ✅ **High performance**: Used by Yahoo, Apple for large-scale caching
- ✅ **Lua plugin support**: Can extend with Lua for custom logic
- ✅ **Metrics**: Built-in statistics system

**Cons**:
- ❌ **Heavy footprint**: More complex than needed for Jenkins plugins
- ❌ **Steep learning curve**: ATS configuration is complex
- ❌ **Prometheus integration**: Requires custom exporter
- ❌ **Checksum validation**: Must implement via Lua plugin
- ❌ **Admin API**: Must implement via plugin
- ❌ **Overkill**: Designed for CDN-scale traffic, not 50 Jenkins instances
- ❌ **Less common**: Smaller community than Nginx or Squid

**Implementation Complexity**: **High**

---

### Option 3: Squid Caching Proxy

**Description**: Use Squid, a traditional HTTP caching proxy.

**Architecture**:
```
Squid Proxy
  ↓
Upstream Jenkins Update Center
```

**Pros**:
- ✅ **Mature caching proxy**: 25+ years of development
- ✅ **LRU eviction**: Native support for various eviction policies
- ✅ **Well-documented**: Extensive documentation and community
- ✅ **Simple configuration**: Easier than ATS

**Cons**:
- ❌ **No Prometheus metrics**: Requires SNMP exporter (complex)
- ❌ **Limited extensibility**: No Lua/scripting support (ICAP for external processing only)
- ❌ **Checksum validation**: Not supported natively
- ❌ **Admin API**: Must implement external service communicating via Squid management interface
- ❌ **Concurrent deduplication**: Not customizable
- ❌ **Legacy architecture**: Designed for 1990s web caching, not modern API mirroring

**Implementation Complexity**: **Medium**

---

### Option 4: Varnish Cache

**Description**: Modern HTTP caching server with VCL (Varnish Configuration Language) for custom logic.

**Architecture**:
```
Varnish Cache (HTTP Cache)
  ↓
VCL (Custom Logic)
  ↓
Upstream Jenkins Update Center
```

**Pros**:
- ✅ **High performance**: In-memory caching, extremely fast
- ✅ **VCL scripting**: Powerful configuration language for custom cache logic
- ✅ **Modern architecture**: Designed for high-traffic web applications
- ✅ **Good documentation**: Well-maintained, active community
- ✅ **Flexible cache policies**: Fine-grained control over caching behavior

**Cons**:
- ❌ **In-memory only**: Varnish caches in RAM, not ideal for 10GB persistent storage
- ❌ **Prometheus metrics**: Requires varnish_exporter (external tool)
- ❌ **Checksum validation**: VCL doesn't support cryptographic operations
- ❌ **LRU tracking**: VCL has limited state management for custom LRU
- ❌ **Admin API**: Must implement via external service + VCL communication
- ❌ **Persistence**: Requires disk-backed storage layer (Varnish doesn't persist cache across restarts)

**Implementation Complexity**: **Medium-High**

---

### Option 5: Custom Go Server (Original Plan)

**Description**: Develop HTTP mirror server in Go 1.21+.

**Architecture**:
```
Custom Go HTTP Server
  ↓
LRU Cache (hashicorp/golang-lru)
  ↓
Filesystem Storage
  ↓
Upstream HTTP Client
```

**Pros**:
- ✅ **Full control**: Complete control over cache logic, eviction, metrics
- ✅ **Simple deployment**: Single binary, no external dependencies
- ✅ **Native Prometheus**: Official prometheus/client_golang library
- ✅ **Testability**: Easy to write unit/integration tests in Go
- ✅ **Concurrent safety**: Go's sync primitives handle concurrent requests cleanly
- ✅ **Singleflight pattern**: `golang.org/x/sync/singleflight` solves duplicate download problem elegantly
- ✅ **Checksum validation**: stdlib `crypto/sha256` for checksums
- ✅ **Admin API**: Standard `net/http` for admin endpoints
- ✅ **Debugging**: Compiled language, strong typing, excellent error messages
- ✅ **Minimal dependencies**: 3 external libraries (Prometheus, LRU, YAML)

**Cons**:
- ❌ **Development effort**: Requires writing ~1000-1500 lines of Go code
- ❌ **Testing effort**: Must write comprehensive test suite
- ❌ **HTTP server from scratch**: Not leveraging proven web server (but Go stdlib HTTP is battle-tested)
- ❌ **Maintenance**: Team must maintain custom code

**Implementation Complexity**: **Medium**

**Code Estimate**:
- HTTP server routing: ~150 lines
- Storage + LRU logic: ~300 lines
- Upstream client: ~200 lines
- Prometheus metrics: ~150 lines
- Admin API: ~100 lines
- Configuration: ~80 lines
- **Total application code**: ~980 lines
- **Tests**: ~1500 lines (comprehensive coverage)

---

## Comparison Matrix

| Criteria | Nginx + Lua | ATS | Squid | Varnish | Custom Go |
|----------|-------------|-----|-------|---------|-----------|
| **HTTP Performance** | Excellent | Excellent | Good | Excellent | Very Good |
| **LRU Eviction** | Custom Lua | Native | Native | Limited | Full Control |
| **Prometheus Metrics** | External Module | Custom Exporter | SNMP Exporter | External Exporter | Native |
| **Checksum Validation** | Custom Lua | Lua Plugin | Not Supported | Not Supported | Native |
| **Concurrent Dedup** | Complex Lua | Limited | Not Customizable | Limited | Native (Singleflight) |
| **Admin API** | Custom Lua | Plugin | External Service | External Service | Native |
| **Configuration Complexity** | Medium-High | High | Medium | Medium | Low |
| **Development Effort** | Medium | High | Low-Medium | Medium | Medium |
| **Operational Complexity** | Low | Medium-High | Low | Medium | Very Low |
| **Testing Complexity** | Medium-High | High | Medium | Medium | Low |
| **Debugging** | Medium (Lua) | Medium | Low | Medium | High (Go) |
| **Single Binary Deploy** | No (Nginx + Lua) | No | No | No | Yes |
| **Memory Footprint** | Low (~50MB) | High (~200MB+) | Medium (~100MB) | High (RAM cache) | Low (~80MB) |
| **Community/Support** | Excellent | Good | Excellent | Very Good | Excellent (Go) |
| **Learning Curve** | Medium | High | Low | Medium | Low-Medium |

---

## Decision Matrix by Requirements

### FR-006: Concurrent Request Deduplication

| Solution | Implementation | Complexity |
|----------|----------------|------------|
| **Nginx + Lua** | Lua shared dict + custom locking | Complex, race condition risks |
| **ATS** | Limited control, basic coalescing | Medium, inflexible |
| **Squid** | Not customizable | N/A |
| **Varnish** | VCL has basic request coalescing | Limited customization |
| **Custom Go** | `sync.Singleflight` (23 lines) | Simple, proven pattern |

**Winner**: Custom Go (built-in solution, battle-tested)

### FR-007: Checksum Validation

| Solution | Implementation | Complexity |
|----------|----------------|------------|
| **Nginx + Lua** | Lua crypto library (~50 lines) | Medium, external dependency |
| **ATS** | Lua plugin | Medium |
| **Squid** | Not feasible | N/A |
| **Varnish** | Not feasible | N/A |
| **Custom Go** | stdlib `crypto/sha256` (10 lines) | Trivial |

**Winner**: Custom Go (no external dependency, native support)

### FR-012: Prometheus Metrics

| Solution | Implementation | Complexity |
|----------|----------------|------------|
| **Nginx + Lua** | nginx-lua-prometheus module | Medium, external module |
| **ATS** | Custom exporter service | High, separate service |
| **Squid** | SNMP exporter | High, SNMP complexity |
| **Varnish** | varnish_exporter | Medium, separate service |
| **Custom Go** | `prometheus/client_golang` (~150 lines) | Low, official library |

**Winner**: Custom Go (first-class support, no external exporter)

### FR-014: LRU Eviction

| Solution | Implementation | Complexity |
|----------|----------------|------------|
| **Nginx + Lua** | Custom Lua + shared dict (~150 lines) | Medium-High, manual tracking |
| **ATS** | Native cache eviction | Low, less control |
| **Squid** | Native LRU | Low, less control |
| **Varnish** | Limited VCL support | High, workarounds needed |
| **Custom Go** | `hashicorp/golang-lru` (20 lines) | Low, mature library |

**Winner**: Custom Go (full control, proven library)

### Operational Simplicity (K8s Deployment)

| Solution | Deployment | Configuration |
|----------|------------|---------------|
| **Nginx + Lua** | Nginx image + Lua files in ConfigMap | Medium (config + Lua code) |
| **ATS** | ATS image + complex config | High |
| **Squid** | Squid image + config | Medium |
| **Varnish** | Varnish image + VCL config | Medium |
| **Custom Go** | Single binary in Alpine image (~15MB) | Low (single YAML file) |

**Winner**: Custom Go (smallest image, simplest config)

---

## Recommendation

### Primary Recommendation: **Custom Go Server**

**Reasoning**:

1. **Simplicity** (Constitution Principle III):
   - Single binary deployment (no Nginx + Lua coordination)
   - One configuration file vs Nginx config + Lua scripts
   - No external metrics exporters needed
   - Minimal dependencies (3 libraries, all well-maintained)

2. **Testability** (Constitution Principle IV):
   - Go testing framework is excellent for TDD
   - Easy to mock HTTP requests, filesystem, upstream
   - Integration tests don't require full Nginx deployment
   - Benchmark support built into Go for performance testing

3. **Requirements Alignment**:
   - FR-006 (concurrent dedup): `singleflight` is idiomatic Go solution
   - FR-007 (checksums): stdlib support, zero external deps
   - FR-012 (Prometheus): official client library, first-class support
   - FR-014 (LRU): `hashicorp/golang-lru` is industry standard

4. **Development vs Operational Tradeoff**:
   - **Development effort**: ~2000 lines Go code (app + tests)
   - **Operational simplicity**: Single binary, no runtime dependencies, simple config
   - **Long-term maintenance**: Type-safe, compiled language easier to refactor than Lua scripts
   - **Team familiarity**: Go is common in K8s ecosystems, likely familiar to platform teams

5. **Performance**:
   - Go stdlib HTTP server handles 50 concurrent connections easily
   - Meets <500ms p95 requirement (disk I/O is bottleneck, not HTTP layer)
   - Lower memory footprint than Nginx + OpenResty (~80MB vs ~100MB+)

### Alternative Recommendation: **Nginx + OpenResty** (if custom development is not acceptable)

**When to choose Nginx**:
- Team has strong Nginx + Lua expertise
- No capacity for Go development
- Willing to accept higher testing complexity
- Comfortable maintaining Lua scripts + Nginx config

**Implementation notes if choosing Nginx**:
- Use `nginx-lua-prometheus` for metrics
- Implement LRU tracking in Lua shared dict
- Use `resty.http` for upstream requests with checksum validation
- Separate admin API via Lua content handlers
- **Estimated effort**: ~400 lines Lua + 100 lines Nginx config

---

## Updated Recommendation

**Decision**: **Custom Go Server** (maintain original plan)

**Rationale Summary**:
- Best alignment with Constitution principles (simplicity, testability, minimal dependencies)
- Native support for all functional requirements without external modules
- Easier to test, debug, and maintain long-term
- Single binary deployment is operationally simpler in Kubernetes
- Go is appropriate for platform/infrastructure tooling in K8s environments

**Development vs OSS Tradeoff**:
- OSS web servers (Nginx, Varnish) excel at HTTP caching, but require significant customization for:
  - Custom LRU logic
  - Checksum validation
  - Prometheus metrics
  - Concurrent request deduplication
  - Admin API
- Custom Go server: ~2000 lines of straightforward, testable code vs ~400+ lines of Lua + complex Nginx config + external exporters
- **Conclusion**: Development effort is justified by operational simplicity and better requirement fit

---

## Action Items

1. ✅ Maintain Go 1.21+ as the implementation language
2. ✅ Keep existing research.md decisions (Prometheus, LRU library, stdlib HTTP)
3. ⚠️ Document this alternatives analysis for future reference
4. ⚠️ If team prefers Nginx + Lua, create separate implementation plan

---

## Questions for User

1. **Does your team have Go development capacity?**
   - If yes → Proceed with Custom Go Server
   - If no → Consider Nginx + OpenResty alternative

2. **What is the team's experience level with:**
   - Go development?
   - Nginx + Lua (OpenResty)?
   - Operating HTTP caching proxies in production?

3. **What is more valuable long-term:**
   - Minimal operational complexity (favors Custom Go)
   - Leveraging existing web server infrastructure (favors Nginx)

4. **Development timeline constraints:**
   - Need working MVP in < 2 weeks? → Consider Nginx (faster to prototype)
   - Comfortable with 3-4 week development cycle? → Custom Go (better long-term)
