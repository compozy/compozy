# Examples Plan: Temporal Standalone Mode

## Conventions

- Folder prefix: `examples/temporal-standalone/*`
- Use standalone mode by default for all examples (better DX)
- Demonstrate `temporal.NewServer()` approach (NOT Temporalite)
- Use `:memory:` for ephemeral demos, file paths for persistence demos
- Show Web UI access (http://localhost:8233)
- No secrets or external dependencies

## Example Matrix

### 1. examples/temporal-standalone/basic
**Purpose:** Demonstrate simplest possible setup with standalone mode
**Files:**
- `compozy.yaml` – Minimal config with `temporal.mode: standalone`
- `workflows/hello.yaml` – Basic workflow (single task, print message)
- `README.md` – Quick start guide
- `.gitignore` – Exclude temporal.db if generated
**Demonstrates:**
- Zero-config startup (no Docker required)
- Instant workflow execution (<5 seconds from start to first workflow)
- In-memory persistence (no database file)
- Four-service architecture (frontend, history, matching, worker)
- Web UI access for debugging
**Walkthrough:**
```bash
cd examples/temporal-standalone/basic
compozy start
# Server starts with embedded Temporal (4 services + UI)
# Open Web UI: http://localhost:8233
# In another terminal:
compozy workflow trigger hello --input='{"name": "World"}'
# View workflow in Web UI
```
**Expected Output:**
- Server starts in <5 seconds
- Logs show: "Starting embedded Temporal server (mode=standalone, database=:memory:, ui_enabled=true)"
- Logs show: "Embedded Temporal server started successfully (frontend_addr=127.0.0.1:7233, ui_addr=http://127.0.0.1:8233)"
- Workflow completes successfully
- Web UI shows workflow execution

### 2. examples/temporal-standalone/persistent
**Purpose:** Show file-based persistence across server restarts
**Files:**
- `compozy.yaml` – Standalone mode with `database_file: ./data/temporal.db`
- `workflows/counter.yaml` – Workflow that increments a counter
- `data/` – Directory for SQLite database (created on first run)
- `README.md` – Guide to persistence behavior
- `.gitignore` – Exclude `data/temporal.db` from git
**Demonstrates:**
- SQLite file-based persistence
- Workflow state survives server restart
- Database file management
- WAL mode for better reliability
**Walkthrough:**
```bash
cd examples/temporal-standalone/persistent
mkdir -p data
compozy start &
sleep 2
# Trigger workflow
compozy workflow trigger counter
ls -lh data/temporal.db  # Database file created (~50KB)
# Stop server
killall compozy
# Restart - workflow state persists
compozy start &
sleep 2
compozy workflow describe counter  # Shows workflow history from before restart
```
**Expected Output:**
- Database file created in `data/temporal.db`
- File grows as workflows execute
- Workflow history persists across restarts
- WAL mode files (`.db-shm`, `.db-wal`) present

### 3. examples/temporal-standalone/custom-ports
**Purpose:** Demonstrate custom port configuration
**Files:**
- `compozy.yaml` – Standalone with custom ports (8233 frontend, 9233 UI)
- `workflows/demo.yaml` – Simple workflow
- `README.md` – Port configuration guide
**Demonstrates:**
- Configurable ports for all services
- Avoiding port conflicts
- Custom UI port
- Multiple standalone instances on same machine
**Walkthrough:**
```bash
cd examples/temporal-standalone/custom-ports
compozy start
# Frontend on 8233 instead of 7233
# Web UI on 9233 instead of 8233
# Open Web UI: http://localhost:9233
```
**Config:**
```yaml
temporal:
  mode: standalone
  standalone:
    frontend_port: 8233
    ui_port: 9233
```

### 4. examples/temporal-standalone/no-ui
**Purpose:** Disable Web UI for minimal resource usage
**Files:**
- `compozy.yaml` – Standalone with `enable_ui: false`
- `workflows/minimal.yaml` – Lightweight workflow
- `README.md` – Minimal configuration guide
**Demonstrates:**
- Disabling Web UI
- Faster startup
- Lower resource usage
- Headless operation for CI
**Walkthrough:**
```bash
cd examples/temporal-standalone/no-ui
compozy start
# No Web UI server started
# Faster startup, lower memory usage
```

### 5. examples/temporal-standalone/debugging
**Purpose:** Advanced debugging with Web UI and debug logging
**Files:**
- `compozy.yaml` – Standalone with `log_level: debug`
- `workflows/buggy.yaml` – Workflow with intentional error
- `workflows/fixed.yaml` – Corrected version
- `README.md` – Debugging techniques guide
**Demonstrates:**
- Debug logging from Temporal server
- Using Web UI for workflow inspection
- Examining workflow history
- Error handling and retry behavior
- Viewing activity timeouts
**Walkthrough:**
```bash
cd examples/temporal-standalone/debugging
compozy start
# Debug logs show Temporal internals
# Trigger buggy workflow
compozy workflow trigger buggy
# Open Web UI: http://localhost:8233
# Inspect error in Web UI (shows stack trace, retry attempts)
# Fix and retry
compozy workflow trigger fixed
```

### 6. examples/temporal-standalone/migration-from-remote
**Purpose:** Switching between remote and standalone modes
**Files:**
- `compozy.remote.yaml` – Remote mode configuration
- `compozy.standalone.yaml` – Equivalent standalone configuration
- `workflows/demo.yaml` – Same workflow works in both modes
- `docker-compose.yml` – Optional external Temporal for remote mode
- `README.md` – Migration guide
**Demonstrates:**
- Configuration differences between modes
- Workflow compatibility (no code changes)
- When to use each mode
- Migration procedure
- Zero code changes required
**Walkthrough:**
```bash
cd examples/temporal-standalone/migration-from-remote

# Option 1: Remote mode (requires Docker)
docker-compose up -d
compozy start --config=compozy.remote.yaml

# Option 2: Standalone mode (no Docker)
compozy start --config=compozy.standalone.yaml

# Same workflows work in both modes (zero code changes)
```

### 7. examples/temporal-standalone/integration-testing
**Purpose:** Integration testing with standalone Temporal
**Files:**
- `compozy.yaml` – Standalone with ephemeral ports for parallel tests
- `workflows/calculator.yaml` – Deterministic workflow for testing
- `tests/integration_test.go` – Go integration tests using standalone mode
- `Makefile` – Test runner
- `README.md` – Testing guide
**Demonstrates:**
- Using standalone mode in automated tests
- Ephemeral ports (no port conflicts)
- Fast test execution (no Docker startup)
- Parallel test execution
- Deterministic workflow testing
**Walkthrough:**
```bash
cd examples/temporal-standalone/integration-testing
make test
# Runs integration tests with isolated standalone Temporal instances
# Each test gets its own Temporal server (ephemeral ports)
# Tests complete in <30 seconds
```

## Minimal YAML Shapes

### Basic Standalone Configuration
```yaml
# compozy.yaml - Simplest standalone mode
temporal:
  mode: standalone
  namespace: default
  task_queue: my-app-queue
  standalone:
    database_file: ":memory:"
    # Defaults: frontend_port=7233, bind_ip=127.0.0.1, enable_ui=true, ui_port=8233
```

### Persistent Standalone Configuration
```yaml
# compozy.yaml - File-based persistence
temporal:
  mode: standalone
  namespace: default
  standalone:
    database_file: ./data/temporal.db
    frontend_port: 7233
    enable_ui: true
    ui_port: 8233
    log_level: info
```

### Custom Ports Configuration
```yaml
# compozy.yaml - Custom ports to avoid conflicts
temporal:
  mode: standalone
  standalone:
    database_file: ":memory:"
    frontend_port: 8233  # Frontend on 8233 instead of 7233
    ui_port: 9233        # UI on 9233 instead of 8233
```

### No UI Configuration
```yaml
# compozy.yaml - Minimal resource usage, no Web UI
temporal:
  mode: standalone
  standalone:
    database_file: ":memory:"
    enable_ui: false  # Disable Web UI
    log_level: warn   # Less verbose logging
```

### Debug Configuration
```yaml
# compozy.yaml - Maximum visibility for debugging
temporal:
  mode: standalone
  standalone:
    database_file: ":memory:"
    enable_ui: true
    log_level: debug  # Verbose Temporal server logs
```

### Remote Mode Configuration (for comparison)
```yaml
# compozy.yaml - External Temporal cluster
temporal:
  mode: remote  # Default, can be omitted
  host_port: localhost:7233
  namespace: default
  task_queue: my-app-queue
```

### Environment Variable Override
```bash
# Use standalone mode via env var (overrides config file)
export TEMPORAL_MODE=standalone
export TEMPORAL_STANDALONE_DATABASE_FILE=":memory:"
export TEMPORAL_STANDALONE_ENABLE_UI=true
compozy start
```

## Test & CI Coverage

### Integration Tests to Add

**test/integration/temporal/standalone_test.go:**
- TestStandaloneMemoryMode - In-memory workflow execution
- TestStandaloneFileMode - Persistent SQLite execution
- TestStandaloneServerLifecycle - Start/stop behavior
- TestStandaloneWorkflowExecution - End-to-end workflow
- TestStandaloneMultipleNamespaces - Namespace isolation
- TestStandaloneWebUI - UI server accessible when enabled
- TestStandaloneNoUI - UI server not started when disabled
- TestStandaloneCustomPorts - Custom port configuration

**test/integration/temporal/mode_switching_test.go:**
- TestDefaultModeIsRemote - Validates default behavior
- TestStandaloneModeActivation - Config-based mode switch
- TestStandaloneStartupFailure - Error handling
- TestStandalonePortConflict - Port collision handling
- TestStandaloneTimeout - Startup timeout behavior

**test/integration/temporal/persistence_test.go:**
- TestStandalonePersistence - Workflow state survives restart
- TestStandaloneWALMode - WAL mode enabled for file-based
- TestStandaloneDatabaseCleanup - Cleanup on server stop

## Runbooks per Example

### basic/
**Prereqs:** None (zero dependencies!)
**Commands:**
```bash
cd examples/temporal-standalone/basic
compozy start &  # Start server in background
sleep 5          # Wait for startup
# Server logs show:
# - "Starting embedded Temporal server"
# - "Embedded Temporal server started successfully"
# - "Temporal Web UI: http://127.0.0.1:8233"
open http://localhost:8233  # Open Web UI
compozy workflow trigger hello --input='{"name": "Developer"}'
compozy workflow list  # See workflow history
killall compozy  # Stop server
```
**Expected Output:**
- Server starts in <5 seconds
- Web UI accessible immediately
- Workflow completes successfully
- Output: "Hello, Developer"
- Web UI shows workflow in Completed state

### persistent/
**Prereqs:** Write permissions in `./data/`
**Commands:**
```bash
cd examples/temporal-standalone/persistent
mkdir -p data
compozy start &
sleep 5
compozy workflow trigger counter
ls -lh data/temporal.db*  # Database file + WAL files
# Stop server
killall compozy
# Restart - workflow state persists
compozy start &
sleep 5
compozy workflow describe counter  # Shows workflow history
```
**Expected Output:**
- Database file created: `data/temporal.db` (~50KB)
- WAL files: `data/temporal.db-shm`, `data/temporal.db-wal`
- Workflow history survives restart
- Counter state persists

### custom-ports/
**Prereqs:** Ports 8233-8236 and 9233 available
**Commands:**
```bash
cd examples/temporal-standalone/custom-ports
compozy start &
sleep 5
# Frontend on 8233, UI on 9233
open http://localhost:9233  # Custom UI port
compozy workflow trigger demo
```
**Expected Output:**
- Server starts on custom ports
- Logs show: "frontend_addr=127.0.0.1:8233, ui_addr=http://127.0.0.1:9233"
- Web UI accessible on port 9233

### no-ui/
**Prereqs:** None
**Commands:**
```bash
cd examples/temporal-standalone/no-ui
compozy start &
sleep 3  # Faster startup without UI
# No Web UI server started
compozy workflow trigger minimal
```
**Expected Output:**
- Faster startup (<3 seconds)
- Logs show: "ui_enabled=false"
- Lower memory usage (~50MB less without UI)

### debugging/
**Prereqs:** None
**Commands:**
```bash
cd examples/temporal-standalone/debugging
compozy start &
sleep 5
# Debug logs show Temporal internals
compozy workflow trigger buggy --input='{"value": -1}'
# Workflow fails with validation error
open http://localhost:8233  # Inspect in Web UI
# Fix and retry
compozy workflow trigger fixed --input='{"value": 42"}'
```
**Expected Output:**
- Debug logs show workflow execution details
- Buggy workflow fails with clear error
- Web UI shows error details and retry attempts
- Fixed workflow completes successfully

### migration-from-remote/
**Prereqs:** Docker (for remote mode option only)
**Commands:**
```bash
cd examples/temporal-standalone/migration-from-remote
# Test standalone mode
compozy start --config=compozy.standalone.yaml &
sleep 5
compozy workflow trigger demo
killall compozy

# Test remote mode (requires Docker)
docker-compose up -d
sleep 10  # Wait for Temporal to start
compozy start --config=compozy.remote.yaml &
sleep 2
compozy workflow trigger demo
docker-compose down
```
**Expected Output:**
- Same workflow runs in both modes
- No code changes required
- Output identical regardless of mode

### integration-testing/
**Prereqs:** Go 1.23+, make
**Commands:**
```bash
cd examples/temporal-standalone/integration-testing
make test
# Or manually:
go test -v ./tests/... -race -parallel=4
```
**Expected Output:**
- All tests pass in <30 seconds
- No port conflicts (ephemeral ports used)
- Test coverage report generated
- Logs show each test gets isolated Temporal instance

## Acceptance Criteria

- [ ] All 7 example directories exist with complete files
- [ ] Each example has a comprehensive README with:
  - Purpose and key concepts
  - Prerequisites
  - Step-by-step walkthrough
  - Expected output
  - Troubleshooting section
  - Link to reference implementation
- [ ] All YAML files are valid and tested
- [ ] All examples demonstrate Web UI access (except no-ui example)
- [ ] Integration tests exist in `test/integration/temporal/`
- [ ] Tests pass in CI pipeline
- [ ] Examples are referenced in main documentation
- [ ] Code in examples follows go-coding-standards.mdc
- [ ] No secrets or credentials in example files
- [ ] .gitignore properly excludes database files and binaries
- [ ] Each example completes in <10 seconds

## Implementation Priority

1. **basic/** - Quickest win, best onboarding experience, shows Web UI
2. **persistent/** - Shows advanced use case with file-based persistence
3. **integration-testing/** - Critical for CI/CD adoption
4. **debugging/** - Showcases Web UI capabilities
5. **custom-ports/** - Solves common port conflict issues
6. **no-ui/** - Minimal resource usage option
7. **migration-from-remote/** - Eases transition

## Notes

- All examples should start in <5 seconds to highlight standalone mode's speed advantage
- Emphasize Web UI access in READMEs (major differentiator from test-only solutions)
- Use `compozy config diagnostics` in READMEs to show configuration sources
- Include "What's Next" section in each README linking to relevant docs
- Consider adding asciinema recordings for visual walkthroughs
- Link examples to specific docs sections for deeper learning
- Reference GitHub implementation: https://github.com/abtinf/temporal-a-day/blob/main/001-all-in-one-hello/main.go

## Key Messages

- **NOT Temporalite** - Uses production-grade `temporal.NewServer()`
- **Web UI included** - Better debugging experience (http://localhost:8233)
- **Four services** - Frontend, history, matching, worker (mirrors production)
- **Zero Docker** - No external dependencies for local development
- **Fast startup** - <5 seconds from start to ready

## Related Planning Artifacts
- tasks/prd-temporal/_techspec.md
- tasks/prd-temporal/_docs.md
- tasks/prd-temporal/_tests.md
