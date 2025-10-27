# PostgreSQL to SQLite Replacement Analysis

## Executive Summary

This document provides a comprehensive analysis of PostgreSQL usage throughout the Compozy project and evaluates the feasibility of supporting SQLite as an alternative database backend.

**Key Findings:**

- **Current State:** PostgreSQL is deeply integrated across 4 major subsystems
- **Complexity:** Medium-to-High effort required for SQLite support
- **Blockers:** Vector extension (pgvector) has no SQLite equivalent
- **Recommendation:** Possible but requires significant architectural changes

---

## 1. PostgreSQL Usage Overview

### 1.1 Affected Packages

PostgreSQL is used in the following areas:

| Package                     | Purpose                      | Tables                 | Key Features Used                         |
| --------------------------- | ---------------------------- | ---------------------- | ----------------------------------------- |
| `engine/infra/postgres`     | Core database driver         | All                    | Connection pooling, transactions, metrics |
| `engine/infra/repo`         | Repository provider          | All                    | Factory for all repositories              |
| `engine/workflow`           | Workflow persistence         | `workflow_states`      | JSONB, transactions, upserts              |
| `engine/task`               | Task state management        | `task_states`          | JSONB, foreign keys, hierarchical queries |
| `engine/auth`               | Authentication/authorization | `users`, `api_keys`    | Constraints, indexes                      |
| `engine/knowledge/vectordb` | Vector embeddings            | Dynamic (via pgvector) | **pgvector extension** (CRITICAL)         |
| `engine/memory`             | Memory storage               | N/A                    | Uses Redis (not Postgres)                 |

### 1.2 Database Schema

#### Tables Created by Migrations:

1. **`workflow_states`** (20250603124835)
   - Stores workflow execution state
   - Columns: workflow_exec_id, workflow_id, status, usage (JSONB), input (JSONB), output (JSONB), error (JSONB), timestamps
   - Indexes: status, workflow_id, composite indexes
   - Features: JSONB with type checking

2. **`task_states`** (20250603124915)
   - Stores task execution state with hierarchical relationships
   - Columns: task_exec_id, task_id, workflow_exec_id, execution_type, parent_state_id, usage (JSONB), input/output/error (JSONB), etc.
   - Foreign Keys: workflow_exec_id → workflow_states, parent_state_id → task_states (CASCADE)
   - Complex CHECK constraints for execution_type validation
   - Extensive indexing (20+ indexes)

3. **`users`** (20250711163857)
   - User authentication
   - Columns: id, email (UNIQUE), role (CHECK), created_at
   - Case-insensitive email index: `lower(email)`

4. **`api_keys`** (20250711163858)
   - API key management
   - Foreign Key: user_id → users (CASCADE)
   - Indexes: hash, user_id, prefix

5. **Vector Tables** (Dynamic, via pgvector)
   - Default: `knowledge_chunks`
   - Columns: id, embedding (vector type), document, metadata (JSONB)
   - Indexes: GIN (metadata), IVFFlat/HNSW (vectors)

---

## 2. PostgreSQL-Specific Features Analysis

### 2.1 Critical Features (High Migration Effort)

#### 🚨 **pgvector Extension**

- **Location:** `engine/knowledge/vectordb/pgvector.go`
- **Usage:** Vector similarity search for knowledge bases (RAG)
- **Features:**
  - `vector(N)` data type for embeddings
  - Distance operators: `<=>` (cosine), `<->` (L2), `<#>` (inner product)
  - Specialized indexes: IVFFlat, HNSW
  - Search parameters: `ivfflat.probes`, `hnsw.ef_search`
- **SQLite Equivalent:** ❌ **NONE** - No native vector support
- **Alternatives:**
  - `sqlite-vss` extension (experimental, limited)
  - External vector DB (Qdrant, Redis) - already supported!
  - In-memory vector store (filesystem provider exists)

#### JSONB Support

- **Usage:** Extensive use for flexible data storage
- **Tables:** workflow_states, task_states, knowledge chunks (metadata)
- **Operations:**
  - Storage: `usage jsonb`, `input jsonb`, `output jsonb`, `error jsonb`
  - Type validation: `CHECK (usage IS NULL OR jsonb_typeof(usage) = 'array')`
  - Operators: `->>` (metadata queries in pgvector)
- **SQLite Equivalent:** ✅ **JSON support available** (since 3.38.0)
  - Similar JSON functions: `json_extract()`, `json_type()`
  - Can create functional indexes on JSON fields
  - **Caveat:** Slightly different syntax and capabilities

### 2.2 Important Features (Medium Migration Effort)

#### Transactions with Row Locking

- **Usage:** `GetStateForUpdate(ctx, id)` - row-level locking
- **Location:** `engine/infra/postgres/taskrepo.go`
- **Feature:** `SELECT ... FOR UPDATE`
- **SQLite Equivalent:** ⚠️ **Partial** - SQLite has database-level locking, not row-level

#### Foreign Keys with Cascade

- **Usage:**
  - `task_states.workflow_exec_id` → `workflow_states`
  - `task_states.parent_state_id` → `task_states` (self-reference)
  - `api_keys.user_id` → `users`
- **SQLite Equivalent:** ✅ **Supported** (must enable with PRAGMA)

#### Advisory Locks

- **Usage:** Migration coordination in `ApplyMigrationsWithLock()`
- **Location:** `engine/infra/postgres/migrations.go`
- **SQLite Equivalent:** ❌ **Not available**
- **Impact:** Low - only used for migrations, can use file locks

#### Array Operations

- **Usage:** `WHERE workflow_exec_id = ANY($1::uuid[])`
- **Location:** `engine/infra/postgres/workflowrepo.go`
- **SQLite Equivalent:** ⚠️ **Different syntax** - Need `IN (?, ?, ?)` with parameter expansion

### 2.3 Compatible Features (Low Migration Effort)

#### Upsert (ON CONFLICT)

- **Usage:** Extensive in all repositories
- **SQLite Equivalent:** ✅ **Supported** - Same `ON CONFLICT` syntax

#### Timestamps

- **Usage:** `timestamptz` for all created_at/updated_at
- **SQLite Equivalent:** ✅ **Supported** - Use `DATETIME` or store as TEXT/INTEGER

#### Check Constraints

- **Usage:** Role validation, execution_type validation
- **SQLite Equivalent:** ✅ **Supported** (since 3.3.0)

#### GIN Indexes

- **Usage:** Metadata JSONB indexes in pgvector tables
- **SQLite Equivalent:** ⚠️ **Different** - Can use expression indexes

---

## 3. Code Architecture Analysis

### 3.1 Current Architecture

```
┌─────────────────────────────────────────────┐
│ Application Layer (Domain Packages)        │
│ - engine/workflow                          │
│ - engine/task                              │
│ - engine/auth                              │
│ - engine/knowledge                         │
└──────────────┬──────────────────────────────┘
               │ (interfaces)
               ▼
┌─────────────────────────────────────────────┐
│ Infrastructure Layer                        │
│                                             │
│ ┌─────────────────────────────────────┐   │
│ │ engine/infra/repo/provider.go       │   │
│ │ - NewAuthRepo()                     │   │
│ │ - NewTaskRepo()                     │   │
│ │ - NewWorkflowRepo()                 │   │
│ └─────────────┬───────────────────────┘   │
│               │                             │
│               ▼                             │
│ ┌─────────────────────────────────────┐   │
│ │ engine/infra/postgres/              │   │
│ │ - AuthRepo (pgxpool)                │   │
│ │ - TaskRepo (pgxpool)                │   │
│ │ - WorkflowRepo (pgxpool)            │   │
│ │ - Store (connection management)     │   │
│ └─────────────────────────────────────┘   │
└─────────────────────────────────────────────┘
```

**Key Observations:**

- ✅ **Good:** Repository pattern with interfaces (DIP compliance)
- ✅ **Good:** Domain layer doesn't depend on pgx directly
- ❌ **Problem:** `repo.Provider` is **hardcoded to pgxpool**
- ❌ **Problem:** No database abstraction layer
- ❌ **Problem:** Direct pgx usage in all repository implementations

### 3.2 Dependencies

#### Direct PostgreSQL Dependencies:

```go
// Core driver
github.com/jackc/pgx/v5
github.com/jackc/pgx/v5/pgxpool
github.com/jackc/pgx/v5/pgconn

// Vector support
github.com/pgvector/pgvector-go

// Scanning utilities
github.com/georgysavva/scany/v2/pgxscan

// Query builder
github.com/Masterminds/squirrel  // Database-agnostic

// Migrations
github.com/pressly/goose/v3      // Supports multiple databases
```

**Impact:**

- `pgx` types leak through repository implementations
- `pgxpool.Pool` is the concrete type in `repo.Provider`
- `pgxscan` used extensively for row scanning
- `squirrel` is database-agnostic (✅ can reuse)

### 3.3 Query Patterns

#### Raw SQL Queries:

- ~80% of queries are raw SQL strings
- Heavy use of placeholders ($1, $2, etc.)
- PostgreSQL-specific syntax in several places

#### Query Builder (Squirrel):

- Used for dynamic filters in `ListStates()`
- Already database-agnostic
- Would need placeholder format change for SQLite (`?` instead of `$1`)

---

## 4. Migration Complexity Assessment

### 4.1 Effort Breakdown

| Component                  | Complexity      | Estimated Effort | Notes                                            |
| -------------------------- | --------------- | ---------------- | ------------------------------------------------ |
| **Vector Storage**         | 🔴 **CRITICAL** | N/A              | No SQLite equivalent - **must use alternative**  |
| Database Driver            | 🟡 Medium       | 3-5 days         | Create SQLite driver alongside Postgres          |
| Repository Implementations | 🟡 Medium       | 5-7 days         | Dual implementation or abstraction layer         |
| SQL Syntax Compatibility   | 🟡 Medium       | 3-4 days         | Placeholder formats, array operations, JSON      |
| Migrations                 | 🟢 Low          | 2-3 days         | Goose supports SQLite, need dual migration files |
| Transaction Handling       | 🟡 Medium       | 2-3 days         | Different locking semantics                      |
| Testing                    | 🟡 Medium       | 5-7 days         | Test suite for both databases                    |
| **Total**                  |                 | **20-29 days**   | **4-6 weeks**                                    |

### 4.2 Critical Blockers

#### 1. **Vector Storage (pgvector)**

**Problem:** SQLite has no native vector similarity search.

**Solutions:**
a) **Use Existing Alternatives** (✅ **RECOMMENDED**)

- Qdrant (already supported in `engine/knowledge/vectordb/qdrant.go`)
- Redis with RediSearch (already supported in `engine/knowledge/vectordb/redis.go`)
- Filesystem provider (already exists in `engine/knowledge/vectordb/filesystem.go`)

b) **sqlite-vss Extension** (⚠️ experimental)

- Third-party extension
- Limited maturity
- Requires compilation with extension

c) **Hybrid Approach**

- SQLite for relational data
- External vector DB for embeddings
- Most practical for production use

**Recommendation:** When using SQLite, require users to configure an external vector DB (Qdrant/Redis) for knowledge base features.

#### 2. **Row-Level Locking**

**Problem:** SQLite has database-level locking only.

**Impact:**

- `GetStateForUpdate()` used for concurrent task updates
- Less granular concurrency control

**Solutions:**

- Use `BEGIN IMMEDIATE` for write transactions
- Implement optimistic locking (version columns)
- Accept reduced concurrency (may be acceptable for single-tenant deployments)

#### 3. **Driver Abstraction**

**Problem:** `repo.Provider` uses concrete `*pgxpool.Pool` type.

**Solutions:**
a) **Interface-Based Abstraction**

```go
// Define common DB interface
type DB interface {
    Exec(ctx context.Context, query string, args ...any) (Result, error)
    Query(ctx context.Context, query string, args ...any) (Rows, error)
    QueryRow(ctx context.Context, query string, args ...any) Row
    Begin(ctx context.Context) (Tx, error)
}

// Provider chooses implementation
type Provider struct {
    db DB  // Could be pgxpool or database/sql wrapper
}
```

b) **Factory Pattern by Database Type**

```go
func NewProvider(cfg *config.DatabaseConfig) (*Provider, error) {
    switch cfg.Driver {
    case "postgres":
        return newPostgresProvider(cfg)
    case "sqlite":
        return newSQLiteProvider(cfg)
    default:
        return nil, fmt.Errorf("unsupported driver: %s", cfg.Driver)
    }
}
```

---

## 5. Implementation Strategies

### 5.1 Strategy 1: Dual Implementation (Recommended)

**Approach:** Keep PostgreSQL implementation, add parallel SQLite implementation.

```
engine/infra/
├── postgres/          # Existing implementation
│   ├── store.go
│   ├── authrepo.go
│   ├── taskrepo.go
│   ├── workflowrepo.go
│   └── migrations/
│
├── sqlite/            # New implementation
│   ├── store.go
│   ├── authrepo.go
│   ├── taskrepo.go
│   ├── workflowrepo.go
│   └── migrations/
│
└── repo/
    └── provider.go    # Factory selects implementation
```

**Pros:**

- ✅ No changes to existing Postgres code
- ✅ Each implementation can use native features
- ✅ Easier to maintain database-specific optimizations
- ✅ Lower risk of breaking existing functionality

**Cons:**

- ❌ Code duplication (~80% similar code)
- ❌ Must maintain two codebases

### 5.2 Strategy 2: Abstraction Layer

**Approach:** Create a database-agnostic abstraction layer.

```
engine/infra/
├── store/              # Abstract storage layer
│   ├── interfaces.go   # DB interface
│   ├── common.go       # Shared logic
│   └── provider.go     # Factory
│
├── drivers/
│   ├── postgres/       # Postgres-specific
│   └── sqlite/         # SQLite-specific
│
└── repo/               # Generic repository implementations
    ├── authrepo.go
    ├── taskrepo.go
    └── workflowrepo.go
```

**Pros:**

- ✅ Single repository implementation
- ✅ Easier to add more databases in the future
- ✅ Less code duplication

**Cons:**

- ❌ Loss of database-specific optimizations
- ❌ Complex abstraction layer
- ❌ Harder to leverage advanced features
- ❌ More refactoring of existing code

### 5.3 Recommendation: **Hybrid Approach**

**Best of both worlds:**

1. **Keep critical paths database-specific:**
   - Task state management (complex queries, heavy use)
   - Workflow orchestration (transactions, locking)
   - Keep PostgreSQL optimizations

2. **Abstract simple operations:**
   - User authentication (simple CRUD)
   - API key management (simple CRUD)
   - Use `database/sql` standard library

3. **Separate vector storage:**
   - Make vector DB pluggable (already is!)
   - Require external vector DB when using SQLite

**Implementation:**

```go
// engine/infra/repo/provider.go
type Provider struct {
    driver     string
    taskRepo   task.Repository
    workflowRepo workflow.Repository
    authRepo   auth.Repository
}

func NewProvider(cfg *config.DatabaseConfig) (*Provider, error) {
    switch cfg.Driver {
    case "postgres":
        pool := setupPostgresPool(cfg)
        return &Provider{
            driver:       "postgres",
            taskRepo:     postgres.NewTaskRepo(pool),
            workflowRepo: postgres.NewWorkflowRepo(pool),
            authRepo:     postgres.NewAuthRepo(pool),
        }, nil
    case "sqlite":
        db := setupSQLiteDB(cfg)
        return &Provider{
            driver:       "sqlite",
            taskRepo:     sqlite.NewTaskRepo(db),
            workflowRepo: sqlite.NewWorkflowRepo(db),
            authRepo:     sqlite.NewAuthRepo(db),  // Could share with postgres using database/sql
        }, nil
    }
}
```

---

## 6. Feature Compatibility Matrix

| Feature               | PostgreSQL                 | SQLite                 | Migration Strategy                               |
| --------------------- | -------------------------- | ---------------------- | ------------------------------------------------ |
| **JSONB**             | Native `jsonb`             | `JSON` functions       | Map JSON operations to SQLite equivalents        |
| **Vector Search**     | `pgvector` extension       | ❌ None                | **Require external vector DB**                   |
| **Transactions**      | Full ACID                  | Full ACID              | Compatible                                       |
| **Foreign Keys**      | Native                     | Native (enable PRAGMA) | Compatible                                       |
| **Row Locking**       | `FOR UPDATE`               | ❌ DB-level only       | Use optimistic locking or immediate transactions |
| **Upsert**            | `ON CONFLICT`              | `ON CONFLICT`          | Compatible                                       |
| **Check Constraints** | Native                     | Native                 | Compatible                                       |
| **Array Operations**  | `ANY($1::type[])`          | ❌ None                | Expand to `IN (?, ?, ?)`                         |
| **Advisory Locks**    | `pg_advisory_lock`         | ❌ None                | Use file locks for migrations                    |
| **Timestamptz**       | Native                     | Store as TEXT/INTEGER  | Convert on read/write                            |
| **Indexes**           | B-tree, GIN, IVFFlat, HNSW | B-tree, expression     | GIN → expression indexes                         |
| **Placeholders**      | `$1, $2, $3`               | `?, ?, ?`              | Convert in SQL builder                           |

---

## 7. Configuration Changes Required

### 7.1 Database Configuration

Add driver selection to `pkg/config/config.go`:

```go
type DatabaseConfig struct {
    Driver       string        `koanf:"driver" json:"driver" yaml:"driver" env:"DB_DRIVER"`  // "postgres" or "sqlite"

    // PostgreSQL-specific
    ConnString   string        `koanf:"conn_string" json:"conn_string" yaml:"conn_string"`
    Host         string        `koanf:"host" json:"host" yaml:"host"`
    Port         string        `koanf:"port" json:"port" yaml:"port"`
    User         string        `koanf:"user" json:"user" yaml:"user"`
    Password     string        `koanf:"password" json:"password" yaml:"password"`
    DBName       string        `koanf:"dbname" json:"dbname" yaml:"dbname"`

    // SQLite-specific
    Path         string        `koanf:"path" json:"path" yaml:"path"`  // Path to .db file

    // Common
    MaxOpenConns int           `koanf:"max_open_conns" json:"max_open_conns" yaml:"max_open_conns"`
    // ... rest
}
```

### 7.2 Vector Database Requirements

When using SQLite, enforce vector DB configuration:

```go
func validateDatabaseConfig(cfg *config.Config) error {
    if cfg.Database.Driver == "sqlite" {
        // SQLite cannot use pgvector, require external vector DB
        if len(cfg.Knowledge.VectorDBs) == 0 {
            return fmt.Errorf("SQLite requires external vector database (Qdrant, Redis, or Filesystem)")
        }

        // Ensure no pgvector configurations
        for _, vdb := range cfg.Knowledge.VectorDBs {
            if vdb.Provider == "pgvector" {
                return fmt.Errorf("pgvector provider not compatible with SQLite driver")
            }
        }
    }
    return nil
}
```

---

## 8. Testing Strategy

### 8.1 Test Infrastructure

**Current State:**

- Tests use real PostgreSQL (via testcontainers or local instance)
- Integration tests in `test/integration/`
- Heavy use of `pgxmock` for unit tests

**Required Changes:**

1. **Parameterized Tests**

   ```go
   func TestRepositories(t *testing.T) {
       drivers := []string{"postgres", "sqlite"}
       for _, driver := range drivers {
           t.Run(driver, func(t *testing.T) {
               repo := setupRepo(t, driver)
               testCRUDOperations(t, repo)
               testTransactions(t, repo)
               testConcurrency(t, repo)
           })
       }
   }
   ```

2. **Test Helpers**

   ```go
   // test/helpers/database.go
   func SetupTestDatabase(t *testing.T, driver string) (cleanup func())
   func CreateTestProvider(t *testing.T, driver string) *repo.Provider
   ```

3. **CI/CD Matrix**
   - Run full test suite against both databases
   - Separate test jobs for PostgreSQL and SQLite

### 8.2 Performance Testing

**Key Areas:**

- Query performance comparison
- Transaction throughput
- Concurrent write performance (critical for SQLite)
- Database file size growth (SQLite)

---

## 9. Migration Path for Users

### 9.1 Backwards Compatibility

**✅ No Breaking Changes:**

- Default driver remains `postgres`
- Existing configurations work unchanged
- New `driver` field optional (defaults to postgres)

### 9.2 Migration Tools

**Required Tooling:**

```bash
# Export from PostgreSQL
compozy export --output=backup.json

# Import to SQLite
compozy import --driver=sqlite --db-path=compozy.db --input=backup.json
```

### 9.3 Documentation Updates

**Required Docs:**

1. **Installation Guide:**
   - SQLite setup instructions
   - When to use SQLite vs PostgreSQL
   - Vector DB configuration requirements

2. **Configuration Guide:**
   - Driver selection
   - SQLite-specific settings
   - Performance tuning

3. **Migration Guide:**
   - PostgreSQL → SQLite migration steps
   - Limitations and tradeoffs

---

## 10. Recommendation & Roadmap

### 10.1 Should We Implement SQLite Support?

**✅ YES, IF:**

- Target users need single-binary deployment without external dependencies
- Embedded/edge deployments are a priority
- Development/testing simplification is valuable
- Willing to accept vector DB as external dependency

**❌ NO, IF:**

- High concurrency is critical (PostgreSQL is superior)
- Advanced PostgreSQL features are heavily used
- Team size is small (maintenance burden)
- Vector search is core feature (pgvector is best-in-class)

### 10.2 Phased Implementation

#### **Phase 1: Foundation (2-3 weeks)**

- ✅ Define database abstraction interfaces
- ✅ Create SQLite driver package
- ✅ Implement basic repository pattern for SQLite
- ✅ Add configuration support for driver selection
- ✅ Update test infrastructure

#### **Phase 2: Core Features (3-4 weeks)**

- ✅ Migrate auth repositories to SQLite
- ✅ Migrate workflow repository to SQLite
- ✅ Migrate task repository to SQLite
- ✅ Implement transaction handling
- ✅ Create SQLite migration files

#### **Phase 3: Advanced Features (2-3 weeks)**

- ✅ Handle concurrency patterns
- ✅ Implement optimistic locking
- ✅ Query optimization
- ✅ Performance tuning

#### **Phase 4: Integration (1-2 weeks)**

- ✅ Vector DB integration (enforce external DB)
- ✅ End-to-end testing
- ✅ Documentation
- ✅ Migration tooling

**Total Estimated Time: 8-12 weeks**

### 10.3 Alternative: Improve Current Setup

**Instead of SQLite support, consider:**

1. **Simplify PostgreSQL Deployment:**
   - Provide Docker Compose presets
   - Embedded PostgreSQL for development (pgembed)
   - PostgreSQL in Docker for local development

2. **Improve Development Experience:**
   - Better test database management
   - Faster integration tests
   - Database seeding utilities

3. **Focus on Core Features:**
   - Better pgvector integration
   - Improved query performance
   - Advanced PostgreSQL features (partitioning, replication)

---

## 11. Conclusion

### 11.1 Summary

**PostgreSQL Usage:**

- **Deeply integrated** across 4 major subsystems
- **Advanced features** heavily utilized (JSONB, pgvector, transactions)
- **Clean architecture** with repository pattern (good for abstraction)
- **Critical dependency** on pgvector extension for knowledge base features

**SQLite Feasibility:**

- ✅ **Technically possible** for relational data
- ⚠️ **Medium complexity** (8-12 weeks effort)
- ❌ **Critical blocker:** pgvector has no SQLite equivalent
- ⚠️ **Concurrency limitations** due to database-level locking

### 11.2 Final Recommendation

**Recommend implementing SQLite support with the following caveats:**

1. **Vector Storage Strategy:**
   - **Requirement:** When using SQLite, users MUST configure external vector DB (Qdrant/Redis/Filesystem)
   - **Validation:** Enforce this requirement at startup
   - **Documentation:** Clear guidance on vector DB selection

2. **Use Case Targeting:**
   - **SQLite for:** Development, testing, edge deployments, single-tenant, low-concurrency
   - **PostgreSQL for:** Production, multi-tenant, high-concurrency, advanced features

3. **Implementation Strategy:**
   - Use **Hybrid Approach** (dual implementation for critical paths)
   - Keep PostgreSQL as default and recommended option
   - SQLite as "opt-in" for specific use cases

4. **Backwards Compatibility:**
   - No breaking changes to existing configurations
   - PostgreSQL remains the default driver

### 11.3 Risk Assessment

| Risk                      | Severity  | Mitigation                                     |
| ------------------------- | --------- | ---------------------------------------------- |
| Vector search degradation | 🔴 High   | Require external vector DB; provide clear docs |
| Concurrency issues        | 🟡 Medium | Document limitations; use optimistic locking   |
| Maintenance burden        | 🟡 Medium | Comprehensive test coverage; CI for both DBs   |
| Code duplication          | 🟢 Low    | Share common logic; abstract where possible    |
| User confusion            | 🟡 Medium | Clear docs on when to use which database       |

---

## Appendices

### A. File Inventory

**PostgreSQL-Specific Files:**

```
engine/infra/postgres/
├── authrepo.go          (178 lines)
├── config.go            (24 lines)
├── doc.go               (10 lines)
├── dsn.go               (50 lines)
├── jsonb.go             (helper functions)
├── metrics.go           (69 lines)
├── migrations.go        (migration runner)
├── migrations/          (9 SQL files)
├── placeholders.go      (query helpers)
├── queries.go           (common queries)
├── scan.go              (result scanning)
├── store.go             (connection pool)
├── taskrepo.go          (~500 lines)
└── workflowrepo.go      (~300 lines)

engine/infra/repo/
└── provider.go          (34 lines - HARDCODED to pgxpool)

engine/knowledge/vectordb/
└── pgvector.go          (756 lines - CRITICAL DEPENDENCY)
```

**Estimated Lines of Code:**

- Core Postgres driver: ~1,500 lines
- Repositories: ~1,000 lines
- pgvector integration: ~800 lines
- **Total: ~3,300 lines** to replicate for SQLite

### B. Dependencies to Add for SQLite

```go
// SQLite driver
github.com/mattn/go-sqlite3  // CGO-based, most mature

// OR

modernc.org/sqlite          // Pure Go, no CGO

// For migrations (already have goose)
github.com/pressly/goose/v3  // Already supports SQLite
```

### C. Performance Characteristics

| Operation             | PostgreSQL | SQLite     | Notes                         |
| --------------------- | ---------- | ---------- | ----------------------------- |
| **Read Performance**  | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐   | SQLite excellent for reads    |
| **Write Performance** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐     | SQLite has write bottleneck   |
| **Concurrent Writes** | ⭐⭐⭐⭐⭐ | ⭐⭐       | SQLite limited by DB lock     |
| **Complex Queries**   | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐   | Both excellent                |
| **Vector Search**     | ⭐⭐⭐⭐⭐ | ❌         | SQLite requires external DB   |
| **Deployment**        | ⭐⭐⭐     | ⭐⭐⭐⭐⭐ | SQLite = single file          |
| **Scaling**           | ⭐⭐⭐⭐⭐ | ⭐⭐       | PostgreSQL scales much better |

---

**Document Version:** 1.0  
**Date:** 2025-01-27  
**Author:** AI Analysis  
**Status:** Analysis Complete
