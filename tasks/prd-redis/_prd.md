# PRD: Standalone Mode - Redis Alternatives

## Executive Summary

Enable Compozy to run in standalone mode without external dependencies (Redis, etc.), targeting local development, small teams, and edge deployments. This feature implements embedded alternatives for all Redis-backed functionality while maintaining the distributed mode for production scalability.

## Problem Statement

### Current State
- Compozy requires external Redis for core features (caching, pub/sub, locking, memory storage)
- Complex deployment setup (Redis + PostgreSQL + Temporal + Compozy)
- High barrier to entry for new users and local development
- Over-provisioning for single-user or small team scenarios

### Pain Points
1. **Deployment Complexity**: Users must install and configure Redis before using Compozy
2. **Resource Overhead**: Redis requires additional memory/CPU for simple use cases
3. **Development Friction**: Local development requires Docker Compose or external services
4. **Cost**: Cloud deployments incur Redis hosting costs even for light usage

### Target Users
- **Developers**: Local development and testing without Docker Compose
- **Small Teams**: 1-10 users deploying on single VM or container
- **Edge Deployments**: IoT, embedded systems, air-gapped environments
- **Evaluators**: Trying Compozy without infrastructure commitment

## Goals & Non-Goals

### Goals
1. **Single Binary Deployment**: `compozy start --standalone` works with zero external dependencies (except PostgreSQL)
2. **Feature Parity**: All core features functional in standalone mode (agents, workflows, tasks, memory, tools)
3. **Performance Adequacy**: Acceptable performance for single-user and small team workloads (10-100 req/sec)
4. **Seamless Upgrade**: Clear migration path from standalone to distributed mode as needs grow
5. **Backward Compatibility**: Existing Redis-based deployments continue to work unchanged

### Non-Goals
1. **Horizontal Scaling**: Standalone mode is single-process only (no distributed workloads)
2. **Production High-Availability**: No replication, failover, or clustering in standalone
3. **Performance Parity**: Standalone may be slower than Redis for high-concurrency workloads
4. **Hybrid Mode**: Cannot mix standalone and distributed backends within same deployment

## Success Metrics

### Primary Metrics
- **Installation Success Rate**: >95% of users successfully start standalone mode on first attempt
- **Feature Completeness**: 100% of core features work in standalone mode
- **Performance**: Single-user workflows complete within 1.5x of Redis time
- **Adoption**: 30%+ of new deployments use standalone mode within 3 months

### Secondary Metrics
- **Documentation Quality**: <5% of support questions relate to standalone setup
- **Migration Success**: >90% of users successfully migrate to distributed mode when needed
- **Resource Usage**: Standalone uses <50% memory/CPU vs Redis-based deployment for equivalent workload

## User Stories

### US-1: Local Development
**As a** developer  
**I want to** run Compozy locally without external dependencies  
**So that** I can develop and test workflows quickly

**Acceptance Criteria**:
- Run `compozy start --standalone` and server starts successfully
- All CLI commands work (agents, workflows, tools, tasks)
- Workflow execution with memory and tools functions correctly
- Data persists across restarts using local filesystem

---

### US-2: Small Team Deployment
**As a** small team lead  
**I want to** deploy Compozy on a single VM without managing Redis  
**So that** we can start using AI workflows immediately

**Acceptance Criteria**:
- Deploy single Docker container or binary
- Configuration through environment variables or YAML
- Multi-user access (authentication required)
- Performance adequate for 5-10 concurrent workflows

---

### US-3: Migration to Production
**As a** platform engineer  
**I want to** migrate from standalone to distributed mode  
**So that** we can scale as usage grows

**Acceptance Criteria**:
- Clear documentation on migration process
- Configuration changes clearly documented
- Data export/import utilities available
- No workflow rewrites required

---

### US-4: Edge Deployment
**As an** IoT engineer  
**I want to** run Compozy on resource-constrained edge devices  
**So that** we can process workflows locally without cloud dependencies

**Acceptance Criteria**:
- Binary size <100MB
- Memory footprint <512MB for idle server
- Works on ARM64 and x86_64
- Configurable data retention policies

## Technical Requirements

### Functional Requirements

#### FR-1: Embedded Redis Server (miniredis)
- Use miniredis (pure Go in-memory Redis implementation)
- 100% Redis protocol compatibility including Lua scripts
- Support all existing Redis operations without code changes
- Native Pub/Sub and transaction (TxPipeline) support

#### FR-2: Optional Persistence Layer
- BadgerDB for periodic snapshots of miniredis state
- Configurable snapshot interval (default: 5 minutes)
- Snapshot on graceful shutdown
- Restore last snapshot on startup

#### FR-3: Memory Store Compatibility
- All existing Lua scripts work natively (AppendAndTrimWithMetadataScript, etc.)
- TxPipeline operations maintain atomicity guarantees
- Conversation history consistency (messages + metadata)
- No consumer code changes required

#### FR-4: Resource Store Compatibility
- Lua script-based optimistic locking (PutIfMatch) works natively
- TxPipeline for atomic multi-key operations (value + etag)
- Watch notifications via Redis Pub/Sub
- No consumer code changes required

#### FR-5: Streaming Features
- Redis Pub/Sub for real-time events
- Pattern subscriptions for workflow/task events
- Native go-redis PubSub types
- No emulation complexity

#### FR-6: Configuration Management
- Add global `mode` configuration field (standalone | distributed)
- Support per-component mode overrides for mixed deployments
- Mode inheritance: component mode > global mode > "distributed" default
- Provide standalone-specific configuration section (`redis.standalone.*`)
- Support environment variable overrides
- Validate mode-specific requirements

### Non-Functional Requirements

#### NFR-1: Performance
- Single-user workflow latency: <2x Redis baseline
- Throughput: Support 10-100 requests/sec
- Memory usage: <512MB for typical standalone workload
- Disk I/O: Optimize for SSD, acceptable on HDD

#### NFR-2: Reliability
- Data durability: No data loss on graceful shutdown
- Error handling: Proper recovery from BadgerDB errors
- Graceful degradation: Inform users of limitations

#### NFR-3: Maintainability
- Clean adapter interfaces following existing patterns
- Comprehensive unit and integration tests
- Clear separation between mode-specific code
- Documentation for future contributors

#### NFR-4: Compatibility
- Backward compatible with existing Redis deployments
- No breaking changes to APIs or configurations
- Default mode remains "distributed" for production

## Architecture Overview

### Components

#### 1. Miniredis Integration (`engine/infra/cache/miniredis_standalone.go`)
- Embeds miniredis v2 (pure Go Redis server)
- Starts in-memory Redis on random available port
- Standard go-redis client connects to embedded server
- Zero emulation complexity - full Redis compatibility

#### 2. Snapshot Manager (`engine/infra/cache/snapshot_manager.go`)
- Periodically saves miniredis state to BadgerDB
- Configurable snapshot interval (default: 5min)
- Graceful snapshot on server shutdown
- Restores last snapshot on startup
- Optional (can run purely in-memory)

#### 3. Mode-Aware Factory (`engine/infra/cache/mod.go`)
- SetupCache reads configuration from config.FromContext(ctx)
- Uses resolver pattern: cfg.EffectiveRedisMode() for mode determination
- Mode resolution: redis.mode > global mode > "distributed" default
- Constructs appropriate backend (Redis or miniredis)
- Returns unified cache.Cache interface
- Uses logger.FromContext(ctx) for all logging

### Data Flow

```
User Request
    ↓
Server Dependencies (dependencies.go)
    ↓
SetupCache (mode-aware factory)
    ├─ cfg.EffectiveRedisMode() [resolver]
    │  ├─ Check redis.mode (explicit override)
    │  ├─ Check global mode (inheritance)
    │  └─ Default to "distributed"
    ↓
    ├→ [distributed] External Redis Client
    └→ [standalone] Embedded miniredis + go-redis Client
    ↓
Unified Cache Interface (go-redis)
    ↓
Domain Services (memory, resources, tasks)
    ↓
    [standalone only]
    ↓
Periodic Snapshot Manager → BadgerDB (optional persistence)
```

### Configuration Schema

```yaml
# Global deployment mode (applies to all components by default)
mode: standalone  # or "distributed"

# Component-specific mode overrides (optional)
redis:
  mode: ""  # empty = inherit from global; "standalone" | "distributed" = explicit override
  addr: localhost:6379  # used when mode = "distributed"
  standalone:
    # Optional persistence (can run purely in-memory)
    persistence:
      enabled: true
      data_dir: ./compozy-data
      snapshot_interval: 5m      # Save state periodically
      snapshot_on_shutdown: true # Save on graceful exit
      restore_on_startup: true   # Restore last snapshot

temporal:
  mode: ""  # empty = inherit from global
  host_port: localhost:7233

mcpproxy:
  mode: ""  # empty = inherit from global
  host: 127.0.0.1
  port: 6001
```

**Mode Resolution**:
- Component mode takes precedence if explicitly set
- Otherwise inherits from global `mode`
- Default fallback is "distributed" if neither is set

## Implementation Phases

### Phase 1: Core Integration (Day 1-2)
1. Add miniredis dependency to go.mod
2. Create MiniredisStandalone wrapper
3. Implement mode-aware factory in SetupCache
4. Add configuration schema for standalone mode
5. Basic integration tests

### Phase 2: Optional Persistence (Day 3-4)
6. Create SnapshotManager for BadgerDB integration
7. Implement periodic snapshot logic
8. Implement graceful shutdown snapshot
9. Implement startup restore logic
10. Snapshot lifecycle tests

### Phase 3: Testing & Validation (Day 5-6)
11. Verify all Lua scripts work (memory store)
12. Verify TxPipeline operations (resource store)
13. Verify Pub/Sub for streaming
14. End-to-end integration tests
15. Performance benchmarking

### Phase 4: Documentation & Polish (Day 7)
16. User documentation (deployment guide)
17. Migration guide (standalone → distributed)
18. CLI improvements (`--standalone` flag)
19. Example configurations

**Total Timeline: 1-2 weeks (vs original 9-10 weeks)**

## Risks & Mitigations

### Risk 1: Data Loss on Crash (Between Snapshots)
**Risk**: Miniredis is in-memory; data since last snapshot lost on unexpected crash  
**Impact**: Potential loss of recent workflow state or messages  
**Mitigation**:
- Default 5-minute snapshot interval minimizes exposure window
- Graceful shutdown always saves snapshot
- Target use cases (dev, small teams) can tolerate 5min data loss
- Document this limitation clearly
- Production deployments use distributed mode

### Risk 2: Memory Growth
**Risk**: Long-running standalone instances accumulate data in memory  
**Impact**: Increased memory usage over time  
**Mitigation**:
- Configure TTLs on all cached data (already in place)
- Document memory expectations for typical workloads
- Add optional memory limit configuration for miniredis
- Monitor memory usage metrics

### Risk 3: Snapshot Performance Impact
**Risk**: Large snapshots may cause brief latency spikes  
**Impact**: Slow request processing during snapshot  
**Mitigation**:
- Snapshot operation is non-blocking (background goroutine)
- Use BadgerDB streaming writes to minimize memory
- Make snapshot interval configurable
- Skip snapshots if persistence disabled

### Risk 4: Migration Complexity
**Risk**: Cannot migrate data from miniredis snapshots to external Redis  
**Impact**: Must rebuild state when switching modes  
**Mitigation**:
- Document that mode switch requires clean start
- Provide export/import utilities for workflows and configs
- Agent memory can be re-initialized
- Configuration files remain unchanged

## Dependencies

### External Libraries
- **miniredis v2** (github.com/alicebob/miniredis/v2) - MIT license, pure Go Redis server
- **BadgerDB v4** (github.com/dgraph-io/badger/v4) - MPL-2.0 license, for optional persistence

### Internal Dependencies
- engine/infra/cache - Core cache abstraction layer
- engine/infra/server - Dependency injection and initialization
- pkg/config - Configuration management
- engine/memory/store - Memory store interfaces
- engine/resources - Resource store interfaces

### No Breaking Changes
- All existing code continues to work
- Redis remains default backend
- New code uses existing interfaces

## Open Questions

1. **Vector Database**: Should Qdrant be required in standalone mode, or make it optional?
2. **Snapshot Frequency**: What's the optimal default snapshot interval (5min, 10min, 15min)?
3. **Snapshot Size Limits**: Should we enforce max snapshot size to prevent BadgerDB bloat?
4. **Memory Limits**: Should miniredis have configurable memory limits in standalone mode?
5. **Monitoring**: Do we need separate metrics for standalone vs distributed?

## Future Considerations

### Post-MVP Enhancements
- Automatic mode switching based on workload detection
- Hybrid mode (local cache + remote Redis)
- Replication support for standalone high-availability
- S3-backed persistence for BadgerDB
- Kubernetes operator with automatic mode selection

### Alternative Libraries
- Pebble (CockroachDB) - Higher performance alternative to BadgerDB
- NutsDB - Redis-like data structures in Go
- SQLite - Universal embedded database (less performant for cache)

## Glossary

- **Standalone Mode**: Single-process deployment with embedded dependencies
- **Distributed Mode**: Multi-instance deployment with external Redis
- **Adapter**: Implementation of cache interfaces for specific backend
- **BadgerDB**: LSM-tree based embedded key-value store
- **TTL**: Time-to-live, automatic key expiration
- **Atomic Operation**: Multiple operations guaranteed to execute together or not at all

---

**Document Version**: 1.0  
**Created**: 2025-01-27  
**Last Updated**: 2025-01-27  
**Status**: Approved  
**Stakeholders**: Engineering, Product, DevOps

