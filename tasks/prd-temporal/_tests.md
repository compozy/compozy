# Tests Plan: Temporal Standalone Mode

## Guiding Principles

- Follow `.cursor/rules/test-standards.mdc` and project rules
- Use `t.Run("Should …")` pattern, testify assertions, context helpers
- Test both in-memory and file-based persistence
- Test Web UI server lifecycle
- Verify four-service architecture
- No mocks for Temporal server itself (real embedded instance)

## Coverage Matrix

Map PRD/Tech Spec requirements to concrete test files:

| Requirement | Test File | Test Cases |
|-------------|-----------|------------|
| Embedded server starts successfully | `engine/worker/embedded/server_test.go` | TestNewServer, TestServerStart |
| Configuration validation | `engine/worker/embedded/config_test.go` | TestValidateConfig, TestApplyDefaults |
| SQLite in-memory mode | `test/integration/temporal/standalone_test.go` | TestStandaloneMemoryMode |
| SQLite file-based mode | `test/integration/temporal/standalone_test.go` | TestStandaloneFileMode |
| Web UI server | `engine/worker/embedded/ui_test.go` | TestUIServerStart, TestUIServerStop |
| Mode selection | `test/integration/temporal/mode_switching_test.go` | TestDefaultModeIsRemote, TestStandaloneModeActivation |
| Port configuration | `test/integration/temporal/standalone_test.go` | TestStandaloneCustomPorts |
| Workflow execution | `test/integration/temporal/standalone_test.go` | TestStandaloneWorkflowExecution |
| Persistence across restarts | `test/integration/temporal/persistence_test.go` | TestStandalonePersistence |
| Graceful shutdown | `engine/worker/embedded/server_test.go` | TestServerStop |
| Error handling | `test/integration/temporal/errors_test.go` | TestStartupFailure, TestPortConflict |
| Config from context | `pkg/config/config_test.go` | TestTemporalConfigValidation |

## Unit Tests

### engine/worker/embedded/config_test.go
- **Should validate required fields**
  - Empty DatabaseFile with file mode → error
  - Invalid FrontendPort (negative, >65535) → error
  - Invalid UIPort (negative, >65535) → error
  - Invalid LogLevel (not debug|info|warn|error) → error
  - Invalid BindIP (malformed) → error

- **Should apply defaults correctly**
  - Nil config → defaults applied
  - Partial config → missing fields get defaults
  - FrontendPort default = 7233
  - BindIP default = "127.0.0.1"
  - EnableUI default = true
  - UIPort default = 8233
  - LogLevel default = "warn"
  - StartTimeout default = 30s

- **Should build SQLite connect attributes**
  - ":memory:" → mode=memory, cache=shared
  - File path → _journal_mode=WAL, _synchronous=NORMAL
  - Valid attributes map returned

- **Should build static hosts configuration**
  - Frontend = BindIP:FrontendPort
  - History = BindIP:FrontendPort+1
  - Matching = BindIP:FrontendPort+2
  - Worker = BindIP:FrontendPort+3

### engine/worker/embedded/server_test.go
- **Should create server with valid config**
  - Valid config → server instance created
  - Server not yet started
  - FrontendAddress returns correct value

- **Should reject invalid config**
  - Invalid port range → error
  - Invalid log level → error
  - Bad database file path (no permissions) → error

- **Should start server successfully**
  - Call Start() → no error
  - Server accepts connections on frontend port
  - Services respond to health checks
  - Namespace created

- **Should timeout if server doesn't start**
  - Very short StartTimeout → context.DeadlineExceeded
  - Server resources cleaned up

- **Should stop server gracefully**
  - Call Stop() after Start() → no error
  - Services shut down cleanly
  - Ports released

- **Should handle port conflicts**
  - Port already in use → descriptive error
  - Error message includes port number
  - Error message suggests resolution

- **Should wait for ready state**
  - waitForReady() polls until frontend accessible
  - Returns nil when ready
  - Returns error on timeout

### engine/worker/embedded/namespace_test.go
- **Should create namespace in SQLite**
  - Namespace created before server start
  - Namespace queryable via Temporal client
  - Default namespace = "default"
  - Custom namespace supported

- **Should handle namespace creation errors**
  - Invalid namespace name → error
  - Database connection failure → error

### engine/worker/embedded/ui_test.go
- **Should create UI server when enabled**
  - EnableUI=true → UI server created
  - UI server not started yet

- **Should start UI server**
  - Call Start() → no error
  - HTTP server accessible on UIPort
  - Web UI serves pages

- **Should not create UI server when disabled**
  - EnableUI=false → UI server is nil
  - Start() skips UI server

- **Should stop UI server gracefully**
  - Call Stop() → no error
  - HTTP server shuts down
  - Port released

### pkg/config/config_test.go
- **Should validate TemporalConfig.Mode**
  - Mode="remote" → valid
  - Mode="standalone" → valid
  - Mode="invalid" → validation error
  - Empty Mode → defaults to "remote"

- **Should validate StandaloneConfig fields**
  - Valid FrontendPort range
  - Valid UIPort range
  - Valid LogLevel enum
  - Valid BindIP format

- **Should provide defaults for Standalone**
  - Empty StandaloneConfig → defaults applied
  - DatabaseFile default = ":memory:"
  - FrontendPort default = 7233
  - EnableUI default = true

## Integration Tests

### test/integration/temporal/standalone_test.go
- **Should start Compozy server in standalone mode with memory persistence**
  - Set Mode="standalone", DatabaseFile=":memory:"
  - Server starts successfully
  - Temporal client connects to embedded server
  - Worker registers workflows
  - Execute simple workflow end-to-end
  - Workflow completes successfully
  - Server shuts down cleanly

- **Should start Compozy server in standalone mode with file persistence**
  - Set DatabaseFile="./test-temporal.db"
  - Server starts successfully
  - Database file created
  - Workflow executes
  - Database file contains workflow history

- **Should execute workflows end-to-end**
  - Start server, trigger workflow
  - Workflow transitions through states
  - Activities execute
  - Workflow completes
  - Result retrievable via client

- **Should isolate namespaces correctly**
  - Create workflows in namespace "test-1"
  - Create workflows in namespace "test-2"
  - Workflows isolated per namespace
  - No cross-namespace contamination

- **Should handle worker registration**
  - Worker registers with embedded server
  - Worker polls task queue
  - Worker executes workflows
  - Multiple workers supported

- **Should expose Web UI when enabled**
  - EnableUI=true
  - HTTP request to UIPort → 200 OK
  - Web UI serves HTML

- **Should not expose Web UI when disabled**
  - EnableUI=false
  - HTTP request to UIPort → connection refused

- **Should support custom ports**
  - FrontendPort=8233, UIPort=9233
  - Server binds to custom ports
  - Client connects to custom frontend port
  - Web UI accessible on custom UI port

- **Should shut down cleanly**
  - Server.Stop() called
  - All services stop
  - Ports released
  - No goroutine leaks

### test/integration/temporal/mode_switching_test.go
- **Should start in remote mode by default**
  - No Mode specified → defaults to "remote"
  - Server attempts to connect to HostPort
  - (Test may mock or skip actual remote connection)

- **Should start in standalone mode when configured**
  - Mode="standalone"
  - Embedded server starts
  - HostPort overridden to embedded server address

- **Should fail gracefully if embedded server fails to start**
  - Simulate startup failure (port in use)
  - Clear error message
  - Server startup aborted
  - Resources cleaned up

- **Should fail gracefully if port already in use**
  - Occupy port 7233
  - Attempt to start standalone mode
  - Error message mentions port conflict
  - Suggests resolution (change port or free up port)

- **Should log warnings about standalone limitations**
  - Mode="standalone"
  - Log includes: "Standalone mode active - optimized for development, not production"
  - Log includes: "Using embedded Temporal server"

- **Should override HostPort when standalone mode active**
  - Config has HostPort="remote-server:7233"
  - Mode="standalone"
  - HostPort dynamically set to "127.0.0.1:7233"
  - Client connects to embedded server, not remote

### test/integration/temporal/persistence_test.go
- **Should persist workflows across server restarts (file-based SQLite)**
  - Start server with DatabaseFile="./test-temporal.db"
  - Trigger workflow A
  - Stop server
  - Restart server with same DatabaseFile
  - Workflow A history still accessible
  - Trigger workflow B
  - Both A and B in history

- **Should not persist workflows in memory mode**
  - Start server with DatabaseFile=":memory:"
  - Trigger workflow
  - Stop server
  - Restart server with DatabaseFile=":memory:"
  - Previous workflow history NOT present (ephemeral)

- **Should use WAL mode for file-based SQLite**
  - DatabaseFile="./test-temporal.db"
  - WAL files present: .db-shm, .db-wal
  - Better concurrency and reliability

- **Should handle database cleanup on stop**
  - File-based mode: database file remains
  - In-memory mode: database destroyed on stop

### test/integration/temporal/startup_lifecycle_test.go
- **Should start embedded server before worker**
  - Monitor startup sequence
  - Embedded server starts first
  - Worker initialization waits for server ready
  - Worker connects to embedded server

- **Should wait for server readiness before proceeding**
  - Server starts asynchronously
  - waitForReady() polls until services available
  - Timeout if services don't become ready

- **Should clean up embedded server on shutdown**
  - Server shutdown sequence
  - Embedded server stops before other cleanup
  - Ports released
  - Resources freed

- **Should handle server startup timeout**
  - Set very short StartTimeout
  - Server doesn't become ready in time
  - Startup fails with timeout error
  - Cleanup executed

- **Should handle concurrent startup/shutdown**
  - Simulate rapid start/stop cycles
  - No race conditions
  - Resources properly managed

### test/integration/temporal/errors_test.go
- **Should handle port conflicts gracefully**
  - Occupy frontend port (7233)
  - Attempt to start standalone mode
  - Error: "failed to start temporal server: port 7233 already in use"
  - Resolution suggested in error message

- **Should handle database errors**
  - Invalid database file path
  - No write permissions for database file
  - Database corruption
  - Clear error messages for each scenario

- **Should handle namespace creation errors**
  - Simulate namespace creation failure
  - Server startup aborted
  - Clear error message

- **Should handle UI server errors**
  - UI port conflict
  - UI server fails to start
  - Temporal server still starts (UI is optional failure)
  - Warning logged

## Fixtures & Testdata

Add under `engine/worker/embedded/testdata/`:
- `valid-config.yaml` - Valid standalone configuration
- `invalid-config.yaml` - Invalid configuration for error testing
- `custom-ports.yaml` - Custom port configuration

Add under `test/integration/temporal/testdata/`:
- `compozy-standalone-memory.yaml` - Memory mode configuration
- `compozy-standalone-file.yaml` - File-based persistence configuration
- `compozy-standalone-no-ui.yaml` - UI disabled configuration
- `compozy-standalone-custom-ports.yaml` - Custom ports
- `simple-workflow.yaml` - Minimal workflow for testing
- `multi-task-workflow.yaml` - Complex workflow for end-to-end testing

## Mocks & Stubs

- **No mocks for Temporal server** - Use real embedded server in integration tests
- **May mock:** External services called by workflows/activities
- **Prefer:** Real implementations over mocks whenever possible

## Performance & Limits

- **Startup Time:** Embedded server must start in <30 seconds (default timeout)
- **Shutdown Time:** Graceful shutdown must complete in <10 seconds
- **Memory Usage:** Baseline memory usage should be documented
- **Concurrent Workflows:** Test with 10+ concurrent workflows to verify stability
- **File Size:** Monitor SQLite file size growth (warn if >100MB in tests)

## Observability Assertions

- **Metrics Presence:**
  - `compozy_temporal_standalone_enabled` gauge = 1 when standalone mode active
  - `compozy_temporal_standalone_starts_total` counter increments on start
  - `compozy_temporal_standalone_stops_total` counter increments on stop
  - `compozy_temporal_standalone_errors_total` counter increments on errors

- **Logs Presence:**
  - "Starting embedded Temporal server" logged at Info level
  - "Embedded Temporal server started successfully" logged at Info level
  - "Standalone mode active" warning logged
  - Error logs include context (phase, port, error details)

- **Metrics Labels:**
  - Mode: "memory" or "file"
  - Error type: "start", "stop", "namespace"

## CLI Tests (if applicable)

- **Should show standalone mode in config output**
  - `compozy config show -f table`
  - Output includes: `temporal.mode = standalone`
  - Output includes standalone configuration fields

- **Should accept --temporal-mode flag**
  - `compozy start --temporal-mode=standalone`
  - Overrides config file
  - Server starts in standalone mode

## Exit Criteria

- [ ] All unit tests exist and pass locally
- [ ] All integration tests exist and pass locally
- [ ] Test coverage >80% for new code in `engine/worker/embedded/`
- [ ] CI pipeline updated to run standalone mode tests
- [ ] Tests complete in <2 minutes total
- [ ] No flaky tests (run 10 times, all pass)
- [ ] Memory leaks checked (no goroutine leaks)
- [ ] Performance benchmarks documented
- [ ] Error scenarios covered with clear assertions
- [ ] Web UI accessibility tested
- [ ] Persistence across restarts verified
- [ ] Port conflicts handled gracefully

## Test Execution Commands

```bash
# Run all tests
make test

# Run only standalone mode tests
go test -v ./engine/worker/embedded/... -race
go test -v ./test/integration/temporal/... -race

# Run specific test
go test -v ./test/integration/temporal -run TestStandaloneWorkflowExecution -race

# Run with coverage
go test -v ./engine/worker/embedded/... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Check for race conditions
go test -v ./... -race -parallel=4

# Benchmark
go test -bench=. ./engine/worker/embedded/...
```

## CI Integration

- Add `test-standalone` job to CI pipeline
- Run standalone mode integration tests
- Verify no Docker required for these tests
- Check for goroutine leaks
- Verify memory usage stays within bounds
- Ensure tests pass on macOS, Linux, Windows

## Notes

- Tests should run quickly (<2 minutes total)
- Use `t.Context()` instead of `context.Background()`
- Clean up resources (database files, ports) after tests
- Use table-driven tests for configuration validation
- Document expected behavior in test names
- Reference official Temporal tests when applicable

## Related Planning Artifacts
- tasks/prd-temporal/_techspec.md
- tasks/prd-temporal/_docs.md
- tasks/prd-temporal/_examples.md

## References
- Test Standards: `.cursor/rules/test-standards.mdc`
- GitHub Reference: https://github.com/abtinf/temporal-a-day/blob/main/001-all-in-one-hello/main.go
- Temporal Testing Docs: https://docs.temporal.io/develop/go/testing-suite
