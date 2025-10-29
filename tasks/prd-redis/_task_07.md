## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/infra/cache</domain>
<type>implementation</type>
<scope>persistence_layer</scope>
<complexity>medium</complexity>
<dependencies>miniredis|badgerdb</dependencies>
</task_context>

# Task 7.0: Snapshot Manager Implementation

## Overview

Implement the optional persistence layer for standalone mode using BadgerDB to create periodic snapshots of miniredis state. This provides optional durability for standalone deployments while maintaining the simplicity of in-memory Redis. The snapshot manager runs in the background, taking periodic snapshots at configurable intervals and ensuring a final snapshot on graceful shutdown.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
- **MUST** use `logger.FromContext(ctx)` for all logging - never pass logger as parameter
- **MUST** use `config.FromContext(ctx)` to read persistence configuration
- **NEVER** use `context.Background()` in runtime code - always inherit context
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Snapshot manager MUST be optional (only used when persistence.enabled = true)
- MUST use BadgerDB v4 for snapshot storage
- Periodic snapshots MUST run at configurable interval (default 5 minutes)
- Graceful shutdown MUST trigger final snapshot when configured
- Startup restore MUST load last snapshot when configured
- Snapshot operations MUST run in background (non-blocking)
- MUST use `logger.FromContext(ctx)` for all logging
- MUST use `config.FromContext(ctx)` for reading configuration
- MUST use proper goroutine lifecycle management (start/stop)
- MUST handle BadgerDB errors gracefully
- All code MUST follow `.cursor/rules/go-coding-standards.mdc`
- All code MUST follow context patterns from `.cursor/rules/global-config.mdc`
</requirements>

## Subtasks

- [x] 7.1 Create engine/infra/cache/snapshot_manager.go with SnapshotManager struct
- [x] 7.2 Implement NewSnapshotManager constructor with context patterns
- [x] 7.3 Implement Snapshot() method for creating snapshots
- [x] 7.4 Implement Restore() method for loading snapshots
- [x] 7.5 Implement StartPeriodicSnapshots() with background goroutine
- [x] 7.6 Implement Stop() method for graceful shutdown
- [x] 7.7 Add snapshot metrics (duration, size, count)
- [x] 7.8 Create unit tests in snapshot_manager_test.go
- [x] 7.9 Test periodic snapshot functionality
- [x] 7.10 Test graceful shutdown snapshot
- [x] 7.11 Test snapshot restore on startup
- [x] 7.12 Test error handling (corrupt snapshots, disk full, etc.)
- [x] 7.13 Run full test suite and ensure >80% coverage

## Implementation Details

Implement the snapshot manager as a separate component that wraps miniredis and provides optional persistence. The manager should be non-blocking and use proper goroutine lifecycle management.

### Relevant Files

- `engine/infra/cache/snapshot_manager.go` - NEW: SnapshotManager implementation
- `engine/infra/cache/snapshot_manager_test.go` - NEW: Unit tests
- `engine/infra/cache/miniredis_standalone.go` - UPDATE: Integrate snapshot manager
- `pkg/config/config.go` - Configuration already added in Task 1.0

### Dependent Files

- `engine/infra/cache/miniredis_standalone.go` - Uses snapshot manager when persistence enabled
- `pkg/config/config.go` - RedisPersistenceConfig struct
- `pkg/config/resolver.go` - Mode resolution logic

### Key Technical Details from Tech Spec

**SnapshotManager Responsibilities**:
- Create periodic snapshots of miniredis state to BadgerDB
- Restore last snapshot on startup
- Snapshot on graceful shutdown
- Non-blocking operations (background goroutine)
- Configurable snapshot interval

**BadgerDB Integration**:
- Store snapshots as key-value pairs (miniredis key → value)
- Use BadgerDB transactions for atomicity
- Store metadata (timestamp, snapshot version)
- Handle BadgerDB lifecycle (open, close)

**Configuration**:
```go
type RedisPersistenceConfig struct {
    Enabled              bool          // Enable/disable persistence
    DataDir              string        // Directory for BadgerDB storage
    SnapshotInterval     time.Duration // How often to snapshot (default 5m)
    SnapshotOnShutdown   bool          // Snapshot on graceful shutdown
    RestoreOnStartup     bool          // Restore last snapshot on startup
}
```

### Implementation Skeleton

```go
// engine/infra/cache/snapshot_manager.go

package cache

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/alicebob/miniredis/v2"
    "github.com/dgraph-io/badger/v4"

    "github.com/compozy/compozy3/pkg/config"
    "github.com/compozy/compozy3/pkg/logger"
)

type SnapshotManager struct {
    miniredis *miniredis.Miniredis
    db        *badger.DB
    stopCh    chan struct{}
    wg        sync.WaitGroup
    mu        sync.RWMutex
}

// ✅ CORRECT: No config stored, retrieved from context
func NewSnapshotManager(ctx context.Context, mr *miniredis.Miniredis, cfg config.RedisPersistenceConfig) (*SnapshotManager, error) {
    log := logger.FromContext(ctx) // ✅ MUST use context pattern

    // Open BadgerDB
    opts := badger.DefaultOptions(cfg.DataDir)
    db, err := badger.Open(opts)
    if err != nil {
        return nil, fmt.Errorf("open badger: %w", err)
    }

    log.Info("Opened BadgerDB for snapshots", "data_dir", cfg.DataDir)

    return &SnapshotManager{
        miniredis: mr,
        db:        db,
        stopCh:    make(chan struct{}),
    }, nil
}

// Snapshot creates a snapshot of current miniredis state
func (sm *SnapshotManager) Snapshot(ctx context.Context) error {
    log := logger.FromContext(ctx) // ✅ MUST use context pattern

    start := time.Now()

    // Get all keys from miniredis
    keys := sm.miniredis.Keys()

    // Write to BadgerDB in transaction
    err := sm.db.Update(func(txn *badger.Txn) error {
        // Store metadata
        metadata := map[string]string{
            "timestamp": time.Now().Format(time.RFC3339),
            "version":   "1.0",
        }

        // Write metadata
        for k, v := range metadata {
            if err := txn.Set([]byte("_meta:"+k), []byte(v)); err != nil {
                return err
            }
        }

        // Write all keys
        for _, key := range keys {
            value, _ := sm.miniredis.Get(key)
            if err := txn.Set([]byte(key), []byte(value)); err != nil {
                return err
            }
        }

        return nil
    })

    if err != nil {
        log.Error("Snapshot failed", "error", err)
        return fmt.Errorf("snapshot transaction: %w", err)
    }

    duration := time.Since(start)
    log.Info("Snapshot completed",
        "duration_ms", duration.Milliseconds(),
        "keys", len(keys),
    )

    return nil
}

// Restore loads the last snapshot into miniredis
func (sm *SnapshotManager) Restore(ctx context.Context) error {
    log := logger.FromContext(ctx) // ✅ MUST use context pattern

    start := time.Now()
    keyCount := 0

    err := sm.db.View(func(txn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        it := txn.NewIterator(opts)
        defer it.Close()

        for it.Rewind(); it.Valid(); it.Next() {
            item := it.Item()
            key := string(item.Key())

            // Skip metadata keys
            if strings.HasPrefix(key, "_meta:") {
                continue
            }

            err := item.Value(func(val []byte) error {
                sm.miniredis.Set(key, string(val))
                keyCount++
                return nil
            })
            if err != nil {
                return err
            }
        }
        return nil
    })

    if err != nil {
        log.Error("Restore failed", "error", err)
        return fmt.Errorf("restore transaction: %w", err)
    }

    duration := time.Since(start)
    log.Info("Restore completed",
        "duration_ms", duration.Milliseconds(),
        "keys", keyCount,
    )

    return nil
}

// StartPeriodicSnapshots starts background goroutine for periodic snapshots
func (sm *SnapshotManager) StartPeriodicSnapshots(ctx context.Context) {
    cfg := config.FromContext(ctx)  // ✅ MUST use context pattern
    log := logger.FromContext(ctx)  // ✅ MUST use context pattern

    interval := cfg.Redis.Standalone.Persistence.SnapshotInterval

    log.Info("Starting periodic snapshots", "interval", interval)

    sm.wg.Add(1)
    go func() {
        defer sm.wg.Done()

        ticker := time.NewTicker(interval)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                if err := sm.Snapshot(ctx); err != nil {
                    log.Error("Periodic snapshot failed", "error", err)
                }
            case <-sm.stopCh:
                log.Info("Stopping periodic snapshots")
                return
            }
        }
    }()
}

// Stop gracefully stops the snapshot manager
func (sm *SnapshotManager) Stop() {
    close(sm.stopCh)
    sm.wg.Wait()
    sm.db.Close()
}
```

## Deliverables

- `engine/infra/cache/snapshot_manager.go` - Complete SnapshotManager implementation
- `engine/infra/cache/snapshot_manager_test.go` - Comprehensive unit tests
- Updated `engine/infra/cache/miniredis_standalone.go` - Integration with snapshot manager
- Snapshot metrics added to `engine/infra/cache/metrics.go`
- Test fixtures and helpers for snapshot testing
- Documentation of snapshot format and recovery procedures

## Tests

Unit tests mapped from `_tests.md`:

- [ ] Should create snapshots of miniredis state
  - Test: Create snapshot, verify BadgerDB contains all keys
  - Test: Snapshot includes metadata (timestamp, version)
  - Test: Large datasets snapshot correctly

- [ ] Should restore snapshots to miniredis
  - Test: Restore snapshot, verify all keys present in miniredis
  - Test: Restored values match original values
  - Test: Metadata properly restored

- [ ] Should handle snapshot failures gracefully
  - Test: BadgerDB write failure doesn't crash
  - Test: Partial snapshot is rolled back
  - Test: Errors logged with proper context

- [ ] Should run periodic snapshots at configured interval
  - Test: Snapshots created at correct intervals
  - Test: Interval configurable via config
  - Test: Goroutine doesn't leak

- [ ] Should stop periodic snapshots on manager close
  - Test: Stop() terminates goroutine cleanly
  - Test: No goroutine leaks after Stop()
  - Test: WaitGroup properly synchronized

- [ ] Should create snapshot directory if missing
  - Test: DataDir created if doesn't exist
  - Test: Proper file permissions set

- [ ] Should handle corrupt snapshots gracefully
  - Test: Corrupt BadgerDB detected and handled
  - Test: Restore fails gracefully with error
  - Test: System remains operational after restore failure

- [ ] Should track snapshot metrics
  - Test: Duration metric recorded
  - Test: Size metric updated
  - Test: Success/failure count tracked

### Test Structure Example

```go
// engine/infra/cache/snapshot_manager_test.go

func TestSnapshotManager_Lifecycle(t *testing.T) {
    t.Run("Should snapshot and restore miniredis state", func(t *testing.T) {
        ctx := t.Context()
        tempDir := t.TempDir()

        // Setup miniredis with data
        mr := miniredis.NewMiniRedis()
        require.NoError(t, mr.Start())
        defer mr.Close()

        mr.Set("key1", "value1")
        mr.Set("key2", "value2")

        // Create snapshot manager
        cfg := testPersistenceConfig(tempDir)
        ctx = config.ContextWithConfig(ctx, &config.Config{
            Redis: config.RedisConfig{
                Standalone: config.RedisStandaloneConfig{
                    Persistence: cfg,
                },
            },
        })

        sm, err := NewSnapshotManager(ctx, mr, cfg)
        require.NoError(t, err)
        defer sm.Stop()

        // Take snapshot
        err = sm.Snapshot(ctx)
        require.NoError(t, err)

        // Create new miniredis
        mr2 := miniredis.NewMiniRedis()
        require.NoError(t, mr2.Start())
        defer mr2.Close()

        // Restore snapshot
        sm2, err := NewSnapshotManager(ctx, mr2, cfg)
        require.NoError(t, err)
        defer sm2.Stop()

        err = sm2.Restore(ctx)
        require.NoError(t, err)

        // Verify data restored
        val1, _ := mr2.Get("key1")
        assert.Equal(t, "value1", val1)

        val2, _ := mr2.Get("key2")
        assert.Equal(t, "value2", val2)
    })
}

func TestSnapshotManager_Periodic(t *testing.T) {
    t.Run("Should take periodic snapshots", func(t *testing.T) {
        ctx := t.Context()
        tempDir := t.TempDir()

        mr := setupMiniredis(t)
        defer mr.Close()

        // Short interval for testing
        cfg := testPersistenceConfig(tempDir)
        cfg.SnapshotInterval = 1 * time.Second

        ctx = config.ContextWithConfig(ctx, &config.Config{
            Redis: config.RedisConfig{
                Standalone: config.RedisStandaloneConfig{
                    Persistence: cfg,
                },
            },
        })

        sm, err := NewSnapshotManager(ctx, mr, cfg)
        require.NoError(t, err)
        defer sm.Stop()

        // Start periodic snapshots
        sm.StartPeriodicSnapshots(ctx)

        // Wait for at least 2 snapshots
        time.Sleep(2500 * time.Millisecond)

        // Verify snapshots were created (check BadgerDB or metrics)
        // TODO: Add verification logic
    })
}
```

## Success Criteria

- [ ] SnapshotManager implementation complete and tested
- [ ] Snapshot creation works correctly (saves to BadgerDB)
- [ ] Snapshot restore works correctly (loads from BadgerDB)
- [ ] Periodic snapshots run at configured interval
- [ ] Graceful shutdown triggers final snapshot
- [ ] Goroutine lifecycle managed properly (no leaks)
- [ ] Error handling works for all failure modes
- [ ] Snapshot metrics tracked and exposed
- [ ] Test coverage >80% for snapshot manager code
- [ ] `make lint` passes with no warnings
- [ ] `make test` passes with no failures
- [ ] All code follows context patterns (logger, config from context)
- [ ] All tests use `t.Context()` (no `context.Background()`)
- [ ] Integration with MiniredisStandalone complete
- [ ] Documentation updated with snapshot procedures

## Dependencies

- **Blocks**: Task 8.0 (Persistence Integration Tests) - requires snapshot manager implementation
- **Blocked By**: Task 2.0 (MiniredisStandalone Wrapper) - requires miniredis to be available

## Estimated Effort

**Size**: M (Medium - 1-2 days)

**Breakdown**:
- SnapshotManager struct and constructor: 2 hours
- Snapshot() implementation: 3 hours
- Restore() implementation: 3 hours
- Periodic snapshot goroutine: 2 hours
- Error handling and metrics: 2 hours
- Unit tests: 4 hours
- Integration and edge case testing: 2 hours

**Total**: ~18 hours (1-2 days)

## Risk Assessment

**Risks**:
1. BadgerDB corruption or write failures
2. Large snapshot operations blocking miniredis
3. Goroutine leaks on improper shutdown
4. Snapshot interval too aggressive causing performance issues

**Mitigations**:
1. Proper BadgerDB error handling and transaction rollback
2. Snapshots run in background goroutine (non-blocking)
3. Proper WaitGroup and channel-based shutdown
4. Default 5-minute interval, configurable for tuning
5. Metrics to monitor snapshot performance

## Validation Checklist

Before marking this task complete:

- [ ] All subtasks completed
- [ ] All tests in "Tests" section implemented and passing
- [ ] Test coverage verified (>80%)
- [ ] `make lint` passes with no warnings
- [ ] `make test` passes with no failures
- [ ] Code follows `.cursor/rules/go-coding-standards.mdc`
- [ ] Context patterns followed (logger, config from context)
- [ ] Goroutine lifecycle properly managed
- [ ] No goroutine leaks (verified with tests)
- [ ] BadgerDB integration working correctly
- [ ] Metrics properly tracked and exposed
- [ ] Documentation updated with snapshot procedures
