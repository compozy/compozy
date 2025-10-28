## markdown

## status: completed

<task_context>
<domain>test/integration/database</domain>
<type>testing</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>database</dependencies>
</task_context>

# Task 6.0: Multi-Driver Integration Tests

## Overview

Create comprehensive parameterized integration tests that run against both PostgreSQL and SQLite drivers, ensuring consistent behavior across database backends. This validates end-to-end workflow execution, task hierarchy, concurrent operations, and database-specific edge cases.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** @.cursor/rules/test-standards.mdc for testing patterns
- **ALWAYS READ** @tasks/prd-postgres/_tests.md for complete test requirements
- **DEPENDENCY:** Requires Tasks 1.0-5.0 complete
- **MANDATORY:** Use `t.Context()` (never `context.Background()`)
- **MANDATORY:** Test both drivers with same test logic (parameterized tests)
- **MANDATORY:** Use real databases (no mocks for database operations)
- **MANDATORY:** Conservative concurrency for SQLite (5-10 workflows max)
</critical>

<requirements>
- Parameterized tests for PostgreSQL + SQLite
- End-to-end workflow execution tests
- Task hierarchy validation tests
- Concurrent workflow tests (driver-appropriate limits)
- SQLite-specific behavior tests (database locking, in-memory mode)
- Test helpers for multi-driver setup
- All tests must pass for both drivers
</requirements>

## Subtasks

- [x] 6.1 Create test infrastructure (`test/helpers/database.go`)
- [x] 6.2 Implement parameterized workflow execution tests
- [x] 6.3 Implement task hierarchy tests
- [x] 6.4 Implement concurrent workflow tests
- [x] 6.5 Implement SQLite-specific tests
- [x] 6.6 Implement transaction tests
- [x] 6.7 Implement edge case tests

## Implementation Details

### 6.1 Test Infrastructure

**Create:** `test/helpers/database.go`

```go
package helpers

import (
    "context"
    "testing"
    
    "github.com/compozy/compozy/engine/infra/repo"
    "github.com/compozy/compozy/engine/infra/postgres"
    "github.com/compozy/compozy/engine/infra/sqlite"
    "github.com/compozy/compozy/pkg/config"
    "github.com/stretchr/testify/require"
)

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
    
    provider, cleanup, err := repo.NewProvider(t.Context(), cfg)
    require.NoError(t, err)
    
    return provider, cleanup
}

func setupPostgresTest(t *testing.T) (*repo.Provider, func()) {
    t.Helper()
    
    // Use test PostgreSQL (from environment or testcontainer)
    cfg := &config.DatabaseConfig{
        Driver:   "postgres",
        Host:     getEnvOrDefault("TEST_DB_HOST", "localhost"),
        Port:     getEnvOrDefault("TEST_DB_PORT", "5432"),
        User:     getEnvOrDefault("TEST_DB_USER", "test"),
        Password: getEnvOrDefault("TEST_DB_PASSWORD", "test"),
        DBName:   getEnvOrDefault("TEST_DB_NAME", "test_compozy"),
    }
    
    provider, cleanup, err := repo.NewProvider(t.Context(), cfg)
    require.NoError(t, err)
    
    return provider, cleanup
}

func getEnvOrDefault(key, defaultValue string) string {
    if val := os.Getenv(key); val != "" {
        return val
    }
    return defaultValue
}
```

### 6.2 Workflow Execution Tests

**Create:** `test/integration/database/multi_driver_test.go`

```go
package database_test

import (
    "testing"
    
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/engine/workflow"
    "github.com/compozy/compozy/test/helpers"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMultiDriver_WorkflowExecution(t *testing.T) {
    drivers := []string{"postgres", "sqlite"}
    
    for _, driver := range drivers {
        t.Run(driver, func(t *testing.T) {
            provider, cleanup := helpers.SetupTestDatabase(t, driver)
            defer cleanup()
            
            t.Run("Should execute workflow end to end", func(t *testing.T) {
                testWorkflowExecution(t, provider)
            })
            
            t.Run("Should persist task hierarchy", func(t *testing.T) {
                testTaskHierarchy(t, provider)
            })
            
            t.Run("Should handle concurrent workflows", func(t *testing.T) {
                // Conservative limit for SQLite
                concurrency := 5
                if driver == "postgres" {
                    concurrency = 25
                }
                testConcurrentWorkflows(t, provider, concurrency)
            })
        })
    }
}

func testWorkflowExecution(t *testing.T, provider *repo.Provider) {
    ctx := t.Context()
    workflowRepo := provider.NewWorkflowRepo()
    
    // Create workflow state
    state := &workflow.State{
        WorkflowExecID: core.NewID(),
        WorkflowID:     "test-workflow",
        Status:         core.StatusRunning,
        Input:          map[string]any{"test": "data"},
    }
    
    // Upsert
    err := workflowRepo.UpsertState(ctx, state)
    require.NoError(t, err)
    
    // Retrieve
    retrieved, err := workflowRepo.GetState(ctx, state.WorkflowExecID)
    require.NoError(t, err)
    assert.Equal(t, state.WorkflowID, retrieved.WorkflowID)
    assert.Equal(t, state.Status, retrieved.Status)
    assert.Equal(t, state.Input, retrieved.Input)
    
    // Update status
    err = workflowRepo.UpdateStatus(ctx, state.WorkflowExecID, core.StatusCompleted)
    require.NoError(t, err)
    
    // Verify update
    retrieved, err = workflowRepo.GetState(ctx, state.WorkflowExecID)
    require.NoError(t, err)
    assert.Equal(t, core.StatusCompleted, retrieved.Status)
}
```

### 6.3 Task Hierarchy Tests

```go
func testTaskHierarchy(t *testing.T, provider *repo.Provider) {
    ctx := t.Context()
    taskRepo := provider.NewTaskRepo()
    workflowRepo := provider.NewWorkflowRepo()
    
    // Create workflow
    workflowExecID := core.NewID()
    workflowState := &workflow.State{
        WorkflowExecID: workflowExecID,
        WorkflowID:     "test-workflow",
        Status:         core.StatusRunning,
    }
    err := workflowRepo.UpsertState(ctx, workflowState)
    require.NoError(t, err)
    
    // Create parent task
    parentID := core.NewID()
    parentTask := &task.State{
        TaskExecID:     parentID,
        TaskID:         "parent-task",
        WorkflowExecID: workflowExecID,
        WorkflowID:     "test-workflow",
        Component:      "task",
        Status:         core.StatusRunning,
        ExecutionType:  "basic",
    }
    err = taskRepo.UpsertState(ctx, parentTask)
    require.NoError(t, err)
    
    // Create child tasks
    child1ID := core.NewID()
    child1 := &task.State{
        TaskExecID:     child1ID,
        TaskID:         "child-task-1",
        WorkflowExecID: workflowExecID,
        WorkflowID:     "test-workflow",
        Component:      "task",
        Status:         core.StatusRunning,
        ExecutionType:  "basic",
        ParentStateID:  parentID,
    }
    err = taskRepo.UpsertState(ctx, child1)
    require.NoError(t, err)
    
    child2ID := core.NewID()
    child2 := &task.State{
        TaskExecID:     child2ID,
        TaskID:         "child-task-2",
        WorkflowExecID: workflowExecID,
        WorkflowID:     "test-workflow",
        Component:      "task",
        Status:         core.StatusCompleted,
        ExecutionType:  "basic",
        ParentStateID:  parentID,
    }
    err = taskRepo.UpsertState(ctx, child2)
    require.NoError(t, err)
    
    // List children
    children, err := taskRepo.ListChildren(ctx, parentID)
    require.NoError(t, err)
    assert.Len(t, children, 2)
    
    // Verify hierarchy
    childIDs := []core.ID{children[0].TaskExecID, children[1].TaskExecID}
    assert.Contains(t, childIDs, child1ID)
    assert.Contains(t, childIDs, child2ID)
}
```

### 6.4 Concurrent Workflow Tests

```go
func testConcurrentWorkflows(t *testing.T, provider *repo.Provider, concurrency int) {
    ctx := t.Context()
    workflowRepo := provider.NewWorkflowRepo()
    
    // Create multiple workflows concurrently
    var wg sync.WaitGroup
    errors := make(chan error, concurrency)
    
    for i := 0; i < concurrency; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            
            state := &workflow.State{
                WorkflowExecID: core.NewID(),
                WorkflowID:     fmt.Sprintf("workflow-%d", idx),
                Status:         core.StatusRunning,
            }
            
            if err := workflowRepo.UpsertState(ctx, state); err != nil {
                errors <- err
                return
            }
            
            // Update status
            if err := workflowRepo.UpdateStatus(ctx, state.WorkflowExecID, core.StatusCompleted); err != nil {
                errors <- err
            }
        }(i)
    }
    
    wg.Wait()
    close(errors)
    
    // Check for errors
    for err := range errors {
        require.NoError(t, err, "concurrent workflow operation failed")
    }
}
```

### 6.5 SQLite-Specific Tests

**Create:** `test/integration/database/sqlite_specific_test.go`

```go
package database_test

import (
    "testing"
    
    "github.com/compozy/compozy/test/helpers"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestSQLite_Specific(t *testing.T) {
    provider, cleanup := helpers.SetupTestDatabase(t, "sqlite")
    defer cleanup()
    
    t.Run("Should support in memory mode", func(t *testing.T) {
        // Already using :memory: from test helper
        // Verify operations work
        workflowRepo := provider.NewWorkflowRepo()
        
        state := createTestWorkflowState()
        err := workflowRepo.UpsertState(t.Context(), state)
        require.NoError(t, err)
        
        retrieved, err := workflowRepo.GetState(t.Context(), state.WorkflowExecID)
        require.NoError(t, err)
        assert.Equal(t, state.WorkflowID, retrieved.WorkflowID)
    })
    
    t.Run("Should enforce foreign keys", func(t *testing.T) {
        taskRepo := provider.NewTaskRepo()
        
        // Attempt to create task with non-existent workflow
        state := &task.State{
            TaskExecID:     core.NewID(),
            TaskID:         "test-task",
            WorkflowExecID: core.NewID(),  // Non-existent workflow
            WorkflowID:     "test-workflow",
            Component:      "task",
            Status:         core.StatusRunning,
            ExecutionType:  "basic",
        }
        
        err := taskRepo.UpsertState(t.Context(), state)
        assert.Error(t, err, "should fail due to foreign key constraint")
    })
    
    t.Run("Should handle concurrent reads", func(t *testing.T) {
        // SQLite should handle many concurrent reads fine
        workflowRepo := provider.NewWorkflowRepo()
        
        // Create workflow
        state := createTestWorkflowState()
        err := workflowRepo.UpsertState(t.Context(), state)
        require.NoError(t, err)
        
        // Concurrent reads
        var wg sync.WaitGroup
        for i := 0; i < 100; i++ {
            wg.Add(1)
            go func() {
                defer wg.Done()
                _, err := workflowRepo.GetState(t.Context(), state.WorkflowExecID)
                require.NoError(t, err)
            }()
        }
        
        wg.Wait()
    })
}
```

### Relevant Files

**New Files:**
- `test/helpers/database.go` - Test database setup helpers
- `test/integration/database/multi_driver_test.go` - Parameterized tests
- `test/integration/database/sqlite_specific_test.go` - SQLite edge cases
- `test/integration/database/workflow_test.go` - Workflow-specific tests
- `test/integration/database/task_test.go` - Task-specific tests
- `test/integration/database/transaction_test.go` - Transaction tests

**Reference Files:**
- `test/helpers/` - Existing test utilities
- `test/fixtures/` - Test data fixtures

### Dependent Files

- All previous tasks (1.0-5.0) must be complete
- `engine/infra/repo/provider.go` - Repository factory
- `engine/infra/sqlite/*` - SQLite repositories
- `engine/infra/postgres/*` - PostgreSQL repositories

## Deliverables

- [x] `test/helpers/database.go` with multi-driver setup
- [x] Parameterized integration tests for both drivers
- [x] End-to-end workflow execution tests
- [x] Task hierarchy tests
- [x] Concurrent workflow tests (driver-appropriate)
- [x] SQLite-specific behavior tests
- [x] Transaction tests
- [x] All tests pass for both PostgreSQL and SQLite
- [x] Test coverage ≥ 80% for new code

## Tests

### Integration Test Categories

**Workflow Execution:**
- [x] `TestMultiDriver_WorkflowExecution/Should_execute_workflow_end_to_end`
- [x] `TestMultiDriver_WorkflowExecution/Should_persist_task_hierarchy`
- [x] `TestMultiDriver_WorkflowExecution/Should_handle_concurrent_workflows`

**Task Operations:**
- [x] `TestMultiDriver_TaskOperations/Should_create_and_retrieve_tasks`
- [x] `TestMultiDriver_TaskOperations/Should_list_tasks_by_workflow`
- [x] `TestMultiDriver_TaskOperations/Should_list_children_of_parent`
- [x] `TestMultiDriver_TaskOperations/Should_handle_deep_hierarchy`

**Authentication:**
- [x] `TestMultiDriver_Authentication/Should_create_and_retrieve_users`
- [x] `TestMultiDriver_Authentication/Should_authenticate_with_api_key`
- [x] `TestMultiDriver_Authentication/Should_cascade_delete_api_keys`

**Transactions:**
- [x] `TestMultiDriver_Transactions/Should_rollback_on_error`
- [x] `TestMultiDriver_Transactions/Should_commit_on_success`
- [x] `TestMultiDriver_Transactions/Should_handle_nested_transactions`

**SQLite-Specific:**
- [x] `TestSQLite_Specific/Should_support_in_memory_mode`
- [x] `TestSQLite_Specific/Should_enforce_foreign_keys`
- [x] `TestSQLite_Specific/Should_handle_concurrent_reads`
- [x] `TestSQLite_Specific/Should_serialize_concurrent_writes`

## Success Criteria

- [x] All parameterized tests pass for both PostgreSQL and SQLite
- [x] Concurrent workflow tests work (5-10 for SQLite, 25+ for PostgreSQL)
- [x] Task hierarchy correctly handled in both databases
- [x] Foreign key constraints enforced in both databases
- [x] Transactions commit/rollback correctly in both databases
- [x] SQLite-specific edge cases handled properly
- [x] Test helpers work for both drivers
- [x] All tests use `t.Context()` (not `context.Background()`)
- [x] All tests follow `t.Run("Should ...")` pattern
- [x] No test flakiness or race conditions
- [x] Tests run successfully: `go test ./test/integration/database/... -v -race`
- [x] Code coverage: `go test -coverprofile=coverage.out ./test/integration/database/...`
