# Task 5.0: Server Lifecycle Integration

## status: completed

**Size:** M (2 days)  
**Priority:** HIGH - Enables end-to-end functionality  
**Dependencies:** Tasks 2.0, 3.0

## Overview

Integrate embedded Temporal server into main server startup sequence in `engine/infra/server/dependencies.go`.

## Deliverables

- [x] `engine/infra/server/dependencies.go` - Add maybeStartStandaloneTemporal function
- [x] Integration point tested

## Acceptance Criteria

- [x] `maybeStartStandaloneTemporal(ctx, cfg)` function created
- [x] Function called BEFORE `maybeStartWorker()` in setupDependencies
- [x] Embedded server started when Mode="standalone"
- [x] Nothing happens when Mode="remote"
- [x] cfg.Temporal.HostPort dynamically overridden in standalone mode
- [x] Cleanup function registered for graceful shutdown
- [x] Startup logging added (Info: "Starting in standalone mode", Warn: "Not for production")
- [x] Integration verified with manual test
- [x] No linter errors

## Implementation Approach

See `_techspec.md` "Server Lifecycle Integration" section.

**Function Signature:**
```go
func maybeStartStandaloneTemporal(ctx context.Context, cfg *config.Config) (cleanup func(), err error)
```

**Logic:**
1. Check if `cfg.Temporal.Mode == "standalone"`
2. If false, return nil cleanup and nil error
3. If true:
   - Build embedded.Config from cfg.Temporal.Standalone
   - Call embedded.NewServer(ctx, embeddedCfg)
   - Call server.Start(ctx)
   - Override cfg.Temporal.HostPort = server.FrontendAddress()
   - Log startup info and production warning
   - Return cleanup function that calls server.Stop()

**Integration Point:**
Insert in `setupDependencies()` between Temporal client creation prep and worker startup.

## Tests

Manual integration test:
1. Set Mode="standalone" in config
2. Start compozy server
3. Verify embedded server starts
4. Verify worker connects
5. Execute simple workflow
6. Verify UI accessible at http://localhost:8233
7. Verify graceful shutdown

## Files to Modify

- `engine/infra/server/dependencies.go` - Add maybeStartStandaloneTemporal function and call it

## Notes

- Use `logger.FromContext(ctx)` for all logging
- Override HostPort AFTER server starts (to get actual frontend address)
- Register cleanup in cleanup chain (append to existing cleanups)
- Log at Info level: "Temporal standalone mode started at <address>"
- Log at Warn level: "Temporal standalone mode is not recommended for production"

## Validation

```bash
# Manual test
compozy start --temporal-mode=standalone

# Check logs for:
# - "Temporal standalone mode started"
# - Warning about production usage
# - Worker connection success
# - Workflow execution

# Visit http://localhost:8233 to verify UI
```
