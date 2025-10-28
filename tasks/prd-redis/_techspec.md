# Technical Specification: Standalone Mode - Redis Alternatives

## Executive Summary

This specification details the implementation of standalone mode for Compozy, enabling single-process deployments without external Redis dependencies. The solution embeds **miniredis** (pure Go Redis server) for 100% Redis compatibility, with optional BadgerDB persistence layer for snapshots. This approach provides full feature parity including Lua scripts, TxPipeline operations, and Pub/Sub, with zero consumer code changes.

**Key Technical Decisions:**
- **Storage Backend**: miniredis v2 (in-memory Redis server in pure Go)
- **Persistence Layer**: BadgerDB v4 for optional periodic snapshots
- **Lua Scripts**: Native support via miniredis (AppendAndTrimWithMetadataScript, etc.)
- **Transactions**: Native TxPipeline support (memory store, resource store atomicity)
- **Pub/Sub**: Native Redis Pub/Sub for streaming features
- **Mode Selection**: Factory pattern in SetupCache based on configuration from context

## System Architecture

### Domain Placement

**Primary Domain**: `engine/infra/cache/`
- New adapters and providers live alongside existing Redis implementation
- Follows established package structure and naming conventions

**Affected Domains**:
- `engine/infra/server/` - Dependency injection and mode selection logic
- `engine/memory/store/` - Memory store uses cache adapter
- `engine/resources/` - Resource store uses cache adapter
- `engine/task/services/` - Task config store uses cache adapter
- `pkg/config/` - Configuration schema additions
- `pkg/mcp-proxy/` - MCP proxy storage uses cache adapter

**Testing Infrastructure**:
- `test/integration/cache/` - Cache adapter contract tests
- `test/integration/standalone/` - End-to-end standalone mode tests

### Component Overview

#### 1. MiniredisStandalone (`engine/infra/cache/miniredis_standalone.go`)
**Responsibility**: Embed and manage miniredis server lifecycle

**Key Features**:
- Starts miniredis on random available port
- Creates go-redis client pointing to embedded server
- Full Redis protocol compatibility (Lua, TxPipeline, Pub/Sub)
- Zero emulation complexity - native Redis behavior
- Graceful shutdown with optional snapshot

#### 2. SnapshotManager (`engine/infra/cache/snapshot_manager.go`)
**Responsibility**: Optional persistence layer for miniredis state

**Key Features**:
- Periodic snapshots to BadgerDB (configurable interval)
- Snapshot on graceful shutdown
- Restore last snapshot on startup
- Non-blocking snapshot operations (background goroutine)
- Configurable via `standalone.persistence.*` config

#### 3. Mode-Aware Factory (`engine/infra/cache/mod.go` - refactored)
**Responsibility**: Construct appropriate cache backend based on configuration

**Key Relationships**:
- Reads configuration from `config.FromContext(ctx)`
- Uses resolver pattern: `cfg.EffectiveRedisMode()` for mode determination
- Mode resolution priority: `redis.mode` > global `mode` > "distributed" default
- Uses `logger.FromContext(ctx)` for all logging
- Returns cache.Cache with mode-appropriate implementations
- Handles cleanup and lifecycle management

**CRITICAL PATTERN COMPLIANCE**:
- ✅ MUST use `config.FromContext(ctx)` - never store config
- ✅ MUST use `logger.FromContext(ctx)` - never pass logger as parameter
- ✅ Follow `.cursor/rules/global-config.mdc` and `.cursor/rules/logger-config.mdc`

### Data Flow

```
Server Startup
    ↓
SetupCache(ctx) reads config.FromContext(ctx)
    ↓
cfg.EffectiveRedisMode() [Resolver]
    ├─ Check redis.mode (explicit override)
    ├─ Check global mode (inheritance)
    └─ Default to "distributed"
    ↓
SetupCache(ctx) [Factory]
    ├─ [mode=distributed]
    │  ├─ Connect to external Redis
    │  ├─ NewRedis() → cache.Redis
    │  ├─ NewRedisLockManager()
    │  └─ NewRedisNotificationSystem()
    │
    └─ [mode=standalone]
       ├─ Start miniredis (embedded Redis server)
       ├─ Create go-redis client → localhost:randomPort
       ├─ NewRedis() → cache.Redis (same type!)
       ├─ NewRedisLockManager() (same!)
       ├─ NewRedisNotificationSystem() (same!)
       └─ Optional: NewSnapshotManager() for persistence
    ↓
Unified cache.Cache Interface (identical for both modes)
    ↓
Domain Services (ZERO changes - same go-redis client)
    ├─ Memory Store (Lua scripts work natively)
    ├─ Resource Store (TxPipeline works natively)
    └─ Task Store, Webhook Store (all unchanged)
```

## Implementation Design

### Key Insight: No Interface Changes Needed

**The breakthrough**: Miniredis implements the Redis protocol. Consumer code already uses `cache.RedisInterface`, which is just the go-redis client interface. We simply point the go-redis client at an embedded miniredis server instead of an external Redis server.

```go
// NO NEW INTERFACES - We use existing cache.RedisInterface
// engine/infra/cache/redis.go (already exists)

type RedisInterface interface {
    // Already defined with ~48 methods including:
    Get(ctx context.Context, key string) *redis.StringCmd
    Set(ctx context.Context, key string, value any, ttl time.Duration) *redis.StatusCmd
    Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd
    Subscribe(ctx context.Context, channels ...string) *redis.PubSub
    TxPipeline() redis.Pipeliner
    // ... all Redis commands
}

// Consumer code is UNCHANGED
// engine/memory/store/redis.go
type RedisMemoryStore struct {
    client cache.RedisInterface  // Same interface, different backend!
}

// Works identically whether client points to:
// - External Redis server (distributed mode)
// - Embedded miniredis (standalone mode)
```

### MiniredisStandalone Implementation (CORRECT PATTERNS)

```go
// engine/infra/cache/miniredis_standalone.go

type MiniredisStandalone struct {
    server   *miniredis.Miniredis
    client   *redis.Client
    snapshot *SnapshotManager
    closed   atomic.Bool
}

// ✅ CORRECT: No config stored, retrieved from context
func NewMiniredisStandalone(ctx context.Context) (*MiniredisStandalone, error) {
    log := logger.FromContext(ctx)  // ✅ MUST use context pattern
    cfg := config.FromContext(ctx)  // ✅ MUST use context pattern
    
    // Start embedded Redis server
    mr := miniredis.NewMiniRedis()
    if err := mr.Start(); err != nil {
        return nil, fmt.Errorf("start miniredis: %w", err)
    }
    
    log.Info("Started embedded Redis server",
        "addr", mr.Addr(),
        "mode", "standalone",
    )
    
    // Create standard go-redis client pointing to embedded server
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
            standalone.Close()
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

func (m *MiniredisStandalone) Client() *redis.Client {
    return m.client
}

func (m *MiniredisStandalone) Close(ctx context.Context) error {
    if !m.closed.CompareAndSwap(false, true) {
        return nil
    }
    
    log := logger.FromContext(ctx)  // ✅ MUST use context pattern
    cfg := config.FromContext(ctx)  // ✅ MUST use context pattern
    
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

### Data Models

#### Configuration Schema

```go
// pkg/config/config.go - ADD

type Config struct {
    // ... existing fields ...
    
    // Mode controls global deployment model (applies to all components by default)
    // "distributed" (default): External services required
    // "standalone": Embedded services, single-process
    // Components can override with their own mode field
    Mode string `koanf:"mode" json:"mode" yaml:"mode" mapstructure:"mode" validate:"omitempty,oneof=standalone distributed"`
    
    // Redis cache configuration
    Redis RedisConfig `koanf:"redis" json:"redis" yaml:"redis" mapstructure:"redis"`
}

type RedisConfig struct {
    // Mode controls Redis deployment model
    // "" (empty): Inherit from global Config.Mode
    // "distributed": Use external Redis (explicit override)
    // "standalone": Use embedded miniredis (explicit override)
    Mode string `koanf:"mode" json:"mode" yaml:"mode" mapstructure:"mode" validate:"omitempty,oneof=standalone distributed"`
    
    // Addr is the Redis server address (used when mode = "distributed")
    Addr string `koanf:"addr" json:"addr" yaml:"addr" mapstructure:"addr"`
    
    // Password for Redis authentication
    Password config.SensitiveString `koanf:"password" json:"password" yaml:"password" mapstructure:"password" sensitive:"true"`
    
    // Standalone configuration (used when mode = "standalone")
    Standalone RedisStandaloneConfig `koanf:"standalone" json:"standalone" yaml:"standalone" mapstructure:"standalone"`
}

type RedisStandaloneConfig struct {
    // Persistence configuration for optional BadgerDB snapshots
    Persistence RedisPersistenceConfig `koanf:"persistence" json:"persistence" yaml:"persistence" mapstructure:"persistence"`
}

type RedisPersistenceConfig struct {
    // Enabled controls whether snapshots are taken
    Enabled bool `koanf:"enabled" json:"enabled" yaml:"enabled" mapstructure:"enabled"`
    
    // DataDir is the directory for BadgerDB snapshot storage
    DataDir string `koanf:"data_dir" json:"data_dir" yaml:"data_dir" mapstructure:"data_dir"`
    
    // SnapshotInterval controls how often snapshots are taken
    SnapshotInterval time.Duration `koanf:"snapshot_interval" json:"snapshot_interval" yaml:"snapshot_interval" mapstructure:"snapshot_interval"`
    
    // SnapshotOnShutdown controls whether to snapshot during graceful shutdown
    SnapshotOnShutdown bool `koanf:"snapshot_on_shutdown" json:"snapshot_on_shutdown" yaml:"snapshot_on_shutdown" mapstructure:"snapshot_on_shutdown"`
    
    // RestoreOnStartup controls whether to restore last snapshot on startup
    RestoreOnStartup bool `koanf:"restore_on_startup" json:"restore_on_startup" yaml:"restore_on_startup" mapstructure:"restore_on_startup"`
}
```

#### Mode Resolution Logic

```go
// pkg/config/resolver.go - NEW FILE

package config

// ResolveMode determines the effective deployment mode for a component.
//
// Resolution priority:
//  1. Component mode (if explicitly set)
//  2. Global mode (if set in Config.Mode)
//  3. Default fallback ("distributed")
func ResolveMode(cfg *Config, componentMode string) string {
    if componentMode != "" {
        return componentMode // Explicit component override
    }
    if cfg.Mode != "" {
        return cfg.Mode // Inherit from global
    }
    return "distributed" // Default fallback
}

// EffectiveRedisMode returns the resolved Redis deployment mode.
// Returns "standalone" or "distributed"
func (cfg *Config) EffectiveRedisMode() string {
    return ResolveMode(cfg, cfg.Redis.Mode)
}

// EffectiveTemporalMode returns the resolved Temporal deployment mode.
// Returns "standalone" or "remote" (normalizes "distributed" -> "remote")
func (cfg *Config) EffectiveTemporalMode() string {
    mode := ResolveMode(cfg, cfg.Temporal.Mode)
    if mode == "distributed" {
        return "remote" // Temporal uses "remote" not "distributed"
    }
    return mode
}

// EffectiveMCPProxyMode returns the resolved MCPProxy deployment mode.
// Returns "standalone" or "distributed"
func (cfg *Config) EffectiveMCPProxyMode() string {
    return ResolveMode(cfg, cfg.MCPProxy.Mode)
}
```

### API Endpoints

No new API endpoints required. Existing endpoints work transparently with either backend. The go-redis client interface is identical regardless of whether it connects to external Redis or embedded miniredis.

## Integration Points

### External Libraries Assessment

#### miniredis v2
- **Repository**: github.com/alicebob/miniredis/v2
- **License**: MIT (permissive)
- **Stars**: 1k+ GitHub stars
- **Maintenance**: Actively maintained
- **Maturity**: Production-grade, widely used for testing Redis applications
- **Performance**: In-memory, ~100k+ ops/sec
- **Pros**: 
  - **100% Redis compatibility** (Lua, TxPipeline, Pub/Sub, all data structures)
  - Pure Go, no external dependencies
  - Zero emulation complexity - native Redis protocol
  - Well-tested, used by thousands of projects
- **Cons**: In-memory only (mitigated by optional BadgerDB snapshots)
- **Integration Fit**: **PERFECT** - drop-in replacement for external Redis

#### BadgerDB v4 (For Persistence Only)
- **Repository**: github.com/dgraph-io/badger/v4
- **License**: MPL-2.0 (permissive)
- **Stars**: 13k+ GitHub stars
- **Maintenance**: Actively maintained by Dgraph team
- **Maturity**: Production-grade
- **Usage**: Optional snapshot storage only (not primary storage)
- **Pros**: Pure Go, ACID transactions, proven reliability
- **Integration Fit**: Excellent for snapshot persistence

**Build vs Buy Decision**: **BUY** - miniredis eliminates 8-12 weeks of complex emulation work. It's a production-ready library that provides full Redis compatibility with zero implementation complexity.

### Migration Considerations

**From Redis to BadgerDB**: Data export/import utilities (future work)
**From BadgerDB to Redis**: Configuration change only (data not portable)

## Impact Analysis

| Affected Component | Type of Impact | Description & Risk Level | Required Action |
|-------------------|----------------|--------------------------|----------------|
| `engine/infra/cache/` | New Code | Add miniredis wrapper and snapshot manager. **Very Low risk** - simple integration. | Create 2 new files |
| `engine/infra/server/dependencies.go` | Logic Change | Add mode-aware cache setup. **Very Low risk** - small conditional. | Update SetupCache |
| `pkg/config/` | Schema Addition | Add standalone config section. **Very Low risk** - new fields only. | Update Config struct |
| `engine/memory/store/` | **No Change** | Already uses cache.RedisInterface. **Zero risk**. | **None** - works with miniredis automatically |
| `engine/resources/` | **No Change** | Already uses cache.RedisInterface. **Zero risk**. | **None** - works with miniredis automatically |
| `engine/task/services/` | **No Change** | Already uses cache.RedisInterface. **Zero risk**. | **None** - works with miniredis automatically |
| `pkg/mcp-proxy/` | **No Change** | Storage already abstracted. **Zero risk**. | **None** - transparent |
| Documentation | New Content | Add standalone mode guides. **Very Low risk**. | Write user documentation |
| Tests | New Tests | Verify Lua scripts, TxPipeline, Pub/Sub work. **Low risk**. | Integration tests only |

**Performance Impact**: Standalone mode expected to have ~similar performance to Redis for in-memory operations. Snapshot operations may cause brief latency spikes but are configurable.

## Testing Approach

### Unit Tests

**Critical Test Scenarios**:
1. MiniredisStandalone starts and stops cleanly
2. Snapshot manager creates and restores snapshots
3. Configuration validation works correctly
4. Graceful shutdown triggers final snapshot
5. Periodic snapshots run without blocking

**Mock Requirements**: None - use real miniredis and temp directories for BadgerDB

**Test Structure**:
```go
// engine/infra/cache/miniredis_standalone_test.go

func TestMiniredisStandalone_Lifecycle(t *testing.T) {
    t.Run("Should start embedded Redis server", func(t *testing.T) {
        ctx := t.Context()  // ✅ Use t.Context() in tests
        mr, err := NewMiniredisStandalone(ctx)
        require.NoError(t, err)
        defer mr.Close(ctx)
        
        // Verify connection works
        err = mr.Client().Ping(ctx).Err()
        assert.NoError(t, err)
    })
}

func TestSnapshotManager_Persistence(t *testing.T) {
    t.Run("Should snapshot and restore miniredis state", func(t *testing.T) {
        // Test snapshot/restore cycle
    })
}
```

### Integration Tests

**Test Location**: `test/integration/standalone/`

**Test Scenarios**:
1. **End-to-End Workflow Execution**: Run complete workflow in standalone mode
2. **Lua Scripts Work**: Verify AppendAndTrimWithMetadataScript executes (memory store)
3. **TxPipeline Works**: Verify atomic multi-key operations (resource store)
4. **Pub/Sub Works**: Test workflow and task event notifications (streaming)
5. **Snapshot Persistence**: Verify state survives restarts
6. **Mode Switching**: Start standalone, switch to distributed (config only)

**Test Data Requirements**:
- Sample workflows with agents, tasks, and tools
- Conversation history with multiple messages
- Resource configurations with versioning

**Validation Tests**: Verify miniredis behaves identically to external Redis for all consumer operations

## Development Sequencing

### Build Order (1-2 Weeks Total)

#### Phase 1: Core Integration (Days 1-2)
1. **Add miniredis Dependency** - Add to go.mod (`go get github.com/alicebob/miniredis/v2`)
2. **Add Configuration Schema** - Add global `mode`, `RedisConfig` with mode and standalone sections
3. **Create Mode Resolver** - Implement `pkg/config/resolver.go` with resolution logic and helper methods
4. **Create MiniredisStandalone** - Wrapper for embedded Redis server (~100 lines)
5. **Update SetupCache Factory** - Use `cfg.EffectiveRedisMode()` for mode detection
6. **Update Temporal Factory** - Use `cfg.EffectiveTemporalMode()` in `maybeStartStandaloneTemporal()`
7. **Update MCPProxy Factory** - Use `cfg.EffectiveMCPProxyMode()` in `shouldEmbedMCPProxy()`
8. **Basic Integration Test** - Verify miniredis works and mode inheritance functions correctly

**Why First**: Establishes foundation; proves miniredis compatibility and unified mode inheritance pattern immediately

#### Phase 2: Persistence Layer (Days 3-4)
6. **Create SnapshotManager** - BadgerDB integration for periodic snapshots
7. **Implement Snapshot Logic** - Save/restore miniredis state
8. **Add Graceful Shutdown** - Snapshot before exit
9. **Test Snapshot Lifecycle** - Verify persistence across restarts

**Why Second**: Optional feature; can be developed/tested independently

#### Phase 3: Validation (Days 5-6)
10. **Verify Lua Scripts** - Test AppendAndTrimWithMetadataScript (memory store)
11. **Verify TxPipeline** - Test atomic operations (resource store)
12. **Verify Pub/Sub** - Test event notifications (streaming)
13. **End-to-End Tests** - Complete workflow execution in standalone mode
14. **Performance Benchmarking** - Compare standalone vs distributed

**Why Third**: Validates that no consumer code changes are needed

#### Phase 4: Documentation & Polish (Day 7)
15. **User Documentation** - Deployment guide, configuration examples
16. **Migration Guide** - Document standalone → distributed transition
17. **CLI Improvements** - Add `--standalone` flag and error messages
18. **Example Configurations** - Sample compozy.yaml files

**Why Last**: User-facing polish after implementation is proven

### Technical Dependencies

**Blocking Dependencies**:
1. None - standalone mode is additive, doesn't break existing code
2. miniredis v2 must be added to go.mod
3. BadgerDB v4 for optional persistence (only if snapshots enabled)

**Optional Dependencies**:
- Qdrant for vector search (already optional in current architecture)

### Critical Path

```
Config Schema + Resolver (0.5d) → MiniredisStandalone (1d) → 
  Factory Pattern Update (0.5d) → Update Temporal/MCP Factories (0.5d) →
  Basic Integration Test (0.5d) → SnapshotManager (1d) → Validation Tests (1d) →
  Documentation (1d)

Total: ~6-8 days (1-2 weeks with buffer)
```

### Parallel Workstreams

**Stream A (Core)**: Days 1-2
- Config schema (global mode + RedisConfig)
- Mode resolver (pkg/config/resolver.go)
- MiniredisStandalone wrapper
- Factory updates (cache, temporal, mcpproxy)
- Basic tests

**Stream B (Persistence - Optional)**: Days 3-4
- SnapshotManager
- BadgerDB integration
- Snapshot tests

**Stream C (Validation)**: Days 5-6
- Lua script testing
- TxPipeline testing
- Pub/Sub testing
- E2E integration

**Stream D (Documentation)**: Day 7 (can start anytime)
- User guides
- Migration documentation
- Example configurations

## Monitoring & Observability

### Metrics (Prometheus Format)

```go
// engine/infra/cache/metrics.go

var (
    cacheOperations = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "compozy_cache_operations_total",
            Help: "Total cache operations by backend and operation type",
        },
        []string{"backend", "operation", "status"}, // backend=miniredis|redis
    )
    
    cacheOperationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "compozy_cache_operation_duration_seconds",
            Help: "Cache operation latency",
            Buckets: prometheus.DefBuckets,
        },
        []string{"backend", "operation"},
    )
    
    standaloneSnapshotDuration = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name: "compozy_standalone_snapshot_duration_seconds",
            Help: "Snapshot operation duration",
        },
    )
    
    standaloneSnapshotSize = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "compozy_standalone_snapshot_size_bytes",
            Help: "Last snapshot size in bytes",
        },
    )
)
```

### Key Logs (Using Context Patterns)

```go
// ✅ CORRECT: Always use logger.FromContext(ctx)
log := logger.FromContext(ctx)

log.Info("Cache backend initialized",
    "backend", "miniredis",
    "mode", "standalone",
    "persistence", cfg.Redis.Standalone.Persistence.Enabled)

log.Info("Taking periodic snapshot",
    "interval", cfg.Redis.Standalone.Persistence.SnapshotInterval)

log.Warn("Snapshot operation slow", 
    "duration_ms", duration.Milliseconds(),
    "size_mb", sizeMB)

log.Error("Snapshot failed", 
    "error", err, 
    "operation", "periodic_snapshot")
```

### Grafana Dashboard Updates

Add panel to existing dashboards:
- **Cache Operations by Backend**: Line graph showing ops/sec for miniredis vs redis
- **Snapshot Operations**: Success/failure rate over time
- **Snapshot Duration**: Histogram of snapshot operation latency
- **Snapshot Size**: Gauge showing last snapshot size

## Technical Considerations

### Key Decisions

**Decision 1: miniredis over BadgerDB Emulation**
- **Rationale**: 
  - 100% Redis compatibility (Lua, TxPipeline, Pub/Sub all work natively)
  - Production-proven library used by thousands of projects
  - Eliminates 8-12 weeks of complex emulation work
  - Zero risk of behavioral differences from Redis
- **Trade-offs**: In-memory only (mitigated by optional BadgerDB snapshots)
- **Alternatives Rejected**: 
  - BadgerDB with emulation: 8-12 weeks, high complexity, emulation bugs likely
  - Hybrid approach: Still 4-6 weeks, partial emulation needed

**Decision 2: Optional Persistence Layer**
- **Rationale**: Target users (dev, small teams) can tolerate brief data loss between snapshots
- **Trade-offs**: Not suitable for strict durability requirements (use distributed mode instead)
- **Alternatives Rejected**: 
  - WAL (Write-Ahead Log): Adds complexity, reduces performance
  - Synchronous snapshots: Block operations, poor user experience

**Decision 3: Global Mode with Component Inheritance**
- **Rationale**: 
  - Simple UX: Set `mode: standalone` once at top level
  - Flexible: Per-component overrides for mixed deployments
  - Non-breaking: Existing configs without global mode continue working
  - Follows Go composition patterns (not inheritance)
- **Trade-offs**: Requires mode resolver pattern (minimal complexity)
- **Alternatives Rejected**: 
  - Per-component only: Repetitive configuration, easy to misconfigure
  - Deployment profiles: Overkill for simple mode selection

**Decision 4: Context-First Patterns (MANDATORY)**
- **Rationale**: Project standards require `config.FromContext(ctx)` and `logger.FromContext(ctx)`
- **Trade-offs**: None - this is the established project pattern
- **Compliance**: All code examples follow `.cursor/rules/global-config.mdc` and `.cursor/rules/logger-config.mdc`

### Known Risks

**Risk 1: Data Loss Between Snapshots**
- **Challenge**: In-memory storage means data since last snapshot lost on crash
- **Mitigation**: 
  - Default 5-minute snapshot interval minimizes exposure
  - Graceful shutdown always saves snapshot
  - Target users (dev, small teams) can tolerate this
  - Production deployments use distributed mode
  - Document this limitation clearly
- **Monitoring**: Track snapshot success rate, alert on failures

**Risk 2: Memory Growth Over Time**
- **Challenge**: Long-running instances may accumulate data in memory
- **Mitigation**:
  - Existing TTL configuration on cached data
  - Document expected memory usage for typical workloads
  - Add optional memory limit configuration for miniredis
  - Monitor memory metrics
- **Monitoring**: Alert when memory usage > 80% of configured limit

**Risk 3: Snapshot Performance Impact**
- **Challenge**: Large snapshots may briefly impact performance
- **Mitigation**:
  - Snapshots run in background goroutine (non-blocking)
  - Use streaming writes to BadgerDB to minimize memory
  - Configurable snapshot interval
  - Skip snapshots if persistence disabled
- **Monitoring**: Track snapshot duration, alert if > 5 seconds

### Special Requirements

**Performance Requirements**:
- Single-user workflow latency: Similar to external Redis (in-memory)
- Throughput: Support 50+ concurrent workflows (standalone target workload)
- Memory: Baseline + ~500MB for miniredis data
- Disk: Only for optional snapshots (~1-2GB for typical deployment)

**Security Considerations**:
- BadgerDB encryption at rest for snapshots (optional, via EncryptionKey)
- File permissions: Ensure data directory is readable only by process user
- No network exposure beyond localhost (miniredis binds to 127.0.0.1)

### Standards Compliance

**Architecture Principles** (from .cursor/rules/architecture.mdc):
- ✅ **SOLID**: Wrapper pattern (OCP), Interface reuse (ISP), Context injection (DIP)
- ✅ **Clean Architecture**: Domain layer unchanged, wrapper in infrastructure layer
- ✅ **DRY**: Reuse existing RedisInterface, zero duplication

**Go Coding Standards** (from .cursor/rules/go-coding-standards.mdc):
- ✅ **Error Handling**: Context-aware errors, proper wrapping
- ✅ **Context Propagation**: context.Context as first parameter everywhere
- ✅ **Resource Cleanup**: Defer patterns, cleanup functions
- ✅ **Concurrency**: Proper goroutine lifecycle for snapshot manager

**Critical Pattern Compliance** (MANDATORY):
- ✅ **Config Access**: MUST use `config.FromContext(ctx)` - never store config
- ✅ **Logger Access**: MUST use `logger.FromContext(ctx)` - never pass as parameter
- ✅ **Test Context**: MUST use `t.Context()` in tests, never `context.Background()`

**Testing Standards** (from .cursor/rules/test-standards.mdc):
- ✅ **Unit Tests**: `t.Run("Should...")` pattern, testify assertions
- ✅ **Integration Tests**: `test/integration/` directory, cleanup in t.Cleanup()
- ✅ **No Mocks**: Use real miniredis with temp directories for BadgerDB

**No Breaking Changes**:
- ✅ Existing Redis deployments continue to work unchanged
- ✅ Default mode is "distributed" for backward compatibility
- ✅ Consumer code ZERO changes (same go-redis client interface)

## Libraries Assessment Summary

| Library | License | Stars | Maintenance | Decision | Rationale |
|---------|---------|-------|-------------|----------|-----------|
| **miniredis v2** | MIT | 1k+ | Active | **✅ ADOPT** | **Primary choice** - 100% Redis compatibility, eliminates 8-12 weeks emulation work, production-proven |
| BadgerDB v4 | MPL-2.0 | 13k+ | Active (Dgraph) | **Adopt** | Optional persistence layer for snapshots only |
| Qdrant | Apache-2.0 | 20k+ | Active | **Keep Optional** | Vector DB already optional, works in standalone mode |

**License Compatibility**: All licenses are permissive and compatible with Compozy's BSL-1.1 license.

**Implementation Complexity Comparison**:
- **miniredis approach**: 1-2 weeks, ~200 lines of code, zero emulation
- **BadgerDB emulation approach** (rejected): 8-12 weeks, ~5,000+ lines, high complexity

---

**Technical Specification Version**: 2.1  
**Created**: 2025-01-27  
**Updated**: 2025-10-28 (Major revision: miniredis approach + global mode configuration)  
**Zen MCP Analysis**: Completed with Gemini 2.5 Pro  
**Expert Review**: Validated miniredis approach, confirmed elimination of critical issues  
**Configuration Pattern**: Global mode with component inheritance (composition over inheritance)  
**Status**: ✅ Ready for Implementation with Global Mode Configuration Pattern

