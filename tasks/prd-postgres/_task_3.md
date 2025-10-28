## markdown

## status: completed

<task_context>
<domain>engine/infra/sqlite</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>database</dependencies>
</task_context>

# Task 3.0: Workflow Repository (SQLite)

## Overview

Implement SQLite-backed workflow state repository for workflow execution persistence. Port the PostgreSQL `workflowrepo.go` implementation to SQLite, handling JSONB → JSON TEXT conversion, workflow state management, and transaction support.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** @tasks/prd-postgres/_techspec.md section on Workflow Repository
- **ALWAYS READ** @tasks/prd-postgres/_tests.md for test requirements
- **DEPENDENCY:** Requires Task 1.0 (Foundation) complete
- **MANDATORY:** Convert JSONB to JSON TEXT for SQLite
- **MANDATORY:** Use `json.Marshal`/`json.Unmarshal` for JSON fields
- **MANDATORY:** Handle NULL JSON fields correctly
- **MANDATORY:** Use transactions for atomic operations
</critical>

<requirements>
- Implement `WorkflowRepo` struct using `*sql.DB`
- Port all methods from `engine/infra/postgres/workflowrepo.go`
- CRUD operations: UpsertState, GetState, ListStates, UpdateStatus
- Handle JSON fields: usage, input, output, error
- Support filtering by status and workflow_id
- Transaction support for atomic updates
- Use `?` placeholders (not `$1`)
- Handle TEXT timestamps (ISO8601 format)
</requirements>

## Subtasks

- [x] 3.1 Create `engine/infra/sqlite/workflowrepo.go` structure
- [x] 3.2 Implement workflow state upsert
- [x] 3.3 Implement workflow state retrieval
- [x] 3.4 Implement list with filtering
- [x] 3.5 Implement status updates
- [x] 3.6 Add JSON marshaling helpers
- [x] 3.7 Write unit tests for CRUD operations
- [x] 3.8 Write unit tests for JSON handling
- [x] 3.9 Write integration tests for transactions

## Implementation Details

### Reference Implementation

**Source:** `engine/infra/postgres/workflowrepo.go`

### 3.1 Repository Structure

```go
package sqlite

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/engine/workflow"
)

type WorkflowRepo struct {
    db *sql.DB
}

func NewWorkflowRepo(db *sql.DB) workflow.Repository {
    return &WorkflowRepo{db: db}
}
```

### 3.2 Upsert State

```go
func (r *WorkflowRepo) UpsertState(ctx context.Context, state *workflow.State) error {
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
        INSERT INTO workflow_states (
            workflow_exec_id, workflow_id, status,
            usage, input, output, error,
            created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT (workflow_exec_id) DO UPDATE SET
            workflow_id = excluded.workflow_id,
            status = excluded.status,
            usage = excluded.usage,
            input = excluded.input,
            output = excluded.output,
            error = excluded.error,
            updated_at = excluded.updated_at
    `
    
    now := time.Now().Format(time.RFC3339)
    
    _, err = r.db.ExecContext(ctx, query,
        state.WorkflowExecID.String(),
        state.WorkflowID,
        state.Status,
        usageJSON,
        inputJSON,
        outputJSON,
        errorJSON,
        state.CreatedAt.Format(time.RFC3339),
        now,
    )
    if err != nil {
        return fmt.Errorf("upsert workflow state: %w", err)
    }
    
    return nil
}
```

### 3.3 Get State

```go
func (r *WorkflowRepo) GetState(ctx context.Context, workflowExecID core.ID) (*workflow.State, error) {
    query := `
        SELECT workflow_exec_id, workflow_id, status,
               usage, input, output, error,
               created_at, updated_at
        FROM workflow_states
        WHERE workflow_exec_id = ?
    `
    
    var state workflow.State
    var usageJSON, inputJSON, outputJSON, errorJSON sql.NullString
    var createdAt, updatedAt string
    
    err := r.db.QueryRowContext(ctx, query, workflowExecID.String()).Scan(
        &state.WorkflowExecID,
        &state.WorkflowID,
        &state.Status,
        &usageJSON,
        &inputJSON,
        &outputJSON,
        &errorJSON,
        &createdAt,
        &updatedAt,
    )
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("workflow state not found: %w", core.ErrNotFound)
    }
    if err != nil {
        return nil, fmt.Errorf("get workflow state: %w", err)
    }
    
    // Unmarshal JSON fields
    if err := unmarshalJSON(usageJSON, &state.Usage); err != nil {
        return nil, fmt.Errorf("unmarshal usage: %w", err)
    }
    
    if err := unmarshalJSON(inputJSON, &state.Input); err != nil {
        return nil, fmt.Errorf("unmarshal input: %w", err)
    }
    
    if err := unmarshalJSON(outputJSON, &state.Output); err != nil {
        return nil, fmt.Errorf("unmarshal output: %w", err)
    }
    
    if err := unmarshalJSON(errorJSON, &state.Error); err != nil {
        return nil, fmt.Errorf("unmarshal error: %w", err)
    }
    
    // Parse timestamps
    state.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
    state.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
    
    return &state, nil
}
```

### 3.4 List States

```go
func (r *WorkflowRepo) ListStates(ctx context.Context, filter workflow.Filter) ([]*workflow.State, error) {
    query := `
        SELECT workflow_exec_id, workflow_id, status,
               usage, input, output, error,
               created_at, updated_at
        FROM workflow_states
        WHERE 1=1
    `
    
    args := []any{}
    
    // Apply filters
    if filter.WorkflowID != "" {
        query += " AND workflow_id = ?"
        args = append(args, filter.WorkflowID)
    }
    
    if filter.Status != "" {
        query += " AND status = ?"
        args = append(args, filter.Status)
    }
    
    query += " ORDER BY created_at DESC"
    
    if filter.Limit > 0 {
        query += " LIMIT ?"
        args = append(args, filter.Limit)
    }
    
    rows, err := r.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("list workflow states: %w", err)
    }
    defer rows.Close()
    
    var states []*workflow.State
    for rows.Next() {
        var state workflow.State
        var usageJSON, inputJSON, outputJSON, errorJSON sql.NullString
        var createdAt, updatedAt string
        
        if err := rows.Scan(
            &state.WorkflowExecID,
            &state.WorkflowID,
            &state.Status,
            &usageJSON,
            &inputJSON,
            &outputJSON,
            &errorJSON,
            &createdAt,
            &updatedAt,
        ); err != nil {
            return nil, fmt.Errorf("scan workflow state: %w", err)
        }
        
        // Unmarshal JSON fields
        unmarshalJSON(usageJSON, &state.Usage)
        unmarshalJSON(inputJSON, &state.Input)
        unmarshalJSON(outputJSON, &state.Output)
        unmarshalJSON(errorJSON, &state.Error)
        
        state.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
        state.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
        
        states = append(states, &state)
    }
    
    return states, rows.Err()
}
```

### 3.5 Update Status

```go
func (r *WorkflowRepo) UpdateStatus(ctx context.Context, workflowExecID core.ID, status string) error {
    query := `
        UPDATE workflow_states
        SET status = ?, updated_at = ?
        WHERE workflow_exec_id = ?
    `
    
    result, err := r.db.ExecContext(ctx, query,
        status,
        time.Now().Format(time.RFC3339),
        workflowExecID.String(),
    )
    if err != nil {
        return fmt.Errorf("update workflow status: %w", err)
    }
    
    rows, _ := result.RowsAffected()
    if rows == 0 {
        return fmt.Errorf("workflow state not found: %w", core.ErrNotFound)
    }
    
    return nil
}
```

### 3.6 JSON Helpers

```go
// marshalJSON converts a value to JSON string (or NULL)
func marshalJSON(v any) (sql.NullString, error) {
    if v == nil {
        return sql.NullString{Valid: false}, nil
    }
    
    data, err := json.Marshal(v)
    if err != nil {
        return sql.NullString{}, err
    }
    
    return sql.NullString{String: string(data), Valid: true}, nil
}

// unmarshalJSON parses JSON string into value (handles NULL)
func unmarshalJSON(ns sql.NullString, v any) error {
    if !ns.Valid || ns.String == "" {
        return nil
    }
    
    return json.Unmarshal([]byte(ns.String), v)
}
```

### Relevant Files

**New Files:**
- `engine/infra/sqlite/workflowrepo.go`
- `engine/infra/sqlite/workflowrepo_test.go`
- `engine/infra/sqlite/json_helpers.go` (optional, for shared JSON functions)

**Reference Files:**
- `engine/infra/postgres/workflowrepo.go` - Source implementation
- `engine/workflow/repository.go` - Interface definition
- `engine/workflow/state.go` - Workflow state model

### Dependent Files

- `engine/infra/sqlite/store.go` - Database connection (from Task 1.0)
- `engine/infra/sqlite/migrations/*.sql` - Schema (from Task 1.0)

## Deliverables

- [ ] `engine/infra/sqlite/workflowrepo.go` with complete implementation
- [ ] All workflow CRUD operations working
- [ ] JSON fields (usage, input, output, error) handled correctly
- [ ] Filtering by status and workflow_id working
- [ ] Transaction support implemented
- [ ] All unit tests passing
- [ ] All integration tests passing
- [ ] Code passes linting

## Tests

### Unit Tests (`engine/infra/sqlite/workflowrepo_test.go`)

- [ ] `TestWorkflowRepo/Should_upsert_workflow_state`
- [ ] `TestWorkflowRepo/Should_get_workflow_state_by_exec_id`
- [ ] `TestWorkflowRepo/Should_list_workflows_by_status`
- [ ] `TestWorkflowRepo/Should_list_workflows_by_workflow_id`
- [ ] `TestWorkflowRepo/Should_update_workflow_status`
- [ ] `TestWorkflowRepo/Should_complete_workflow_with_output`
- [ ] `TestWorkflowRepo/Should_handle_jsonb_usage_field`
- [ ] `TestWorkflowRepo/Should_handle_jsonb_input_field`
- [ ] `TestWorkflowRepo/Should_handle_jsonb_output_field`
- [ ] `TestWorkflowRepo/Should_handle_jsonb_error_field`
- [ ] `TestWorkflowRepo/Should_handle_null_json_fields`
- [ ] `TestWorkflowRepo/Should_merge_usage_statistics`

### Integration Tests

- [ ] `TestWorkflowRepo/Should_execute_transaction_atomically`
- [ ] `TestWorkflowRepo/Should_rollback_on_error`
- [ ] `TestWorkflowRepo/Should_handle_concurrent_updates`

### Edge Cases

- [ ] `TestWorkflowRepo/Should_handle_missing_workflow_gracefully`
- [ ] `TestWorkflowRepo/Should_handle_empty_json_arrays`
- [ ] `TestWorkflowRepo/Should_handle_complex_nested_json`
- [ ] `TestWorkflowRepo/Should_validate_json_type_constraint`

## Success Criteria

- [ ] All workflow CRUD operations work correctly
- [ ] JSONB fields marshaled/unmarshaled correctly (usage, input, output, error)
- [ ] NULL JSON fields handled properly
- [ ] Filtering by status and workflow_id works
- [ ] Timestamps stored and retrieved correctly (ISO8601 format)
- [ ] Upsert logic (INSERT ... ON CONFLICT) works correctly
- [ ] Transactions commit and rollback correctly
- [ ] All tests pass: `go test ./engine/infra/sqlite/workflowrepo_test.go`
- [ ] Code passes linting: `golangci-lint run ./engine/infra/sqlite/workflowrepo.go`
- [ ] Repository implements `workflow.Repository` interface correctly
