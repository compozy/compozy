## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/infra/cache, engine/infra/server</domain>
<type>implementation, integration</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>Task 1.0, Task 2.0</dependencies>
</task_context>

# Task 3.0: Mode-Aware Cache Factory

## Overview

Update the cache factory (`SetupCache`) to use mode resolution and construct the appropriate backend (external Redis or embedded miniredis) based on configuration. Also update Temporal and MCPProxy factories to use the new resolver pattern for unified mode inheritance.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
- **MUST** use `config.FromContext(ctx)` - never store config
- **MUST** use `logger.FromContext(ctx)` - never pass logger as parameter
- **NEVER** use `context.Background()` in tests, use `t.Context()` instead
</critical>

<research>
# When you need information about the existing cache setup:
- Read existing `engine/infra/cache/mod.go` to understand current SetupCache pattern
- Read `engine/infra/server/dependencies.go` to see how cache is currently initialized
- Read `engine/infra/server/temporal.go` for maybeStartStandaloneTemporal pattern
- Read `engine/infra/server/mcp.go` for shouldEmbedMCPProxy pattern
</research>

<requirements>
- Update `SetupCache()` in `engine/infra/cache/mod.go` to check effective mode
- Use `cfg.EffectiveRedisMode()` for mode determination
- Create external Redis client when mode is "distributed"
- Create MiniredisStandalone when mode is "standalone"
- Return unified cache.Cache interface for both backends
- Update `maybeStartStandaloneTemporal()` to use `cfg.EffectiveTemporalMode()`
- Update `shouldEmbedMCPProxy()` to use `cfg.EffectiveMCPProxyMode()`
- Ensure cleanup functions work for both backends
- Maintain backward compatibility (default "distributed")
</requirements>

## Subtasks

- [ ] 3.1 Update SetupCache() to read config from context
- [ ] 3.2 Add mode resolution using cfg.EffectiveRedisMode()
- [ ] 3.3 Implement distributed mode branch (existing external Redis)
- [ ] 3.4 Implement standalone mode branch (new MiniredisStandalone)
- [ ] 3.5 Update maybeStartStandaloneTemporal() to use cfg.EffectiveTemporalMode()
- [ ] 3.6 Update shouldEmbedMCPProxy() to use cfg.EffectiveMCPProxyMode()
- [ ] 3.7 Create unit tests in `engine/infra/cache/mod_test.go`
- [ ] 3.8 Create unit tests for Temporal factory pattern
- [ ] 3.9 Create unit tests for MCPProxy factory pattern

## Implementation Details

### Update SetupCache Factory

Update `engine/infra/cache/mod.go`:

```go
package cache

import (
    "context"
    "fmt"

    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/pkg/logger"
)

// SetupCache creates a mode-aware cache backend
func SetupCache(ctx context.Context) (Cache, func(), error) {
    log := logger.FromContext(ctx)
    cfg := config.FromContext(ctx)

    mode := cfg.EffectiveRedisMode()
    log.Info("Initializing cache backend", "mode", mode)

    switch mode {
    case "standalone":
        return setupStandaloneCache(ctx)
    case "distributed":
        return setupDistributedCache(ctx)
    default:
        return nil, nil, fmt.Errorf("unsupported redis mode: %s", mode)
    }
}

// setupStandaloneCache creates embedded miniredis backend
func setupStandaloneCache(ctx context.Context) (Cache, func(), error) {
    log := logger.FromContext(ctx)

    // Start embedded Redis server
    standalone, err := NewMiniredisStandalone(ctx)
    if err != nil {
        return nil, nil, fmt.Errorf("create miniredis standalone: %w", err)
    }

    // Create unified cache with miniredis client
    cache := &Redis{
        client: standalone.Client(),
    }

    lockManager := NewRedisLockManager(standalone.Client())
    notificationSystem := NewRedisNotificationSystem(standalone.Client())

    cleanup := func() {
        if err := standalone.Close(ctx); err != nil {
            log.Error("Failed to close standalone cache", "error", err)
        }
    }

    log.Info("Standalone cache initialized",
        "persistence", cfg.Redis.Standalone.Persistence.Enabled,
    )

    return &Cache{
        Redis:              cache,
        LockManager:        lockManager,
        NotificationSystem: notificationSystem,
    }, cleanup, nil
}

// setupDistributedCache creates external Redis backend
func setupDistributedCache(ctx context.Context) (Cache, func(), error) {
    log := logger.FromContext(ctx)
    cfg := config.FromContext(ctx)

    // Connect to external Redis
    opts := &redis.Options{
        Addr:     cfg.Redis.Addr,
        Password: string(cfg.Redis.Password),
    }

    client := redis.NewClient(opts)

    // Test connection
    if err := client.Ping(ctx).Err(); err != nil {
        return nil, nil, fmt.Errorf("connect to redis: %w", err)
    }

    cache := &Redis{
        client: client,
    }

    lockManager := NewRedisLockManager(client)
    notificationSystem := NewRedisNotificationSystem(client)

    cleanup := func() {
        if err := client.Close(); err != nil {
            log.Error("Failed to close redis client", "error", err)
        }
    }

    log.Info("Distributed cache initialized", "addr", cfg.Redis.Addr)

    return &Cache{
        Redis:              cache,
        LockManager:        lockManager,
        NotificationSystem: notificationSystem,
    }, cleanup, nil
}
```

### Update Temporal Factory

Update `engine/infra/server/temporal.go` (or wherever `maybeStartStandaloneTemporal` is defined):

```go
func maybeStartStandaloneTemporal(ctx context.Context) (*temporalite.Server, error) {
    cfg := config.FromContext(ctx)
    log := logger.FromContext(ctx)

    mode := cfg.EffectiveTemporalMode()
    if mode != "standalone" {
        log.Debug("Temporal mode is remote, skipping embedded server")
        return nil, nil
    }

    log.Info("Starting embedded Temporal server", "mode", "standalone")
    // ... existing implementation ...
}
```

### Update MCPProxy Factory

Update `engine/infra/server/mcp.go` (or wherever `shouldEmbedMCPProxy` is defined):

```go
func shouldEmbedMCPProxy(ctx context.Context) bool {
    cfg := config.FromContext(ctx)
    mode := cfg.EffectiveMCPProxyMode()
    return mode == "standalone"
}
```

### Relevant Files

- `engine/infra/cache/mod.go` - UPDATE - Mode-aware factory
- `engine/infra/server/dependencies.go` - UPDATE - Temporal factory usage
- `engine/infra/server/mcp.go` - UPDATE - MCPProxy factory usage

### Dependent Files

- `pkg/config/config.go` - Uses Config from Task 1.0
- `pkg/config/resolver.go` - Uses resolver from Task 1.0
- `engine/infra/cache/miniredis_standalone.go` - Uses MiniredisStandalone from Task 2.0

## Deliverables

- [ ] SetupCache() updated to use mode resolution
- [ ] setupStandaloneCache() function created for miniredis backend
- [ ] setupDistributedCache() function created for external Redis backend
- [ ] Both backends return unified Cache interface
- [ ] maybeStartStandaloneTemporal() uses cfg.EffectiveTemporalMode()
- [ ] shouldEmbedMCPProxy() uses cfg.EffectiveMCPProxyMode()
- [ ] Cleanup functions work for both backends
- [ ] All logging uses logger.FromContext(ctx)
- [ ] All config access uses config.FromContext(ctx)

## Tests

Unit tests in `engine/infra/cache/mod_test.go`:

### Mode-Aware Factory Tests

- [ ] Should create external Redis in distributed mode
  ```go
  func TestSetupCache_ModeAware(t *testing.T) {
      t.Run("Should create external Redis in distributed mode", func(t *testing.T) {
          ctx := t.Context()
          cfg := &config.Config{
              Mode: "distributed",
              Redis: config.RedisConfig{
                  Addr: "localhost:6379",
              },
          }
          ctx = config.ContextWithManager(ctx, cfg)

          cache, cleanup, err := SetupCache(ctx)
          require.NoError(t, err)
          defer cleanup()

          assert.NotNil(t, cache)
          assert.NotNil(t, cache.Redis)
          assert.NotNil(t, cache.LockManager)
          assert.NotNil(t, cache.NotificationSystem)
      })
  }
  ```

- [ ] Should create miniredis in standalone mode
  ```go
  t.Run("Should create miniredis in standalone mode", func(t *testing.T) {
      ctx := t.Context()
      cfg := &config.Config{
          Mode: "standalone",
          Redis: config.RedisConfig{
              Standalone: config.RedisStandaloneConfig{
                  Persistence: config.RedisPersistenceConfig{
                      Enabled: false,
                  },
              },
          },
      }
      ctx = config.ContextWithManager(ctx, cfg)

      cache, cleanup, err := SetupCache(ctx)
      require.NoError(t, err)
      defer cleanup()

      assert.NotNil(t, cache)
      assert.NotNil(t, cache.Redis)

      // Verify it's working by testing basic operation
      err = cache.Redis.Set(ctx, "test-key", "test-value", 0).Err()
      assert.NoError(t, err)
  })
  ```

- [ ] Should respect component mode override
  ```go
  t.Run("Should respect component mode override", func(t *testing.T) {
      ctx := t.Context()
      cfg := &config.Config{
          Mode: "distributed", // Global mode
          Redis: config.RedisConfig{
              Mode: "standalone", // Component override
          },
      }
      ctx = config.ContextWithManager(ctx, cfg)

      cache, cleanup, err := SetupCache(ctx)
      require.NoError(t, err)
      defer cleanup()

      assert.NotNil(t, cache)
      // Should be miniredis due to override
  })
  ```

- [ ] Should handle startup errors for both modes
  ```go
  t.Run("Should handle Redis connection errors", func(t *testing.T) {
      ctx := t.Context()
      cfg := &config.Config{
          Mode: "distributed",
          Redis: config.RedisConfig{
              Addr: "invalid:9999", // Invalid address
          },
      }
      ctx = config.ContextWithManager(ctx, cfg)

      _, _, err := SetupCache(ctx)
      assert.Error(t, err)
  })
  ```

- [ ] Should return proper cleanup functions
  ```go
  t.Run("Should cleanup standalone cache", func(t *testing.T) {
      ctx := t.Context()
      cfg := testConfigStandalone()
      ctx = config.ContextWithManager(ctx, cfg)

      cache, cleanup, err := SetupCache(ctx)
      require.NoError(t, err)
      assert.NotNil(t, cache)

      // Cleanup should not error
      cleanup()
  })
  ```

### Temporal Factory Tests

- [ ] Should start embedded Temporal when mode is standalone
  ```go
  func TestMaybeStartStandaloneTemporal(t *testing.T) {
      t.Run("Should start embedded Temporal in standalone mode", func(t *testing.T) {
          ctx := t.Context()
          cfg := &config.Config{Mode: "standalone"}
          ctx = config.ContextWithManager(ctx, cfg)

          server, err := maybeStartStandaloneTemporal(ctx)
          require.NoError(t, err)
          assert.NotNil(t, server)
          defer server.Stop()
      })
  }
  ```

- [ ] Should skip embedded Temporal when mode is remote
  ```go
  t.Run("Should skip embedded Temporal in remote mode", func(t *testing.T) {
      ctx := t.Context()
      cfg := &config.Config{Mode: "distributed"}
      ctx = config.ContextWithManager(ctx, cfg)

      server, err := maybeStartStandaloneTemporal(ctx)
      require.NoError(t, err)
      assert.Nil(t, server)
  })
  ```

### MCPProxy Factory Tests

- [ ] Should embed MCPProxy when mode is standalone
  ```go
  func TestShouldEmbedMCPProxy(t *testing.T) {
      t.Run("Should embed MCPProxy in standalone mode", func(t *testing.T) {
          ctx := t.Context()
          cfg := &config.Config{Mode: "standalone"}
          ctx = config.ContextWithManager(ctx, cfg)

          result := shouldEmbedMCPProxy(ctx)
          assert.True(t, result)
      })
  }
  ```

- [ ] Should skip MCPProxy when mode is distributed
  ```go
  t.Run("Should skip MCPProxy in distributed mode", func(t *testing.T) {
      ctx := t.Context()
      cfg := &config.Config{Mode: "distributed"}
      ctx = config.ContextWithManager(ctx, cfg)

      result := shouldEmbedMCPProxy(ctx)
      assert.False(t, result)
  })
  ```

## Success Criteria

- [ ] SetupCache() correctly routes to appropriate backend based on mode
- [ ] Both backends (distributed and standalone) work correctly
- [ ] Unified Cache interface returned for both modes
- [ ] Temporal factory uses cfg.EffectiveTemporalMode()
- [ ] MCPProxy factory uses cfg.EffectiveMCPProxyMode()
- [ ] Cleanup functions work for both backends
- [ ] All factory tests pass (`go test ./engine/infra/cache/... ./engine/infra/server/...`)
- [ ] `make lint` passes with zero warnings
- [ ] No context.Background() used in tests
- [ ] All logging uses logger.FromContext(ctx)
- [ ] All config access uses config.FromContext(ctx)
