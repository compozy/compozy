# Task 6.0: Core Integration Tests

**Size:** L (3 days)  
**Priority:** CRITICAL - Validates core functionality  
**Dependencies:** Task 5.0

## Overview

Create comprehensive integration tests for standalone mode covering memory/file persistence, mode switching, and workflow execution.

## Deliverables

- [ ] `test/integration/temporal/standalone_test.go`
- [ ] `test/integration/temporal/mode_switching_test.go`
- [ ] `test/integration/temporal/persistence_test.go`
- [ ] Test fixtures and testdata

## Acceptance Criteria

- [ ] In-memory mode test passes (ephemeral storage)
- [ ] File-based mode test passes (persistent storage)
- [ ] Custom ports test passes
- [ ] Workflow execution test passes (end-to-end)
- [ ] Mode switching test passes (default is remote)
- [ ] Persistence test passes (restart with same database)
- [ ] All tests use `t.Context()`, not `context.Background()`
- [ ] All tests pass
- [ ] No linter errors

## Implementation Approach

See `_tests.md` "Integration Tests" section for detailed test cases.

**standalone_test.go:**
- `TestStandaloneMemoryMode` - DatabaseFile=":memory:", verify ephemeral
- `TestStandaloneFileMode` - DatabaseFile="./test.db", verify persistent
- `TestStandaloneCustomPorts` - FrontendPort=17233, verify services on custom ports
- `TestStandaloneWorkflowExecution` - Execute simple workflow end-to-end

**mode_switching_test.go:**
- `TestDefaultModeIsRemote` - No config → remote mode
- `TestStandaloneModeActivation` - Mode="standalone" → embedded starts

**persistence_test.go:**
- `TestStandalonePersistence` - Start server, create workflow, stop, restart, verify workflow still exists

## Test Patterns

Use test helpers from `test/helpers/`:
- `SetupWorkflowEnvironment()` for common setup
- Cleanup with `t.Cleanup()`
- Use temporary directories for database files

## Files to Create

- `test/integration/temporal/standalone_test.go`
- `test/integration/temporal/mode_switching_test.go`
- `test/integration/temporal/persistence_test.go`

## Notes

- Tests MUST use real embedded Temporal server (no mocks)
- Use `t.TempDir()` for database files
- Clean up server with `defer server.Stop(ctx)`
- Allow generous timeouts for CI (30s+)
- Skip slow tests with `testing.Short()`

## Validation

```bash
# Run integration tests
gotestsum --format pkgname -- -race -parallel=4 ./test/integration/temporal

# Run full test suite
make test
```
