# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

Jenkins Larder is a caching reverse proxy for Jenkins plugin downloads. It intercepts requests matching the Jenkins Update Center URL pattern (`/download/plugins/{name}/{version}/{file}`), serves from local cache if available, or fetches from upstream (`updates.jenkins.io`) and caches for future requests. LRU eviction keeps storage within configured limits.

## Commands

```bash
make build          # Build binary to bin/larder
make run            # Build and run with config/test.yaml
make test           # Run all tests (unit + integration + contract)
make test-unit      # Unit tests only: go test -v ./tests/unit/...
make test-integration  # Integration tests (requires -tags=integration)
make test-contract  # Contract tests: go test -v ./tests/contract/...
make lint           # golangci-lint run ./...
make fmt            # go fmt ./...
```

Run a single test: `go test -v -run TestName ./tests/unit/...`

Configuration is loaded from `CONFIG_PATH` env var (defaults to `config/default.yaml`). Use `config/test.yaml` for local dev.

## Architecture

The app runs three separate HTTP servers on different ports:
- **Plugin server** (`:8080`) - handles `GET /download/plugins/{name}/{version}/{file}`, the only public endpoint
- **Admin server** (`:8081`) - cache invalidation, stats, health check
- **Metrics server** (`:9090`) - Prometheus metrics

Request flow: `cmd/larder/main.go` loads config and creates `server.Server` -> `server.Server` initializes `CacheService` -> download requests hit `DownloadHandler` which checks cache (via `storage/` package) -> on miss, fetches from upstream (`upstream/client.go`) and streams through while caching -> `storage/lru.go` handles eviction when storage limit is reached. Request deduplication (`server/deduplication.go`) collapses concurrent requests for the same plugin.

Key packages under `src/`:
- `config/` - YAML config loading and validation
- `storage/` - filesystem-backed cache with LRU eviction, checksum verification, metadata tracking
- `upstream/` - HTTP client for fetching from Jenkins Update Center
- `server/` - HTTP handlers, cache service orchestration, request dedup, logging middleware
- `admin/` - admin API handlers
- `metrics/` - Prometheus metric definitions

## Testing

Tests live in `tests/` (not alongside source files):
- `tests/unit/` - config parsing, LRU logic, storage operations
- `tests/integration/` - end-to-end download flows (use build tag `integration`)
- `tests/contract/` - API contract verification for admin, metrics, and plugin endpoints

## Tooling

Go and golangci-lint are managed via `mise.toml`. Run `make setup` after cloning to configure git hooks (pre-commit runs lint).
