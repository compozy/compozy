# Tests Plan: SQLite Database Backend Support

## Guiding Principles

- Follow `.cursor/rules/test-standards.mdc` strictly
- Use `t.Run("Should ...")` pattern for all test cases
- Use testify assertions for clarity
- Context from `t.Context()` (never `context.Background()`)
- Real databases (no mocks for database operations)
- Parameterized tests for multi-driver scenarios

## Coverage Matrix

| PRD Requirement | Test Files | Coverage Type |
|----------------|------------|---------------|
| FR-1: Driver Selection | `pkg/config/config_test.go` | Unit |
| FR-2: SQLite Driver | `engine/infra/sqlite/*_test.go` | Unit + Integration |
| FR-3: Database Config | `pkg/config/loader_test.go` | Unit |
| FR-4: Migrations | `engine/infra/sqlite/migrations_test.go` | Integration |
| FR-5: Vector DB Validation | `engine/infra/server/dependencies_test.go` | Unit |
| NFR-1: Performance | `test/integration/database/performance_test.go` | Performance |
| NFR-2: Compatibility | `test/integration/database/compatibility_test.go` | Integration |
| NFR-3: Test Coverage | All test files | Coverage Report |

## Unit Tests

### `pkg/config/config_test.go` (UPDATE)

**Add tests for database driver selection:**

- `TestDatabaseConfig/Should_default_to_postgres_when_driver_empty`
- `TestDatabaseConfig/Should_accept_postgres_driver_explicitly`
- `TestDatabaseConfig/Should_accept_sqlite_driver`
- `TestDatabaseConfig/Should_reject_invalid_driver`
- `TestDatabaseConfig/Should_require_path_for_sqlite`
- `TestDatabaseConfig/Should_require_connection_params_for_postgres`
- `TestDatabaseConfig/Should_validate_sqlite_path_format`

**Example:**
```go
func TestDatabaseConfig(t *testing.T) {
    t.Run("Should default to postgres when driver empty", func(t *testing.T) {
        cfg := &config.DatabaseConfig{
            Host: "localhost",
            User: "test",
        }
        err := cfg.Validate()
        assert.NoError(t, err)
        assert.Equal(t, "postgres", cfg.Driver)  // default
    })
    
    t.Run("Should accept sqlite driver", func(t *testing.T) {
        cfg := &config.DatabaseConfig{
            Driver: "sqlite",
            Path:   "./test.db",
        }
        err := cfg.Validate()
        assert.NoError(t, err)
    })
    
    t.Run("Should reject invalid driver", func(t *testing.T) {
        cfg := &config.DatabaseConfig{
            Driver: "mysql",
        }
        err := cfg.Validate()
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "unsupported driver")
    })
}
```

### `engine/infra/sqlite/store_test.go` (NEW)

**Test SQLite connection management:**

- `TestStore/Should_create_file_database_at_specified_path`
- `TestStore/Should_create_in_memory_database_for_memory_path`
- `TestStore/Should_enable_foreign_keys_on_connection`
- `TestStore/Should_handle_concurrent_connections`
- `TestStore/Should_return_error_for_invalid_path`
- `TestStore/Should_close_cleanly`
- `TestStore/Should_perform_health_check_successfully`

### `engine/infra/sqlite/authrepo_test.go` (NEW)

**Test user and API key operations:**

- `TestAuthRepo/Should_create_user_successfully`
- `TestAuthRepo/Should_get_user_by_id`
- `TestAuthRepo/Should_get_user_by_email_case_insensitive`
- `TestAuthRepo/Should_return_error_for_duplicate_email`
- `TestAuthRepo/Should_list_all_users`
- `TestAuthRepo/Should_update_user`
- `TestAuthRepo/Should_delete_user`
- `TestAuthRepo/Should_create_api_key`
- `TestAuthRepo/Should_cascade_delete_api_keys_when_user_deleted`
- `TestAuthRepo/Should_enforce_foreign_key_constraint`

### `engine/infra/sqlite/taskrepo_test.go` (NEW)

**Test task state persistence and hierarchy:**

- `TestTaskRepo/Should_upsert_task_state`
- `TestTaskRepo/Should_get_task_state_by_id`
- `TestTaskRepo/Should_list_tasks_by_workflow`
- `TestTaskRepo/Should_list_tasks_by_status`
- `TestTaskRepo/Should_list_children_of_parent_task`
- `TestTaskRepo/Should_get_task_tree_recursively`
- `TestTaskRepo/Should_handle_jsonb_fields_correctly`
- `TestTaskRepo/Should_enforce_foreign_key_to_workflow`
- `TestTaskRepo/Should_cascade_delete_children_when_parent_deleted`
- `TestTaskRepo/Should_execute_transaction_atomically`
- `TestTaskRepo/Should_handle_concurrent_updates`

**Example:**
```go
func TestTaskRepo(t *testing.T) {
    db := setupTestSQLite(t)
    repo := sqlite.NewTaskRepo(db)
    
    t.Run("Should upsert task state", func(t *testing.T) {
        ctx := t.Context()
        state := &task.State{
            TaskExecID:     core.NewID(),
            TaskID:         "test-task",
            WorkflowExecID: setupTestWorkflow(t, db),
            Status:         core.StatusRunning,
            Component:      "task",
        }
        
        err := repo.UpsertState(ctx, state)
        assert.NoError(t, err)
        
        // Verify
        retrieved, err := repo.GetState(ctx, state.TaskExecID)
        assert.NoError(t, err)
        assert.Equal(t, state.TaskID, retrieved.TaskID)
    })
    
    t.Run("Should handle jsonb fields correctly", func(t *testing.T) {
        ctx := t.Context()
        input := map[string]any{"key": "value"}
        state := &task.State{
            TaskExecID:     core.NewID(),
            WorkflowExecID: setupTestWorkflow(t, db),
            Input:          &core.Input{Data: input},
        }
        
        err := repo.UpsertState(ctx, state)
        assert.NoError(t, err)
        
        retrieved, err := repo.GetState(ctx, state.TaskExecID)
        assert.NoError(t, err)
        assert.Equal(t, input, retrieved.Input.Data)
    })
}
```

### `engine/infra/sqlite/workflowrepo_test.go` (NEW)

**Test workflow state operations:**

- `TestWorkflowRepo/Should_upsert_workflow_state`
- `TestWorkflowRepo/Should_get_workflow_state_by_exec_id`
- `TestWorkflowRepo/Should_list_workflows_by_status`
- `TestWorkflowRepo/Should_update_workflow_status`
- `TestWorkflowRepo/Should_complete_workflow_with_output`
- `TestWorkflowRepo/Should_merge_usage_statistics`
- `TestWorkflowRepo/Should_handle_jsonb_usage_field`

### `engine/infra/sqlite/migrations_test.go` (NEW)

**Test migration system:**

- `TestMigrations/Should_apply_all_migrations_successfully`
- `TestMigrations/Should_create_all_required_tables`
- `TestMigrations/Should_create_all_indexes`
- `TestMigrations/Should_enforce_foreign_keys`
- `TestMigrations/Should_enforce_check_constraints`
- `TestMigrations/Should_rollback_migrations`
- `TestMigrations/Should_be_idempotent`

### `engine/infra/repo/provider_test.go` (UPDATE)

**Test factory pattern:**

- `TestProvider/Should_create_postgres_provider_by_default`
- `TestProvider/Should_create_postgres_provider_explicitly`
- `TestProvider/Should_create_sqlite_provider`
- `TestProvider/Should_return_error_for_invalid_driver`
- `TestProvider/Should_configure_postgres_repositories_correctly`
- `TestProvider/Should_configure_sqlite_repositories_correctly`

### `engine/infra/server/dependencies_test.go` (UPDATE)

**Test vector DB validation:**

- `TestValidateDatabaseConfig/Should_pass_postgres_with_pgvector`
- `TestValidateDatabaseConfig/Should_pass_postgres_without_vector_db`
- `TestValidateDatabaseConfig/Should_pass_sqlite_with_qdrant`
- `TestValidateDatabaseConfig/Should_pass_sqlite_with_redis`
- `TestValidateDatabaseConfig/Should_pass_sqlite_with_filesystem`
- `TestValidateDatabaseConfig/Should_fail_sqlite_with_pgvector`
- `TestValidateDatabaseConfig/Should_fail_sqlite_without_vector_db_when_knowledge_enabled`

**Example:**
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
    })
}
```

## Integration Tests

### `test/integration/database/multi_driver_test.go` (NEW)

**Parameterized tests for both drivers:**

```go
func TestMultiDriver_WorkflowExecution(t *testing.T) {
    drivers := []string{"postgres", "sqlite"}
    
    for _, driver := range drivers {
        t.Run(driver, func(t *testing.T) {
            provider, cleanup := setupTestDatabase(t, driver)
            defer cleanup()
            
            t.Run("Should execute workflow end-to-end", func(t *testing.T) {
                testWorkflowExecution(t, provider)
            })
            
            t.Run("Should persist task hierarchy", func(t *testing.T) {
                testTaskHierarchy(t, provider)
            })
            
            t.Run("Should handle concurrent workflows", func(t *testing.T) {
                testConcurrentWorkflows(t, provider, 5)  // Conservative for SQLite
            })
        })
    }
}
```

**Test Cases:**
- `TestMultiDriver_WorkflowExecution/Should_execute_workflow_end_to_end`
- `TestMultiDriver_WorkflowExecution/Should_persist_task_hierarchy`
- `TestMultiDriver_WorkflowExecution/Should_handle_concurrent_workflows`
- `TestMultiDriver_Authentication/Should_authenticate_user_with_api_key`
- `TestMultiDriver_Transactions/Should_rollback_on_error`
- `TestMultiDriver_Transactions/Should_commit_on_success`

### `test/integration/database/sqlite_specific_test.go` (NEW)

**SQLite-specific behavior tests:**

- `TestSQLite/Should_handle_database_locked_gracefully`
- `TestSQLite/Should_support_in_memory_mode`
- `TestSQLite/Should_create_database_file_if_not_exists`
- `TestSQLite/Should_enforce_foreign_keys`
- `TestSQLite/Should_handle_concurrent_reads`
- `TestSQLite/Should_serialize_concurrent_writes`
- `TestSQLite/Should_work_with_wal_mode`

### `test/integration/database/performance_test.go` (NEW)

**Performance benchmarks:**

```go
func BenchmarkDatabase_ReadOperations(b *testing.B) {
    drivers := []string{"postgres", "sqlite"}
    
    for _, driver := range drivers {
        b.Run(driver, func(b *testing.B) {
            provider, cleanup := setupBenchDatabase(b, driver)
            defer cleanup()
            
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                _, err := provider.NewWorkflowRepo().GetState(
                    context.Background(),
                    testWorkflowID,
                )
                if err != nil {
                    b.Fatal(err)
                }
            }
        })
    }
}
```

**Benchmarks:**
- `BenchmarkDatabase_ReadOperations`
- `BenchmarkDatabase_WriteOperations`
- `BenchmarkDatabase_TransactionOperations`
- `BenchmarkDatabase_HierarchicalQueries`

### `test/integration/database/compatibility_test.go` (NEW)

**Test backward compatibility:**

- `TestCompatibility/Should_work_with_existing_postgres_config`
- `TestCompatibility/Should_not_break_existing_workflows`
- `TestCompatibility/Should_migrate_schema_without_data_loss`

## Fixtures & Testdata

### New Fixtures

**`test/fixtures/database/sqlite-config.yaml`:**
```yaml
database:
  driver: sqlite
  path: ":memory:"
```

**`test/fixtures/database/postgres-config.yaml`:**
```yaml
database:
  driver: postgres
  host: localhost
  port: 5432
  user: test
  password: test
  dbname: test_compozy
```

**`test/fixtures/database/sqlite-qdrant-config.yaml`:**
```yaml
database:
  driver: sqlite
  path: "./test.db"

knowledge:
  vector_dbs:
    - id: test
      provider: qdrant
      url: http://localhost:6333
```

### Test Helpers

**`test/helpers/database.go` (UPDATE):**

```go
// SetupTestDatabase creates a test database for the specified driver
func SetupTestDatabase(t *testing.T, driver string) (*repo.Provider, func()) {
    t.Helper()
    
    switch driver {
    case "postgres":
        return setupPostgresTest(t)
    case "sqlite":
        return setupSQLiteTest(t)
    default:
        t.Fatalf("unsupported driver: %s", driver)
        return nil, nil
    }
}

func setupSQLiteTest(t *testing.T) (*repo.Provider, func()) {
    t.Helper()
    
    // Use in-memory SQLite for fast tests
    cfg := &config.DatabaseConfig{
        Driver: "sqlite",
        Path:   ":memory:",
    }
    
    db, err := sqlite.NewStore(t.Context(), cfg)
    require.NoError(t, err)
    
    // Apply migrations
    err = sqlite.ApplyMigrations(t.Context(), ":memory:")
    require.NoError(t, err)
    
    provider := repo.NewProvider(db.DB())
    
    cleanup := func() {
        db.Close(t.Context())
    }
    
    return provider, cleanup
}
```

## Mocks & Stubs

**Minimal mocking strategy:**
- ❌ No mocks for database operations (use real databases)
- ✅ Mock external LLM providers
- ✅ Mock external MCP servers
- ✅ Mock external vector DBs (optional, if Qdrant/Redis not available)

**Mock Files:**
- No new mocks required (database operations test against real DBs)

## API Contract Assertions (if applicable)

No API changes - database selection is configuration-driven.

## Observability Assertions

### Metrics Tests

**`engine/infra/monitoring/database_test.go` (UPDATE):**

- `TestDatabaseMetrics/Should_emit_query_duration_with_driver_label`
- `TestDatabaseMetrics/Should_emit_query_count_with_driver_label`
- `TestDatabaseMetrics/Should_emit_connection_pool_metrics`
- `TestDatabaseMetrics/Should_emit_sqlite_specific_metrics`

**Example:**
```go
func TestDatabaseMetrics(t *testing.T) {
    t.Run("Should emit query duration with driver label", func(t *testing.T) {
        // Setup metric collector
        registry := prometheus.NewRegistry()
        
        // Execute query
        provider, cleanup := setupTestDatabase(t, "sqlite")
        defer cleanup()
        
        _, err := provider.NewWorkflowRepo().GetState(t.Context(), testID)
        require.NoError(t, err)
        
        // Assert metric
        metrics, _ := registry.Gather()
        found := false
        for _, mf := range metrics {
            if *mf.Name == "database_query_duration_seconds" {
                for _, m := range mf.Metric {
                    for _, label := range m.Label {
                        if *label.Name == "driver" && *label.Value == "sqlite" {
                            found = true
                        }
                    }
                }
            }
        }
        assert.True(t, found, "metric not found")
    })
}
```

### Logging Tests

**`engine/infra/sqlite/store_test.go` (extend):**

- `TestStore/Should_log_initialization_with_driver_label`
- `TestStore/Should_log_warnings_for_concurrency_limits`
- `TestStore/Should_log_errors_with_context`

## Performance & Limits

### Performance Tests

**`test/integration/database/performance_test.go` (detailed):**

```go
func TestSQLitePerformance(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping performance test in short mode")
    }
    
    t.Run("Should handle 10 concurrent workflows", func(t *testing.T) {
        provider, cleanup := setupTestDatabase(t, "sqlite")
        defer cleanup()
        
        ctx := t.Context()
        concurrency := 10
        errors := make(chan error, concurrency)
        
        start := time.Now()
        
        for i := 0; i < concurrency; i++ {
            go func(id int) {
                workflow := createTestWorkflow(id)
                errors <- provider.NewWorkflowRepo().UpsertState(ctx, workflow)
            }(i)
        }
        
        // Collect errors
        for i := 0; i < concurrency; i++ {
            err := <-errors
            assert.NoError(t, err)
        }
        
        duration := time.Since(start)
        t.Logf("10 concurrent workflows completed in %v", duration)
        
        // Assert performance target
        assert.Less(t, duration, 5*time.Second, "should complete within 5s")
    })
    
    t.Run("Should maintain p99 latency under 100ms", func(t *testing.T) {
        provider, cleanup := setupTestDatabase(t, "sqlite")
        defer cleanup()
        
        latencies := make([]time.Duration, 100)
        
        for i := 0; i < 100; i++ {
            start := time.Now()
            _, err := provider.NewWorkflowRepo().GetState(t.Context(), testID)
            require.NoError(t, err)
            latencies[i] = time.Since(start)
        }
        
        // Calculate p99
        sort.Slice(latencies, func(i, j int) bool {
            return latencies[i] < latencies[j]
        })
        p99 := latencies[98]
        
        t.Logf("p99 latency: %v", p99)
        assert.Less(t, p99, 100*time.Millisecond)
    })
}
```

**Performance Targets:**
- Read operations: p99 < 50ms
- Write operations: p99 < 100ms
- 10 concurrent workflows: Complete in < 5s
- Database file size: < 10MB for 100 workflows

### Limit Tests

**`test/integration/database/limits_test.go` (NEW):**

- `TestLimits/Should_handle_1000_workflows`
- `TestLimits/Should_handle_deep_task_hierarchy`
- `TestLimits/Should_handle_large_jsonb_fields`

## CLI Tests (Goldens)

### Golden Tests

**`cli/cmd/start_test.go` (UPDATE):**

- `TestStart/Should_show_sqlite_in_startup_logs`
- `TestStart/Should_show_postgres_in_startup_logs`
- `TestStart/Should_warn_if_sqlite_with_high_concurrency`

**Golden Files:**
- `cli/cmd/testdata/start-sqlite.golden`
- `cli/cmd/testdata/start-postgres.golden`

**Example:**
```go
func TestStartCommand(t *testing.T) {
    t.Run("Should show sqlite in startup logs", func(t *testing.T) {
        output := captureOutput(func() {
            cmd := startCommand()
            cmd.SetArgs([]string{
                "--db-driver=sqlite",
                "--db-path=:memory:",
            })
            err := cmd.Execute()
            require.NoError(t, err)
        })
        
        golden.Assert(t, output, "start-sqlite.golden")
        assert.Contains(t, output, "driver=sqlite")
        assert.Contains(t, output, "path=:memory:")
    })
}
```

## CI/CD Configuration

### GitHub Actions Matrix

**`.github/workflows/test.yml` (UPDATE):**

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        database: [postgres, sqlite]
    
    services:
      postgres:
        image: pgvector/pgvector:pg16
        if: matrix.database == 'postgres'
        env:
          POSTGRES_PASSWORD: test
          POSTGRES_USER: test
          POSTGRES_DB: test_compozy
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: "1.25.2"
      
      - name: Run tests (PostgreSQL)
        if: matrix.database == 'postgres'
        env:
          DB_DRIVER: postgres
          DB_HOST: localhost
          DB_PORT: 5432
          DB_USER: test
          DB_PASSWORD: test
          DB_NAME: test_compozy
        run: make test
      
      - name: Run tests (SQLite)
        if: matrix.database == 'sqlite'
        env:
          DB_DRIVER: sqlite
          DB_PATH: ":memory:"
        run: make test
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out
          flags: ${{ matrix.database }}
```

## Exit Criteria

- [ ] All unit tests pass (both drivers): `make test`
- [ ] All integration tests pass (both drivers): `make test-integration`
- [ ] Code coverage ≥ 80% for new SQLite code
- [ ] Performance benchmarks meet targets (documented above)
- [ ] No regressions in PostgreSQL tests
- [ ] CI/CD matrix tests pass (both drivers)
- [ ] Golden files updated and validated
- [ ] Metrics assertions pass
- [ ] Linting passes: `make lint`
- [ ] Race detector passes: `go test -race ./...`
- [ ] Memory leaks checked with `pprof`
- [ ] All test fixtures valid and loadable
- [ ] Test helpers documented and reusable

## Test Execution Commands

```bash
# Run all tests
make test

# Run database-specific tests
go test ./engine/infra/sqlite/... -v

# Run parameterized tests (both drivers)
go test ./test/integration/database/... -v

# Run with race detector
go test -race ./engine/infra/sqlite/...

# Run performance benchmarks
go test -bench=. -benchmem ./test/integration/database/performance_test.go

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific test
go test -v ./engine/infra/sqlite -run TestTaskRepo/Should_upsert_task_state
```

## Known Test Challenges

### Challenge 1: SQLite Concurrency

**Issue:** SQLite write serialization may cause tests to be flaky

**Mitigation:**
- Use `t.Parallel()` judiciously (not for write-heavy tests)
- Add retries for "database locked" errors
- Conservative concurrency in tests (5-10 workflows max)

### Challenge 2: In-Memory vs File-Based

**Issue:** Different behavior between `:memory:` and file-based SQLite

**Mitigation:**
- Test both modes explicitly
- Use in-memory for unit tests (speed)
- Use file-based for integration tests (realistic)

### Challenge 3: Migration Testing

**Issue:** Need to test migrations for both databases

**Mitigation:**
- Parameterized migration tests
- Separate migration files per driver
- Schema comparison tests

---

**Plan Version:** 1.0  
**Date:** 2025-01-27  
**Status:** Ready for Implementation
