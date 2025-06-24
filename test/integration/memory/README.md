# Memory Integration Tests

This directory contains comprehensive integration tests for the Compozy memory system.

## Test Structure

### Core Test Files

1. **helpers.go** - Test environment setup and utilities

    - `TestEnvironment` - Manages Redis, Temporal, and Memory Manager setup
    - Automatic cleanup procedures
    - Test memory configuration registration

2. **e2e_test.go** - End-to-end memory lifecycle tests

    - Complete memory lifecycle testing
    - Concurrent agent access scenarios
    - Flush workflow integration
    - Privacy metadata handling
    - Memory expiration

3. **distributed_locking_test.go** - Distributed locking tests

    - Concurrent append operations
    - Concurrent clear operations
    - Flush prevention with locks
    - Mixed operations with lock isolation
    - Lock timeout handling

4. **flush_cleanup_test.go** - Flush and cleanup workflow tests

    - Complete flush workflow with summarization
    - Multiple flush strategies (FIFO, summarization)
    - Cleanup workflow for expired memories
    - Concurrent flush and cleanup interaction

5. **token_counting_test.go** - Token counting integration tests

    - Multi-provider token counting
    - Token counting consistency
    - Token counting with flush operations
    - Edge cases and concurrency

6. **resilience_test.go** - Resilience and failure handling tests
    - Redis failure scenarios
    - Timeout handling
    - Concurrent failures
    - Memory pressure scenarios
    - Circuit breaker patterns
    - Data corruption handling
    - Privacy under failure conditions

### Support Infrastructure

7. **test_data.go** - Test data management

    - `TestDataManager` - Tracks and cleans up test instances
    - `StandardTestDataSets` - Common test scenarios
    - Helper functions for populating and verifying memory

8. **health_monitor.go** - Test environment health monitoring

    - `HealthMonitor` - Monitors Redis, Temporal, Memory Manager
    - Metrics collection and reporting
    - Alert generation on failures
    - Health check helpers for tests

9. **debug_tools.go** - Debugging and maintenance utilities
    - `DebugTools` - Captures Redis/memory state for debugging
    - `MaintenanceTools` - Cleanup and leak detection
    - `InteractiveDebugger` - Interactive debugging mode
    - Test report generation

## Running the Tests

### Basic Test Execution

```bash
# Run all integration tests
go test -v ./test/integration/memory

# Run specific test suites
go test -v ./test/integration/memory -run TestE2E
go test -v ./test/integration/memory -run TestDistributed
go test -v ./test/integration/memory -run TestResilience
```

### With Redis Available

Tests will automatically skip if Redis is not available. To run with Redis:

```bash
# Start Redis locally (using Docker)
docker run -d -p 6379:6379 redis:latest

# Run tests
go test -v ./test/integration/memory
```

### Debug Mode

Enable debug mode to capture detailed state information:

```bash
# Enable debug captures
export MEMORY_TEST_DEBUG=true
go test -v ./test/integration/memory

# Debug captures will be saved to testdata/debug/
```

### Interactive Debugging

Enable interactive debugging to pause tests and inspect state:

```bash
# Enable interactive mode
export MEMORY_TEST_INTERACTIVE=true
go test -v ./test/integration/memory -run TestInteractiveDebugger

# Available commands in interactive mode:
# - messages: Show all messages in memory
# - health: Show memory health status
# - tokens: Show token count
# - redis: List all Redis keys
# - continue: Resume test execution
```

## Test Patterns

### Graceful Skipping

All tests check for Redis availability and skip gracefully if not available:

```go
err := env.redis.Ping(ctx).Err()
if err != nil {
    t.Skipf("Redis not available: %v", err)
}
```

### Thread-Safe Operations

Tests use proper synchronization for concurrent scenarios:

```go
var wg sync.WaitGroup
var mu sync.Mutex
results := make([]Result, 0)

for i := 0; i < numWorkers; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        // Concurrent operations
        mu.Lock()
        results = append(results, result)
        mu.Unlock()
    }(i)
}
wg.Wait()
```

### Cleanup Procedures

All tests properly clean up resources:

```go
env := NewTestEnvironment(t)
defer env.Cleanup() // Automatic cleanup

// Manual cleanup for specific instances
dataHelper := NewTestDataHelper(env)
defer dataHelper.Cleanup(t)
```

## Test Data Sets

Standard test data sets are available for consistent testing:

- Basic Conversation - Simple user/assistant interaction
- Long Conversation - 50+ message conversation for capacity testing
- Multilingual Content - Tests with various languages
- Technical Support - Domain-specific conversation

## Health Monitoring

The test suite includes comprehensive health monitoring:

- Component health checks (Redis, Temporal, Memory Manager)
- Performance metrics collection
- Alert generation on failures
- HTML report generation

## Maintenance

Use maintenance tools to keep the test environment clean:

```go
// Clean up stale test data older than 1 hour
maintenance := NewMaintenanceTools(env)
maintenance.CleanupStaleData(t, 1*time.Hour)

// Verify no test data leaks
maintenance.VerifyNoLeaks(t, []string{"compozy:test-project:"})

// Generate test report
maintenance.GenerateTestReport(t, results)
```

## Contributing

When adding new integration tests:

1. Follow the existing patterns for setup and cleanup
2. Use the TestEnvironment for consistent setup
3. Include graceful skipping when dependencies are unavailable
4. Add appropriate test data cleanup procedures
5. Document any new test utilities or patterns
