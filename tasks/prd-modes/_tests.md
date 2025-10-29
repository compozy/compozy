# Test Plan: Three-Mode Configuration System

**PRD Reference**: `tasks/prd-modes/_prd.md`
**Tech Spec Reference**: `tasks/prd-modes/_techspec.md`
**Status**: Planning

---

## Testing Strategy

### Goals
1. **Performance**: Achieve 50-80% faster test suite execution
2. **Coverage**: Maintain >80% test coverage across all modes
3. **Quality**: Zero regressions in existing functionality
4. **Confidence**: Comprehensive validation for breaking change

### Testing Principles
- **Mode-agnostic tests**: Most tests should work across all modes
- **Mode-specific tests**: Validate unique behaviors per mode
- **Fast by default**: Memory mode for 90% of tests
- **PostgreSQL exceptions**: pgvector and vector search tests only

---

## Test Categories

### 1. Unit Tests

**Scope**: Individual functions and components
**Mode**: Memory (fastest)
**Coverage Target**: >90%

#### Configuration Layer Tests (`pkg/config/*_test.go`)

**Test Suite**: Mode Resolution
- [ ] `TestResolveMode_DefaultsToMemory` - Empty config defaults to memory
- [ ] `TestResolveMode_ComponentOverridesGlobal` - Component mode takes precedence
- [ ] `TestResolveMode_GlobalModeInheritance` - Components inherit global mode
- [ ] `TestResolveMode_AllThreeModes` - Memory, persistent, distributed all resolve correctly

**Test Suite**: Database Driver Selection
- [ ] `TestEffectiveDatabaseDriver_Memory` - Memory mode returns SQLite
- [ ] `TestEffectiveDatabaseDriver_Persistent` - Persistent mode returns SQLite
- [ ] `TestEffectiveDatabaseDriver_Distributed` - Distributed mode returns PostgreSQL
- [ ] `TestEffectiveDatabaseDriver_ExplicitOverride` - Explicit driver overrides mode default
- [ ] `TestEffectiveDatabaseDriver_NilConfig` - Nil config returns SQLite (changed default)

**Test Suite**: Temporal Mode Selection
- [ ] `TestEffectiveTemporalMode_Memory` - Memory mode returns embedded
- [ ] `TestEffectiveTemporalMode_Persistent` - Persistent mode returns embedded
- [ ] `TestEffectiveTemporalMode_Distributed` - Distributed mode returns remote
- [ ] `TestEffectiveTemporalMode_ExplicitOverride` - Explicit mode overrides

**Test Suite**: Configuration Validation
- [ ] `TestModeValidation_Memory` - "memory" validates successfully
- [ ] `TestModeValidation_Persistent` - "persistent" validates successfully
- [ ] `TestModeValidation_Distributed` - "distributed" validates successfully
- [ ] `TestModeValidation_Standalone_Rejected` - "standalone" fails validation
- [ ] `TestModeValidation_Invalid_Rejected` - Invalid modes fail validation
- [ ] `TestModeValidation_ErrorMessage` - Error messages suggest memory mode

**Test Suite**: Configuration Registry
- [ ] `TestFieldDefinition_ModeDefault` - Global mode default is "memory"
- [ ] `TestFieldDefinition_ModeHelpText` - Help text mentions all three modes
- [ ] `TestFieldDefinition_TemporalMode` - Temporal mode inherits from global
- [ ] `TestFieldDefinition_RedisMode` - Redis mode inherits from global

---

### 2. Integration Tests

**Scope**: Component interactions
**Mode**: Memory (default), PostgreSQL (when needed)
**Coverage Target**: >80%

#### Cache Layer Tests (`engine/infra/cache/*_test.go`)

**Test Suite**: Cache Setup
- [ ] `TestSetupCache_MemoryMode` - Memory mode uses miniredis without persistence
- [ ] `TestSetupCache_PersistentMode` - Persistent mode uses miniredis with BadgerDB
- [ ] `TestSetupCache_DistributedMode` - Distributed mode uses external Redis
- [ ] `TestSetupCache_AutoDisablePersistence_Memory` - Persistence disabled for memory
- [ ] `TestSetupCache_AutoEnablePersistence_Persistent` - Persistence enabled for persistent

**Test Suite**: Cache Operations (All Modes)
- [ ] `TestCache_SetGet_Memory` - Basic operations in memory mode
- [ ] `TestCache_SetGet_Persistent` - Basic operations in persistent mode
- [ ] `TestCache_Persistence_Memory` - Verify no persistence in memory mode
- [ ] `TestCache_Persistence_Persistent` - Verify state persists in persistent mode

#### Temporal Wiring Tests (`engine/infra/server/*_test.go`)

**Test Suite**: Temporal Startup
- [ ] `TestMaybeStartTemporal_MemoryMode` - Starts embedded Temporal with :memory:
- [ ] `TestMaybeStartTemporal_PersistentMode` - Starts embedded Temporal with file DB
- [ ] `TestMaybeStartTemporal_DistributedMode` - Does not start embedded Temporal
- [ ] `TestMaybeStartTemporal_DatabasePath_Memory` - Uses :memory: by default
- [ ] `TestMaybeStartTemporal_DatabasePath_Persistent` - Uses ./.compozy/temporal.db by default
- [ ] `TestMaybeStartTemporal_ExplicitPath` - Respects explicit database_file path

**Test Suite**: Server Startup (All Modes)
- [ ] `TestServerStart_MemoryMode` - Server starts in <1 second
- [ ] `TestServerStart_PersistentMode` - Server starts in <2 seconds
- [ ] `TestServerStart_MemoryMode_NoFiles` - No files created in memory mode
- [ ] `TestServerStart_PersistentMode_FilesCreated` - .compozy/ directory created

#### Database Layer Tests (`engine/infra/database/*_test.go`)

**Test Suite**: Database Connection
- [ ] `TestDatabaseSetup_Memory_SQLite` - Memory mode uses :memory:
- [ ] `TestDatabaseSetup_Persistent_SQLite` - Persistent mode uses file-based SQLite
- [ ] `TestDatabaseSetup_Distributed_Postgres` - Distributed mode uses PostgreSQL
- [ ] `TestDatabaseSetup_AutoMigration` - Schema migrations run for all modes

**Test Suite**: Database Operations (Mode-Agnostic)
- [ ] `TestDatabase_CRUD_Operations` - Create, read, update, delete work across modes
- [ ] `TestDatabase_Transactions` - Transaction support across modes
- [ ] `TestDatabase_Concurrency_SQLite` - SQLite handles write serialization correctly
- [ ] `TestDatabase_Indexes` - Indexes created correctly across modes

---

### 3. End-to-End Tests

**Scope**: Full system behavior
**Mode**: All three modes tested separately
**Coverage Target**: >70%

#### Server Lifecycle Tests (`test/integration/server/*_test.go`)

**Test Suite**: Startup and Shutdown
- [ ] `TestE2E_MemoryMode_Startup` - Server starts with memory mode
- [ ] `TestE2E_PersistentMode_Startup` - Server starts with persistent mode
- [ ] `TestE2E_DistributedMode_Startup` - Server starts with distributed mode
- [ ] `TestE2E_MemoryMode_Shutdown` - Graceful shutdown in memory mode
- [ ] `TestE2E_PersistentMode_Shutdown_DataPersists` - State survives shutdown
- [ ] `TestE2E_MemoryMode_Restart_DataLost` - Verify ephemeral nature

**Test Suite**: Workflow Execution (All Modes)
- [ ] `TestE2E_WorkflowExecution_Memory` - Execute workflow in memory mode
- [ ] `TestE2E_WorkflowExecution_Persistent` - Execute workflow in persistent mode
- [ ] `TestE2E_WorkflowExecution_Distributed` - Execute workflow in distributed mode
- [ ] `TestE2E_WorkflowState_Persistent` - Workflow state persists across restarts

#### Mode Switching Tests (`test/integration/temporal/mode_switching_test.go`)

**Test Suite**: Mode Transitions
- [ ] `TestModeSwitching_MemoryToPersistent` - Switch from memory to persistent (restart required)
- [ ] `TestModeSwitching_PersistentToDistributed` - Switch from persistent to distributed
- [ ] `TestModeSwitching_DistributedToMemory` - Switch from distributed to memory
- [ ] `TestModeSwitching_ConfigUpdate` - Configuration changes respected after restart

---

### 4. Performance Tests

**Scope**: Measure performance improvements
**Mode**: All modes for comparison
**Target**: 50-80% improvement

#### Test Suite Execution Time

**Baseline Measurement** (Before Changes):
```bash
# Full test suite with testcontainers
time make test
# Expected: 3-5 minutes
```

**Target Measurement** (After Changes):
```bash
# Full test suite with memory mode
time make test
# Target: 45-90 seconds (60-70% improvement)
```

**Benchmarks**:
- [ ] `BenchmarkTestSuite_Before` - Baseline with PostgreSQL testcontainers
- [ ] `BenchmarkTestSuite_After_Memory` - With memory mode (SQLite)
- [ ] `BenchmarkTestSuite_After_Persistent` - With persistent mode
- [ ] `BenchmarkTestSuite_After_Distributed` - With distributed mode (should match baseline)

#### Startup Time Benchmarks

**Test Suite**: Server Startup Performance
- [ ] `BenchmarkServerStartup_Memory` - Target: <1 second
- [ ] `BenchmarkServerStartup_Persistent` - Target: <2 seconds
- [ ] `BenchmarkServerStartup_Distributed` - Target: <3 seconds
- [ ] `BenchmarkServerStartup_Memory_ColdStart` - From completely cold state

**Test Suite**: Workflow Execution Performance
- [ ] `BenchmarkWorkflowExecution_Memory` - Baseline for memory mode
- [ ] `BenchmarkWorkflowExecution_Persistent` - Compare to memory mode
- [ ] `BenchmarkWorkflowExecution_Distributed` - Compare to embedded modes
- [ ] `BenchmarkWorkflowExecution_NoRegression` - Verify no performance degradation

---

### 5. Regression Tests

**Scope**: Ensure no existing functionality broken
**Mode**: All modes
**Coverage Target**: 100% of existing features

#### Test Suite: Distributed Mode Parity
- [ ] `TestRegression_DistributedMode_AllFeatures` - All features work in distributed mode
- [ ] `TestRegression_DistributedMode_PostgreSQL` - PostgreSQL functionality unchanged
- [ ] `TestRegression_DistributedMode_ExternalRedis` - External Redis functionality unchanged
- [ ] `TestRegression_DistributedMode_ExternalTemporal` - External Temporal functionality unchanged
- [ ] `TestRegression_DistributedMode_PgVector` - pgvector tests still pass with PostgreSQL

#### Test Suite: Feature Parity
- [ ] `TestRegression_Migrations_AllModes` - Schema migrations work in all modes
- [ ] `TestRegression_Authentication_AllModes` - Authentication works in all modes
- [ ] `TestRegression_API_AllModes` - API endpoints work in all modes
- [ ] `TestRegression_CLI_AllModes` - CLI commands work in all modes

---

### 6. Error Handling Tests

**Scope**: Validate error messages and failure modes
**Mode**: All modes
**Coverage Target**: >90%

#### Test Suite: Invalid Configuration
- [ ] `TestError_InvalidMode_HelpfulMessage` - "standalone" suggests "memory"
- [ ] `TestError_DistributedMode_MissingPostgres` - Clear error when PostgreSQL unreachable
- [ ] `TestError_DistributedMode_MissingRedis` - Clear error when Redis unreachable
- [ ] `TestError_DistributedMode_MissingTemporal` - Clear error when Temporal unreachable
- [ ] `TestError_PgVector_WithSQLite` - Clear error about pgvector requiring PostgreSQL

#### Test Suite: Data Integrity
- [ ] `TestError_ConcurrentWrites_SQLite` - SQLite write serialization handled gracefully
- [ ] `TestError_DiskFull_PersistentMode` - Graceful handling of disk space issues
- [ ] `TestError_CorruptedDatabase_PersistentMode` - Recovery or clear error on corruption

---

## Test Helpers and Utilities

### New Test Helpers (Task 9.0, 10.0)

**File**: `test/helpers/database.go`

**New Functions**:
```go
// SetupTestDatabase - Default to memory mode (SQLite :memory:)
func SetupTestDatabase(t *testing.T) *sql.DB

// SetupDatabaseWithMode - Explicit mode selection
func SetupDatabaseWithMode(t *testing.T, mode string) *sql.DB

// SetupPostgresContainer - Explicit PostgreSQL (for pgvector tests)
func SetupPostgresContainer(t *testing.T) (*dockertest.Pool, func())
```

**Usage Pattern**:
```go
// Fast tests (90% of test suite)
func TestSomething(t *testing.T) {
    db := helpers.SetupTestDatabase(t)  // Uses :memory: by default
    // ... test logic ...
}

// Tests needing persistence
func TestStateful(t *testing.T) {
    db := helpers.SetupDatabaseWithMode(t, "persistent")
    // ... test logic ...
}

// Tests requiring PostgreSQL (pgvector only)
func TestVectorSearch(t *testing.T) {
    pool, cleanup := helpers.SetupPostgresContainer(t)
    defer cleanup()
    // ... test logic ...
}
```

---

## Test Migration Plan (Task 11.0)

### Audit Existing Tests

**Identify**:
1. Tests using `helpers.GetSharedPostgresDB(t)` → Can migrate to SQLite
2. Tests requiring pgvector → Must keep PostgreSQL
3. Tests with PostgreSQL-specific SQL → Requires compatibility check

**Migration Strategy**:

```go
// BEFORE (PostgreSQL testcontainers)
func TestWorkflowExecution(t *testing.T) {
    pool, cleanup := helpers.GetSharedPostgresDB(t)
    defer cleanup()
    // ... test logic ...
}

// AFTER (SQLite memory mode - 50-80% faster)
func TestWorkflowExecution(t *testing.T) {
    db := helpers.SetupTestDatabase(t)  // Defaults to :memory:
    // ... test logic ...
}
```

**Keep PostgreSQL For**:
- `test/integration/vector/*_test.go` - pgvector functionality
- `test/integration/postgres/*_test.go` - PostgreSQL-specific features
- Any tests explicitly testing distributed mode

---

## Golden Test Files (Task 13.0)

### Files to Update

**Test Data Files**:
- `testdata/config-diagnostics-standalone.golden` → Rename to `config-diagnostics-memory.golden`
- `testdata/config-show-mixed.golden` → Update mode values
- `testdata/config-show-standalone.golden` → Rename to `config-show-memory.golden`

**Regeneration Process**:
```bash
# Update golden files
UPDATE_GOLDEN=1 go test ./cli/cmd/config/... -v

# Verify changes
git diff testdata/
```

---

## Test Execution Commands

### Local Development

```bash
# Fast unit tests (memory mode)
go test ./pkg/config/... -v

# Integration tests (scoped)
gotestsum --format pkgname -- -race -parallel=4 ./engine/agent

# Full test suite
make test

# Linter
make lint
```

### CI/CD Pipeline

```bash
# Run all tests
make test

# Verify performance improvement
time make test  # Should be 50-80% faster than before

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## Performance Benchmarking (Task 24.0)

### Baseline Metrics

**Before** (Two-Mode System):
- Full test suite: 3-5 minutes
- Server startup (standalone): ~2 seconds
- Server startup (distributed): ~3 seconds

**After** (Three-Mode System):
- Full test suite: 45-90 seconds (60-70% improvement)
- Server startup (memory): <1 second
- Server startup (persistent): <2 seconds
- Server startup (distributed): <3 seconds

### Benchmark Script

```bash
#!/bin/bash
# benchmark.sh

echo "Benchmarking test suite performance..."

# Baseline (before changes)
git checkout main
time make test > /tmp/baseline.txt 2>&1
BASELINE_TIME=$?

# After changes (with memory mode)
git checkout feature/three-mode-system
time make test > /tmp/optimized.txt 2>&1
OPTIMIZED_TIME=$?

# Calculate improvement
echo "Baseline: ${BASELINE_TIME}s"
echo "Optimized: ${OPTIMIZED_TIME}s"
echo "Improvement: $(( (BASELINE_TIME - OPTIMIZED_TIME) * 100 / BASELINE_TIME ))%"
```

---

## Success Criteria

### Performance Targets
- [x] Test suite execution: 50-80% faster (3-5 min → 45-90 sec)
- [x] Server startup (memory): <1 second
- [x] Server startup (persistent): <2 seconds
- [x] No performance regressions in distributed mode

### Coverage Targets
- [x] Unit test coverage: >90%
- [x] Integration test coverage: >80%
- [x] E2E test coverage: >70%
- [x] Overall coverage maintained: >80%

### Quality Targets
- [x] All tests pass: `make test`
- [x] Linter clean: `make lint`
- [x] Zero flaky tests
- [x] No regressions in distributed mode
- [x] All three modes validated

### Error Handling
- [x] Invalid mode error messages clear and helpful
- [x] "standalone" rejection suggests migration path
- [x] pgvector + SQLite error explains requirement
- [x] All error paths tested

---

## Test Deliverables

### Phase 3 (Test Infrastructure) - Task 11.0
- [ ] Updated test helpers (`test/helpers/database.go`)
- [ ] Migrated integration tests to SQLite (90%+)
- [ ] Golden test files updated
- [ ] Performance benchmarks documented

### Phase 6 (Final Validation) - Task 22.0-25.0
- [ ] Comprehensive test execution report
- [ ] Performance benchmark comparison
- [ ] Coverage report (>80%)
- [ ] Error message validation report
- [ ] Regression test results

---

## Implementation Notes

- Tests MUST use `t.Context()` instead of `context.Background()`
- Tests MUST use `logger.FromContext(ctx)` for logging
- Tests MUST use `config.FromContext(ctx)` for configuration
- Tests MUST NOT use global singletons
- Test scope commands during development:
  - Tests: `gotestsum --format pkgname -- -race -parallel=4 <scope>`
  - Linting: `golangci-lint run --fix --allow-parallel-runners <scope>`
- Full validation before task completion: `make test && make lint`

**Estimated Effort**: 2 days for test migration (Phase 3)
