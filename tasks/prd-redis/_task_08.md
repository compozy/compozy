## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>test/integration/standalone</domain>
<type>testing</type>
<scope>integration_testing</scope>
<complexity>medium</complexity>
<dependencies>snapshot_manager|badgerdb</dependencies>
</task_context>

# Task 8.0: Persistence Integration Tests

## Overview

Create comprehensive integration tests for the persistence layer, validating the full snapshot/restore cycle, data persistence across restarts, snapshot failure handling, and corrupt snapshot recovery. These tests ensure that the optional BadgerDB persistence layer works correctly in real-world scenarios and handles edge cases gracefully.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Integration tests MUST verify full snapshot/restore cycle
- MUST test data persistence across simulated restarts
- MUST test snapshot failure handling and recovery
- MUST test corrupt snapshot detection and recovery
- MUST test periodic snapshot behavior under load
- MUST test graceful shutdown snapshot creation
- All tests MUST use t.Context() (never context.Background())
- All tests MUST follow "Should..." naming convention with testify assertions
- MUST use real miniredis and real BadgerDB (no mocks)
- Tests MUST use temp directories (t.TempDir())
- Tests MUST clean up resources with t.Cleanup()
</requirements>

## Subtasks

- [ ] 8.1 Create test/integration/standalone/persistence_test.go with test suite
- [ ] 8.2 Test full snapshot/restore cycle (complete data persistence)
- [ ] 8.3 Test data persistence across simulated restarts
- [ ] 8.4 Test snapshot failure handling (disk full, BadgerDB errors)
- [ ] 8.5 Test corrupt snapshot detection and recovery
- [ ] 8.6 Test periodic snapshot behavior under concurrent load
- [ ] 8.7 Test graceful shutdown snapshot creation
- [ ] 8.8 Test snapshot restore on startup (cold start scenario)
- [ ] 8.9 Add test fixtures and data generators
- [ ] 8.10 Run full test suite and ensure >80% coverage for integration code

## Implementation Details

This task creates integration tests that validate the snapshot manager's behavior in real-world scenarios, including edge cases and failure modes.

### Relevant Files

- `test/integration/standalone/persistence_test.go` - NEW: Integration tests for persistence
- `engine/infra/cache/snapshot_manager.go` - Created in Task 7.0, tested here
- `engine/infra/cache/miniredis_standalone.go` - MiniredisStandalone with persistence
- `test/helpers/standalone.go` - Test environment helpers

### Dependent Files

- `engine/infra/cache/snapshot_manager.go` - Snapshot manager implementation
- `engine/infra/cache/miniredis_standalone.go` - MiniredisStandalone wrapper
- `pkg/config/config.go` - RedisPersistenceConfig

### Key Technical Details from Tech Spec

**Persistence Testing Focus**:
- Full lifecycle: start → populate data → snapshot → shutdown → restart → verify data
- Edge cases: corrupt snapshots, disk full, BadgerDB errors
- Concurrency: snapshots under load, concurrent reads/writes
- Configuration: different snapshot intervals, restore options

**Test Environment Requirements**:
- Temp directories for BadgerDB (t.TempDir())
- Ability to simulate restarts (close and re-open)
- Ability to inject failures (corrupt files, disk errors)
- Proper cleanup (t.Cleanup())

## Deliverables

- `test/integration/standalone/persistence_test.go` - Full integration test suite
- Test fixtures and data generators in `test/fixtures/standalone/`
- Helper functions in `test/helpers/standalone.go` for persistence testing
- Documentation of test scenarios and expected behaviors
- Updated CI pipeline in `.github/workflows/test.yml` (if needed)

## Tests

Integration tests mapped from `_tests.md`:

- [ ] Should persist and restore data across full cycle
  - Test: Create miniredis, populate data, snapshot, close
  - Test: Create new miniredis, restore snapshot, verify data identical
  - Test: Large datasets (1000+ keys) persist correctly

- [ ] Should persist data across simulated restarts
  - Test: Phase 1 - start, populate, snapshot, graceful shutdown
  - Test: Phase 2 - restart, restore, verify data persisted
  - Test: Multiple restart cycles maintain data integrity

- [ ] Should handle snapshot failures gracefully
  - Test: Disk full during snapshot (mock filesystem full)
  - Test: BadgerDB write error during snapshot
  - Test: Snapshot manager continues working after failure
  - Test: Next snapshot succeeds after previous failure

- [ ] Should detect and recover from corrupt snapshots
  - Test: Corrupt BadgerDB file (truncate, corrupt bytes)
  - Test: Restore fails gracefully with error
  - Test: System remains operational with empty state
  - Test: Fresh snapshot can be created after corruption

- [ ] Should handle periodic snapshots under load
  - Test: Continuous writes during periodic snapshots
  - Test: No data loss between snapshots
  - Test: Snapshots don't block operations
  - Test: Performance acceptable during snapshots

- [ ] Should create snapshot on graceful shutdown
  - Test: Configure snapshot_on_shutdown: true
  - Test: Trigger shutdown, verify final snapshot created
  - Test: Restored data includes all data up to shutdown

- [ ] Should restore snapshot on startup when configured
  - Test: Configure restore_on_startup: true
  - Test: Start with existing snapshot, verify data restored
  - Test: Start without snapshot, system initializes empty

### Test Structure Example

```go
// test/integration/standalone/persistence_test.go

func TestPersistence_FullCycle(t *testing.T) {
    t.Run("Should persist and restore data across full cycle", func(t *testing.T) {
        ctx := t.Context()
        tempDir := t.TempDir()

        // Phase 1: Create data and snapshot
        testData := map[string]string{
            "user:1": "alice",
            "user:2": "bob",
            "count":  "42",
        }

        {
            env := setupStandaloneWithPersistence(ctx, t, tempDir)

            // Populate data
            for k, v := range testData {
                err := env.Client.Set(ctx, k, v, 0).Err()
                require.NoError(t, err)
            }

            // Trigger snapshot
            err := env.SnapshotManager.Snapshot(ctx)
            require.NoError(t, err)

            // Clean shutdown
            env.Shutdown(ctx)
        }

        // Phase 2: Restore and verify
        {
            env := setupStandaloneWithPersistence(ctx, t, tempDir)
            defer env.Shutdown(ctx)

            // Restore snapshot
            err := env.SnapshotManager.Restore(ctx)
            require.NoError(t, err)

            // Verify all data restored
            for k, expectedVal := range testData {
                val, err := env.Client.Get(ctx, k).Result()
                require.NoError(t, err)
                assert.Equal(t, expectedVal, val, "Key %s value mismatch", k)
            }
        }
    })

    t.Run("Should persist data across multiple restarts", func(t *testing.T) {
        ctx := t.Context()
        tempDir := t.TempDir()

        // Multiple restart cycles
        for cycle := 1; cycle <= 3; cycle++ {
            env := setupStandaloneWithPersistence(ctx, t, tempDir)

            // Add data in this cycle
            key := fmt.Sprintf("cycle:%d", cycle)
            err := env.Client.Set(ctx, key, fmt.Sprintf("data-%d", cycle), 0).Err()
            require.NoError(t, err)

            // Snapshot and shutdown
            err = env.SnapshotManager.Snapshot(ctx)
            require.NoError(t, err)
            env.Shutdown(ctx)
        }

        // Final restore - verify all cycles' data present
        env := setupStandaloneWithPersistence(ctx, t, tempDir)
        defer env.Shutdown(ctx)

        err := env.SnapshotManager.Restore(ctx)
        require.NoError(t, err)

        for cycle := 1; cycle <= 3; cycle++ {
            key := fmt.Sprintf("cycle:%d", cycle)
            val, err := env.Client.Get(ctx, key).Result()
            require.NoError(t, err)
            assert.Equal(t, fmt.Sprintf("data-%d", cycle), val)
        }
    })
}

func TestPersistence_FailureHandling(t *testing.T) {
    t.Run("Should handle snapshot failures gracefully", func(t *testing.T) {
        ctx := t.Context()
        tempDir := t.TempDir()

        env := setupStandaloneWithPersistence(ctx, t, tempDir)
        defer env.Shutdown(ctx)

        // Populate data
        env.Client.Set(ctx, "key1", "value1", 0)

        // Simulate disk full by making directory read-only
        err := os.Chmod(tempDir, 0444)
        require.NoError(t, err)

        // Snapshot should fail
        err = env.SnapshotManager.Snapshot(ctx)
        assert.Error(t, err)

        // Restore write permissions
        err = os.Chmod(tempDir, 0755)
        require.NoError(t, err)

        // Next snapshot should succeed
        env.Client.Set(ctx, "key2", "value2", 0)
        err = env.SnapshotManager.Snapshot(ctx)
        assert.NoError(t, err)
    })

    t.Run("Should recover from corrupt snapshot", func(t *testing.T) {
        ctx := t.Context()
        tempDir := t.TempDir()

        // Phase 1: Create snapshot
        {
            env := setupStandaloneWithPersistence(ctx, t, tempDir)
            env.Client.Set(ctx, "key1", "value1", 0)
            err := env.SnapshotManager.Snapshot(ctx)
            require.NoError(t, err)
            env.Shutdown(ctx)
        }

        // Corrupt the snapshot (truncate BadgerDB files)
        files, err := os.ReadDir(tempDir)
        require.NoError(t, err)
        if len(files) > 0 {
            filePath := filepath.Join(tempDir, files[0].Name())
            err = os.Truncate(filePath, 0)
            require.NoError(t, err)
        }

        // Phase 2: Restore should fail gracefully
        {
            env := setupStandaloneWithPersistence(ctx, t, tempDir)
            defer env.Shutdown(ctx)

            err := env.SnapshotManager.Restore(ctx)
            assert.Error(t, err, "Restore should fail with corrupt snapshot")

            // System should remain operational (empty state)
            _, err = env.Client.Get(ctx, "key1").Result()
            assert.Error(t, err) // Key not found (empty state)

            // Should be able to create new data and snapshot
            env.Client.Set(ctx, "key2", "value2", 0)
            err = env.SnapshotManager.Snapshot(ctx)
            assert.NoError(t, err, "Should create new snapshot after corruption")
        }
    })
}

func TestPersistence_PeriodicSnapshots(t *testing.T) {
    t.Run("Should take periodic snapshots under load", func(t *testing.T) {
        ctx := t.Context()
        tempDir := t.TempDir()

        // Short interval for testing
        env := setupStandaloneWithPeriodicSnapshots(ctx, t, tempDir, 2*time.Second)
        defer env.Shutdown(ctx)

        // Start periodic snapshots
        env.SnapshotManager.StartPeriodicSnapshots(ctx)

        // Continuous writes
        stopCh := make(chan struct{})
        var writeCount atomic.Int64

        go func() {
            for {
                select {
                case <-stopCh:
                    return
                default:
                    count := writeCount.Add(1)
                    key := fmt.Sprintf("key:%d", count)
                    env.Client.Set(ctx, key, fmt.Sprintf("value:%d", count), 0)
                    time.Sleep(10 * time.Millisecond)
                }
            }
        }()

        // Wait for at least 2 periodic snapshots
        time.Sleep(5 * time.Second)
        close(stopCh)

        // Final snapshot
        err := env.SnapshotManager.Snapshot(ctx)
        require.NoError(t, err)

        finalCount := writeCount.Load()
        t.Logf("Wrote %d keys during periodic snapshots", finalCount)

        // Shutdown and restore
        env.Shutdown(ctx)

        env2 := setupStandaloneWithPersistence(ctx, t, tempDir)
        defer env2.Shutdown(ctx)

        err = env2.SnapshotManager.Restore(ctx)
        require.NoError(t, err)

        // Verify data restored (check sample keys)
        for i := int64(1); i <= finalCount; i += 100 {
            key := fmt.Sprintf("key:%d", i)
            _, err := env2.Client.Get(ctx, key).Result()
            assert.NoError(t, err, "Key %s should exist", key)
        }
    })
}

func TestPersistence_GracefulShutdown(t *testing.T) {
    t.Run("Should snapshot on graceful shutdown", func(t *testing.T) {
        ctx := t.Context()
        tempDir := t.TempDir()

        // Phase 1: Create data and shutdown (should auto-snapshot)
        {
            env := setupStandaloneWithPersistence(ctx, t, tempDir)

            // Populate data
            for i := 0; i < 100; i++ {
                key := fmt.Sprintf("key:%d", i)
                env.Client.Set(ctx, key, fmt.Sprintf("value:%d", i), 0)
            }

            // Graceful shutdown (should trigger final snapshot)
            env.Shutdown(ctx)
        }

        // Phase 2: Restore and verify shutdown snapshot was created
        {
            env := setupStandaloneWithPersistence(ctx, t, tempDir)
            defer env.Shutdown(ctx)

            err := env.SnapshotManager.Restore(ctx)
            require.NoError(t, err)

            // Verify all data from shutdown snapshot
            for i := 0; i < 100; i++ {
                key := fmt.Sprintf("key:%d", i)
                val, err := env.Client.Get(ctx, key).Result()
                require.NoError(t, err)
                assert.Equal(t, fmt.Sprintf("value:%d", i), val)
            }
        }
    })
}

// Helper functions
func setupStandaloneWithPersistence(ctx context.Context, t *testing.T, dataDir string) *PersistenceTestEnv {
    // Create config with persistence enabled
    // Setup miniredis with snapshot manager
    // Return test environment with cleanup
}

func setupStandaloneWithPeriodicSnapshots(ctx context.Context, t *testing.T, dataDir string, interval time.Duration) *PersistenceTestEnv {
    // Setup with custom snapshot interval for testing
}
```

## Success Criteria

- [ ] All integration tests pass with persistence layer
- [ ] Full snapshot/restore cycle works correctly
- [ ] Data persists across simulated restarts
- [ ] Snapshot failures handled gracefully (system remains operational)
- [ ] Corrupt snapshots detected and recovered from
- [ ] Periodic snapshots work under concurrent load
- [ ] Graceful shutdown creates final snapshot
- [ ] Startup restore works when configured
- [ ] Test coverage >80% for integration code
- [ ] `make test` passes with no failures
- [ ] All tests use `t.Context()` (no `context.Background()`)
- [ ] All tests follow "Should..." naming convention
- [ ] Tests are deterministic (no flaky tests)
- [ ] Test output clearly shows persistence behavior
- [ ] Documentation updated with test scenarios

## Dependencies

- **Blocks**: Task 9.0 (End-to-End Workflow Tests) - requires persistence validation
- **Blocked By**: Task 7.0 (Snapshot Manager Implementation) - requires snapshot manager

## Estimated Effort

**Size**: M (Medium - 1 day)

**Breakdown**:
- Test suite setup: 2 hours
- Full cycle tests: 2 hours
- Restart simulation tests: 2 hours
- Failure handling tests: 2 hours
- Corruption recovery tests: 2 hours
- Load testing and periodic snapshots: 2 hours
- Documentation and cleanup: 1 hour

**Total**: ~13 hours (1 day)

## Risk Assessment

**Risks**:
1. Tests may be flaky due to timing issues with periodic snapshots
2. File system operations may behave differently across platforms
3. Large test data may slow down test suite
4. Cleanup failures may leave temp files

**Mitigations**:
1. Use deterministic delays and synchronization (WaitGroups, channels)
2. Test on multiple platforms (CI covers Linux, macOS, Windows)
3. Use reasonable dataset sizes (balance coverage vs speed)
4. Always use t.TempDir() and t.Cleanup() for automatic cleanup
5. Add timeout protections to prevent hanging tests

## Validation Checklist

Before marking this task complete:

- [ ] All subtasks completed
- [ ] All tests in "Tests" section implemented and passing
- [ ] Test coverage verified (>80%)
- [ ] `make lint` passes with no warnings
- [ ] `make test` passes with no failures
- [ ] Integration tests added to CI pipeline
- [ ] Code follows `.cursor/rules/test-standards.mdc`
- [ ] All uses of context follow patterns (t.Context() in tests)
- [ ] Test fixtures and helpers properly organized
- [ ] No flaky tests (all tests deterministic)
- [ ] Tests run successfully on CI (multiple platforms)
- [ ] Documentation updated with test scenarios
- [ ] Cleanup verified (no leaked temp files or goroutines)
