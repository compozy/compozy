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

**Document Version:** 1.0  
**Date:** 2025-01-27  
**Author:** AI Analysis  
**Status:** Technical Specification Complete
