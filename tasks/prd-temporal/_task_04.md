# Task 4.0: UI Server Implementation

**Size:** M (1-2 days)  
**Priority:** MEDIUM - Optional feature  
**Dependencies:** Tasks 1.0, 2.0

## Overview

Implement optional Temporal Web UI server wrapper for local development debugging.

## status: completed

## Deliverables

- [x] `engine/worker/embedded/ui.go` - UI server implementation
- [x] `engine/worker/embedded/ui_test.go` - UI server tests

## Acceptance Criteria

- [x] `UIServer` struct created
- [x] `newUIServer(cfg *Config) *UIServer` constructor
- [x] `Start(ctx)` starts UI server on configured port
- [x] `Stop(ctx)` gracefully stops UI server
- [x] UI connects to embedded Temporal frontend
- [x] UI accessible at http://localhost:8233 (default)
- [x] UI can be disabled via config (EnableUI=false)
- [x] All tests pass
- [x] No linter errors

## Implementation Approach

See `_techspec.md` "UI Server Manager" section (lines 46-50).

**Key Components:**
- Use `go.temporal.io/server/ui-server/v2` package
- UIServer wraps ui-server with lifecycle management
- Connects to embedded frontend via HostPort
- Supports graceful shutdown

**Integration into Server struct:**
- Server.uiServer field (*UIServer, nil if disabled)
- Created in NewServer if cfg.EnableUI == true
- Started in Server.Start() after temporal server ready
- Stopped in Server.Stop() before temporal server

## Tests (from _tests.md)

**ui_test.go:**
- Should create UI server with valid config
- Should start UI server successfully
- Should stop UI server gracefully
- Should skip UI when disabled (EnableUI=false)
- Should return error if UI port unavailable

## Files to Create

- `engine/worker/embedded/ui.go`
- `engine/worker/embedded/ui_test.go`

## Files to Modify

- `engine/worker/embedded/server.go` - Integrate UI server into lifecycle
- `go.mod` - Add `go.temporal.io/server/ui-server/v2` dependency

## Notes

- UI server is optional - startup should succeed even if UI fails (log warning)
- UI port conflicts should be non-fatal (log error, continue without UI)
- Access UI at: http://localhost:<UIPort>

## Validation

```bash
gotestsum --format pkgname -- -race -parallel=4 ./engine/worker/embedded
golangci-lint run --fix --allow-parallel-runners ./engine/worker/embedded/...
```
