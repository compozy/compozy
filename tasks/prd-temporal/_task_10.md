# Task 10.0: Advanced Integration Tests

**Size:** M (2 days)  
**Priority:** MEDIUM - Edge case coverage  
**Dependencies:** Task 5.0

## Overview

Create advanced integration tests covering error handling, startup lifecycle, and edge cases.

## Deliverables

- [ ] `test/integration/temporal/errors_test.go` - Error scenarios
- [ ] `test/integration/temporal/startup_lifecycle_test.go` - Lifecycle edge cases

## Acceptance Criteria

- [ ] Port conflict test passes
- [ ] Startup timeout test passes
- [ ] Invalid config rejection test passes
- [ ] Database corruption handling test passes
- [ ] Graceful shutdown under load test passes
- [ ] Concurrent startup/shutdown test passes
- [ ] All tests use `t.Context()`
- [ ] All tests pass
- [ ] No linter errors

## Implementation Approach

See `_tests.md` "Advanced Integration Tests" section.

**errors_test.go:**
- `TestPortConflict` - Start two servers on same port, expect error
- `TestStartupTimeout` - Very short timeout, expect deadline exceeded
- `TestInvalidDatabasePath` - Bad database path, expect error
- `TestDatabaseCorruption` - Corrupt database file, expect error
- `TestMissingDatabaseDirectory` - Database in non-existent dir, expect error

**startup_lifecycle_test.go:**
- `TestGracefulShutdownDuringStartup` - Cancel context during startup
- `TestMultipleStartCalls` - Start already running server, expect error
- `TestMultipleStopCalls` - Stop already stopped server, should be idempotent
- `TestConcurrentRequests` - Multiple workflows during shutdown
- `TestServerRestartCycle` - Start → Stop → Start → Stop sequence

## Test Patterns

- Use port allocation helpers to avoid conflicts
- Use `t.TempDir()` for isolation
- Mock slow startups with context timeouts
- Test cleanup with `t.Cleanup()`
- Use table-driven tests for error scenarios

## Files to Create

- `test/integration/temporal/errors_test.go`
- `test/integration/temporal/startup_lifecycle_test.go`

## Notes

- Error messages must be descriptive and actionable
- Port conflicts should suggest port configuration
- Database errors should suggest file permissions check
- All errors should be wrapped with context

## Validation

```bash
# Run advanced tests
gotestsum --format pkgname -- -race -parallel=4 ./test/integration/temporal

# Run full test suite
make test

# Verify error messages are helpful
```
