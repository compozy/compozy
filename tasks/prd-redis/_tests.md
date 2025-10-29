# Tests Plan: Standalone Mode - Redis Alternatives

## Guiding Principles

- Follow `.cursor/rules/test-standards.mdc` and project testing rules
- Use `t.Run("Should …")` naming convention with testify assertions
- Use `t.Context()` for test contexts (never `context.Background()`)
- No mocks for internal components - use real miniredis and temp directories
- Mock external services only when necessary
- Ensure all tests are deterministic and can run in parallel where safe

## Coverage Matrix

Map PRD acceptance criteria to test files:

| PRD Criterion | Test File | Test Type |
|---------------|-----------|-----------|
| FR-1: Embedded Redis (miniredis) | `engine/infra/cache/miniredis_standalone_test.go` | Unit |
| FR-2: Optional Persistence | `engine/infra/cache/snapshot_manager_test.go` | Unit |
| FR-3: Memory Store Compatibility | `engine/memory/store/redis_test.go` | Integration |
| FR-4: Resource Store Compatibility | `engine/resources/redis_store_test.go` | Integration |
| FR-5: Streaming Features | `test/integration/standalone/streaming_test.go` | Integration |
| FR-6: Configuration Management | `pkg/config/resolver_test.go` | Unit |
| Mode Resolution Logic | `pkg/config/resolver_test.go` | Unit |
| Factory Pattern | `engine/infra/cache/mod_test.go` | Unit |
| End-to-End Workflow | `test/integration/standalone/workflow_test.go` | Integration |

## Unit Tests

### pkg/config/resolver_test.go (NEW)
**Purpose**: Test mode resolution logic and helper methods

- Should return component mode when explicitly set
- Should return global mode when component mode is empty
- Should return "distributed" default when both are empty
- Should normalize "distributed" to "remote" for Temporal
- Should validate mode values against allowed enums
- Should handle mixed mode configurations correctly
- Should resolve effective modes for all components (Redis, Temporal, MCPProxy)

**Test Structure**:
```go
func TestResolveMode(t *testing.T) {
    t.Run("Should return component mode when explicitly set", func(t *testing.T) {
        cfg := &Config{
            Mode: "standalone",
            Redis: RedisConfig{Mode: "distributed"},
        }
        result := cfg.EffectiveRedisMode()
        assert.Equal(t, "distributed", result)
    })
    
    t.Run("Should inherit from global mode", func(t *testing.T) {
        cfg := &Config{
            Mode: "standalone",
            Redis: RedisConfig{Mode: ""},
        }
        result := cfg.EffectiveRedisMode()
        assert.Equal(t, "standalone", result)
    })
    
    t.Run("Should default to distributed", func(t *testing.T) {
        cfg := &Config{
            Mode: "",
            Redis: RedisConfig{Mode: ""},
        }
        result := cfg.EffectiveRedisMode()
        assert.Equal(t, "distributed", result)
    })
}

func TestEffectiveTemporalMode(t *testing.T) {
    t.Run("Should normalize distributed to remote for Temporal", func(t *testing.T) {
        cfg := &Config{Mode: "distributed"}
        result := cfg.EffectiveTemporalMode()
        assert.Equal(t, "remote", result)
    })
}
```

### engine/infra/cache/miniredis_standalone_test.go (NEW)
**Purpose**: Test MiniredisStandalone lifecycle and operations

- Should start embedded Redis server successfully
- Should create working go-redis client connected to miniredis
- Should handle startup errors gracefully
- Should close cleanly without errors
- Should support all Redis operations (Get, Set, Eval, TxPipeline)
- Should initialize snapshot manager when persistence enabled
- Should skip snapshot manager when persistence disabled
- Should restore snapshot on startup when configured
- Should snapshot on shutdown when configured

**Test Structure**:
```go
func TestMiniredisStandalone_Lifecycle(t *testing.T) {
    t.Run("Should start and stop embedded Redis server", func(t *testing.T) {
        ctx := t.Context()
        // Setup test config with persistence disabled
        cfg := testConfig(false)
        ctx = config.ContextWithManager(ctx, cfg)
        
        mr, err := NewMiniredisStandalone(ctx)
        require.NoError(t, err)
        defer mr.Close(ctx)
        
        // Verify client works
        err = mr.Client().Ping(ctx).Err()
        assert.NoError(t, err)
    })
}

func TestMiniredisStandalone_Operations(t *testing.T) {
    t.Run("Should support basic Redis operations", func(t *testing.T) {
        ctx := t.Context()
        mr := setupMiniredis(ctx, t)
        defer mr.Close(ctx)
        
        // Test Set/Get
        err := mr.Client().Set(ctx, "key", "value", 0).Err()
        require.NoError(t, err)
        
        val, err := mr.Client().Get(ctx, "key").Result()
        require.NoError(t, err)
        assert.Equal(t, "value", val)
    })
    
    t.Run("Should support Lua scripts", func(t *testing.T) {
        // Test Eval operation
    })
    
    t.Run("Should support TxPipeline", func(t *testing.T) {
        // Test transaction pipeline
    })
}
```

### engine/infra/cache/snapshot_manager_test.go (NEW)
**Purpose**: Test BadgerDB snapshot and restore operations

- Should create snapshots of miniredis state
- Should restore snapshots to miniredis
- Should handle snapshot failures gracefully
- Should run periodic snapshots at configured interval
- Should stop periodic snapshots on manager close
- Should create snapshot directory if missing
- Should handle corrupt snapshots gracefully
- Should track snapshot metrics (size, duration, count)

**Test Structure**:
```go
func TestSnapshotManager_Lifecycle(t *testing.T) {
    t.Run("Should snapshot and restore miniredis state", func(t *testing.T) {
        ctx := t.Context()
        tempDir := t.TempDir()
        
        // Create miniredis with data
        mr := setupMiniredisWithData(ctx, t)
        
        // Create snapshot manager
        sm, err := NewSnapshotManager(ctx, mr, persistenceConfig(tempDir))
        require.NoError(t, err)
        defer sm.Close()
        
        // Take snapshot
        err = sm.Snapshot(ctx)
        require.NoError(t, err)
        
        // Create new miniredis
        mr2 := setupMiniredis(ctx, t)
        defer mr2.Close(ctx)
        
        // Restore snapshot
        sm2, _ := NewSnapshotManager(ctx, mr2, persistenceConfig(tempDir))
        err = sm2.Restore(ctx)
        require.NoError(t, err)
        
        // Verify data restored
        verifyDataRestored(ctx, t, mr2)
    })
}

func TestSnapshotManager_Periodic(t *testing.T) {
    t.Run("Should take periodic snapshots", func(t *testing.T) {
        // Test with short interval (1s for testing)
        // Verify multiple snapshots created
    })
    
    t.Run("Should stop periodic snapshots on close", func(t *testing.T) {
        // Test cleanup
    })
}
```

### engine/infra/cache/mod_test.go (UPDATE)
**Purpose**: Test mode-aware factory pattern

- Should create external Redis client when mode is distributed
- Should create miniredis client when mode is standalone
- Should respect mode resolution from config
- Should initialize snapshot manager for standalone with persistence
- Should skip snapshot manager for standalone without persistence
- Should return proper cleanup functions
- Should handle startup errors for both modes

**Test Structure**:
```go
func TestSetupCache_ModeAware(t *testing.T) {
    t.Run("Should create external Redis in distributed mode", func(t *testing.T) {
        ctx := t.Context()
        cfg := configWithMode("distributed")
        ctx = config.ContextWithManager(ctx, cfg)
        
        cache, cleanup, err := SetupCache(ctx)
        require.NoError(t, err)
        defer cleanup()
        
        assert.NotNil(t, cache)
        // Verify it's external Redis
    })
    
    t.Run("Should create miniredis in standalone mode", func(t *testing.T) {
        ctx := t.Context()
        cfg := configWithMode("standalone")
        ctx = config.ContextWithManager(ctx, cfg)
        
        cache, cleanup, err := SetupCache(ctx)
        require.NoError(t, err)
        defer cleanup()
        
        assert.NotNil(t, cache)
        // Verify it's miniredis
    })
}
```

### pkg/config/loader_test.go (UPDATE)
**Purpose**: Test configuration validation for modes

- Should validate global mode field (standalone | distributed)
- Should validate component mode fields
- Should reject invalid mode values
- Should allow empty mode values (inheritance)
- Should validate Redis persistence configuration
- Should validate mode-specific requirements (e.g., MCPProxy port in standalone)

## Integration Tests

### test/integration/standalone/workflow_test.go (NEW)
**Purpose**: End-to-end workflow execution in standalone mode

- Should execute complete workflow with agent, tasks, and tools
- Should persist conversation history across workflow steps
- Should handle workflow state correctly
- Should execute multiple workflows concurrently
- Should handle workflow errors and retries

**Test Structure**:
```go
func TestStandaloneWorkflow_EndToEnd(t *testing.T) {
    t.Run("Should execute workflow in standalone mode", func(t *testing.T) {
        ctx := t.Context()
        
        // Setup test environment with standalone config
        env := setupStandaloneTestEnv(ctx, t)
        defer env.Cleanup()
        
        // Load and execute workflow
        result, err := env.ExecuteWorkflow(ctx, "test-workflow")
        require.NoError(t, err)
        assert.NotNil(t, result)
        
        // Verify workflow completed successfully
        assert.Equal(t, "completed", result.Status)
    })
}
```

### test/integration/standalone/memory_store_test.go (NEW)
**Purpose**: Verify memory store compatibility with miniredis

- Should append messages to conversation history
- Should trim conversation history at max length
- Should preserve message metadata
- Should handle concurrent message appends
- Should execute Lua scripts (AppendAndTrimWithMetadataScript) correctly
- Should maintain consistency across operations

**Test Structure**:
```go
func TestMemoryStore_MiniredisCompatibility(t *testing.T) {
    t.Run("Should execute Lua scripts natively", func(t *testing.T) {
        ctx := t.Context()
        store := setupMemoryStoreWithMiniredis(ctx, t)
        
        // Test AppendAndTrimWithMetadataScript
        err := store.AppendMessage(ctx, agentID, message)
        require.NoError(t, err)
        
        // Verify message stored with metadata
        messages, err := store.GetMessages(ctx, agentID)
        require.NoError(t, err)
        assert.Len(t, messages, 1)
    })
}
```

### test/integration/standalone/resource_store_test.go (NEW)
**Purpose**: Verify resource store compatibility with miniredis

- Should store and retrieve resources atomically
- Should handle optimistic locking (PutIfMatch) via Lua scripts
- Should support TxPipeline for multi-key operations
- Should publish watch notifications via Pub/Sub
- Should maintain ETags correctly
- Should handle concurrent resource updates

**Test Structure**:
```go
func TestResourceStore_MiniredisCompatibility(t *testing.T) {
    t.Run("Should support TxPipeline operations", func(t *testing.T) {
        ctx := t.Context()
        store := setupResourceStoreWithMiniredis(ctx, t)
        
        // Test atomic multi-key operation
        err := store.PutWithETag(ctx, resource)
        require.NoError(t, err)
        
        // Verify ETag stored atomically
        retrieved, err := store.Get(ctx, resource.ID)
        require.NoError(t, err)
        assert.Equal(t, resource.ETag, retrieved.ETag)
    })
    
    t.Run("Should publish watch notifications", func(t *testing.T) {
        // Test Pub/Sub notifications
    })
}
```

### test/integration/standalone/streaming_test.go (NEW)
**Purpose**: Verify streaming and Pub/Sub functionality

- Should publish task events via Redis Pub/Sub
- Should subscribe to workflow events
- Should support pattern subscriptions
- Should handle multiple subscribers
- Should deliver events reliably

**Test Structure**:
```go
func TestStreaming_MiniredisCompatibility(t *testing.T) {
    t.Run("Should publish and subscribe to events", func(t *testing.T) {
        ctx := t.Context()
        publisher := setupPublisherWithMiniredis(ctx, t)
        subscriber := setupSubscriberWithMiniredis(ctx, t)
        
        // Subscribe to channel
        events := make(chan Event, 10)
        err := subscriber.Subscribe(ctx, "workflow:*", events)
        require.NoError(t, err)
        
        // Publish event
        err = publisher.Publish(ctx, "workflow:123", testEvent)
        require.NoError(t, err)
        
        // Verify event received
        select {
        case evt := <-events:
            assert.Equal(t, testEvent, evt)
        case <-time.After(5 * time.Second):
            t.Fatal("Event not received")
        }
    })
}
```

### test/integration/standalone/persistence_test.go (NEW)
**Purpose**: Test snapshot persistence across restarts

- Should persist data to BadgerDB snapshots
- Should restore data from snapshots on startup
- Should handle graceful shutdown snapshots
- Should handle periodic snapshots
- Should recover from snapshot failures

**Test Structure**:
```go
func TestPersistence_SnapshotRestore(t *testing.T) {
    t.Run("Should persist and restore data across restarts", func(t *testing.T) {
        ctx := t.Context()
        tempDir := t.TempDir()
        
        // Phase 1: Create data and snapshot
        {
            env := setupStandaloneWithPersistence(ctx, t, tempDir)
            
            // Store data
            storeTestData(ctx, t, env)
            
            // Trigger snapshot
            env.TriggerSnapshot(ctx)
            
            // Clean shutdown
            env.Shutdown(ctx)
        }
        
        // Phase 2: Restore and verify
        {
            env := setupStandaloneWithPersistence(ctx, t, tempDir)
            defer env.Shutdown(ctx)
            
            // Verify data restored
            verifyTestDataRestored(ctx, t, env)
        }
    })
}
```

### test/integration/standalone/mode_switching_test.go (NEW)
**Purpose**: Test switching between modes (config-only, no data migration)

- Should start in standalone mode
- Should start in distributed mode
- Should handle invalid mode configurations
- Should respect mode overrides

## Fixtures & Testdata

Add fixtures under `test/fixtures/standalone/`:

- `minimal-config.yaml` - Minimal standalone configuration
- `with-persistence-config.yaml` - Standalone with persistence
- `mixed-mode-config.yaml` - Mixed mode configuration
- `workflows/test-workflow.yaml` - Sample workflow for integration tests
- `workflows/stateful-workflow.yaml` - Workflow with memory usage

## Mocks & Stubs

**No mocks needed for internal components** - use real implementations:
- Use real miniredis (in-memory, fast)
- Use real BadgerDB with temp directories
- Use real memory/resource stores

**Mock external services only**:
- LLM providers (use test providers from `test/helpers/`)
- External APIs called by tools
- External MCP servers (if testing MCP integration)

## Contract Tests

### Cache Adapter Contract Tests
Location: `test/integration/cache/adapter_contract_test.go` (UPDATE)

Add test cases for miniredis adapter:
- Should satisfy cache.RedisInterface contract
- Should support all 48 interface methods
- Should behave identically to external Redis adapter
- Should handle error cases consistently

**Test Structure**:
```go
// Run the same test suite against both Redis and miniredis
func TestCacheAdapter_Contract(t *testing.T) {
    adapters := []struct {
        name  string
        setup func(t *testing.T) cache.RedisInterface
    }{
        {"ExternalRedis", setupExternalRedis},
        {"Miniredis", setupMiniredis},
    }
    
    for _, adapter := range adapters {
        t.Run(adapter.name, func(t *testing.T) {
            client := adapter.setup(t)
            
            // Run same tests against both
            testBasicOperations(t, client)
            testLuaScripts(t, client)
            testTxPipeline(t, client)
            testPubSub(t, client)
        })
    }
}
```

## Observability Assertions

### Metrics Presence Tests
Location: `engine/infra/cache/metrics_test.go` (NEW)

- Should increment cache operation counters (by backend and operation)
- Should record operation duration histograms
- Should track snapshot duration and size metrics
- Should label metrics with correct backend ("miniredis" vs "redis")

### Log Output Tests
Location: Integration tests

- Should log "Started embedded Redis server" with address
- Should log "Initializing persistence layer" when enabled
- Should log "Taking periodic snapshot" at intervals
- Should log "Taking final snapshot before shutdown"
- Should log errors with proper context

### Trace Span Tests (if applicable)
- Should create spans for cache operations
- Should propagate context through cache calls
- Should record span attributes (backend, operation, keys)

## Performance & Limits

### Performance Tests
Location: `test/integration/standalone/performance_test.go` (NEW)

- Should handle 100+ ops/sec in standalone mode
- Should complete workflow within 1.5x of Redis time
- Should use <512MB memory for typical workload
- Should complete snapshots within 5 seconds (warn threshold)

**Test Structure**:
```go
func TestPerformance_Standalone(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping performance test in short mode")
    }
    
    t.Run("Should handle 100 ops/sec", func(t *testing.T) {
        ctx := t.Context()
        env := setupStandaloneTestEnv(ctx, t)
        defer env.Cleanup()
        
        // Run 1000 operations
        start := time.Now()
        for i := 0; i < 1000; i++ {
            env.Cache.Set(ctx, fmt.Sprintf("key%d", i), "value", 0)
        }
        duration := time.Since(start)
        
        opsPerSec := 1000 / duration.Seconds()
        assert.GreaterOrEqual(t, opsPerSec, 100.0)
    })
}
```

### Memory Limits Tests
- Should not exceed configured memory limits
- Should enforce TTLs correctly
- Should clean up expired keys

## CLI Tests (Goldens)

### Config Commands
Location: `cli/cmd/config/config_test.go` (UPDATE)

- Should show mode field in `compozy config show`
- Should validate mode configurations in `compozy config validate`
- Should display mode resolution in `compozy config diagnostics`

**Golden Files**:
- `testdata/config-show-standalone.golden` - Expected output for standalone config
- `testdata/config-show-mixed.golden` - Expected output for mixed mode config

### Start Command
Location: `cli/cmd/start/start_test.go` (UPDATE)

- Should accept `--mode standalone` flag
- Should accept `--mode distributed` flag
- Should prioritize config file over CLI flags
- Should show mode in startup logs

## Exit Criteria

- [ ] All unit tests exist and pass (`make test`)
- [ ] All integration tests exist and pass (`make test-all`)
- [ ] Contract tests verify miniredis behaves identically to Redis
- [ ] Performance tests validate NFRs (100 ops/sec, <1.5x latency)
- [ ] Memory tests verify <512MB footprint
- [ ] Metrics, logs, and traces are properly emitted
- [ ] CLI tests with goldens are updated
- [ ] All tests use `t.Context()` (no `context.Background()`)
- [ ] All tests follow `t.Run("Should ...")` naming convention
- [ ] Test coverage >80% for new code
- [ ] CI pipeline updated to run standalone integration tests
- [ ] Flaky tests identified and fixed
- [ ] All tests are deterministic and parallelizable where safe

## CI/CD Integration

Update `.github/workflows/test.yml`:

```yaml
- name: Run Standalone Integration Tests
  run: |
    go test -v -race -tags=integration ./test/integration/standalone/...
  env:
    POSTGRES_HOST: localhost
    POSTGRES_PORT: 5432
```

No Docker Compose needed for standalone tests (uses miniredis in-memory).

## Test Environment Helpers

Create `test/helpers/standalone.go` (NEW):

```go
// SetupStandaloneTestEnv creates a complete test environment in standalone mode
func SetupStandaloneTestEnv(ctx context.Context, t *testing.T) *TestEnv {
    // Setup config with mode: standalone
    // Initialize cache with miniredis
    // Setup database (testcontainers or in-memory)
    // Return configured environment
}

// SetupMiniredisWithData creates miniredis pre-populated with test data
func SetupMiniredisWithData(ctx context.Context, t *testing.T) *MiniredisStandalone {
    // Create miniredis
    // Populate with test data
    // Return configured miniredis
}
```

## Test Data Generators

Create `test/fixtures/generators.go` (NEW):

```go
// GenerateTestWorkflow creates a minimal test workflow
func GenerateTestWorkflow() *workflow.Config { ... }

// GenerateTestMessages creates sample conversation messages
func GenerateTestMessages(count int) []*memory.Message { ... }

// GenerateTestResources creates sample resources with ETags
func GenerateTestResources(count int) []*resources.Resource { ... }
```

## Test Cleanup

All tests must use `t.Cleanup()` for resource cleanup:

```go
func TestExample(t *testing.T) {
    tempDir := t.TempDir()  // Auto-cleanup
    
    mr, _ := NewMiniredisStandalone(ctx)
    t.Cleanup(func() {
        mr.Close(ctx)
    })
    
    // Test code
}
```

## Acceptance Criteria Summary

- [ ] 100% of PRD acceptance criteria have corresponding tests
- [ ] All tests pass locally and in CI
- [ ] Test coverage >80% for `engine/infra/cache/` new code
- [ ] Mode resolution logic has 100% coverage
- [ ] MiniredisStandalone lifecycle fully tested
- [ ] SnapshotManager fully tested with edge cases
- [ ] Memory/resource store compatibility verified
- [ ] Streaming Pub/Sub compatibility verified
- [ ] End-to-end workflow execution tested
- [ ] Performance benchmarks meet NFRs
- [ ] CLI tests updated with mode flags
- [ ] Contract tests prove behavioral parity with Redis
- [ ] All tests follow project standards (naming, assertions, context usage)
- [ ] No flaky tests in standalone test suite

