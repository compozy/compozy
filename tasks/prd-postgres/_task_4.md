## markdown

## status: pending

<task_context>
<domain>engine/infra/sqlite + engine/infra/repo</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>database</dependencies>
</task_context>

# Task 4.0: Task Repository & Factory Integration

## Overview

Implement SQLite-backed task state repository with hierarchical query support and integrate the repository provider factory pattern for multi-driver selection. This is the most complex repository due to parent-child relationships, complex JSONB operations, and array conversions. Also implements the factory pattern to dynamically select PostgreSQL or SQLite repositories based on configuration.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** @tasks/prd-postgres/_techspec.md sections on Task Repository and Factory Pattern
- **ALWAYS READ** @tasks/prd-postgres/_tests.md for test requirements
- **DEPENDENCY:** Requires Tasks 1.0, 2.0, 3.0 complete
- **MANDATORY:** Convert PostgreSQL `ANY($1::uuid[])` to SQLite `IN (?, ?, ?)`
- **MANDATORY:** Handle self-referencing foreign key (parent_state_id)
- **MANDATORY:** Implement optimistic locking for concurrent updates
- **MANDATORY:** Factory pattern must not leak driver-specific types
</critical>

<requirements>
- Implement `TaskRepo` struct for SQLite
- Port all methods from `engine/infra/postgres/taskrepo.go`
- Handle hierarchical queries (parent-child relationships)
- Convert array operations to SQLite IN clauses
- Implement optimistic locking with version columns
- Update `engine/infra/repo/provider.go` with factory pattern
- Support driver selection: "postgres" | "sqlite"
- Return interface implementations (not concrete types)
</requirements>

## Subtasks

- [ ] 4.1 Create `engine/infra/sqlite/taskrepo.go` structure
- [ ] 4.2 Implement task state upsert with JSON handling
- [ ] 4.3 Implement task state retrieval
- [ ] 4.4 Implement hierarchical list queries
- [ ] 4.5 Implement array operation conversions
- [ ] 4.6 Add optimistic locking support
- [ ] 4.7 Update `engine/infra/repo/provider.go` factory
- [ ] 4.8 Write unit tests for task operations
- [ ] 4.9 Write unit tests for hierarchical queries
- [ ] 4.10 Write unit tests for factory pattern
- [ ] 4.11 Write integration tests for concurrency

## Implementation Details

### Part A: Task Repository (SQLite)

#### Reference Implementation

**Source:** `engine/infra/postgres/taskrepo.go`

#### 4.1 Repository Structure

```go
package sqlite

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "strings"
    "time"
    
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/engine/task"
)

type TaskRepo struct {
    db *sql.DB
}

func NewTaskRepo(db *sql.DB) task.Repository {
    return &TaskRepo{db: db}
}
```

#### 4.2 Upsert State

```go
func (r *TaskRepo) UpsertState(ctx context.Context, state *task.State) error {
    // Marshal JSON fields
    usageJSON, err := marshalJSON(state.Usage)
    if err != nil {
        return fmt.Errorf("marshal usage: %w", err)
    }
    
    inputJSON, err := marshalJSON(state.Input)
    if err != nil {
        return fmt.Errorf("marshal input: %w", err)
    }
    
    outputJSON, err := marshalJSON(state.Output)
    if err != nil {
        return fmt.Errorf("marshal output: %w", err)
    }
    
    errorJSON, err := marshalJSON(state.Error)
    if err != nil {
        return fmt.Errorf("marshal error: %w", err)
    }
    
    query := `
        INSERT INTO task_states (
            task_exec_id, task_id, workflow_exec_id, workflow_id,
            usage, component, status, execution_type, parent_state_id,
            agent_id, tool_id, action_id,
            input, output, error, created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT (task_exec_id) DO UPDATE SET
            task_id = excluded.task_id,
            workflow_exec_id = excluded.workflow_exec_id,
            workflow_id = excluded.workflow_id,
            usage = excluded.usage,
            component = excluded.component,
            status = excluded.status,
            execution_type = excluded.execution_type,
            parent_state_id = excluded.parent_state_id,
            agent_id = excluded.agent_id,
            tool_id = excluded.tool_id,
            action_id = excluded.action_id,
            input = excluded.input,
            output = excluded.output,
            error = excluded.error,
            updated_at = excluded.updated_at
    `
    
    now := time.Now().Format(time.RFC3339)
    
    _, err = r.db.ExecContext(ctx, query,
        state.TaskExecID.String(),
        state.TaskID,
        state.WorkflowExecID.String(),
        state.WorkflowID,
        usageJSON,
        state.Component,
        state.Status,
        state.ExecutionType,
        nullString(state.ParentStateID),
        nullString(state.AgentID),
        nullString(state.ToolID),
        nullString(state.ActionID),
        inputJSON,
        outputJSON,
        errorJSON,
        state.CreatedAt.Format(time.RFC3339),
        now,
    )
    if err != nil {
        return fmt.Errorf("upsert task state: %w", err)
    }
    
    return nil
}
```

#### 4.3 Get State

```go
func (r *TaskRepo) GetState(ctx context.Context, taskExecID core.ID) (*task.State, error) {
    query := `
        SELECT task_exec_id, task_id, workflow_exec_id, workflow_id,
               usage, component, status, execution_type, parent_state_id,
               agent_id, tool_id, action_id,
               input, output, error, created_at, updated_at
        FROM task_states
        WHERE task_exec_id = ?
    `
    
    var state task.State
    var usageJSON, inputJSON, outputJSON, errorJSON sql.NullString
    var parentID, agentID, toolID, actionID sql.NullString
    var createdAt, updatedAt string
    
    err := r.db.QueryRowContext(ctx, query, taskExecID.String()).Scan(
        &state.TaskExecID,
        &state.TaskID,
        &state.WorkflowExecID,
        &state.WorkflowID,
        &usageJSON,
        &state.Component,
        &state.Status,
        &state.ExecutionType,
        &parentID,
        &agentID,
        &toolID,
        &actionID,
        &inputJSON,
        &outputJSON,
        &errorJSON,
        &createdAt,
        &updatedAt,
    )
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("task state not found: %w", core.ErrNotFound)
    }
    if err != nil {
        return nil, fmt.Errorf("get task state: %w", err)
    }
    
    // Unmarshal JSON fields
    unmarshalJSON(usageJSON, &state.Usage)
    unmarshalJSON(inputJSON, &state.Input)
    unmarshalJSON(outputJSON, &state.Output)
    unmarshalJSON(errorJSON, &state.Error)
    
    // Handle nullable IDs
    if parentID.Valid {
        state.ParentStateID = core.ID(parentID.String)
    }
    if agentID.Valid {
        state.AgentID = agentID.String
    }
    if toolID.Valid {
        state.ToolID = toolID.String
    }
    if actionID.Valid {
        state.ActionID = actionID.String
    }
    
    // Parse timestamps
    state.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
    state.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
    
    return &state, nil
}
```

#### 4.4 List with Hierarchy

```go
func (r *TaskRepo) ListByWorkflow(ctx context.Context, workflowExecID core.ID) ([]*task.State, error) {
    query := `
        SELECT task_exec_id, task_id, workflow_exec_id, workflow_id,
               usage, component, status, execution_type, parent_state_id,
               agent_id, tool_id, action_id,
               input, output, error, created_at, updated_at
        FROM task_states
        WHERE workflow_exec_id = ?
        ORDER BY created_at ASC
    `
    
    return r.queryTasks(ctx, query, workflowExecID.String())
}

func (r *TaskRepo) ListChildren(ctx context.Context, parentID core.ID) ([]*task.State, error) {
    query := `
        SELECT task_exec_id, task_id, workflow_exec_id, workflow_id,
               usage, component, status, execution_type, parent_state_id,
               agent_id, tool_id, action_id,
               input, output, error, created_at, updated_at
        FROM task_states
        WHERE parent_state_id = ?
        ORDER BY created_at ASC
    `
    
    return r.queryTasks(ctx, query, parentID.String())
}

func (r *TaskRepo) queryTasks(ctx context.Context, query string, args ...any) ([]*task.State, error) {
    rows, err := r.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("query tasks: %w", err)
    }
    defer rows.Close()
    
    var tasks []*task.State
    for rows.Next() {
        state, err := r.scanTaskState(rows)
        if err != nil {
            return nil, err
        }
        tasks = append(tasks, state)
    }
    
    return tasks, rows.Err()
}

func (r *TaskRepo) scanTaskState(scanner interface{ Scan(...any) error }) (*task.State, error) {
    var state task.State
    var usageJSON, inputJSON, outputJSON, errorJSON sql.NullString
    var parentID, agentID, toolID, actionID sql.NullString
    var createdAt, updatedAt string
    
    if err := scanner.Scan(
        &state.TaskExecID,
        &state.TaskID,
        &state.WorkflowExecID,
        &state.WorkflowID,
        &usageJSON,
        &state.Component,
        &state.Status,
        &state.ExecutionType,
        &parentID,
        &agentID,
        &toolID,
        &actionID,
        &inputJSON,
        &outputJSON,
        &errorJSON,
        &createdAt,
        &updatedAt,
    ); err != nil {
        return nil, fmt.Errorf("scan task state: %w", err)
    }
    
    // Unmarshal and parse (same as GetState)
    unmarshalJSON(usageJSON, &state.Usage)
    unmarshalJSON(inputJSON, &state.Input)
    unmarshalJSON(outputJSON, &state.Output)
    unmarshalJSON(errorJSON, &state.Error)
    
    if parentID.Valid {
        state.ParentStateID = core.ID(parentID.String)
    }
    if agentID.Valid {
        state.AgentID = agentID.String
    }
    if toolID.Valid {
        state.ToolID = toolID.String
    }
    if actionID.Valid {
        state.ActionID = actionID.String
    }
    
    state.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
    state.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
    
    return &state, nil
}
```

#### 4.5 Array Operations (PostgreSQL `ANY()` → SQLite `IN()`)

```go
// ListByIDs converts PostgreSQL ANY($1::uuid[]) to SQLite IN (?, ?, ?)
func (r *TaskRepo) ListByIDs(ctx context.Context, ids []core.ID) ([]*task.State, error) {
    if len(ids) == 0 {
        return []*task.State{}, nil
    }
    
    // Build placeholders: ?, ?, ?
    placeholders := make([]string, len(ids))
    args := make([]any, len(ids))
    for i, id := range ids {
        placeholders[i] = "?"
        args[i] = id.String()
    }
    
    query := fmt.Sprintf(`
        SELECT task_exec_id, task_id, workflow_exec_id, workflow_id,
               usage, component, status, execution_type, parent_state_id,
               agent_id, tool_id, action_id,
               input, output, error, created_at, updated_at
        FROM task_states
        WHERE task_exec_id IN (%s)
        ORDER BY created_at ASC
    `, strings.Join(placeholders, ", "))
    
    return r.queryTasks(ctx, query, args...)
}
```

#### 4.6 Optimistic Locking (Optional, for concurrent updates)

```go
// UpdateWithVersion implements optimistic locking
func (r *TaskRepo) UpdateWithVersion(ctx context.Context, state *task.State, expectedVersion int) error {
    query := `
        UPDATE task_states
        SET status = ?, output = ?, error = ?, version = version + 1, updated_at = ?
        WHERE task_exec_id = ? AND version = ?
    `
    
    outputJSON, _ := marshalJSON(state.Output)
    errorJSON, _ := marshalJSON(state.Error)
    
    result, err := r.db.ExecContext(ctx, query,
        state.Status,
        outputJSON,
        errorJSON,
        time.Now().Format(time.RFC3339),
        state.TaskExecID.String(),
        expectedVersion,
    )
    if err != nil {
        return fmt.Errorf("update with version: %w", err)
    }
    
    rows, _ := result.RowsAffected()
    if rows == 0 {
        return fmt.Errorf("concurrent update detected: %w", core.ErrConflict)
    }
    
    return nil
}
```

### Part B: Repository Provider Factory

#### 4.7 Factory Pattern (`engine/infra/repo/provider.go`)

**Update existing file:**

```go
package repo

import (
    "context"
    "fmt"
    
    "github.com/compozy/compozy/engine/auth/uc"
    "github.com/compozy/compozy/engine/task"
    "github.com/compozy/compozy/engine/workflow"
    "github.com/compozy/compozy/engine/infra/postgres"
    "github.com/compozy/compozy/engine/infra/sqlite"
    "github.com/compozy/compozy/pkg/config"
)

type Provider struct {
    driver string
    // Don't store concrete types - only use for construction
}

func NewProvider(ctx context.Context, cfg *config.DatabaseConfig) (*Provider, func(), error) {
    switch cfg.Driver {
    case "postgres", "":  // default to postgres
        return newPostgresProvider(ctx, cfg)
    case "sqlite":
        return newSQLiteProvider(ctx, cfg)
    default:
        return nil, nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
    }
}

func newPostgresProvider(ctx context.Context, cfg *config.DatabaseConfig) (*Provider, func(), error) {
    pgCfg := &postgres.Config{
        Host:         cfg.Host,
        Port:         cfg.Port,
        User:         cfg.User,
        Password:     cfg.Password,
        DBName:       cfg.DBName,
        SSLMode:      cfg.SSLMode,
        MaxOpenConns: cfg.MaxOpenConns,
        MaxIdleConns: cfg.MaxIdleConns,
    }
    
    store, err := postgres.NewStore(ctx, pgCfg)
    if err != nil {
        return nil, nil, fmt.Errorf("create postgres store: %w", err)
    }
    
    provider := &Provider{
        driver:       "postgres",
        authRepo:     postgres.NewAuthRepo(store.Pool()),
        taskRepo:     postgres.NewTaskRepo(store.Pool()),
        workflowRepo: postgres.NewWorkflowRepo(store.Pool()),
    }
    
    cleanup := func() {
        store.Close(ctx)
    }
    
    return provider, cleanup, nil
}

func newSQLiteProvider(ctx context.Context, cfg *config.DatabaseConfig) (*Provider, func(), error) {
    sqliteCfg := &sqlite.Config{
        Path:         cfg.Path,
        MaxOpenConns: cfg.MaxOpenConns,
        MaxIdleConns: cfg.MaxIdleConns,
    }
    
    store, err := sqlite.NewStore(ctx, sqliteCfg)
    if err != nil {
        return nil, nil, fmt.Errorf("create sqlite store: %w", err)
    }
    
    // Apply migrations
    if err := sqlite.ApplyMigrations(ctx, cfg.Path); err != nil {
        store.Close(ctx)
        return nil, nil, fmt.Errorf("apply migrations: %w", err)
    }
    
    provider := &Provider{
        driver:       "sqlite",
        authRepo:     sqlite.NewAuthRepo(store.DB()),
        taskRepo:     sqlite.NewTaskRepo(store.DB()),
        workflowRepo: sqlite.NewWorkflowRepo(store.DB()),
    }
    
    cleanup := func() {
        store.Close(ctx)
    }
    
    return provider, cleanup, nil
}

// Return interface implementations (not concrete types)
func (p *Provider) NewAuthRepo() uc.Repository {
    return p.authRepo
}

func (p *Provider) NewTaskRepo() task.Repository {
    return p.taskRepo
}

func (p *Provider) NewWorkflowRepo() workflow.Repository {
    return p.workflowRepo
}

func (p *Provider) Driver() string {
    return p.driver
}
```

### Relevant Files

**New Files:**
- `engine/infra/sqlite/taskrepo.go`
- `engine/infra/sqlite/taskrepo_test.go`
- `engine/infra/sqlite/helpers.go` (optional, for shared utilities)

**Modified Files:**
- `engine/infra/repo/provider.go` - Factory pattern implementation

**Reference Files:**
- `engine/infra/postgres/taskrepo.go` - Source implementation
- `engine/task/repository.go` - Interface definition
- `engine/task/state.go` - Task state model

### Dependent Files

- `engine/infra/sqlite/store.go` - Database connection (from Task 1.0)
- `engine/infra/sqlite/migrations/*.sql` - Schema (from Task 1.0)
- `engine/infra/sqlite/authrepo.go` - Auth repository (from Task 2.0)
- `engine/infra/sqlite/workflowrepo.go` - Workflow repository (from Task 3.0)

## Deliverables

- [ ] `engine/infra/sqlite/taskrepo.go` with complete implementation
- [ ] All task CRUD operations working
- [ ] Hierarchical queries (parent-child) working
- [ ] Array operations converted to SQLite IN clauses
- [ ] Optimistic locking implemented
- [ ] `engine/infra/repo/provider.go` updated with factory pattern
- [ ] Factory correctly selects PostgreSQL or SQLite based on config
- [ ] All unit tests passing for task repository
- [ ] All unit tests passing for factory pattern
- [ ] All integration tests passing
- [ ] Code passes linting

## Tests

### Unit Tests: Task Repository (`engine/infra/sqlite/taskrepo_test.go`)

- [ ] `TestTaskRepo/Should_upsert_task_state`
- [ ] `TestTaskRepo/Should_get_task_state_by_id`
- [ ] `TestTaskRepo/Should_list_tasks_by_workflow`
- [ ] `TestTaskRepo/Should_list_tasks_by_status`
- [ ] `TestTaskRepo/Should_list_children_of_parent_task`
- [ ] `TestTaskRepo/Should_list_tasks_by_ids_array`
- [ ] `TestTaskRepo/Should_handle_jsonb_fields_correctly`
- [ ] `TestTaskRepo/Should_enforce_foreign_key_to_workflow`
- [ ] `TestTaskRepo/Should_cascade_delete_children_when_parent_deleted`
- [ ] `TestTaskRepo/Should_execute_transaction_atomically`
- [ ] `TestTaskRepo/Should_handle_concurrent_updates`
- [ ] `TestTaskRepo/Should_implement_optimistic_locking`

### Unit Tests: Factory Pattern (`engine/infra/repo/provider_test.go`)

- [ ] `TestProvider/Should_create_postgres_provider_by_default`
- [ ] `TestProvider/Should_create_postgres_provider_explicitly`
- [ ] `TestProvider/Should_create_sqlite_provider`
- [ ] `TestProvider/Should_return_error_for_invalid_driver`
- [ ] `TestProvider/Should_return_auth_repository_interface`
- [ ] `TestProvider/Should_return_task_repository_interface`
- [ ] `TestProvider/Should_return_workflow_repository_interface`
- [ ] `TestProvider/Should_not_leak_concrete_types`

### Integration Tests

- [ ] `TestTaskRepo/Should_handle_deep_task_hierarchy`
- [ ] `TestTaskRepo/Should_handle_complex_json_fields`
- [ ] `TestTaskRepo/Should_enforce_execution_type_constraints`
- [ ] `TestTaskRepo/Should_handle_self_referencing_foreign_key`

## Success Criteria

- [ ] All task CRUD operations work correctly
- [ ] Hierarchical queries return parent-child relationships correctly
- [ ] Array operations (ListByIDs) work with IN clauses
- [ ] JSONB fields marshaled/unmarshaled correctly
- [ ] Foreign keys enforced (workflow_exec_id, parent_state_id)
- [ ] Cascade deletes work for children when parent deleted
- [ ] Optimistic locking prevents concurrent update conflicts
- [ ] Factory pattern selects correct driver based on config
- [ ] Factory returns interface types (not concrete types)
- [ ] PostgreSQL repositories still work (zero regression)
- [ ] All tests pass: `go test ./engine/infra/sqlite/taskrepo_test.go ./engine/infra/repo/provider_test.go`
- [ ] Code passes linting: `golangci-lint run ./engine/infra/sqlite/taskrepo.go ./engine/infra/repo/provider.go`
