# Task 1.0: Embedded Server Package Foundation

## status: completed

**Size:** L (3 days)  
**Priority:** CRITICAL - Blocks all other tasks  
**Dependencies:** None

## Overview

Create the `engine/worker/embedded/` package with core types, validation, builder functions, and namespace creation. This is the foundation for the embedded Temporal server.

## Deliverables

- [x] `engine/worker/embedded/config.go` - Config type and validation
- [x] `engine/worker/embedded/builder.go` - Temporal config builders
- [x] `engine/worker/embedded/namespace.go` - Namespace creation helper
- [x] `engine/worker/embedded/config_test.go` - Config validation tests
- [x] `engine/worker/embedded/builder_test.go` - Builder function tests
- [x] `engine/worker/embedded/namespace_test.go` - Namespace tests
- [x] `go.mod` - Add `go.temporal.io/server` dependency

## Acceptance Criteria

- [x] Package compiles successfully
- [x] All unit tests pass
- [x] Config validation catches invalid ports, bad database paths, invalid log levels
- [x] Defaults applied correctly (FrontendPort=7233, BindIP="127.0.0.1", etc.)
- [x] SQLite connect attributes built correctly for memory and file modes
- [x] Static hosts configuration returns correct 4-service addresses
- [x] Namespace creation logic implemented (will be tested in task 2.0)
- [x] No linter errors

## Implementation Approach

See `_techspec.md` sections:
- "Core Interfaces" (lines 71-117) for Config struct
- "Implementation Design" for builder patterns
- "SQLite Configuration" for connection attributes

**Key Functions:**
- `validateConfig(*Config) error` - Validate all fields
- `applyDefaults(*Config)` - Apply default values
- `buildTemporalConfig(*Config) (*config.Config, error)` - Build Temporal server config
- `buildSQLiteConnectAttrs(*Config) map[string]string` - SQLite connection params
- `buildStaticHosts(*Config) map[string][]string` - Service host mapping
- `createNamespace(*config.Config, *Config) error` - Namespace initialization

## Tests (from _tests.md)

**config_test.go:**
- Should validate required fields
- Should apply defaults correctly
- Should build SQLite connect attributes
- Should build static hosts configuration

**builder_test.go:**
- Should build valid Temporal config
- Should configure SQLite persistence
- Should configure services correctly

**namespace_test.go:**
- Should create namespace in SQLite
- Should handle existing namespace gracefully

## Files to Modify

- `go.mod` - Add dependency: `go.temporal.io/server v1.24.2` (or latest)
- `go.sum` - Auto-updated by go mod

## Notes

- Use context-first: `logger.FromContext(ctx)` for all logging
- Keep functions under 50 lines
- Reference implementation: https://github.com/abtinf/temporal-a-day/blob/main/001-all-in-one-hello/main.go

## Validation

```bash
# Run scoped tests
gotestsum --format pkgname -- -race -parallel=4 ./engine/worker/embedded

# Run scoped lint
golangci-lint run --fix --allow-parallel-runners ./engine/worker/embedded/...
```
