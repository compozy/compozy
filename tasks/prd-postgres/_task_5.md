## markdown

## status: pending

<task_context>
<domain>engine/infra/server</domain>
<type>integration</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server|database</dependencies>
</task_context>

# Task 5.0: Server Integration & Validation

## Overview

Integrate SQLite database driver into server initialization and implement critical validation rules. Ensure SQLite deployments cannot use pgvector and provide clear error messages. Add startup warnings for SQLite concurrency limitations.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** @tasks/prd-postgres/_techspec.md section on Integration Points
- **ALWAYS READ** @tasks/prd-postgres/_tests.md for test requirements
- **DEPENDENCY:** Requires Tasks 1.0, 2.0, 3.0, 4.0 complete
- **MANDATORY:** SQLite + pgvector must fail at startup with clear error
- **MANDATORY:** SQLite requires external vector DB (Qdrant/Redis/Filesystem)
- **MANDATORY:** Log startup information with driver name
- **MANDATORY:** Warn if SQLite used with high concurrency settings
</critical>

<requirements>
- Update `engine/infra/server/dependencies.go` with database setup routing
- Implement vector DB validation for SQLite
- Add startup logging with driver information
- Add concurrency warnings for SQLite
- Route to PostgreSQL or SQLite store creation based on config
- Apply migrations automatically on startup
- Provide clear error messages for misconfigurations
</requirements>

## Subtasks

- [ ] 5.1 Update `setupStore()` to route by driver
- [ ] 5.2 Implement `validateDatabaseConfig()` for vector DB checks
- [ ] 5.3 Add startup logging with driver information
- [ ] 5.4 Add concurrency warnings for SQLite
- [ ] 5.5 Write unit tests for validation logic
- [ ] 5.6 Write integration tests for server startup

## Implementation Details

### 5.1 Database Setup Routing

**Update:** `engine/infra/server/dependencies.go`

```go
func (s *Server) setupStore() (*repo.Provider, func(), error) {
    cfg := config.FromContext(s.ctx)
    
    // Validate database configuration
    if err := s.validateDatabaseConfig(cfg); err != nil {
        return nil, nil, fmt.Errorf("invalid database configuration: %w", err)
    }
    
    // Create repository provider (factory handles driver selection)
    provider, cleanup, err := repo.NewProvider(s.ctx, &cfg.Database)
    if err != nil {
        return nil, nil, fmt.Errorf("create repository provider: %w", err)
    }
    
    // Log startup information
    s.logDatabaseStartup(cfg)
    
    return provider, cleanup, nil
}
```

### 5.2 Vector DB Validation

```go
func (s *Server) validateDatabaseConfig(cfg *config.Config) error {
    log := logger.FromContext(s.ctx)
    
    // SQLite-specific validations
    if cfg.Database.Driver == "sqlite" {
        // Check if knowledge features are enabled
        if len(cfg.Knowledge.VectorDBs) == 0 {
            log.Warn("SQLite mode without vector database - knowledge features will not work",
                "driver", "sqlite",
                "recommendation", "Configure Qdrant, Redis, or Filesystem vector DB")
            // Don't fail - allow running without knowledge features
        }
        
        // Ensure no pgvector provider configured
        for _, vdb := range cfg.Knowledge.VectorDBs {
            if vdb.Provider == "pgvector" {
                return fmt.Errorf(
                    "pgvector provider is incompatible with SQLite driver. "+
                    "SQLite requires external vector database. "+
                    "Please configure one of: Qdrant, Redis, or Filesystem. "+
                    "See documentation: docs/database/sqlite.md#vector-database-requirement")
            }
        }
        
        // Warn about concurrency limitations
        if cfg.Runtime.MaxConcurrentWorkflows > 10 {
            log.Warn("SQLite has concurrency limitations",
                "driver", "sqlite",
                "max_concurrent_workflows", cfg.Runtime.MaxConcurrentWorkflows,
                "recommended_max", 10,
                "note", "Consider using PostgreSQL for high-concurrency production workloads")
        }
    }
    
    // PostgreSQL can use any vector DB
    return nil
}
```

### 5.3 Startup Logging

```go
func (s *Server) logDatabaseStartup(cfg *config.Config) {
    log := logger.FromContext(s.ctx)
    
    switch cfg.Database.Driver {
    case "sqlite", "":
        if cfg.Database.Driver == "" {
            cfg.Database.Driver = "postgres"  // default
        }
    }
    
    if cfg.Database.Driver == "sqlite" {
        log.Info("Database initialized",
            "driver", "sqlite",
            "path", cfg.Database.Path,
            "mode", getSQLiteMode(cfg.Database.Path),
            "vector_db_required", true,
            "concurrency_limit", "low (5-10 workflows recommended)")
    } else {
        log.Info("Database initialized",
            "driver", "postgres",
            "host", cfg.Database.Host,
            "port", cfg.Database.Port,
            "database", cfg.Database.DBName,
            "vector_db", "pgvector (optional)",
            "concurrency_limit", "high (25+ workflows)")
    }
}

func getSQLiteMode(path string) string {
    if path == ":memory:" {
        return "in-memory"
    }
    return "file-based"
}
```

### 5.4 Additional Helper Functions

```go
func buildDatabaseConfig(cfg *config.Config) *config.DatabaseConfig {
    // Set defaults
    if cfg.Database.Driver == "" {
        cfg.Database.Driver = "postgres"
    }
    
    if cfg.Database.MaxOpenConns == 0 {
        cfg.Database.MaxOpenConns = 25
    }
    
    if cfg.Database.MaxIdleConns == 0 {
        cfg.Database.MaxIdleConns = 5
    }
    
    return &cfg.Database
}
```

### Example Error Messages

**Good Error Message (SQLite + pgvector):**
```
Error: pgvector provider is incompatible with SQLite driver.

SQLite requires external vector database for knowledge features.
Please configure one of the following vector database providers:
  - Qdrant: See docs/database/sqlite.md#using-qdrant
  - Redis: See docs/database/sqlite.md#using-redis  
  - Filesystem: See docs/database/sqlite.md#using-filesystem

Example configuration:
  database:
    driver: sqlite
    path: ./data/compozy.db
  
  knowledge:
    vector_dbs:
      - id: main
        provider: qdrant
        url: http://localhost:6333

For more information: docs/database/overview.md
```

### Relevant Files

**Modified Files:**
- `engine/infra/server/dependencies.go` - Database setup and validation

**Reference Files:**
- `engine/infra/repo/provider.go` - Factory pattern (from Task 4.0)
- `pkg/config/config.go` - Configuration (from Task 1.0)

### Dependent Files

- `engine/infra/sqlite/store.go` - SQLite store (from Task 1.0)
- `engine/infra/postgres/store.go` - PostgreSQL store (existing)
- `engine/infra/repo/provider.go` - Repository factory (from Task 4.0)

## Deliverables

- [ ] `engine/infra/server/dependencies.go` updated with routing logic
- [ ] Vector DB validation implemented and working
- [ ] Startup logging shows correct driver information
- [ ] Concurrency warnings displayed for SQLite
- [ ] Clear error messages for misconfigurations
- [ ] All unit tests passing
- [ ] All integration tests passing
- [ ] Code passes linting

## Tests

### Unit Tests (`engine/infra/server/dependencies_test.go`)

- [ ] `TestValidateDatabaseConfig/Should_pass_postgres_with_pgvector`
- [ ] `TestValidateDatabaseConfig/Should_pass_postgres_without_vector_db`
- [ ] `TestValidateDatabaseConfig/Should_pass_sqlite_with_qdrant`
- [ ] `TestValidateDatabaseConfig/Should_pass_sqlite_with_redis`
- [ ] `TestValidateDatabaseConfig/Should_pass_sqlite_with_filesystem`
- [ ] `TestValidateDatabaseConfig/Should_fail_sqlite_with_pgvector`
- [ ] `TestValidateDatabaseConfig/Should_warn_sqlite_without_vector_db`
- [ ] `TestValidateDatabaseConfig/Should_warn_sqlite_with_high_concurrency`

### Integration Tests

- [ ] `TestServerStartup/Should_start_with_postgres_driver`
- [ ] `TestServerStartup/Should_start_with_sqlite_driver`
- [ ] `TestServerStartup/Should_fail_with_invalid_driver`
- [ ] `TestServerStartup/Should_fail_sqlite_plus_pgvector`
- [ ] `TestServerStartup/Should_log_driver_information`

### Error Message Tests

```go
func TestValidateDatabaseConfig(t *testing.T) {
    t.Run("Should fail sqlite with pgvector", func(t *testing.T) {
        cfg := &config.Config{
            Database: config.DatabaseConfig{
                Driver: "sqlite",
                Path:   "./test.db",
            },
            Knowledge: config.KnowledgeConfig{
                VectorDBs: []config.VectorDBConfig{
                    {Provider: "pgvector"},
                },
            },
        }
        
        err := validateDatabaseConfig(cfg)
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "pgvector")
        assert.Contains(t, err.Error(), "incompatible with SQLite")
        assert.Contains(t, err.Error(), "Qdrant, Redis, or Filesystem")
        assert.Contains(t, err.Error(), "docs/database/sqlite.md")
    })
    
    t.Run("Should pass sqlite with qdrant", func(t *testing.T) {
        cfg := &config.Config{
            Database: config.DatabaseConfig{
                Driver: "sqlite",
                Path:   "./test.db",
            },
            Knowledge: config.KnowledgeConfig{
                VectorDBs: []config.VectorDBConfig{
                    {Provider: "qdrant", URL: "http://localhost:6333"},
                },
            },
        }
        
        err := validateDatabaseConfig(cfg)
        assert.NoError(t, err)
    })
    
    t.Run("Should warn sqlite with high concurrency", func(t *testing.T) {
        // Capture logs
        var logOutput bytes.Buffer
        log := setupTestLogger(&logOutput)
        ctx := logger.NewContext(context.Background(), log)
        
        cfg := &config.Config{
            Database: config.DatabaseConfig{
                Driver: "sqlite",
                Path:   ":memory:",
            },
            Runtime: config.RuntimeConfig{
                MaxConcurrentWorkflows: 50,
            },
        }
        
        err := validateDatabaseConfig(cfg)
        assert.NoError(t, err)  // Should not fail
        
        logStr := logOutput.String()
        assert.Contains(t, logStr, "concurrency limitations")
        assert.Contains(t, logStr, "recommended_max")
        assert.Contains(t, logStr, "PostgreSQL")
    })
}
```

### Logging Tests

```go
func TestDatabaseStartupLogging(t *testing.T) {
    t.Run("Should log sqlite information", func(t *testing.T) {
        var logOutput bytes.Buffer
        log := setupTestLogger(&logOutput)
        ctx := logger.NewContext(context.Background(), log)
        
        cfg := &config.Config{
            Database: config.DatabaseConfig{
                Driver: "sqlite",
                Path:   "./data/compozy.db",
            },
        }
        
        logDatabaseStartup(ctx, cfg)
        
        logStr := logOutput.String()
        assert.Contains(t, logStr, "driver=sqlite")
        assert.Contains(t, logStr, "path=./data/compozy.db")
        assert.Contains(t, logStr, "mode=file-based")
        assert.Contains(t, logStr, "vector_db_required=true")
    })
    
    t.Run("Should log postgres information", func(t *testing.T) {
        var logOutput bytes.Buffer
        log := setupTestLogger(&logOutput)
        ctx := logger.NewContext(context.Background(), log)
        
        cfg := &config.Config{
            Database: config.DatabaseConfig{
                Driver: "postgres",
                Host:   "localhost",
                Port:   "5432",
                DBName: "compozy",
            },
        }
        
        logDatabaseStartup(ctx, cfg)
        
        logStr := logOutput.String()
        assert.Contains(t, logStr, "driver=postgres")
        assert.Contains(t, logStr, "host=localhost")
        assert.Contains(t, logStr, "vector_db=pgvector")
    })
}
```

## Success Criteria

- [ ] Server starts successfully with PostgreSQL driver
- [ ] Server starts successfully with SQLite driver
- [ ] Server fails with clear error when SQLite + pgvector configured
- [ ] Server warns (but doesn't fail) when SQLite without vector DB
- [ ] Server warns when SQLite with high concurrency settings
- [ ] Startup logs show correct driver and configuration information
- [ ] Error messages are helpful and include documentation links
- [ ] All validation logic unit tested
- [ ] All integration tests pass
- [ ] Code passes linting: `golangci-lint run ./engine/infra/server/...`
- [ ] Backwards compatibility: Existing PostgreSQL configs work unchanged
