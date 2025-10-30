# Technical Specification: Three-Mode Configuration System

## Executive Summary

Replace the current two-mode system (standalone/distributed) with a clearer three-mode system (memory/persistent/distributed) to improve developer experience, reduce friction for new users, and dramatically speed up test execution.

**Modes:**
- **memory** (NEW DEFAULT): In-memory SQLite + embedded Temporal + embedded Redis (no persistence)
- **persistent**: File-based SQLite + embedded Temporal + embedded Redis (with persistence)
- **distributed**: PostgreSQL + external Temporal + external Redis (production)

**Key Benefits:**
- 50-80% faster test suite (no testcontainers/Docker startup)
- Zero-dependency quickstart (`compozy start` just works)
- Clearer intent-based naming (mode matches use case)
- Simpler onboarding for new developers

**Impact:**
- Breaking change (acceptable in alpha, no backwards compatibility)
- ~40 files to update across 6 implementation phases
- Estimated effort: 6 days with proper phasing

---

## Table of Contents

1. [Current State Analysis](#current-state-analysis)
2. [Proposed Architecture](#proposed-architecture)
3. [Implementation Plan](#implementation-plan)
4. [Phase-by-Phase Details](#phase-by-phase-details)
5. [Testing Strategy](#testing-strategy)
6. [Risk Mitigation](#risk-mitigation)
7. [Success Metrics](#success-metrics)
8. [Migration Guide](#migration-guide)

---

## Current State Analysis

### Existing Mode System

**Mode Constants** (`pkg/config/resolver.go`):
```go
const (
    ModeStandalone  = "standalone"
    ModeDistributed = "distributed"
    ModeRemoteTemporal = "remote"
)
```

**Default Mode:** `ModeDistributed` (line 26 in resolver.go)
- Requires external PostgreSQL, Redis, and Temporal
- High barrier to entry for new users
- Slow test execution (testcontainers startup overhead)

**Database Driver Selection** (`pkg/config/resolver.go:49-65`):
```go
func (cfg *Config) EffectiveDatabaseDriver() string {
    if cfg.Database.Driver != "" {
        return cfg.Database.Driver
    }
    if cfg.Mode == ModeStandalone {
        return databaseDriverSQLite
    }
    return databaseDriverPostgres  // Default
}
```

### Infrastructure Components

**1. Database Layer:**
- PostgreSQL: Production-ready with pgvector support
- SQLite: Full feature parity via migrations, no pgvector

**2. Temporal Layer:**
- External: Production clusters (mode="remote")
- Embedded: In-process server (mode="standalone")
  - Default DB: `:memory:`
  - Configurable via `temporal.standalone.database_file`

**3. Redis/Cache Layer:**
- External: Production Redis clusters (mode="distributed")
- Embedded: Miniredis with optional BadgerDB persistence (mode="standalone")
  - Implemented in Redis PRD (Tasks 1.0-13.0)
  - `MiniredisStandalone` wrapper with `SnapshotManager`

### Test Infrastructure

**Current Pattern:**
```go
// Most tests use testcontainers with Postgres
pool, cleanup := helpers.GetSharedPostgresDB(t)
// Spins up pgvector/pgvector:pg16 container
```

**Impact:**
- Slow startup time (~10-30 seconds per test run)
- Docker dependency for local development
- CI/CD resource overhead

**Alternative Available:**
```go
// SQLite tests (faster, but not default)
provider, cleanup := helpers.SetupTestDatabase(t, "sqlite")
```

### Known Limitations

**SQLite Constraints:**
1. No pgvector support (hard error if configured)
2. Write concurrency limit (~10 concurrent workflows recommended)
3. Single writer at a time (serialized writes)
4. Requires external vector DB for knowledge/RAG features

**Documented Validation** (`engine/infra/server/dependencies.go:122-161`):
```go
func (s *Server) validateDatabaseConfig(cfg *config.Config) error {
    if driver != driverSQLite {
        return nil
    }
    // Validates:
    // - No pgvector with SQLite
    // - Warns if >10 concurrent workflows
    // - Warns if no vector DB configured
}
```

---

## Proposed Architecture

### Three-Mode System

```
┌─────────────────────────────────────────────────────────────┐
│                        Mode System                           │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │   MEMORY    │  │ PERSISTENT  │  │ DISTRIBUTED │         │
│  │  (default)  │  │             │  │             │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│        │                │                 │                  │
│        ▼                ▼                 ▼                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │   SQLite    │  │   SQLite    │  │  Postgres   │         │
│  │  :memory:   │  │   file      │  │  external   │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│        │                │                 │                  │
│        ▼                ▼                 ▼                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  Temporal   │  │  Temporal   │  │  Temporal   │         │
│  │  embedded   │  │  embedded   │  │  external   │         │
│  │  :memory:   │  │   file      │  │   remote    │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│        │                │                 │                  │
│        ▼                ▼                 ▼                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │   Redis     │  │   Redis     │  │   Redis     │         │
│  │  Miniredis  │  │  Miniredis  │  │  external   │         │
│  │  no persist │  │  + BadgerDB │  │   cluster   │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### Mode Characteristics

| Aspect | memory | persistent | distributed |
|--------|--------|-----------|-------------|
| **Database** | SQLite :memory: | SQLite file | PostgreSQL |
| **Temporal** | Embedded :memory: | Embedded file | External |
| **Redis** | Miniredis ephemeral | Miniredis + BadgerDB | External |
| **Persistence** | None | Survives restarts | Full production |
| **Startup** | Instant | Instant | Requires services |
| **Test Speed** | Fastest | Fast | Slow (containers) |
| **Use Case** | Tests, quick dev | Local dev, debug | Production |
| **Data Loss Risk** | On restart | On disk failure | Replicated |

### Configuration Inheritance

**Resolution Hierarchy:**
```
Component Mode (explicit) 
    ↓ (if not set)
Global Mode (config.Mode)
    ↓ (if not set)
Default (memory)  ← NEW DEFAULT
```

**Example:**
```yaml
# Implicit memory mode (all components use memory)
name: my-workflow

# OR explicit mode
mode: persistent
database:
  # Driver auto-selected: sqlite
  path: ./.compozy/compozy.db
temporal:
  # Mode inherited: persistent
  standalone:
    database_file: ./.compozy/temporal.db
redis:
  # Mode inherited: persistent
  standalone:
    persistence:
      enabled: true  # Auto-enabled
      data_dir: ./.compozy/redis
```

---

## Implementation Plan

### Overview

```
Phase 1: Core Config        [Days 1-2] CRITICAL
    ↓
Phase 2: Infrastructure     [Days 2-3] HIGH
    ↓
Phase 3: Test Migration     [Days 3-4] HIGH
    ↓
Phase 4: Documentation      [Days 4-5] MEDIUM
    ↓
Phase 5: Schemas            [Day 5]    MEDIUM
    ↓
Phase 6: Final Validation   [Day 6]    CRITICAL
```

### File Change Summary

```
Core Configuration (7 files):
  pkg/config/resolver.go
  pkg/config/config.go
  pkg/config/definition/schema.go
  pkg/config/loader.go
  pkg/config/resolver_test.go
  pkg/config/config_test.go
  pkg/config/loader_test.go

Infrastructure (4 files):
  engine/infra/cache/mod.go
  engine/infra/server/dependencies.go
  engine/infra/server/server.go
  engine/infra/server/temporal_resolver_test.go

Test Infrastructure (6+ files):
  test/helpers/standalone.go
  test/helpers/database.go
  test/integration/standalone/helpers.go
  test/integration/temporal/mode_switching_test.go
  testdata/*.golden (3 files)
  + integration test updates

Documentation (8+ files):
  docs/content/docs/deployment/*.mdx
  docs/content/docs/configuration/*.mdx
  docs/content/docs/guides/*.mdx
  docs/content/docs/quick-start/*.mdx
  cli/help/global-flags.md

Examples (2-3 files):
  examples/memory-mode/ (renamed from standalone)
  examples/persistent-mode/ (new)
  examples/README.md

Schemas (2 files):
  schemas/config.json
  schemas/compozy.json

TOTAL: ~40 files
```

---

## Phase-by-Phase Details

### Phase 1: Core Configuration [CRITICAL]

**Priority:** BLOCKING - All other work depends on this

**Duration:** 1-2 days

**Goal:** Core configuration system supports three modes with proper validation and resolution

#### 1.1 Update Mode Constants

**File:** `pkg/config/resolver.go`

**Changes:**

**Lines 6-11** - Replace mode constants:
```go
// BEFORE:
const (
    ModeStandalone  = "standalone"
    ModeDistributed = "distributed"
    ModeRemoteTemporal = "remote"
)

// AFTER:
const (
    ModeMemory      = "memory"       // In-memory SQLite, fastest
    ModePersistent  = "persistent"   // File-based SQLite
    ModeDistributed = "distributed"  // Postgres + external services
    ModeRemoteTemporal = "remote"    // Temporal-specific (unchanged)
)
```

**Line 26** - Change default mode:
```go
// BEFORE:
return ModeDistributed

// AFTER:
return ModeMemory
```

**Line 18** - Update docstring:
```go
// BEFORE:
//  3. Default fallback ("distributed")

// AFTER:
//  3. Default fallback ("memory")
```

**Lines 36-42** - Update EffectiveTemporalMode:
```go
func (cfg *Config) EffectiveTemporalMode() string {
    mode := ResolveMode(cfg, cfg.Temporal.Mode)
    if mode == ModeDistributed {
        return ModeRemoteTemporal
    }
    // memory and persistent both use embedded Temporal
    return mode
}
```

**Lines 49-65** - Update EffectiveDatabaseDriver:
```go
func (cfg *Config) EffectiveDatabaseDriver() string {
    if cfg == nil {
        return databaseDriverSQLite  // Changed default
    }
    driver := strings.TrimSpace(cfg.Database.Driver)
    if driver != "" {
        return driver  // Explicit override
    }
    mode := strings.TrimSpace(cfg.Mode)
    if mode == ModeMemory || mode == ModePersistent {
        return databaseDriverSQLite
    }
    if mode == ModeDistributed {
        return databaseDriverPostgres
    }
    return databaseDriverSQLite  // Default to SQLite
}
```

**Validation:**
```bash
go test ./pkg/config -run TestResolveMode
go test ./pkg/config -run TestEffectiveDatabaseDriver
```

#### 1.2 Update Configuration Validation

**File:** `pkg/config/config.go`

**Line 56** - Update Mode field validation:
```go
// BEFORE:
Mode string `koanf:"mode" env:"COMPOZY_MODE" json:"mode" yaml:"mode" mapstructure:"mode" validate:"omitempty,oneof=standalone distributed"`

// AFTER:
Mode string `koanf:"mode" env:"COMPOZY_MODE" json:"mode" yaml:"mode" mapstructure:"mode" validate:"omitempty,oneof=memory persistent distributed"`
```

**Lines 52-55** - Update Mode documentation:
```go
// BEFORE:
// Mode controls global deployment model.
//
// "distributed" (default): External services required
// "standalone": Embedded services, single-process

// AFTER:
// Mode controls global deployment model.
//
// "memory" (default): In-memory SQLite, embedded services, fastest for tests/dev
// "persistent": File-based SQLite, embedded services, local development with persistence
// "distributed": PostgreSQL, external Temporal/Redis, production deployments
```

**Line 17** - Clean up constants:
```go
// BEFORE:
const (
    mcpProxyModeStandalone = "standalone"
    databaseDriverPostgres = "postgres"
    databaseDriverSQLite   = "sqlite"
)

// AFTER:
const (
    databaseDriverPostgres = "postgres"
    databaseDriverSQLite   = "sqlite"
)
```

**Validation:**
```bash
go test ./pkg/config -run TestConfigValidation
```

#### 1.3 Update Configuration Registry

**File:** `pkg/config/definition/schema.go`

**Update mode field registration (~line 733):**
```go
registry.Register(&FieldDef{
    Path:    "mode",
    Default: "memory",  // Changed from "distributed"
    CLIFlag: "mode",
    EnvVar:  "COMPOZY_MODE",
    Type:    reflect.TypeOf(""),
    Help:    "Deployment mode: memory (default, in-memory SQLite), persistent (file SQLite), or distributed (Postgres)",
})
```

**Update temporal.mode registration:**
```go
registry.Register(&FieldDef{
    Path:    "temporal.mode",
    Default: "",  // Empty = inherit from global
    CLIFlag: "temporal-mode",
    EnvVar:  "TEMPORAL_MODE",
    Type:    reflect.TypeOf(""),
    Help:    "Temporal deployment mode (memory/persistent/remote), inherits from global mode if unset",
})
```

**Update redis.mode registration:**
```go
registry.Register(&FieldDef{
    Path:    "redis.mode",
    Default: "",
    CLIFlag: "redis-mode",
    EnvVar:  "REDIS_MODE",
    Type:    reflect.TypeOf(""),
    Help:    "Redis deployment mode (memory/persistent/distributed), inherits from global mode if unset",
})
```

**Validation:**
```bash
go test ./pkg/config/definition -v
```

#### 1.4 Update Configuration Tests

**File:** `pkg/config/resolver_test.go`

Update test cases for mode resolution:
```go
func TestResolveMode(t *testing.T) {
    tests := []struct {
        name          string
        globalMode    string
        componentMode string
        want          string
    }{
        {
            name:          "Should use component mode when set",
            globalMode:    "distributed",
            componentMode: "memory",
            want:          "memory",
        },
        {
            name:          "Should use global mode when component not set",
            globalMode:    "persistent",
            componentMode: "",
            want:          "persistent",
        },
        {
            name:          "Should default to memory when neither set",
            globalMode:    "",
            componentMode: "",
            want:          "memory",  // Changed from "distributed"
        },
    }
    // ... test implementation
}
```

**File:** `pkg/config/config_test.go`

Update validation test cases:
```go
func TestModeValidation(t *testing.T) {
    tests := []struct {
        mode    string
        wantErr bool
    }{
        {"memory", false},
        {"persistent", false},
        {"distributed", false},
        {"standalone", true},  // No longer valid
        {"invalid", true},
    }
    // ... test implementation
}
```

**Validation Point:**
```bash
make lint
go test ./pkg/config/... -v
```

**Success Criteria:**
- All config tests pass
- Linter shows no errors
- Mode resolution works correctly
- Database driver selection works for all modes

---

### Phase 2: Infrastructure Wiring [HIGH]

**Priority:** HIGH - Runtime behavior depends on this

**Duration:** 1-2 days

**Goal:** Runtime systems (Cache, Temporal, Database) work correctly with new modes

#### 2.1 Update Cache Layer

**File:** `engine/infra/cache/mod.go`

**Lines 12-15** - Update mode constants:
```go
// BEFORE:
const (
    modeStandalone  = "standalone"
    modeDistributed = "distributed"
)

// AFTER:
const (
    modeMemory      = "memory"
    modePersistent  = "persistent"
    modeDistributed = "distributed"
)
```

**Lines 60-69** - Update SetupCache switch:
```go
// BEFORE:
switch mode {
case modeStandalone:
    return setupStandaloneCache(ctx, cacheCfg)
case modeDistributed:
    return setupDistributedCache(ctx, cacheCfg)
default:
    return nil, nil, fmt.Errorf("unsupported redis mode: %s", mode)
}

// AFTER:
switch mode {
case modeMemory:
    // Force persistence OFF for memory mode
    cacheCfg.Redis.Standalone.Persistence.Enabled = false
    log.Info("Cache in memory mode (no persistence)")
    return setupStandaloneCache(ctx, cacheCfg)

case modePersistent:
    // Auto-enable persistence for persistent mode
    if !cacheCfg.Redis.Standalone.Persistence.Enabled {
        cacheCfg.Redis.Standalone.Persistence.Enabled = true
        if cacheCfg.Redis.Standalone.Persistence.DataDir == "" {
            cacheCfg.Redis.Standalone.Persistence.DataDir = "./.compozy/redis"
        }
        log.Info("Cache in persistent mode (auto-enabled persistence)",
            "data_dir", cacheCfg.Redis.Standalone.Persistence.DataDir,
        )
    }
    return setupStandaloneCache(ctx, cacheCfg)

case modeDistributed:
    return setupDistributedCache(ctx, cacheCfg)

default:
    return nil, nil, fmt.Errorf("unsupported redis mode: %s", mode)
}
```

**Key Insight:** Both `memory` and `persistent` use the SAME `setupStandaloneCache()` function (from Redis PRD), just with different persistence settings!

**Validation:**
```bash
go test ./engine/infra/cache/... -v
```

#### 2.2 Update Temporal Wiring

**File:** `engine/infra/server/dependencies.go`

**Lines 378-414** - Update maybeStartStandaloneTemporal:
```go
// BEFORE:
func maybeStartStandaloneTemporal(ctx context.Context) (func(), error) {
    cfg := config.FromContext(ctx)
    if cfg == nil {
        return nil, fmt.Errorf("configuration is required to start Temporal")
    }
    if cfg.EffectiveTemporalMode() != modeStandalone {
        return nil, nil
    }
    // ... start embedded Temporal
}

// AFTER:
func maybeStartStandaloneTemporal(ctx context.Context) (func(), error) {
    cfg := config.FromContext(ctx)
    if cfg == nil {
        return nil, fmt.Errorf("configuration is required to start Temporal")
    }
    mode := cfg.EffectiveTemporalMode()
    // Start embedded Temporal for both memory and persistent modes
    if mode != config.ModeMemory && mode != config.ModePersistent {
        return nil, nil  // Distributed mode uses external Temporal
    }
    embeddedCfg := standaloneEmbeddedConfig(cfg)
    log := logger.FromContext(ctx)
    log.Info(
        "Starting embedded Temporal",
        "mode", mode,
        "database", embeddedCfg.DatabaseFile,
        "frontend_port", embeddedCfg.FrontendPort,
        "ui_enabled", embeddedCfg.EnableUI,
    )
    // ... rest unchanged
}
```

**Lines 416-430** - Update standaloneEmbeddedConfig:
```go
// BEFORE:
func standaloneEmbeddedConfig(cfg *config.Config) *embedded.Config {
    standalone := cfg.Temporal.Standalone
    return &embedded.Config{
        DatabaseFile: standalone.DatabaseFile,  // Defaults to ":memory:"
        // ... rest of config
    }
}

// AFTER:
func standaloneEmbeddedConfig(cfg *config.Config) *embedded.Config {
    standalone := cfg.Temporal.Standalone
    
    // Determine database file based on mode
    dbFile := standalone.DatabaseFile
    if dbFile == "" {
        // Set intelligent defaults based on mode
        if cfg.Mode == config.ModePersistent {
            dbFile = "./.compozy/temporal.db"
        } else {
            dbFile = ":memory:"  // Default for memory mode
        }
    }
    
    return &embedded.Config{
        DatabaseFile: dbFile,
        FrontendPort: standalone.FrontendPort,
        BindIP:       standalone.BindIP,
        Namespace:    standalone.Namespace,
        ClusterName:  standalone.ClusterName,
        EnableUI:     standalone.EnableUI,
        RequireUI:    standalone.RequireUI,
        UIPort:       standalone.UIPort,
        LogLevel:     standalone.LogLevel,
        StartTimeout: standalone.StartTimeout,
    }
}
```

**Lines 133-160** - Update validateDatabaseConfig:
```go
// Replace string references to "standalone" with mode checks
func (s *Server) validateDatabaseConfig(cfg *config.Config) error {
    if cfg == nil {
        return fmt.Errorf("config is required for database validation")
    }
    driver := strings.TrimSpace(cfg.Database.Driver)
    if driver == "" {
        driver = driverPostgres
    }
    if driver != driverSQLite {
        return nil
    }
    
    log := logger.FromContext(s.ctx)
    mode := cfg.Mode  // Add for logging
    
    // Vector DB validation
    if len(cfg.Knowledge.VectorDBs) == 0 {
        log.Warn("SQLite mode without vector database - knowledge features will not work",
            "mode", mode,
            "driver", driverSQLite,
            "recommendation", "Configure Qdrant, Redis, or Filesystem vector DB",
        )
    }
    
    // pgvector incompatibility check (unchanged)
    for _, vdb := range cfg.Knowledge.VectorDBs {
        provider := strings.TrimSpace(vdb.Provider)
        if strings.EqualFold(provider, "pgvector") {
            return fmt.Errorf(
                "pgvector provider is incompatible with SQLite driver. " +
                    "SQLite requires an external vector database. " +
                    "Configure one of: Qdrant, Redis, or Filesystem. " +
                    "See documentation: docs/database/sqlite.md#vector-database-requirement",
            )
        }
    }
    
    // Concurrency warning
    maxWorkflows := cfg.Worker.MaxConcurrentWorkflowExecutionSize
    if maxWorkflows > recommendedSQLiteConcurrency {
        log.Warn("SQLite has concurrency limitations",
            "mode", mode,
            "driver", driverSQLite,
            "max_concurrent_workflows", maxWorkflows,
            "recommended_max", recommendedSQLiteConcurrency,
            "note", "Consider using mode: distributed for high-concurrency workloads",
        )
    }
    
    return nil
}
```

**Validation:**
```bash
go test ./engine/infra/server/... -run TestMaybeStartStandaloneTemporal
go test ./engine/infra/server/... -run TestValidateDatabaseConfig
```

#### 2.3 Update Server Logging

**File:** `engine/infra/server/server.go`

Search for any hardcoded "standalone" strings in logging and update to use the actual mode value:

```go
// BEFORE:
log.Info("Starting in standalone mode", ...)

// AFTER:
log.Info("Starting server", "mode", cfg.Mode, ...)
```

**Validation:**
```bash
# Manual test
compozy start --mode memory
compozy start --mode persistent
compozy start --mode distributed
```

**Success Criteria:**
- Server starts successfully in each mode
- Correct infrastructure components activate per mode
- Logging clearly shows active mode
- Default mode (memory) works without any config

---

### Phase 3: Test Infrastructure [HIGH]

**Priority:** HIGH - Unblocks test suite

**Duration:** 1-2 days

**Goal:** Test suite runs with SQLite by default, 50%+ faster

#### 3.1 Update Test Helpers

**File:** `test/helpers/standalone.go`

**Line 20** - Update constant:
```go
// BEFORE:
const testModeStandalone = "standalone"

// AFTER:
const testModeMemory = "memory"
```

**Lines 51-52, 103-104, 212-213, 284-286** - Update mode assignments:
```go
// BEFORE:
cfg.Mode = testModeStandalone
cfg.Redis.Mode = testModeStandalone

// AFTER:
cfg.Mode = testModeMemory
cfg.Redis.Mode = testModeMemory
```

**Validation:**
```bash
go test ./test/helpers/... -v
```

#### 3.2 Add Database Mode Helper

**File:** `test/helpers/database.go`

Add new helper function (after line 306):
```go
// SetupTestDatabaseForMode sets up database based on explicit mode.
// This helper routes to the appropriate database backend:
//   - memory/persistent → SQLite (fast, no containers)
//   - distributed → PostgreSQL (full features, slower)
//
// Example usage:
//   provider, cleanup := helpers.SetupTestDatabaseForMode(t, "memory")
//   defer cleanup()
func SetupTestDatabaseForMode(t *testing.T, mode string) (*repo.Provider, func()) {
    t.Helper()
    switch mode {
    case "memory", "persistent":
        // Use SQLite for fast in-memory testing
        return SetupTestDatabase(t, "sqlite")
    case "distributed":
        // Use PostgreSQL for full features (pgvector, etc.)
        return SetupTestDatabase(t, "postgres")
    default:
        // Default to SQLite (memory mode)
        t.Logf("Unknown mode %q, defaulting to sqlite", mode)
        return SetupTestDatabase(t, "sqlite")
    }
}
```

**Validation:**
```bash
go test ./test/helpers -run TestSetupTestDatabase
```

#### 3.3 Audit and Migrate Tests

**Strategy:**
1. Most tests should use SQLite (memory mode) by default
2. Explicitly mark tests requiring PostgreSQL
3. Tests requiring pgvector must use distributed mode
4. High-concurrency tests (>10 workflows) should use distributed mode

**Find tests using Postgres:**
```bash
grep -r "GetSharedPostgresDB" test/ --include="*.go"
```

**Migration Pattern:**

**For tests that CAN use SQLite:**
```go
// BEFORE:
func TestWorkflowExecution(t *testing.T) {
    pool, cleanup := helpers.GetSharedPostgresDB(t)
    defer cleanup()
    // ... test code
}

// AFTER:
func TestWorkflowExecution(t *testing.T) {
    provider, cleanup := helpers.SetupTestDatabase(t, "sqlite")
    defer cleanup()
    // ... test code (no changes)
}
```

**For tests that NEED PostgreSQL:**
```go
func TestPgVectorEmbedding(t *testing.T) {
    // Explicitly require distributed mode for pgvector
    provider, cleanup := helpers.SetupTestDatabase(t, "postgres")
    defer cleanup()
    // ... test code
}
```

**Files to Audit:**
- `test/integration/store/operations_test.go`
- `test/integration/worker/*/database.go`
- `test/integration/server/executions_integration_test.go`
- `test/integration/tool/helpers.go`
- `test/integration/repo/repo_test_helpers.go`
- Any knowledge/RAG tests using pgvector

**Validation:**
```bash
# Run tests and measure time
time make test

# Should see significant speedup
# Before: ~2-5 minutes (with testcontainers)
# After:  ~30-90 seconds (with SQLite)
```

#### 3.4 Update Integration Test Helpers

**File:** `test/integration/standalone/helpers.go`

Update any references to "standalone" mode in test setup.

**File:** `test/integration/temporal/mode_switching_test.go`

Update test cases for mode switching:
```go
func TestModeResolver_Distributed(t *testing.T) {
    // ... test distributed mode
}

func TestModeResolver_Memory(t *testing.T) {
    // ... test memory mode (renamed from standalone)
}

func TestModeResolver_Persistent(t *testing.T) {
    // ... test persistent mode (new)
}
```

#### 3.5 Update Golden Test Files

**Files:**
- `testdata/config-diagnostics-standalone.golden`
- `testdata/config-show-mixed.golden`
- `testdata/config-show-standalone.golden`

Update these files to reflect new mode names:
```yaml
# BEFORE:
mode: standalone

# AFTER:
mode: memory
```

**Regenerate golden files:**
```bash
# Run tests with UPDATE_GOLDEN=1 to regenerate
UPDATE_GOLDEN=1 go test ./cli/cmd/config/...
```

**Success Criteria:**
- Test suite passes completely
- Test execution time reduced by 50%+
- No regressions in test coverage
- pgvector tests explicitly use distributed mode

---

### Phase 4: Documentation [MEDIUM]

**Priority:** MEDIUM - User-facing changes

**Duration:** 1-2 days

**Goal:** All documentation reflects new modes, clear migration guide

#### 4.1 Update Deployment Documentation

**File:** `docs/content/docs/deployment/standalone-mode.mdx`

**Action:** Rename to `memory-mode.mdx` and update content:

```mdx
---
title: "Memory Mode Deployment"
description: "Run Compozy with in-memory SQLite for fastest testing and development"
icon: Zap
---

<Callout type="info" title="Who is this for?">
Memory mode is optimized for tests, rapid development, and CI/CD pipelines. 
All data is ephemeral and lost on restart.
</Callout>

## When to Use Memory Mode

<List>
  <ListItem title="Testing" icon="TestTube">
    Run test suites 50-80% faster without Docker dependencies
  </ListItem>
  <ListItem title="Quick Development" icon="Code">
    Instant startup for rapid iteration cycles
  </ListItem>
  <ListItem title="CI/CD Pipelines" icon="Workflow">
    Deterministic, fast builds without external services
  </ListItem>
</List>

## Quick Start

Memory mode is the default - just run:

```bash
compozy start
```

All data is stored in-memory and lost on restart.

## Configuration

```yaml
name: my-workflow
# mode: memory  # Default, can be omitted

models:
  - provider: openai
    model: gpt-4o-mini
    api_key: "${OPENAI_API_KEY}"
```

## Characteristics

- **Database:** SQLite :memory: (ephemeral)
- **Temporal:** Embedded in-memory
- **Redis:** Miniredis without persistence
- **Startup:** Instant
- **Data:** Lost on restart
- **Use Cases:** Tests, demos, quick experimentation

## Limitations

- No data persistence
- No pgvector support (use Qdrant/Redis for embeddings)
- Single process only
- Write concurrency limited (~10 concurrent workflows)

## Next Steps

- Need persistence? Use [persistent mode](/docs/deployment/persistent-mode)
- Ready for production? See [distributed mode](/docs/deployment/distributed-mode)
```

**Create:** `docs/content/docs/deployment/persistent-mode.mdx`

```mdx
---
title: "Persistent Mode Deployment"
description: "Run Compozy with file-based SQLite for local development with data persistence"
icon: Save
---

<Callout type="info" title="Who is this for?">
Persistent mode is ideal for local development where you need to preserve 
state between restarts without managing external services.
</Callout>

## When to Use Persistent Mode

<List>
  <ListItem title="Local Development" icon="Monitor">
    Develop workflows with state preservation across restarts
  </ListItem>
  <ListItem title="Debugging" icon="Bug">
    Inspect database state to debug complex workflows
  </ListItem>
  <ListItem title="Small Teams" icon="Users">
    Single-instance deployment for 5-10 users
  </ListItem>
</List>

## Quick Start

```bash
compozy start --mode persistent
```

Or configure in `compozy.yaml`:

```yaml
name: my-workflow
mode: persistent

# Optional: customize paths
database:
  path: ./.compozy/compozy.db

temporal:
  standalone:
    database_file: ./.compozy/temporal.db

redis:
  standalone:
    persistence:
      data_dir: ./.compozy/redis
      snapshot_interval: 5m
```

## Default Paths

When not specified, persistent mode uses:
- Database: `./.compozy/compozy.db`
- Temporal: `./.compozy/temporal.db`
- Redis: `./.compozy/redis/`

## Characteristics

- **Database:** SQLite file
- **Temporal:** Embedded with file storage
- **Redis:** Miniredis with BadgerDB snapshots
- **Startup:** Instant
- **Data:** Persists across restarts
- **Use Cases:** Local dev, debugging, small teams

## Backup and Recovery

```bash
# Backup
cp -r .compozy ./backup-$(date +%Y%m%d)

# Restore
cp -r ./backup-20250101 .compozy
```

## Limitations

- Same as memory mode (no pgvector, write concurrency limits)
- Single point of failure (no replication)
- Not recommended for production

## Next Steps

- Need scalability? See [distributed mode](/docs/deployment/distributed-mode)
- Need high availability? Use [distributed mode](/docs/deployment/distributed-mode)
```

**Update:** `docs/content/docs/deployment/distributed-mode.mdx`

Add comparison section at the top:

```mdx
## Mode Comparison

| Feature | Memory | Persistent | Distributed |
|---------|--------|-----------|-------------|
| Persistence | None | File-based | Full |
| Startup | Instant | Instant | Requires services |
| Scalability | Single process | Single process | Horizontal |
| HA | No | No | Yes |
| Production | No | No | Yes |

Use distributed mode when you need:
- Horizontal scaling
- High availability
- pgvector embeddings
- >10 concurrent workflows
```

#### 4.2 Update Configuration Documentation

**File:** `docs/content/docs/configuration/mode-configuration.mdx`

```mdx
---
title: "Mode Configuration"
description: "Control deployment modes: memory, persistent, or distributed"
icon: GitBranch
---

Compozy supports three deployment modes, each optimized for different use cases.

## Overview

<List>
  <ListItem title="Global mode" icon="Globe2">
    Set `mode: memory|persistent|distributed` at root level
  </ListItem>
  <ListItem title="Component override" icon="Settings">
    Override per component (temporal, redis, database)
  </ListItem>
  <ListItem title="Default: memory" icon="Zap">
    No external dependencies required
  </ListItem>
</List>

## Mode Options

### memory (Default)

Fastest mode for testing and development:
- SQLite :memory:
- Embedded Temporal (in-memory)
- Embedded Redis (no persistence)
- Zero external dependencies

```yaml
mode: memory  # Default, can be omitted
```

### persistent

File-based SQLite for local development:
- SQLite file storage
- Embedded Temporal (file storage)
- Embedded Redis (BadgerDB persistence)
- State survives restarts

```yaml
mode: persistent
database:
  path: ./.compozy/compozy.db
```

### distributed

Production-ready with external services:
- PostgreSQL with pgvector
- External Temporal cluster
- External Redis cluster
- Horizontal scaling

```yaml
mode: distributed
database:
  driver: postgres
  host: postgres.prod.internal
  port: 5432
temporal:
  mode: remote
  host_port: temporal.prod.internal:7233
redis:
  mode: distributed
  distributed:
    addr: redis.prod.internal:6379
```

## Component Override

```yaml
mode: memory  # Global default

temporal:
  mode: persistent  # Override for Temporal only
  standalone:
    database_file: ./.compozy/temporal.db

redis:
  mode: memory  # Explicitly use memory mode
```

## Resolution Order

1. Component-specific mode (if set)
2. Global mode (if set)
3. Default (memory)

## Examples

See complete examples:
- [Memory mode example](/docs/examples/memory-mode)
- [Persistent mode example](/docs/examples/persistent-mode)
- [Distributed mode example](/docs/examples/distributed-mode)
```

#### 4.3 Update Migration Guide

**File:** `docs/content/docs/guides/mode-migration-guide.mdx`

**Rename to:** `mode-migration-guide.mdx`

```mdx
---
title: "Mode Migration Guide"
description: "Migrate between memory, persistent, and distributed modes"
icon: ArrowRightLeft
---

## Migration Paths

```
memory (fast, ephemeral)
  ↓
persistent (fast, saved)
  ↓
distributed (production)
```

## Migrating from Alpha Versions

### Old standalone → New memory/persistent

**Before (Alpha):**
```yaml
mode: standalone
```

**After:**
```yaml
# For ephemeral (testing)
mode: memory

# OR for persistence (development)
mode: persistent
```

**What changed:**
- `standalone` mode split into `memory` (ephemeral) and `persistent` (files)
- Default changed from `distributed` to `memory`
- Configuration is otherwise identical

### Old distributed → New distributed

No changes needed - distributed mode works identically.

## Memory → Persistent

Add persistence without changing infrastructure:

```yaml
# Before
mode: memory

# After
mode: persistent
database:
  path: ./.compozy/compozy.db
temporal:
  standalone:
    database_file: ./.compozy/temporal.db
```

**Data Migration:** None (memory mode has no data to migrate)

## Persistent → Distributed

**Step 1:** Export data (if needed)
```bash
# Export workflows
compozy workflow list --format json > workflows.json

# Export memory/knowledge data using API
curl http://localhost:8080/api/v0/memory > memory.json
```

**Step 2:** Update configuration
```yaml
# Before
mode: persistent

# After
mode: distributed
database:
  driver: postgres
  host: localhost
  port: 5432
  user: compozy
  password: ${DB_PASSWORD}
  name: compozy
temporal:
  mode: remote
  host_port: localhost:7233
redis:
  mode: distributed
  distributed:
    addr: localhost:6379
```

**Step 3:** Import data
```bash
# Import workflows using API
compozy workflow import workflows.json
```

## Common Issues

### pgvector Error with SQLite

**Error:**
```
pgvector provider is incompatible with SQLite driver
```

**Solution:**
Use Qdrant, Redis, or Filesystem vector DB:
```yaml
mode: persistent
knowledge:
  vector_dbs:
    - name: default
      provider: qdrant  # or redis, or filesystem
      config:
        host: localhost
        port: 6333
```

### Concurrent Workflow Limit

**Warning:**
```
SQLite has concurrency limitations (max_concurrent_workflows=50, recommended_max=10)
```

**Solution:**
Migrate to distributed mode:
```yaml
mode: distributed
database:
  driver: postgres
```
```

#### 4.4 Update Quick Start

**File:** `docs/content/docs/quick-start/index.mdx`

Update the getting started section:

```mdx
## Quick Start

```bash
# Install
brew install compozy

# Start (default: memory mode, no external deps)
compozy start

# Your first workflow
compozy workflow run examples/hello-world.yaml
```

**Default mode:** memory (fastest, no persistence)
**Need persistence?** Add `mode: persistent` to config
**Production?** Use `mode: distributed` with external services
```

#### 4.5 Update CLI Help

**File:** `cli/help/global-flags.md`

Update mode flag description:

```markdown
### --mode

Deployment mode: memory (default), persistent, or distributed

- **memory**: In-memory SQLite, embedded services (fastest)
- **persistent**: File-based SQLite, embedded services (local dev)
- **distributed**: PostgreSQL, external services (production)

**Default:** memory

**Environment:** COMPOZY_MODE
```

**Success Criteria:**
- All references to "standalone" removed (except historical context)
- Clear explanation of when to use each mode
- Migration guide covers all scenarios
- Examples work in each mode

---

### Phase 5: Schemas & Metadata [MEDIUM]

**Priority:** MEDIUM - Tooling and validation

**Duration:** 0.5-1 day

**Goal:** Schemas reflect new modes, tooling updated

#### 5.1 Update JSON Schemas

**File:** `schemas/config.json`

Update mode enum:
```json
{
  "properties": {
    "mode": {
      "type": "string",
      "enum": ["memory", "persistent", "distributed"],
      "default": "memory",
      "description": "Deployment mode: memory (in-memory, fastest), persistent (file-based), or distributed (production)"
    }
  }
}
```

**File:** `schemas/compozy.json`

Update root-level mode and component modes:
```json
{
  "properties": {
    "mode": {
      "type": "string",
      "enum": ["memory", "persistent", "distributed"],
      "default": "memory"
    },
    "temporal": {
      "properties": {
        "mode": {
          "type": "string",
          "enum": ["memory", "persistent", "remote"],
          "description": "Temporal mode (empty = inherit from global)"
        }
      }
    },
    "redis": {
      "properties": {
        "mode": {
          "type": "string",
          "enum": ["memory", "persistent", "distributed"],
          "description": "Redis mode (empty = inherit from global)"
        }
      }
    }
  }
}
```

**Validation:**
```bash
# Validate schemas
go run scripts/validate-schemas.go

# Or if using JSON schema validator
jsonschema -i examples/memory-mode/compozy.yaml schemas/compozy.json
```

#### 5.2 Update Generated Files

**Regenerate Swagger docs:**
```bash
make swagger
```

**Regenerate schemas if auto-generated:**
```bash
# If using schemagen
go run pkg/schemagen/main.go
```

**Update golden files:**
```bash
UPDATE_GOLDEN=1 go test ./cli/cmd/config/...
```

**Success Criteria:**
- Schema validation passes
- IDE autocomplete shows correct modes
- Generated docs are correct
- No validation errors for example configs

---

### Phase 6: Final Validation [CRITICAL]

**Priority:** CRITICAL - Ship readiness

**Duration:** 1 day

**Goal:** Production-ready quality, all validations pass

#### 6.1 Comprehensive Testing

**Run full test suite:**
```bash
# Clean build
make clean
make build

# Full test suite
make test

# Expected: All pass, 50%+ faster than before
```

**Run linter:**
```bash
make lint

# Expected: Zero warnings
```

**Test each mode:**
```bash
# Memory mode (default)
compozy start
# In another terminal:
compozy workflow run examples/hello-world.yaml

# Persistent mode
compozy start --mode persistent
# Restart and verify state persists
compozy start --mode persistent
# Should show previous workflows

# Distributed mode (requires services)
docker-compose up -d postgres redis temporal
compozy start --mode distributed
```

#### 6.2 Validate Examples

**Test memory mode example:**
```bash
cd examples/memory-mode
compozy start
# Should start instantly
```

**Test persistent mode example:**
```bash
cd examples/persistent-mode
compozy start
# Should create .compozy/ directory
ls -la .compozy/
# Should show: compozy.db, temporal.db, redis/
```

**Test distributed mode example:**
```bash
cd examples/distributed-mode
docker-compose up -d
compozy start
# Should connect to external services
```

#### 6.3 Performance Benchmarking

**Measure test suite performance:**
```bash
# Before (with testcontainers)
time make test
# Expected: 2-5 minutes

# After (with SQLite memory mode)
time make test
# Expected: 30-90 seconds (50-80% faster)
```

**Measure server startup:**
```bash
# Memory mode
time compozy start --timeout 10s
# Expected: <1 second

# Persistent mode
time compozy start --mode persistent --timeout 10s
# Expected: <2 seconds

# Distributed mode
time compozy start --mode distributed --timeout 30s
# Expected: 5-15 seconds (external service connection)
```

#### 6.4 Error Message Validation

**Test invalid mode:**
```bash
compozy start --mode invalid
# Should show helpful error:
# Error: invalid mode "invalid". Valid modes: memory, persistent, distributed
```

**Test pgvector with SQLite:**
```bash
# Create config with pgvector + SQLite
cat > test-config.yaml <<EOF
mode: memory
knowledge:
  vector_dbs:
    - provider: pgvector
EOF

compozy start --config test-config.yaml
# Should show helpful error:
# Error: pgvector provider is incompatible with SQLite driver.
# SQLite requires an external vector database.
# Configure one of: Qdrant, Redis, or Filesystem.
```

#### 6.5 Documentation Validation

**Check for broken links:**
```bash
# In docs directory
npm run lint:links
```

**Validate all code examples:**
```bash
# Extract and test all code examples from docs
npm run test:examples
```

**Success Criteria:**
- All tests pass
- Linter clean
- All examples work
- Performance improved 50%+
- Error messages are helpful
- Documentation is complete

---

## Testing Strategy

### Test Categories

**Unit Tests:**
- Config mode resolution
- Database driver selection
- Temporal mode resolution
- Redis mode selection
- Validation rules

**Integration Tests:**
- Cache setup in each mode
- Temporal startup in each mode
- Database connectivity in each mode
- Mode switching scenarios

**E2E Tests:**
- Full workflow execution in memory mode
- Full workflow execution in persistent mode
- Full workflow execution in distributed mode
- State persistence validation
- Mode migration validation

### Test Matrix

```
┌────────────────────┬─────────┬────────────┬──────────────┐
│ Test Scenario      │ Memory  │ Persistent │ Distributed  │
├────────────────────┼─────────┼────────────┼──────────────┤
│ Server Startup     │    ✓    │     ✓      │      ✓       │
│ Workflow Execution │    ✓    │     ✓      │      ✓       │
│ State Persistence  │    ✗    │     ✓      │      ✓       │
│ Restart Recovery   │    ✗    │     ✓      │      ✓       │
│ Concurrent (10)    │    ✓    │     ✓      │      ✓       │
│ Concurrent (50)    │    ✗    │     ✗      │      ✓       │
│ pgvector           │    ✗    │     ✗      │      ✓       │
│ Knowledge (Qdrant) │    ✓    │     ✓      │      ✓       │
└────────────────────┴─────────┴────────────┴──────────────┘
```

### Performance Benchmarks

**Baseline (Current):**
- Test suite: 3-5 minutes (with testcontainers)
- Server startup: 10-30 seconds (Docker compose)

**Target (After):**
- Test suite: 45-90 seconds (50-70% faster)
- Server startup memory: <1 second (instant)
- Server startup persistent: <2 seconds (instant)
- Server startup distributed: 5-15 seconds (external connections)

---

## Risk Mitigation

### High-Impact Risks

#### 1. SQLite Limitations in Tests

**Risk:** Some tests may fail with SQLite

**Probability:** Medium

**Impact:** High

**Mitigation:**
- Keep testcontainers code intact
- Mark failing tests explicitly with distributed mode
- Provide clear error messages
- Add `COMPOZY_TEST_MODE=distributed` env var override

**Detection:**
```bash
# Run with distributed mode if issues
COMPOZY_TEST_MODE=distributed make test
```

**Fallback:**
- Can revert specific tests to Postgres
- Does not block overall migration

#### 2. Temporal Embedded Stability

**Risk:** Embedded Temporal may have issues in production-like scenarios

**Probability:** Low

**Impact:** High

**Mitigation:**
- Extensive testing in memory and persistent modes
- Monitor startup/shutdown carefully
- Test long-running workflows
- Validate WAL corruption scenarios

**Detection:**
- Monitor Temporal logs for errors
- Test crash recovery scenarios

**Fallback:**
- Users can always use distributed mode
- Embedded Temporal only for dev/test

#### 3. Breaking User Configs

**Risk:** Existing configs will break

**Probability:** High (intentional)

**Impact:** Medium

**Mitigation:**
- Clear migration guide
- Helpful error messages
- Alpha caveat (breaking changes OK)
- Version bump

**Example Error:**
```
Error: invalid mode "standalone"
Hint: The "standalone" mode has been replaced with:
  - mode: memory (for ephemeral testing)
  - mode: persistent (for file-based storage)
See migration guide: docs/guides/mode-migration-guide.mdx
```

### Medium-Impact Risks

#### 4. Test Performance Not Improved

**Risk:** SQLite might not be faster than testcontainers

**Probability:** Low

**Impact:** Medium

**Mitigation:**
- Benchmark before/after
- SQLite is inherently faster (in-memory, no network)
- Can parallelize tests better without container conflicts

**Expected Improvement:** 50-80%

#### 5. Documentation Gaps

**Risk:** Missed mode references in docs

**Probability:** Medium

**Impact:** Low

**Mitigation:**
- Systematic grep for "standalone"
- Review all deployment docs
- Test all examples
- Cross-reference documentation

**Detection:**
```bash
# Find all "standalone" references
grep -r "standalone" docs/ examples/ README.md
```

---

## Success Metrics

### Primary Metrics

**Code Quality:**
- [ ] All tests pass (`make test`)
- [ ] Linter clean (`make lint`)
- [ ] No "standalone" references remain (except historical)
- [ ] Code coverage >80% for new code

**Performance:**
- [ ] Test suite 50%+ faster
- [ ] Server startup <1s in memory mode
- [ ] Server startup <2s in persistent mode

**Functionality:**
- [ ] All three modes work correctly
- [ ] State persists in persistent mode
- [ ] No regressions in distributed mode

**Documentation:**
- [ ] All mode references updated
- [ ] Migration guide complete
- [ ] Examples work in each mode
- [ ] API docs generated

### Secondary Metrics

**User Experience:**
- [ ] `compozy start` works with zero config
- [ ] Error messages are helpful
- [ ] Mode switching is clear
- [ ] Configuration is intuitive

**Quality:**
- [ ] No flaky tests
- [ ] Graceful error handling
- [ ] Comprehensive logging
- [ ] Clear diagnostics

---

## Migration Guide

### For Users

**Alpha users with existing configs:**

```yaml
# OLD (Alpha):
mode: standalone

# NEW:
# For ephemeral (testing):
mode: memory

# OR for persistence (development):
mode: persistent
database:
  path: ./.compozy/compozy.db
```

**No changes needed for distributed mode.**

### For Developers

**Test Migration:**

```go
// OLD:
func TestSomething(t *testing.T) {
    pool, cleanup := helpers.GetSharedPostgresDB(t)
    defer cleanup()
}

// NEW (default):
func TestSomething(t *testing.T) {
    provider, cleanup := helpers.SetupTestDatabase(t, "sqlite")
    defer cleanup()
}

// NEW (requires Postgres):
func TestWithPgVector(t *testing.T) {
    provider, cleanup := helpers.SetupTestDatabase(t, "postgres")
    defer cleanup()
}
```

**Configuration Code:**

```go
// OLD:
if cfg.Mode == config.ModeStandalone {
    // ...
}

// NEW:
if cfg.Mode == config.ModeMemory || cfg.Mode == config.ModePersistent {
    // ... embedded services
}
```

---

## CHANGELOG Entry

```markdown
## [VERSION] - YYYY-MM-DD

### ⚠️ BREAKING CHANGES

**Mode System Overhaul** (#XXX)

Replaced two-mode system (standalone/distributed) with clearer three-mode system:

- **mode: memory** (NEW DEFAULT): In-memory SQLite, embedded services, fastest for tests
- **mode: persistent**: File-based SQLite, embedded services, local dev with persistence
- **mode: distributed**: PostgreSQL, external services, production deployments

**Migration:**
- `mode: standalone` → `mode: memory` (ephemeral) or `mode: persistent` (with files)
- `mode: distributed` → No changes (works identically)
- Default mode changed from `distributed` to `memory`

**Benefits:**
- 50-80% faster test suite (no testcontainers)
- Zero-dependency quickstart
- Clearer intent-based naming

**See:** docs/guides/mode-migration-guide.mdx

### Features

- **config**: Add memory/persistent/distributed mode system (#XXX)
- **cache**: Auto-configure persistence based on mode (#XXX)
- **temporal**: Set database path based on mode (#XXX)
- **tests**: Migrate to SQLite for faster execution (#XXX)

### Documentation

- Add memory mode deployment guide
- Add persistent mode deployment guide
- Update mode configuration reference
- Add mode migration guide
```

---

## Definition of Done

### Code Complete

- [ ] All ~40 files updated
- [ ] All tests passing (`make test`)
- [ ] Linter clean (`make lint`)
- [ ] No "standalone" references remain (except historical context in docs)
- [ ] All imports updated
- [ ] No dead code

### Quality Complete

- [ ] Performance benchmarked (test suite 50%+ faster)
- [ ] Examples tested in each mode
- [ ] Server starts in each mode
- [ ] State persists correctly in persistent mode
- [ ] Error messages are helpful and clear
- [ ] Logging shows correct mode information

### Documentation Complete

- [ ] All mode references updated
- [ ] Migration guide written and tested
- [ ] Examples updated and verified
- [ ] CHANGELOG entry written
- [ ] API docs regenerated
- [ ] CLI help updated

### Ship Ready

- [ ] Smoke tests pass in all modes
- [ ] No regressions in distributed mode
- [ ] Clear error messages for config issues
- [ ] Team reviewed and approved
- [ ] Version bumped appropriately

---

## Next Steps

**Immediate Actions:**

1. **Review Plan** - Team review and approval
2. **Create Branch** - `feature/three-mode-system`
3. **Start Phase 1** - Update core config system
4. **Iterative Implementation** - Follow phases in order

**First Commits:**

```bash
# Phase 1: Core Configuration
git checkout -b feature/three-mode-system

# Commit 1: Update mode constants and resolution
git commit -m "feat(config): add memory/persistent/distributed modes"

# Commit 2: Update validation and registry
git commit -m "feat(config): update validation for new modes"

# Commit 3: Update config tests
git commit -m "test(config): update mode resolution tests"

# Phase 2: Infrastructure
git commit -m "feat(cache): auto-configure persistence by mode"
git commit -m "feat(temporal): set database path by mode"

# Continue through remaining phases...
```

---

## Appendix

### Related Redis PRD Work

This implementation builds on the Redis PRD infrastructure:
- `MiniredisStandalone` wrapper (Tasks 2.0)
- `SnapshotManager` with BadgerDB (Task 7.0)
- Mode-aware cache factory (Task 3.0)
- Comprehensive testing (Tasks 8.0-10.0)

**Key Insight:** The three-mode plan is a **refactoring** of the Redis PRD work, not new infrastructure. Both `memory` and `persistent` modes use the same `MiniredisStandalone` implementation, just with different persistence settings.

### File Reference Map

**Core Configuration (7 files):**
- `pkg/config/resolver.go` - Mode constants, resolution, driver selection
- `pkg/config/config.go` - Validation, struct tags, documentation
- `pkg/config/definition/schema.go` - Registry defaults and help text
- `pkg/config/loader.go` - Custom validation logic
- `pkg/config/resolver_test.go` - Mode resolution tests
- `pkg/config/config_test.go` - Validation tests
- `pkg/config/loader_test.go` - Loader tests

**Infrastructure (4 files):**
- `engine/infra/cache/mod.go` - Cache factory mode switch
- `engine/infra/server/dependencies.go` - Temporal startup and DB validation
- `engine/infra/server/server.go` - Logging and initialization
- `engine/infra/server/temporal_resolver_test.go` - Mode switching tests

**Test Helpers (6 files):**
- `test/helpers/standalone.go` - Mode constants and helpers
- `test/helpers/database.go` - Database setup helpers
- `test/integration/standalone/helpers.go` - Integration test helpers
- `test/integration/temporal/mode_switching_test.go` - Mode tests
- `testdata/config-diagnostics-standalone.golden` - Golden file
- `testdata/config-show-*.golden` - Golden files

**Documentation (8 files):**
- `docs/content/docs/deployment/memory-mode.mdx` - Memory mode guide
- `docs/content/docs/deployment/persistent-mode.mdx` - Persistent mode guide
- `docs/content/docs/deployment/distributed-mode.mdx` - Distributed mode guide
- `docs/content/docs/configuration/mode-configuration.mdx` - Mode config reference
- `docs/content/docs/guides/mode-migration-guide.mdx` - Migration guide
- `docs/content/docs/quick-start/index.mdx` - Quick start update
- `docs/content/docs/troubleshooting/common-issues.mdx` - Troubleshooting
- `cli/help/global-flags.md` - CLI help

**Examples (3 files):**
- `examples/memory-mode/` - Memory mode example (renamed)
- `examples/persistent-mode/` - Persistent mode example (new)
- `examples/README.md` - Examples index

**Schemas (2 files):**
- `schemas/config.json` - Configuration schema
- `schemas/compozy.json` - Root schema

---

**Document Version:** 1.0  
**Status:** Ready for Implementation  
**Estimated Effort:** 6 days (with proper phasing)  
**Breaking Change:** Yes (acceptable in alpha)
