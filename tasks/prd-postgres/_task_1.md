## markdown

## status: completed

<task_context>
<domain>engine/infra/sqlite + pkg/config</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>database</dependencies>
</task_context>

# Task 1.0: SQLite Foundation Infrastructure

## Overview

Create the foundational SQLite infrastructure including configuration system, connection management, and database migrations. This task establishes the complete SQLite driver implementation with proper schema definitions, enabling all subsequent repository implementations.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** @tasks/prd-postgres/_techspec.md before start
- **ALWAYS READ** @tasks/prd-postgres/_tests.md for test requirements
- **MANDATORY:** Use `modernc.org/sqlite` (pure Go, no CGO)
- **MANDATORY:** Enable `PRAGMA foreign_keys = ON` on all connections
- **MANDATORY:** Separate migration files for SQLite (do not reuse PostgreSQL migrations)
</critical>

<requirements>
- Add `driver` field to `DatabaseConfig` with validation (`postgres` | `sqlite`)
- Implement SQLite `Store` with connection pooling and PRAGMA configuration
- Port all 4 PostgreSQL migration files to SQLite syntax
- Support both file-based and in-memory (`:memory:`) databases
- Health check implementation for SQLite
- Migration runner without advisory locks (SQLite doesn't support them)
</requirements>

## Subtasks

- [x] 1.1 Add database driver configuration to `pkg/config/config.go`
- [x] 1.2 Create `engine/infra/sqlite/` package structure
- [x] 1.3 Implement `Store` with connection management
- [x] 1.4 Port migration: `create_workflow_states.sql`
- [x] 1.5 Port migration: `create_task_states.sql`
- [x] 1.6 Port migration: `create_users.sql`
- [x] 1.7 Port migration: `create_api_keys.sql`
- [x] 1.8 Implement migration runner (`migrations.go`)
- [x] 1.9 Write unit tests for config validation
- [x] 1.10 Write unit tests for Store operations
- [x] 1.11 Write integration tests for migrations

## Implementation Details

### 1.1 Database Configuration (`pkg/config/config.go`)

**Add to `DatabaseConfig` struct:**

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
    
    // Common settings (existing)
    MaxOpenConns int `koanf:"max_open_conns" json:"max_open_conns" yaml:"max_open_conns"`
    MaxIdleConns int `koanf:"max_idle_conns" json:"max_idle_conns" yaml:"max_idle_conns"`
    // ... rest unchanged
}

// Add validation method
func (c *DatabaseConfig) Validate() error {
    // Default to postgres if not specified
    if c.Driver == "" {
        c.Driver = "postgres"
    }
    
    switch c.Driver {
    case "postgres":
        // Validate PostgreSQL-specific fields
        if c.Host == "" && c.ConnString == "" {
            return fmt.Errorf("postgres driver requires host or conn_string")
        }
    case "sqlite":
        // Validate SQLite-specific fields
        if c.Path == "" {
            return fmt.Errorf("sqlite driver requires path")
        }
    default:
        return fmt.Errorf("unsupported database driver: %s", c.Driver)
    }
    
    return nil
}
```

### 1.2-1.3 SQLite Store (`engine/infra/sqlite/store.go`)

**Reference:** `engine/infra/postgres/store.go` for patterns

```go
package sqlite

import (
    "context"
    "database/sql"
    "fmt"
    "time"
    
    _ "modernc.org/sqlite" // Pure Go SQLite driver
)

type Config struct {
    Path         string
    MaxOpenConns int
    MaxIdleConns int
}

type Store struct {
    db   *sql.DB
    path string
}

func NewStore(ctx context.Context, cfg *Config) (*Store, error) {
    // Apply defaults
    if cfg.MaxOpenConns == 0 {
        cfg.MaxOpenConns = 25
    }
    if cfg.MaxIdleConns == 0 {
        cfg.MaxIdleConns = 5
    }
    
    // Open database
    db, err := sql.Open("sqlite", cfg.Path)
    if err != nil {
        return nil, fmt.Errorf("open sqlite database: %w", err)
    }
    
    // Configure connection pool
    db.SetMaxOpenConns(cfg.MaxOpenConns)
    db.SetMaxIdleConns(cfg.MaxIdleConns)
    db.SetConnMaxLifetime(time.Hour)
    
    // Enable foreign keys (CRITICAL for SQLite)
    if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
        db.Close()
        return nil, fmt.Errorf("enable foreign keys: %w", err)
    }
    
    // Enable WAL mode for better concurrency
    if _, err := db.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
        db.Close()
        return nil, fmt.Errorf("enable WAL mode: %w", err)
    }
    
    store := &Store{
        db:   db,
        path: cfg.Path,
    }
    
    return store, nil
}

func (s *Store) DB() *sql.DB {
    return s.db
}

func (s *Store) Close(ctx context.Context) error {
    if s.db != nil {
        return s.db.Close()
    }
    return nil
}

func (s *Store) HealthCheck(ctx context.Context) error {
    // Check foreign keys are enabled
    var fkEnabled int
    if err := s.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
        return fmt.Errorf("health check failed: %w", err)
    }
    if fkEnabled != 1 {
        return fmt.Errorf("foreign keys not enabled")
    }
    
    // Simple ping
    if err := s.db.PingContext(ctx); err != nil {
        return fmt.Errorf("ping failed: %w", err)
    }
    
    return nil
}
```

### 1.4-1.7 Migration Files

**Create:** `engine/infra/sqlite/migrations/`

**20250603124835_create_workflow_states.sql:**
```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS workflow_states (
    workflow_exec_id TEXT NOT NULL PRIMARY KEY,
    workflow_id      TEXT NOT NULL,
    status           TEXT NOT NULL,
    usage            TEXT,  -- JSON as TEXT
    input            TEXT,  -- JSON as TEXT
    output           TEXT,  -- JSON as TEXT
    error            TEXT,  -- JSON as TEXT
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT NOT NULL DEFAULT (datetime('now')),
    
    CHECK (usage IS NULL OR json_type(usage) = 'array')
);

CREATE INDEX IF NOT EXISTS idx_workflow_states_status ON workflow_states (status);
CREATE INDEX IF NOT EXISTS idx_workflow_states_workflow_id ON workflow_states (workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_states_workflow_status ON workflow_states (workflow_id, status);
CREATE INDEX IF NOT EXISTS idx_workflow_states_created_at ON workflow_states (created_at);
CREATE INDEX IF NOT EXISTS idx_workflow_states_updated_at ON workflow_states (updated_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS workflow_states;
-- +goose StatementEnd
```

**20250603124915_create_task_states.sql:**
```sql
-- +goose Up
-- +goose StatementBegin
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS task_states (
    component        TEXT NOT NULL,
    status           TEXT NOT NULL,
    task_exec_id     TEXT NOT NULL PRIMARY KEY,
    task_id          TEXT NOT NULL,
    workflow_exec_id TEXT NOT NULL,
    workflow_id      TEXT NOT NULL,
    execution_type   TEXT NOT NULL DEFAULT 'basic',
    usage            TEXT,  -- JSON as TEXT
    agent_id         TEXT,
    tool_id          TEXT,
    action_id        TEXT,
    parent_state_id  TEXT,
    input            TEXT,  -- JSON as TEXT
    output           TEXT,  -- JSON as TEXT
    error            TEXT,  -- JSON as TEXT
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT NOT NULL DEFAULT (datetime('now')),
    
    FOREIGN KEY (workflow_exec_id)
        REFERENCES workflow_states (workflow_exec_id)
        ON DELETE CASCADE,
    
    FOREIGN KEY (parent_state_id)
        REFERENCES task_states (task_exec_id)
        ON DELETE CASCADE,
    
    CHECK (
        (execution_type = 'basic' AND (
            (agent_id IS NOT NULL AND action_id IS NOT NULL AND tool_id IS NULL) OR
            (tool_id IS NOT NULL AND agent_id IS NULL AND action_id IS NULL) OR
            (agent_id IS NULL AND action_id IS NULL AND tool_id IS NULL)
        )) OR
        (execution_type = 'router' AND agent_id IS NULL AND action_id IS NULL AND tool_id IS NULL) OR
        (execution_type IN ('parallel', 'collection', 'composite')) OR
        (execution_type NOT IN ('parallel', 'collection', 'composite', 'basic', 'router'))
    ),
    
    CHECK (usage IS NULL OR json_type(usage) = 'array')
);

CREATE INDEX IF NOT EXISTS idx_task_states_status ON task_states (status);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id ON task_states (workflow_id);
CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id ON task_states (workflow_exec_id);
CREATE INDEX IF NOT EXISTS idx_task_states_task_id ON task_states (task_id);
CREATE INDEX IF NOT EXISTS idx_task_states_parent_state_id ON task_states (parent_state_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS task_states;
-- +goose StatementEnd
```

**20250711163857_create_users.sql:**
```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS users (
    id         TEXT NOT NULL PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    role       TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_ci ON users (lower(email));
CREATE INDEX IF NOT EXISTS idx_users_role ON users (role);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
```

**20250711163858_create_api_keys.sql:**
```sql
-- +goose Up
-- +goose StatementBegin
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS api_keys (
    id         TEXT NOT NULL PRIMARY KEY,
    user_id    TEXT NOT NULL,
    hash       BLOB NOT NULL,
    prefix     TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    last_used  TEXT,
    
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys (hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys (user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_created_at ON api_keys (created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS api_keys;
-- +goose StatementEnd
```

### 1.8 Migration Runner (`engine/infra/sqlite/migrations.go`)

```go
package sqlite

import (
    "context"
    "database/sql"
    "embed"
    "fmt"
    
    "github.com/pressly/goose/v3"
    _ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func ApplyMigrations(ctx context.Context, dbPath string) error {
    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        return fmt.Errorf("open db for migrations: %w", err)
    }
    defer db.Close()
    
    // Enable foreign keys
    if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
        return fmt.Errorf("enable foreign keys: %w", err)
    }
    
    goose.SetBaseFS(migrationsFS)
    if err := goose.SetDialect("sqlite3"); err != nil {
        return fmt.Errorf("set dialect: %w", err)
    }
    
    if err := goose.UpContext(ctx, db, "migrations"); err != nil {
        return fmt.Errorf("apply migrations: %w", err)
    }
    
    return nil
}
```

### Relevant Files

**New Files:**
- `engine/infra/sqlite/store.go`
- `engine/infra/sqlite/migrations.go`
- `engine/infra/sqlite/config.go`
- `engine/infra/sqlite/doc.go`
- `engine/infra/sqlite/migrations/20250603124835_create_workflow_states.sql`
- `engine/infra/sqlite/migrations/20250603124915_create_task_states.sql`
- `engine/infra/sqlite/migrations/20250711163857_create_users.sql`
- `engine/infra/sqlite/migrations/20250711163858_create_api_keys.sql`

**Modified Files:**
- `pkg/config/config.go` - Add `Driver` and `Path` fields

### Dependent Files

- `engine/infra/postgres/store.go` - Reference for patterns
- `engine/infra/postgres/migrations/*.sql` - Source for porting

## Deliverables

- [ ] `pkg/config/config.go` updated with `driver` field and validation
- [ ] `engine/infra/sqlite/` package created with proper structure
- [ ] `engine/infra/sqlite/store.go` with connection management
- [ ] `engine/infra/sqlite/migrations.go` with migration runner
- [ ] 4 migration files ported from PostgreSQL to SQLite syntax
- [ ] All unit tests passing for config validation
- [ ] All unit tests passing for Store operations
- [ ] All integration tests passing for migrations
- [ ] Documentation in `engine/infra/sqlite/doc.go`

## Tests

### Unit Tests: Configuration (`pkg/config/config_test.go`)

- [ ] `TestDatabaseConfig/Should_default_to_postgres_when_driver_empty`
- [ ] `TestDatabaseConfig/Should_accept_postgres_driver_explicitly`
- [ ] `TestDatabaseConfig/Should_accept_sqlite_driver`
- [ ] `TestDatabaseConfig/Should_reject_invalid_driver`
- [ ] `TestDatabaseConfig/Should_require_path_for_sqlite`
- [ ] `TestDatabaseConfig/Should_require_connection_params_for_postgres`
- [ ] `TestDatabaseConfig/Should_validate_sqlite_path_format`

### Unit Tests: Store (`engine/infra/sqlite/store_test.go`)

- [ ] `TestStore/Should_create_file_database_at_specified_path`
- [ ] `TestStore/Should_create_in_memory_database_for_memory_path`
- [ ] `TestStore/Should_enable_foreign_keys_on_connection`
- [ ] `TestStore/Should_handle_concurrent_connections`
- [ ] `TestStore/Should_return_error_for_invalid_path`
- [ ] `TestStore/Should_close_cleanly`
- [ ] `TestStore/Should_perform_health_check_successfully`

### Integration Tests: Migrations (`engine/infra/sqlite/migrations_test.go`)

- [ ] `TestMigrations/Should_apply_all_migrations_successfully`
- [ ] `TestMigrations/Should_create_all_required_tables`
- [ ] `TestMigrations/Should_create_all_indexes`
- [ ] `TestMigrations/Should_enforce_foreign_keys`
- [ ] `TestMigrations/Should_enforce_check_constraints`
- [ ] `TestMigrations/Should_rollback_migrations`
- [ ] `TestMigrations/Should_be_idempotent`

## Success Criteria

- [ ] Configuration validates driver correctly (postgres/sqlite)
- [ ] SQLite store creates file-based and in-memory databases
- [ ] Foreign keys are enabled on all connections
- [ ] WAL mode is enabled for better concurrency
- [ ] All 4 tables created with correct schema
- [ ] All indexes created correctly
- [ ] Foreign key constraints enforced
- [ ] Check constraints enforced (role, execution_type, JSON type)
- [ ] Migrations are idempotent (can run multiple times)
- [ ] Health check validates database state
- [ ] All tests pass: `go test ./pkg/config/... ./engine/infra/sqlite/...`
- [ ] Code passes linting: `golangci-lint run ./pkg/config/... ./engine/infra/sqlite/...`
