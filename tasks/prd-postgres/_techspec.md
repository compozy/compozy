# Technical Specification: SQLite Database Backend Support

## Executive Summary

This specification details the implementation strategy for adding SQLite as an alternative database backend to Compozy. The solution uses a **Hybrid Dual-Implementation approach**: maintaining separate PostgreSQL and SQLite implementations behind a unified repository provider pattern, while requiring external vector databases (Qdrant/Redis/Filesystem) when SQLite is selected. PostgreSQL remains the default and recommended option for production workloads.

**Key Architectural Decisions:**
- **Dual Implementation:** Separate `engine/infra/postgres` and `engine/infra/sqlite` packages
- **Factory Pattern:** `engine/infra/repo.Provider` selects implementation based on configuration
- **Vector DB Separation:** SQLite deployments mandate external vector database
- **Zero Breaking Changes:** Existing PostgreSQL code unchanged, fully backwards compatible

## System Architecture

### Domain Placement

**New Components:**
- `engine/infra/sqlite/` - SQLite driver implementation (parallel to `postgres/`)
  - `store.go` - Connection pool management  
  - `authrepo.go` - User/API key repository
  - `taskrepo.go` - Task state repository  
  - `workflowrepo.go` - Workflow state repository
  - `migrations/` - SQLite-specific migration files
  - `helpers.go` - SQLite-specific query utilities

**Modified Components:**
- `engine/infra/repo/provider.go` - Factory selection logic
- `pkg/config/config.go` - Database driver configuration
- `engine/infra/server/dependencies.go` - Database setup routing

**No Changes Required:**
- `engine/workflow`, `engine/task`, `engine/auth` - Domain interfaces unchanged
- `engine/knowledge/vectordb` - Already supports multiple providers

### Component Overview

```
┌─────────────────────────────────────────────────────────┐
│ Configuration Layer (pkg/config)                        │
│ - DatabaseConfig.Driver: "postgres" | "sqlite"          │
│ - Driver-specific fields (PostgreSQL: DSN, SQLite: Path)│
└───────────────────┬─────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────┐
│ Application Layer (Domain Repositories)                 │
│ - workflow.Repository (interface)                       │
│ - task.Repository (interface)                           │
│ - auth.Repository (interface)                           │
└───────────────────┬─────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────┐
│ Infrastructure: Repository Provider (Factory)           │
│ engine/infra/repo/provider.go                           │
│                                                          │
│ func NewProvider(cfg *DatabaseConfig) *Provider         │
│   switch cfg.Driver {                                   │
│     case "postgres": → postgres repositories            │
│     case "sqlite":   → sqlite repositories              │
│   }                                                      │
└───────┬─────────────────────────────────┬───────────────┘
        │                                 │
        ▼                                 ▼
┌──────────────────────┐    ┌──────────────────────────┐
│ PostgreSQL Driver    │    │ SQLite Driver            │
│ engine/infra/postgres│    │ engine/infra/sqlite      │
│ - Uses pgx/pgxpool   │    │ - Uses modernc.org/sqlite│
│ - pgvector support   │    │ - Pure Go, no CGO        │
│ - Row-level locking  │    │ - DB-level locking       │
└──────────────────────┘    └──────────────────────────┘
```

**Data Flow:**
1. Server startup reads `database.driver` from configuration
2. `repo.NewProvider()` creates appropriate repository implementations
3. Domain services receive repository interfaces (unchanged)
4. Repositories execute driver-specific SQL
5. Results returned through common interfaces

## Implementation Design

### Core Interfaces

**Database Driver Interface** (common abstraction for both drivers):

```go
// engine/infra/sqlite/db.go
package sqlite

// DB defines the minimal database interface for SQLite operations
type DB interface {
    Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
    Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    QueryRow(ctx context.Context, query string, args ...any) *sql.Row
    Begin(ctx context.Context) (*sql.Tx, error)
    Close() error
}

// Store implements DB using modernc.org/sqlite
type Store struct {
    db *sql.DB
    path string
}
```

**Repository Provider** (factory pattern):

```go
// engine/infra/repo/provider.go
package repo

type Provider struct {
    driver string
}

func NewProvider(ctx context.Context, cfg *config.DatabaseConfig) (*Provider, error) {
    switch cfg.Driver {
    case "postgres", "":  // default
        pool := setupPostgresPool(ctx, cfg)
        return &Provider{
            driver: "postgres",
            authRepo: postgres.NewAuthRepo(pool),
            taskRepo: postgres.NewTaskRepo(pool),
            workflowRepo: postgres.NewWorkflowRepo(pool),
        }, nil
    case "sqlite":
        db := setupSQLiteDB(ctx, cfg)
        return &Provider{
            driver: "sqlite",
            authRepo: sqlite.NewAuthRepo(db),
            taskRepo: sqlite.NewTaskRepo(db),
            workflowRepo: sqlite.NewWorkflowRepo(db),
        }, nil
    default:
        return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
    }
}
```

**SQLite Repository Implementation** (example - Task):

```go
// engine/infra/sqlite/taskrepo.go
package sqlite

type TaskRepo struct {
    db DB
}

func NewTaskRepo(db DB) *TaskRepo {
    return &TaskRepo{db: db}
}

func (r *TaskRepo) UpsertState(ctx context.Context, state *task.State) error {
    query := `
        INSERT INTO task_states (
            task_exec_id, task_id, workflow_exec_id, workflow_id,
            usage, component, status, execution_type, parent_state_id,
            input, output, error
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT (task_exec_id) DO UPDATE SET
            task_id = excluded.task_id,
            workflow_exec_id = excluded.workflow_exec_id,
            // ... etc (SQLite uses ? placeholders)
    `
    // Convert JSONB fields to JSON strings for SQLite
    usageJSON, _ := json.Marshal(state.Usage)
    inputJSON, _ := json.Marshal(state.Input)
    outputJSON, _ := json.Marshal(state.Output)
    
    _, err := r.db.Exec(ctx, query,
        state.TaskExecID, state.TaskID, state.WorkflowExecID,
        state.WorkflowID, usageJSON, state.Component, state.Status,
        state.ExecutionType, state.ParentStateID, inputJSON, outputJSON, nil)
    return err
}
```

## Planning Artifacts (Must Be Generated With Tech Spec)

The following artifacts have been generated alongside this Tech Spec:

- **Docs Plan:** `tasks/prd-postgres/_docs.md` - Documentation strategy and page outlines
- **Examples Plan:** `tasks/prd-postgres/_examples.md` - Example projects and configurations
- **Tests Plan:** `tasks/prd-postgres/_tests.md` - Test coverage matrix and strategy

### Data Models

**Database Configuration** (extends `pkg/config/config.go`):

```go
type DatabaseConfig struct {
    // Driver selection
    Driver string `koanf:"driver" json:"driver" yaml:"driver" env:"DB_DRIVER" validate:"oneof=postgres sqlite"`
    
    // PostgreSQL-specific (existing fields, unchanged)
    ConnString   string `koanf:"conn_string" json:"conn_string" yaml:"conn_string"`
    Host         string `koanf:"host" json:"host" yaml:"host"`
    Port         string `koanf:"port" json:"port" yaml:"port"`
    User         string `koanf:"user" json:"user" yaml:"user"`
    Password     string `koanf:"password" json:"password" yaml:"password"`
    DBName       string `koanf:"dbname" json:"dbname" yaml:"dbname"`
    SSLMode      string `koanf:"sslmode" json:"sslmode" yaml:"sslmode"`
    
    // SQLite-specific (new)
    Path         string `koanf:"path" json:"path" yaml:"path"`  // File path or ":memory:"
    
    // Common settings
    MaxOpenConns int `koanf:"max_open_conns" json:"max_open_conns" yaml:"max_open_conns"`
    // ... existing common fields
}
```

**Migration File Structure:**

```
engine/infra/sqlite/migrations/
├── 20250603124835_create_workflow_states.sql  (SQLite version)
├── 20250603124915_create_task_states.sql      (SQLite version)
├── 20250711163857_create_users.sql            (SQLite version)
└── 20250711163858_create_api_keys.sql         (SQLite version)
```

### API Endpoints

No new API endpoints required. Database selection is configuration-driven.

## Integration Points

### Vector Database Validation

When using SQLite, the system must validate vector database configuration at startup:

```go
// engine/infra/server/dependencies.go
func (s *Server) validateDatabaseConfig(cfg *config.Config) error {
    if cfg.Database.Driver == "sqlite" {
        // SQLite cannot use pgvector
        if len(cfg.Knowledge.VectorDBs) == 0 {
            return fmt.Errorf(
                "SQLite requires external vector database. " +
                "Configure Qdrant, Redis, or Filesystem in knowledge.vector_dbs")
        }
        
        // Ensure no pgvector provider configured
        for _, vdb := range cfg.Knowledge.VectorDBs {
            if vdb.Provider == "pgvector" {
                return fmt.Errorf(
                    "pgvector provider incompatible with SQLite. " +
                    "Use Qdrant, Redis, or Filesystem instead")
            }
        }
    }
    return nil
}
```

### Migration System

Use `github.com/pressly/goose/v3` (already in project) which supports both PostgreSQL and SQLite:

```go
// engine/infra/sqlite/migrations.go
func ApplyMigrations(ctx context.Context, dbPath string) error {
    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        return fmt.Errorf("open db for migrations: %w", err)
    }
    defer db.Close()
    
    // Enable foreign keys (required for SQLite)
    if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
        return fmt.Errorf("enable foreign keys: %w", err)
    }
    
    goose.SetBaseFS(migrationsFS)
    return goose.Up(db, ".")
}
```

## Impact Analysis

| Affected Component | Type of Impact | Description & Risk Level | Required Action |
|-------------------|---------------|--------------------------|-----------------|
| `pkg/config` | Config Schema Change | Add `database.driver` field. Low risk - optional field with default. | Add field with "postgres" default |
| `engine/infra/repo` | Factory Logic | Switch statement for driver selection. Low risk - isolated change. | Add SQLite case to factory |
| `engine/infra/server` | Startup Logic | Database setup routing. Low risk - early failure path. | Route to SQLite or PostgreSQL setup |
| `test/integration` | Test Infrastructure | Parameterized tests for both drivers. Medium risk - test duplication. | Create test helpers for multi-driver tests |
| `test/helpers` | Test Utilities | Database setup helpers. Low risk - additive change. | Add `SetupTestDatabase(driver)` helper |
| CI/CD Pipeline | Matrix Testing | Run tests against both databases. Medium risk - 2x test time. | Add SQLite test job alongside PostgreSQL |
| Vector DB Validation | Startup Check | New validation logic. Low risk - clear error messages. | Add validation in server dependencies |
| Documentation | Multiple Pages | Database selection guide. Low risk - new content. | Create decision matrix, config examples |

**Performance Impact:**
- SQLite: Acceptable for single-tenant, low-concurrency (5-10 workflows)
- PostgreSQL: Unchanged, remains high-performance for production
- Tests: SQLite in-memory mode may be faster for CI/CD

## Testing Approach

### Unit Tests

**Test Organization:**
```
engine/infra/sqlite/
├── store_test.go          # Connection, configuration
├── authrepo_test.go       # User, API key operations
├── taskrepo_test.go       # Task CRUD, hierarchy, transactions
├── workflowrepo_test.go   # Workflow CRUD, state management
├── migrations_test.go     # Schema creation, foreign keys
```

**Key Test Scenarios:**
- **Store Tests:**
  - Should create database file at specified path
  - Should use in-memory database for `:memory:` path
  - Should enable foreign keys on connection
  - Should handle concurrent connections (pool management)

- **Repository Tests:**
  - Should implement same interface as PostgreSQL repos
  - Should pass shared repository test suite
  - Should handle JSONB → JSON conversion correctly
  - Should enforce foreign key constraints
  - Should support upsert operations

**Mock Strategy:** 
- External services only (LLM providers, MCP servers)
- Database operations use real SQLite (in-memory for speed)
- Avoid mocking repositories - test real implementations

### Integration Tests

**Test Infrastructure:**
```go
// test/helpers/database.go
func SetupTestDatabase(t *testing.T, driver string) (repo.Provider, func()) {
    switch driver {
    case "postgres":
        return setupPostgresTest(t)
    case "sqlite":
        return setupSQLiteTest(t)  // Uses :memory:
    }
}

// Parameterized integration tests
func TestWorkflowExecution(t *testing.T) {
    for _, driver := range []string{"postgres", "sqlite"} {
        t.Run(driver, func(t *testing.T) {
            provider, cleanup := SetupTestDatabase(t, driver)
            defer cleanup()
            
            // Test workflow execution end-to-end
            testWorkflowLifecycle(t, provider)
        })
    }
}
```

**Integration Test Paths:**
- `test/integration/database/` - Database-specific tests
  - `sqlite_concurrency_test.go` - Concurrent write handling
  - `sqlite_transactions_test.go` - Transaction isolation
  - `sqlite_migrations_test.go` - Schema migrations

- `test/integration/workflow/` - Existing workflow tests, run against both drivers
- `test/integration/task/` - Existing task tests, run against both drivers

**Test Data:**
- Use `test/fixtures/workflows/*.yaml` (existing fixtures)
- SQLite-specific fixtures for edge cases if needed

## Development Sequencing

### Build Order

**1. Foundation Infrastructure (Week 1-2)**
- Create `engine/infra/sqlite` package structure
- Implement `Store` (connection management)
- Add `database.driver` to configuration system
- Create migration files (SQLite-specific SQL)
- Set up test infrastructure (helpers, parameterized tests)

**Why First:** Establishes foundation without touching existing code. Low risk, enables parallel development.

**2. Authentication Repository (Week 2)**
- Implement `sqlite.AuthRepo` (users, API keys)
- Port migrations: `create_users.sql`, `create_api_keys.sql`
- Write unit + integration tests

**Why Second:** Simplest repository (basic CRUD, no hierarchy). Validates approach before complex repositories.

**3. Workflow Repository (Week 3)**
- Implement `sqlite.WorkflowRepo` (workflow state)
- Port migration: `create_workflow_states.sql`
- Handle JSON fields (usage, input, output, error)
- Implement transaction handling
- Write tests (especially JSON operations)

**Why Third:** Introduces JSON handling patterns, simpler than task repository (no hierarchy).

**4. Task Repository (Week 4-5)**
- Implement `sqlite.TaskRepo` (task state)
- Port migration: `create_task_states.sql`
- Implement hierarchical queries (parent-child relationships)
- Handle complex CHECK constraints
- Optimize indexes for performance
- Write comprehensive tests (hierarchy, locking)

**Why Fourth:** Most complex repository. Builds on patterns from auth and workflow. Critical for workflow execution.

**5. Repository Provider Factory (Week 5)**
- Update `engine/infra/repo/provider.go` factory
- Add driver selection logic
- Implement configuration routing
- Add vector DB validation for SQLite

**Why Fifth:** Integration point. All repositories must exist first.

**6. Server Integration (Week 6)**
- Update `engine/infra/server/dependencies.go`
- Add database setup routing
- Implement startup validation
- Add configuration validation
- Write integration tests (full server startup)

**Why Sixth:** End-to-end integration. Validates entire stack.

**7. Testing & Performance (Week 7-8)**
- Run full test suite with both drivers
- Performance benchmarking (SQLite vs PostgreSQL)
- Concurrency stress tests (SQLite limitations)
- CI/CD matrix setup
- Fix bugs and optimize

**Why Seventh:** Quality assurance phase. All functionality must exist first.

**8. Documentation (Week 9)**
- Write database selection guide
- Create configuration examples
- Document performance characteristics
- Write migration guide
- Create tutorial/walkthrough

**Why Eighth:** Documentation needs working implementation to test examples.

### Technical Dependencies

**Blocking Dependencies:**
1. **SQLite Driver Selection:** Decision on `modernc.org/sqlite` vs `go-sqlite3`
   - **Resolution:** Use `modernc.org/sqlite` (pure Go, no CGO)
   - **Fallback:** `go-sqlite3` if performance issues

2. **Migration Strategy:** Dual migration files vs shared with conditionals
   - **Resolution:** Dual migration files (cleaner, database-specific SQL)
   - **Location:** `engine/infra/sqlite/migrations/` (parallel to postgres)

3. **Test Infrastructure:** In-memory vs file-based SQLite for tests
   - **Resolution:** In-memory (`:memory:`) for unit tests, file-based for integration
   - **Benefit:** Faster tests, automatic cleanup

**Non-Blocking (Parallel Work):**
- Documentation writing (can start alongside implementation)
- Example project creation (can use PostgreSQL first)
- Performance benchmarking setup (can prepare harness early)

## Monitoring & Observability

### Metrics

Use existing `engine/infra/monitoring` package. Add driver-specific labels:

```go
// Database operation metrics (already exists, add driver label)
database_query_duration_seconds{driver="sqlite", operation="select"}
database_query_total{driver="sqlite", operation="insert", status="success|error"}
database_connection_pool_active{driver="sqlite"}

// SQLite-specific metrics
database_sqlite_wal_size_bytes    // Write-Ahead Log size
database_sqlite_page_count        // Total pages
database_sqlite_file_size_bytes   // Database file size
```

### Logging

Use `logger.FromContext(ctx)` pattern (existing):

```go
log := logger.FromContext(ctx)
log.Info("Database initialized",
    "driver", "sqlite",
    "path", dbPath,
    "mode", mode,  // "file" or "memory"
)

log.Warn("SQLite concurrency limitation",
    "driver", "sqlite",
    "concurrent_workflows", count,
    "recommendation", "Use PostgreSQL for >10 concurrent workflows",
)
```

### Health Checks

Add SQLite-specific health check (integrate with existing `/health` endpoint):

```go
func (s *Store) HealthCheck(ctx context.Context) error {
    // Pragma check
    var fkEnabled int
    if err := s.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
        return fmt.Errorf("health check failed: %w", err)
    }
    if fkEnabled != 1 {
        return errors.New("foreign keys not enabled")
    }
    
    // Simple query
    if err := s.db.PingContext(ctx); err != nil {
        return fmt.Errorf("ping failed: %w", err)
    }
    
    return nil
}
```

## Technical Considerations

### Key Decisions

**Decision 1: Dual Implementation vs Abstraction Layer**

**Chosen:** Dual Implementation (separate `postgres/` and `sqlite/` packages)

**Rationale:**
- Allows database-specific optimizations (PostgreSQL keeps pgx features)
- Cleaner code (no generic abstraction leaking)
- Lower risk (existing PostgreSQL code unchanged)
- Easier debugging (clear separation)

**Trade-off:** Code duplication (~80% similar code)

**Alternatives Rejected:**
- **Abstraction Layer:** Too complex, loses database-specific features, harder to maintain
- **Shared Implementation:** Least-common-denominator approach, loses PostgreSQL advantages

---

**Decision 2: SQLite Driver Selection**

**Chosen:** `modernc.org/sqlite` (pure Go)

**Rationale:**
- No CGO required (easier cross-compilation)
- Fully Go-native (better integration with Go toolchain)
- Active maintenance
- Good performance for target use case

**Trade-off:** Slightly slower than `go-sqlite3` (CGO-based)

**Alternatives Considered:**
- **`github.com/mattn/go-sqlite3`:** More mature, faster, but requires CGO
- **Fallback Plan:** Switch to `go-sqlite3` if performance issues arise

---

**Decision 3: Vector DB Strategy for SQLite**

**Chosen:** Mandatory external vector DB (Qdrant/Redis/Filesystem)

**Rationale:**
- SQLite has no native vector support
- `sqlite-vss` extension too experimental
- Existing vector DB integrations already available
- Clean separation of concerns

**Trade-off:** Additional dependency for knowledge features

**Alternatives Rejected:**
- **sqlite-vss:** Experimental, limited features
- **Disable Knowledge:** Too limiting for users
- **Embed pgvector:** Not possible (requires PostgreSQL)

---

**Decision 4: Transaction Locking Strategy**

**Chosen:** Database-level locking (SQLite default) + optimistic locking for specific cases

**Rationale:**
- SQLite doesn't support row-level locking
- Target use case (single-tenant, low-concurrency) works with DB-level locking
- Optimistic locking (version columns) for critical update paths

**Trade-off:** Lower concurrency than PostgreSQL

**Implementation:**
```go
// Optimistic locking example
func (r *TaskRepo) UpdateWithVersion(ctx context.Context, state *task.State) error {
    query := `
        UPDATE task_states 
        SET status = ?, version = version + 1
        WHERE task_exec_id = ? AND version = ?
    `
    result, err := r.db.Exec(ctx, query, state.Status, state.TaskExecID, state.Version)
    if err != nil {
        return err
    }
    rows, _ := result.RowsAffected()
    if rows == 0 {
        return ErrConcurrentUpdate  // Version mismatch
    }
    return nil
}
```

### Known Risks

**Risk 1: Concurrency Bottleneck**

**Description:** SQLite serializes writes at database level

**Likelihood:** Medium - Users deploy SQLite in high-concurrency scenarios

**Impact:** High - Performance degradation, locked database

**Mitigation:**
- Document concurrency limits clearly (5-10 workflows recommended)
- Add startup warning if SQLite + high-concurrency config detected
- Provide performance benchmarks in documentation
- Keep PostgreSQL as recommended production option

---

**Risk 2: SQL Syntax Differences**

**Description:** PostgreSQL and SQLite have different SQL dialects

**Likelihood:** High - Many subtle differences exist

**Impact:** Medium - Query failures, incorrect results

**Examples:**
- Placeholders: `$1, $2` (PostgreSQL) vs `?, ?` (SQLite)
- Arrays: `ANY($1::type[])` (PostgreSQL) vs `IN (?, ?, ?)` (SQLite)
- JSON: `->>`/`->>` (PostgreSQL) vs `json_extract()` (SQLite)
- Types: `timestamptz` (PostgreSQL) vs `DATETIME` (SQLite)

**Mitigation:**
- Separate migration files for each database
- Comprehensive test suite covering all queries
- SQL syntax compatibility layer for common operations
- Code review checklist for SQL differences

---

**Risk 3: Migration Divergence**

**Description:** PostgreSQL and SQLite schemas drift over time

**Likelihood:** Medium - New features may only add PostgreSQL migrations

**Impact:** Medium - Features unavailable on SQLite

**Mitigation:**
- Automated schema comparison tests
- CI/CD validation of both migration sets
- Code review process includes both databases
- Migration template/checklist for new features

### Special Requirements

**Performance Targets (SQLite):**
- Read latency: <50ms p99 (for single workflow queries)
- Write latency: <100ms p99 (for state updates)
- Concurrent workflows: Support 5-10 simultaneous executions
- Database size: <500MB for typical 1000-workflow history

**Security Considerations:**
- SQLite file permissions: 0600 (owner read/write only)
- Path validation: Prevent directory traversal in `database.path`
- No sensitive data in database file name
- Backup/export commands must respect file permissions

**Monitoring:**
- Track database file growth
- Monitor write contention (lock wait times)
- Alert on excessive database size (>1GB)
- Track query performance per driver

### Standards Compliance

**Architecture Compliance:**
- ✅ Follows Clean Architecture (domain → application → infrastructure layers)
- ✅ Repository pattern with interfaces (Dependency Inversion Principle)
- ✅ Factory pattern for provider selection (Open/Closed Principle)
- ✅ Context-first configuration (`config.FromContext(ctx)`)
- ✅ Context-first logging (`logger.FromContext(ctx)`)

**Go Coding Standards:**
- ✅ No global configuration state
- ✅ Constructor pattern with nil-safe defaults
- ✅ Error wrapping with context (`fmt.Errorf("...: %w", err)`)
- ✅ Context propagation throughout
- ✅ Resource cleanup with defer
- ✅ Test naming: `t.Run("Should ...")`

**Testing Standards:**
- ✅ Unit tests for all new code (80%+ coverage)
- ✅ Integration tests with real databases (no mocks for DB)
- ✅ Parameterized tests for multi-driver scenarios
- ✅ Test helpers in `test/helpers/`
- ✅ Fixtures in `test/fixtures/`

**Backward Compatibility:**
- ✅ No breaking changes (project in alpha, but maintain PostgreSQL compatibility)
- ✅ PostgreSQL remains default driver
- ✅ Existing configurations work unchanged
- ✅ Additive changes only (new `database.driver` field)

## Build vs Buy Analysis

**External Libraries Research:**

| Library | Purpose | License | Adoption | Decision |
|---------|---------|---------|----------|----------|
| `modernc.org/sqlite` | SQLite driver | BSD-3 | 1.1k+ stars | ✅ **ADOPT** |
| `github.com/mattn/go-sqlite3` | SQLite driver (CGO) | MIT | 7.7k+ stars | 🔄 **FALLBACK** |
| `github.com/pressly/goose/v3` | Migrations | MIT | 6.6k+ stars | ✅ **EXISTING** |
| `github.com/Masterminds/squirrel` | Query builder | MIT | 7k+ stars | ✅ **EXISTING** |

**Rationale:**
- **`modernc.org/sqlite`:** Pure Go implementation, no CGO, excellent for target use case (development/edge). Active maintenance, good documentation, sufficient performance.
- **`go-sqlite3`:** Backup option if performance issues. More mature but requires CGO (complicates cross-compilation).
- **`goose`:** Already in project, supports both PostgreSQL and SQLite. No need to change migration tool.
- **`squirrel`:** Already in project, database-agnostic query builder. Reuse for both drivers.

**Build Decision:** Implement repository layer in-house. External libraries only for database drivers and migrations. Custom repository implementations allow:
- Database-specific optimizations
- Clean interface alignment
- Full control over query patterns
- Better testing and debugging

## Libraries Assessment Summary

**Primary Dependency:** `modernc.org/sqlite`
- **License:** BSD-3-Clause (permissive, commercial-friendly)
- **Maintenance:** Active (commits within 30 days)
- **Maturity:** Production-ready, used in many projects
- **Performance:** Acceptable for target workloads (single-tenant, low-concurrency)
- **Integration:** Standard `database/sql` interface (drop-in replacement)
- **Security:** No known CVEs, actively maintained
- **Footprint:** ~2MB added to binary

**Migration Considerations:**
- No breaking changes to existing code
- PostgreSQL driver (`pgx`) remains unchanged
- New dependency only loaded when SQLite driver selected
- Pure Go implementation (no CGO) simplifies deployment

**Alternatives Evaluated:**
- Rejected embedding PostgreSQL (too heavy, defeats purpose)
- Rejected custom database abstraction (too complex)
- Rejected database-agnostic ORM (loses control, leaky abstraction)

---

## Appendices

### Appendix A: File Impact Inventory

**PostgreSQL-Specific Files (To Reference/Port):**

```
engine/infra/postgres/
├── authrepo.go          (~178 lines) - User/API key operations
├── config.go            (~24 lines)  - PostgreSQL configuration
├── doc.go               (~10 lines)  - Package documentation
├── dsn.go               (~50 lines)  - Connection string builder
├── jsonb.go             (~50 lines)  - JSONB helper functions
├── metrics.go           (~69 lines)  - Pool metrics and observability
├── migrations.go        (~150 lines) - Migration runner with advisory locks
├── migrations/          (9 SQL files) - Schema definitions
│   ├── 20250603124835_create_workflow_states.sql     (~34 lines)
│   ├── 20250603124915_create_task_states.sql         (~115 lines)
│   ├── 20250711163857_create_users.sql               (~17 lines)
│   ├── 20250711163858_create_api_keys.sql            (~23 lines)
│   ├── 20250711173300_add_api_key_fingerprint.sql    (~27 lines)
│   ├── 20250712120000_add_task_hierarchy_indexes.sql (~20 lines)
│   ├── 20250916090000_add_task_state_query_indexes.sql (~15 lines)
│   ├── 20251012060000_enable_pgvector_extension.sql  (~10 lines)
│   └── 20251016150000_add_task_states_task_exec_idx.sql (~12 lines)
├── placeholders.go      (~39 lines)  - Query placeholder helpers
├── queries.go           (~50 lines)  - Common query constants
├── scan.go              (~30 lines)  - Result scanning helpers
├── store.go             (~150 lines) - Connection pool management
├── taskrepo.go          (~500 lines) - Task state repository (COMPLEX)
└── workflowrepo.go      (~300 lines) - Workflow state repository

engine/infra/repo/
└── provider.go          (~34 lines)  - NEEDS UPDATE: Factory pattern

engine/knowledge/vectordb/
└── pgvector.go          (~756 lines) - REFERENCE ONLY (cannot port)
```

**Estimated Lines of Code:**
- **Core PostgreSQL driver:** ~1,500 lines
- **Repository implementations:** ~1,000 lines  
- **Migration SQL:** ~300 lines
- **Total to replicate for SQLite:** ~2,800 lines (excluding pgvector)

**New Files to Create for SQLite:**

```
engine/infra/sqlite/
├── store.go             (~150 lines) - Connection management
├── authrepo.go          (~180 lines) - Port from postgres/authrepo.go
├── taskrepo.go          (~520 lines) - Port from postgres/taskrepo.go (add SQLite syntax)
├── workflowrepo.go      (~310 lines) - Port from postgres/workflowrepo.go
├── migrations.go        (~120 lines) - SQLite migration runner (no advisory locks)
├── migrations/          (4 SQL files) - SQLite-specific schema
│   ├── 20250603124835_create_workflow_states.sql
│   ├── 20250603124915_create_task_states.sql
│   ├── 20250711163857_create_users.sql
│   └── 20250711163858_create_api_keys.sql
├── helpers.go           (~80 lines)  - SQLite-specific utilities
├── config.go            (~30 lines)  - SQLite configuration
└── doc.go               (~15 lines)  - Package documentation

test/helpers/
└── database.go          (+50 lines)  - Add SetupTestDatabase(driver) helper
```

### Appendix B: Feature Compatibility Matrix (Detailed)

| PostgreSQL Feature | SQLite Equivalent | Migration Complexity | Notes |
|-------------------|------------------|---------------------|-------|
| **Data Types** | | | |
| `text` | `TEXT` | ✅ Compatible | Same type |
| `jsonb` | `TEXT` (JSON string) | 🟡 Medium | Use `json_extract()`, store as TEXT |
| `timestamptz` | `DATETIME` or `TEXT` | 🟡 Medium | Store as ISO8601 string or Unix timestamp |
| `bytea` | `BLOB` | ✅ Compatible | Binary data support |
| `boolean` | `INTEGER` (0/1) | ✅ Compatible | SQLite uses 0/1 for booleans |
| **Placeholders** | | | |
| `$1, $2, $3` | `?, ?, ?` | 🟡 Medium | Replace in all queries |
| **JSON Operations** | | | |
| `usage->>'key'` | `json_extract(usage, '$.key')` | 🟡 Medium | Different syntax |
| `jsonb_typeof()` | `json_type()` | ✅ Compatible | Similar function |
| `jsonb` operators | JSON functions | 🟡 Medium | More verbose in SQLite |
| **Arrays** | | | |
| `ANY($1::uuid[])` | `IN (?, ?, ?)` | 🔴 High | Expand arrays to multiple placeholders |
| Array operations | String splitting | 🔴 High | SQLite has no native arrays |
| **Constraints** | | | |
| `CHECK (...)` | `CHECK (...)` | ✅ Compatible | Same syntax |
| `FOREIGN KEY ... CASCADE` | `FOREIGN KEY ... CASCADE` | ✅ Compatible | Enable with `PRAGMA foreign_keys = ON` |
| `UNIQUE` | `UNIQUE` | ✅ Compatible | Same syntax |
| **Indexes** | | | |
| B-tree (default) | B-tree (default) | ✅ Compatible | Same |
| `GIN (jsonb)` | Expression index | 🟡 Medium | Use `CREATE INDEX ... ON table(json_extract(...))` |
| Partial indexes | Partial indexes | ✅ Compatible | Same `WHERE` clause syntax |
| `lower(email)` index | `lower(email)` index | ✅ Compatible | Same expression syntax |
| **Transactions** | | | |
| `BEGIN/COMMIT` | `BEGIN/COMMIT` | ✅ Compatible | Same |
| `FOR UPDATE` | ❌ Not supported | 🔴 High | Use optimistic locking with version columns |
| Savepoints | Savepoints | ✅ Compatible | Same |
| **Locking** | | | |
| Row-level locking | Database-level only | 🔴 High | Fundamental difference |
| Advisory locks | ❌ Not available | 🟡 Medium | Use file locks for migrations |
| **Functions** | | | |
| `now()` | `datetime('now')` or `CURRENT_TIMESTAMP` | 🟡 Medium | Different syntax |
| `GREATEST()` | `max()` | ✅ Compatible | Similar |
| **Upsert** | | | |
| `ON CONFLICT ... DO UPDATE` | `ON CONFLICT ... DO UPDATE` | ✅ Compatible | Same syntax (SQLite 3.24+) |
| **Extensions** | | | |
| pgvector | ❌ None | 🔴 **BLOCKER** | Require external vector DB |

**Legend:**
- ✅ **Compatible:** Direct port, minimal changes
- 🟡 **Medium:** Requires syntax changes but straightforward
- 🔴 **High:** Significant changes or workarounds needed

### Appendix C: SQL Schema Examples

**PostgreSQL Migration (existing):**

```sql
-- engine/infra/postgres/migrations/20250603124835_create_workflow_states.sql
CREATE TABLE IF NOT EXISTS workflow_states (
    workflow_exec_id text NOT NULL PRIMARY KEY,
    workflow_id      text NOT NULL,
    status           text NOT NULL,
    usage            jsonb,                           -- PostgreSQL JSONB type
    input            jsonb,
    output           jsonb,
    error            jsonb,
    created_at       timestamptz NOT NULL DEFAULT now(),  -- PostgreSQL timestamptz
    updated_at       timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE workflow_states
    ADD CONSTRAINT chk_workflow_states_usage_json
    CHECK (usage IS NULL OR jsonb_typeof(usage) = 'array');  -- PostgreSQL function

CREATE INDEX idx_workflow_states_status ON workflow_states (status);
```

**SQLite Migration (to create):**

```sql
-- engine/infra/sqlite/migrations/20250603124835_create_workflow_states.sql
CREATE TABLE IF NOT EXISTS workflow_states (
    workflow_exec_id TEXT NOT NULL PRIMARY KEY,
    workflow_id      TEXT NOT NULL,
    status           TEXT NOT NULL,
    usage            TEXT,                             -- Store JSON as TEXT
    input            TEXT,
    output           TEXT,
    error            TEXT,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),  -- SQLite datetime
    updated_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Check constraint using SQLite's json_type()
-- Note: SQLite doesn't have ALTER TABLE ADD CONSTRAINT, so include in CREATE TABLE
-- or create as separate check:
CREATE TABLE IF NOT EXISTS workflow_states (
    -- ... (as above)
    CHECK (usage IS NULL OR json_type(usage) = 'array')  -- SQLite function
);

CREATE INDEX idx_workflow_states_status ON workflow_states (status);
```

**PostgreSQL Task States (complex):**

```sql
-- engine/infra/postgres/migrations/20250603124915_create_task_states.sql
CREATE TABLE IF NOT EXISTS task_states (
    task_exec_id     text NOT NULL PRIMARY KEY,
    workflow_exec_id text NOT NULL,
    parent_state_id  text,
    usage            jsonb,
    input            jsonb,
    output           jsonb,
    error            jsonb,
    -- ... other fields
    
    -- Foreign keys with CASCADE
    CONSTRAINT fk_workflow
      FOREIGN KEY (workflow_exec_id)
      REFERENCES workflow_states (workflow_exec_id)
      ON DELETE CASCADE,
    
    CONSTRAINT fk_parent_task
      FOREIGN KEY (parent_state_id)
      REFERENCES task_states (task_exec_id)
      ON DELETE CASCADE,
    
    -- Complex CHECK constraint
    CONSTRAINT chk_execution_type_consistency
    CHECK (
        (execution_type = 'basic' AND (
            (agent_id IS NOT NULL AND action_id IS NOT NULL) OR
            (tool_id IS NOT NULL AND agent_id IS NULL)
        )) OR
        (execution_type = 'router' AND agent_id IS NULL)
        -- ... more conditions
    )
);
```

**SQLite Task States (ported):**

```sql
-- engine/infra/sqlite/migrations/20250603124915_create_task_states.sql
-- Note: Enable foreign keys first with PRAGMA
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS task_states (
    task_exec_id     TEXT NOT NULL PRIMARY KEY,
    workflow_exec_id TEXT NOT NULL,
    parent_state_id  TEXT,
    usage            TEXT,  -- JSON as TEXT
    input            TEXT,
    output           TEXT,
    error            TEXT,
    -- ... other fields
    
    -- Foreign keys work the same in SQLite (when enabled)
    FOREIGN KEY (workflow_exec_id)
      REFERENCES workflow_states (workflow_exec_id)
      ON DELETE CASCADE,
    
    FOREIGN KEY (parent_state_id)
      REFERENCES task_states (task_exec_id)
      ON DELETE CASCADE,
    
    -- CHECK constraints work the same
    CHECK (
        (execution_type = 'basic' AND (
            (agent_id IS NOT NULL AND action_id IS NOT NULL) OR
            (tool_id IS NOT NULL AND agent_id IS NULL)
        )) OR
        (execution_type = 'router' AND agent_id IS NULL)
        -- ... same conditions
    )
);
```

### Appendix D: Performance Characteristics

**Comparative Performance Analysis:**

| Operation | PostgreSQL | SQLite | Notes |
|-----------|-----------|--------|-------|
| **Read Performance** | ⭐⭐⭐⭐⭐ (Excellent) | ⭐⭐⭐⭐ (Very Good) | Both I/O-bound; PostgreSQL has better caching |
| **Write Performance** | ⭐⭐⭐⭐⭐ (Excellent) | ⭐⭐⭐ (Good) | SQLite write serialization limits throughput |
| **Concurrent Writes** | ⭐⭐⭐⭐⭐ (25+ workflows) | ⭐⭐ (5-10 workflows) | SQLite database-level locking |
| **Concurrent Reads** | ⭐⭐⭐⭐⭐ (Unlimited) | ⭐⭐⭐⭐⭐ (Unlimited) | Both excellent for read-heavy workloads |
| **Complex Queries** | ⭐⭐⭐⭐⭐ (Excellent) | ⭐⭐⭐⭐ (Very Good) | PostgreSQL has query planner advantages |
| **Vector Search** | ⭐⭐⭐⭐⭐ (pgvector built-in) | ❌ (External DB required) | **Critical difference** |
| **JSON Operations** | ⭐⭐⭐⭐⭐ (JSONB native) | ⭐⭐⭐⭐ (JSON1 extension) | PostgreSQL more feature-rich |
| **Deployment** | ⭐⭐⭐ (Separate service) | ⭐⭐⭐⭐⭐ (Single file) | SQLite much simpler |
| **Horizontal Scaling** | ⭐⭐⭐⭐⭐ (Excellent) | ⭐ (Not designed for this) | PostgreSQL for distributed systems |
| **Backup/Recovery** | ⭐⭐⭐⭐ (pg_dump, WAL) | ⭐⭐⭐⭐⭐ (File copy) | SQLite simpler but less granular |
| **Transaction Safety** | ⭐⭐⭐⭐⭐ (ACID) | ⭐⭐⭐⭐⭐ (ACID) | Both fully ACID-compliant |
| **Memory Footprint** | ⭐⭐⭐ (Higher) | ⭐⭐⭐⭐⭐ (Minimal) | SQLite excellent for constrained environments |

**Performance Targets (SQLite):**

```
Latency Targets:
- Read (single workflow):     p50 < 10ms, p99 < 50ms
- Write (state update):       p50 < 20ms, p99 < 100ms
- Hierarchical query (tasks): p50 < 30ms, p99 < 150ms

Throughput Targets:
- Concurrent workflows:       5-10 simultaneous (recommended)
- Workflow starts/hour:       ~500 (moderate load)
- State updates/second:       ~20-30 (write-heavy)

Storage Targets:
- Database file size:         <500MB for 1000 workflows
- WAL size:                   <50MB typical
- Growth rate:                ~400KB per workflow (avg)
```

### Appendix E: Dependencies

**Required Dependencies (New):**

```go
// go.mod additions
require (
    modernc.org/sqlite v1.31.1  // Pure Go SQLite driver (primary choice)
    // OR
    // github.com/mattn/go-sqlite3 v1.14.22  // CGO-based (fallback)
)
```

**Existing Dependencies (Reused):**

```go
// Already in go.mod
github.com/pressly/goose/v3 v3.20.0      // Migrations (supports both DBs)
github.com/Masterminds/squirrel v1.5.4   // Query builder (DB-agnostic)
github.com/jackc/pgx/v5 v5.6.0           // PostgreSQL (keep existing)
github.com/jackc/pgx/v5/pgxpool v5.6.0   // PostgreSQL pool (keep existing)
```

**Development Dependencies:**

```go
// Testing
github.com/stretchr/testify v1.9.0       // Assertions (existing)
github.com/testcontainers/testcontainers-go v0.31.0  // PostgreSQL containers (existing)
```

**Binary Size Impact:**

```
Current binary (with PostgreSQL):   ~45MB
After adding SQLite (pure Go):      ~47MB (+2MB)
After adding SQLite (CGO):          ~46MB (+1MB)
```

### Appendix F: Key Differences Checklist

**SQL Syntax Differences to Handle:**

```
PostgreSQL → SQLite Conversions:

1. Placeholders:
   - PG: $1, $2, $3          → SQLite: ?, ?, ?

2. Data Types:
   - PG: timestamptz         → SQLite: TEXT (ISO8601) or INTEGER (unix)
   - PG: jsonb               → SQLite: TEXT (JSON string)
   - PG: bytea               → SQLite: BLOB

3. JSON Operations:
   - PG: usage->>'key'       → SQLite: json_extract(usage, '$.key')
   - PG: jsonb_typeof()      → SQLite: json_type()
   - PG: usage @> '{"k":"v"}' → SQLite: (parse and compare)

4. Array Operations:
   - PG: ANY($1::uuid[])     → SQLite: IN (?, ?, ...) with expanded params
   - PG: array_agg()         → SQLite: group_concat() or JSON

5. Date Functions:
   - PG: now()               → SQLite: datetime('now') or CURRENT_TIMESTAMP
   - PG: EXTRACT(YEAR ...)   → SQLite: strftime('%Y', ...)

6. String Functions:
   - PG: lower(), upper()    → SQLite: same
   - PG: concat()            → SQLite: || operator or concat()

7. Aggregates:
   - PG: GREATEST()          → SQLite: max()
   - PG: LEAST()             → SQLite: min()

8. Indexes:
   - PG: GIN (jsonb_col)     → SQLite: Expression index on json_extract()
   - PG: Partial indexes     → SQLite: Same (WHERE clause)

9. Constraints:
   - PG: CHECK (inline)      → SQLite: CHECK (inline) - same
   - PG: Foreign keys        → SQLite: Same but need PRAGMA foreign_keys = ON

10. Locking:
    - PG: FOR UPDATE         → SQLite: Not supported (use optimistic locking)
    - PG: Advisory locks     → SQLite: Not supported (use file locks)
```

### Appendix G: Migration Effort Breakdown

**Detailed Task Breakdown:**

| Task | Subtasks | Estimated Hours | Complexity |
|------|----------|----------------|------------|
| **Phase 1: Foundation** | | **80-120 hours** | |
| SQLite store setup | Connection, pool, health checks | 8-12h | Low |
| Configuration | Add driver field, validation | 4-6h | Low |
| Migration system | Port goose setup, PRAGMA handling | 8-12h | Low |
| Test infrastructure | Helpers, parameterized tests | 12-16h | Medium |
| **Phase 2: Auth Repo** | | **40-60 hours** | |
| Port authrepo.go | Users, API keys | 16-24h | Low |
| Port migrations | create_users, create_api_keys | 4-6h | Low |
| Unit tests | All CRUD operations | 12-16h | Low |
| Integration tests | Full auth flow | 8-12h | Medium |
| **Phase 3: Workflow Repo** | | **60-80 hours** | |
| Port workflowrepo.go | State management | 20-28h | Medium |
| JSON handling | JSONB → TEXT conversion | 8-12h | Medium |
| Port migrations | create_workflow_states | 4-6h | Low |
| Unit tests | CRUD + JSON ops | 16-20h | Medium |
| Integration tests | Full workflow lifecycle | 12-16h | Medium |
| **Phase 4: Task Repo** | | **100-140 hours** | |
| Port taskrepo.go | State management | 32-44h | **High** |
| Hierarchical queries | Parent-child relationships | 20-28h | **High** |
| JSON handling | Complex JSONB operations | 12-16h | Medium |
| Array operations | ANY() → IN() conversion | 8-12h | Medium |
| Port migrations | create_task_states (complex) | 8-12h | Medium |
| Unit tests | CRUD + hierarchy + JSON | 24-32h | High |
| Integration tests | Full task execution | 16-20h | High |
| **Phase 5: Integration** | | **40-60 hours** | |
| Provider factory | Driver selection logic | 8-12h | Low |
| Server integration | Startup routing | 8-12h | Medium |
| Vector DB validation | Startup checks | 4-6h | Low |
| End-to-end tests | Full server with both drivers | 16-24h | Medium |
| Bug fixes | Integration issues | 8-12h | Variable |
| **Phase 6: Performance** | | **60-80 hours** | |
| Benchmarking | Performance tests | 16-20h | Medium |
| Optimization | Query tuning, indexes | 20-28h | High |
| Concurrency testing | Stress tests, lock handling | 16-20h | High |
| CI/CD setup | Matrix testing | 8-12h | Medium |
| **Phase 7: Documentation** | | **40-60 hours** | |
| Technical docs | Decision guide, config | 16-20h | Low |
| Examples | 6 example projects | 16-24h | Medium |
| Migration guide | PostgreSQL ↔ SQLite | 8-12h | Medium |
| **Total** | | **420-600 hours** | **8-12 weeks** |

**Risk Contingency:** Add 20% buffer (84-120 hours) for unforeseen issues, totaling **504-720 hours (10-14 weeks)**.

---

**Document Version:** 1.1  
**Date:** 2025-01-27  
**Author:** AI Analysis  
**Status:** Technical Specification Complete (Enhanced with Appendices)
