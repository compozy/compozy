# Task 2.0: Embedded Server Lifecycle

**Size:** M (2 days)  
**Priority:** HIGH - Required for integration  
**Dependencies:** Task 1.0

## Overview

Implement the Server struct with lifecycle management (Start, Stop, ready-state polling). This enables starting and stopping the embedded Temporal server.

## Deliverables

- [ ] `engine/worker/embedded/server.go` - Server lifecycle implementation
- [ ] `engine/worker/embedded/server_test.go` - Lifecycle tests

## Acceptance Criteria

- [ ] `NewServer(ctx, cfg)` creates server without starting it
- [ ] `Start(ctx)` starts all 4 services (frontend, history, matching, worker)
- [ ] `waitForReady(ctx)` polls until frontend accepts connections
- [ ] `Stop(ctx)` gracefully shuts down all services
- [ ] `FrontendAddress()` returns correct address
- [ ] Timeout handling works (returns error if startup exceeds StartTimeout)
- [ ] Port conflicts detected with clear error messages
- [ ] All unit tests pass
- [ ] No linter errors

## Implementation Approach

See `_techspec.md` "Core Interfaces" section (lines 119-205) for Server struct and methods.

**Key Methods:**
- `NewServer(context.Context, *Config) (*Server, error)` - Create server (no start)
- `Start(context.Context) error` - Start server and wait for ready
- `Stop(context.Context) error` - Graceful shutdown
- `waitForReady(context.Context) error` - Poll frontend until accessible
- `FrontendAddress() string` - Return frontend address

## Tests (from _tests.md)

**server_test.go:**
- Should create server with valid config
- Should reject invalid config
- Should start server successfully
- Should timeout if server doesn't start
- Should stop server gracefully
- Should handle port conflicts
- Should wait for ready state

## Files to Create

- `engine/worker/embedded/server.go`
- `engine/worker/embedded/server_test.go`

## Notes

- Use `temporal.NewServer()` with `temporal.WithConfig()`, `temporal.ForServices()`, etc.
- Server creates 4 services on sequential ports (7233-7236 by default)
- Ready-state polling: dial frontend gRPC port with timeout
- Context propagation: pass ctx through all operations
- UI server integration added in task 4.0

## Validation

```bash
gotestsum --format pkgname -- -race -parallel=4 ./engine/worker/embedded
golangci-lint run --fix --allow-parallel-runners ./engine/worker/embedded/...
```
