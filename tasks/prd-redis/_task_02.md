## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/infra/cache</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>miniredis v2, Task 1.0</dependencies>
</task_context>

# Task 2.0: MiniredisStandalone Wrapper

## Overview

Create a lightweight wrapper around miniredis v2 that manages the embedded Redis server lifecycle and provides a standard go-redis client. This wrapper starts miniredis on a random port, creates a go-redis client connection, and handles graceful shutdown with optional snapshot support.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
- **MUST** use `config.FromContext(ctx)` - never store config
- **MUST** use `logger.FromContext(ctx)` - never pass logger as parameter
- **NEVER** use `context.Background()` in tests, use `t.Context()` instead
</critical>

<research>
# When you need information about miniredis or go-redis:
- use perplexity to find miniredis v2 API documentation
- use context7 for go-redis client patterns
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Create `engine/infra/cache/miniredis_standalone.go` wrapper
- Add miniredis v2 dependency to go.mod (`github.com/alicebob/miniredis/v2`)
- Start miniredis on random available port
- Create standard go-redis client connected to embedded server
- Test connection with Ping before returning
- Use atomic.Bool for thread-safe Close tracking
- Support graceful shutdown with optional snapshot
- Use `logger.FromContext(ctx)` for all logging
- Use `config.FromContext(ctx)` for configuration access
- Handle cleanup of miniredis server on Close
</requirements>

## Subtasks

- [x] 2.1 Add miniredis v2 dependency (`go get github.com/alicebob/miniredis/v2`)
- [x] 2.2 Create `engine/infra/cache/miniredis_standalone.go`
- [x] 2.3 Implement MiniredisStandalone struct with server, client, and snapshot fields
- [x] 2.4 Implement NewMiniredisStandalone constructor
- [x] 2.5 Implement Client() method to expose go-redis client
- [x] 2.6 Implement Close() method with graceful shutdown
- [x] 2.7 Add thread-safe close protection with atomic.Bool
- [x] 2.8 Create unit tests in `engine/infra/cache/miniredis_standalone_test.go`

## Implementation Details

### MiniredisStandalone Structure

Create `engine/infra/cache/miniredis_standalone.go`:

```go
package cache

import (
    "context"
    "fmt"
    "sync/atomic"

    "github.com/alicebob/miniredis/v2"
    "github.com/redis/go-redis/v9"

    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/pkg/logger"
)

type MiniredisStandalone struct {
    server   *miniredis.Miniredis
    client   *redis.Client
    snapshot *SnapshotManager
    closed   atomic.Bool
}

// NewMiniredisStandalone creates and starts an embedded Redis server
func NewMiniredisStandalone(ctx context.Context) (*MiniredisStandalone, error) {
    log := logger.FromContext(ctx)
    cfg := config.FromContext(ctx)

    // Start embedded Redis server
    mr := miniredis.NewMiniRedis()
    if err := mr.Start(); err != nil {
        return nil, fmt.Errorf("start miniredis: %w", err)
    }

    log.Info("Started embedded Redis server",
        "addr", mr.Addr(),
        "mode", "standalone",
    )

    // Create standard go-redis client
    client := redis.NewClient(&redis.Options{
        Addr: mr.Addr(),
    })

    // Test connection
    if err := client.Ping(ctx).Err(); err != nil {
        mr.Close()
        return nil, fmt.Errorf("ping miniredis: %w", err)
    }

    standalone := &MiniredisStandalone{
        server: mr,
        client: client,
    }

    // Initialize optional snapshot manager
    if cfg.Redis.Standalone.Persistence.Enabled {
        log.Info("Initializing persistence layer",
            "data_dir", cfg.Redis.Standalone.Persistence.DataDir,
            "snapshot_interval", cfg.Redis.Standalone.Persistence.SnapshotInterval,
        )

        snapshot, err := NewSnapshotManager(ctx, mr, cfg.Redis.Standalone.Persistence)
        if err != nil {
            standalone.Close(ctx)
            return nil, fmt.Errorf("create snapshot manager: %w", err)
        }
        standalone.snapshot = snapshot

        // Restore last snapshot if exists
        if cfg.Redis.Standalone.Persistence.RestoreOnStartup {
            if err := snapshot.Restore(ctx); err != nil {
                log.Warn("Failed to restore snapshot", "error", err)
            } else {
                log.Info("Restored last snapshot")
            }
        }

        // Start periodic snapshots
        snapshot.StartPeriodicSnapshots(ctx)
    }

    return standalone, nil
}

// Client returns the go-redis client connected to the embedded server
func (m *MiniredisStandalone) Client() *redis.Client {
    return m.client
}

// Close gracefully shuts down the embedded Redis server
func (m *MiniredisStandalone) Close(ctx context.Context) error {
    if !m.closed.CompareAndSwap(false, true) {
        return nil // Already closed
    }

    log := logger.FromContext(ctx)
    cfg := config.FromContext(ctx)

    // Snapshot before shutdown if enabled
    if m.snapshot != nil && cfg.Redis.Standalone.Persistence.SnapshotOnShutdown {
        log.Info("Taking final snapshot before shutdown")
        if err := m.snapshot.Snapshot(ctx); err != nil {
            log.Error("Failed to snapshot on shutdown", "error", err)
        }
        m.snapshot.Stop()
    }

    // Close connections
    if err := m.client.Close(); err != nil {
        log.Warn("Failed to close Redis client", "error", err)
    }

    m.server.Close()
    log.Info("Closed embedded Redis server")

    return nil
}
```

### Relevant Files

- `engine/infra/cache/miniredis_standalone.go` - NEW - MiniredisStandalone wrapper
- `go.mod` - Add miniredis v2 dependency

### Dependent Files

- `pkg/config/config.go` - Uses RedisConfig from Task 1.0
- `pkg/config/resolver.go` - Uses config from Task 1.0
- `engine/infra/cache/snapshot_manager.go` - Will be created in Task 7.0 (optional dependency)

## Deliverables

- [x] miniredis v2 dependency added to go.mod
- [x] MiniredisStandalone struct created
- [x] NewMiniredisStandalone constructor implemented
- [x] Client() method returns go-redis client
- [x] Close() method with graceful shutdown
- [x] Thread-safe close protection with atomic.Bool
- [x] Support for optional snapshot manager integration
- [x] All logging uses `logger.FromContext(ctx)`
- [x] All config access uses `config.FromContext(ctx)`
- [x] Connection tested with Ping before returning

## Tests

Unit tests in `engine/infra/cache/miniredis_standalone_test.go`:

### Lifecycle Tests

- [x] Should start and stop embedded Redis server
  ```go
  func TestMiniredisStandalone_Lifecycle(t *testing.T) {
      t.Run("Should start embedded Redis server", func(t *testing.T) {
          ctx := t.Context()
          cfg := testConfigWithStandaloneMode(false) // persistence disabled
          ctx = config.ContextWithManager(ctx, cfg)

          mr, err := NewMiniredisStandalone(ctx)
          require.NoError(t, err)
          defer mr.Close(ctx)

          // Verify connection works
          err = mr.Client().Ping(ctx).Err()
          assert.NoError(t, err)
      })
  }
  ```

- [x] Should close cleanly without errors
  ```go
  t.Run("Should close cleanly without errors", func(t *testing.T) {
      ctx := t.Context()
      cfg := testConfigWithStandaloneMode(false)
      ctx = config.ContextWithManager(ctx, cfg)

      mr, err := NewMiniredisStandalone(ctx)
      require.NoError(t, err)

      err = mr.Close(ctx)
      assert.NoError(t, err)

      // Verify double close is safe
      err = mr.Close(ctx)
      assert.NoError(t, err)
  })
  ```

- [x] Should handle startup errors gracefully
  ```go
  t.Run("Should handle startup errors gracefully", func(t *testing.T) {
      // Test error handling (e.g., invalid config)
  })
  ```

### Basic Operations Tests

- [x] Should support Get/Set operations
  ```go
  func TestMiniredisStandalone_BasicOperations(t *testing.T) {
      t.Run("Should support Get/Set operations", func(t *testing.T) {
          ctx := t.Context()
          mr := setupMiniredisForTest(ctx, t)
          defer mr.Close(ctx)

          // Test Set
          err := mr.Client().Set(ctx, "key", "value", 0).Err()
          require.NoError(t, err)

          // Test Get
          val, err := mr.Client().Get(ctx, "key").Result()
          require.NoError(t, err)
          assert.Equal(t, "value", val)
      })
  }
  ```

- [x] Should support Eval (Lua scripts)
  ```go
  t.Run("Should support Lua scripts", func(t *testing.T) {
      ctx := t.Context()
      mr := setupMiniredisForTest(ctx, t)
      defer mr.Close(ctx)

      script := `return redis.call('SET', KEYS[1], ARGV[1])`
      result, err := mr.Client().Eval(ctx, script, []string{"test-key"}, "test-value").Result()
      require.NoError(t, err)
      assert.NotNil(t, result)

      // Verify value was set
      val, err := mr.Client().Get(ctx, "test-key").Result()
      require.NoError(t, err)
      assert.Equal(t, "test-value", val)
  })
  ```

- [x] Should support TxPipeline operations
  ```go
  t.Run("Should support TxPipeline operations", func(t *testing.T) {
      ctx := t.Context()
      mr := setupMiniredisForTest(ctx, t)
      defer mr.Close(ctx)

      pipe := mr.Client().TxPipeline()
      pipe.Set(ctx, "key1", "value1", 0)
      pipe.Set(ctx, "key2", "value2", 0)

      _, err := pipe.Exec(ctx)
      require.NoError(t, err)

      // Verify both keys set
      val1, _ := mr.Client().Get(ctx, "key1").Result()
      val2, _ := mr.Client().Get(ctx, "key2").Result()
      assert.Equal(t, "value1", val1)
      assert.Equal(t, "value2", val2)
  })
  ```

### Persistence Integration Tests (Optional)

- [ ] Should initialize snapshot manager when persistence enabled
- [ ] Should skip snapshot manager when persistence disabled
- [ ] Should restore snapshot on startup when configured
- [ ] Should snapshot on shutdown when configured

## Success Criteria

- [ ] miniredis v2 dependency added successfully
- [ ] MiniredisStandalone starts and stops cleanly
- [ ] go-redis client successfully connects to embedded server
- [ ] Basic Redis operations (Get, Set) work correctly
- [ ] Lua scripts execute successfully
- [ ] TxPipeline operations work correctly
- [ ] Close() is thread-safe and idempotent
- [ ] All tests pass (`go test ./engine/infra/cache/...`)
- [ ] `make lint` passes with zero warnings
- [ ] All logging uses logger.FromContext(ctx)
- [ ] All config access uses config.FromContext(ctx)
- [ ] No context.Background() used in tests
