package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
)

// ErrTaskNotFound is returned when a task state is not found.
var ErrTaskNotFound = fmt.Errorf("task state not found")

const maxTaskTreeDepth = 100

// TaskRepo implements the task.Repository interface.
type TaskRepo struct {
	db    DBInterface
	dbPtr *DB // Keep reference to concrete DB for transaction operations
}

func NewTaskRepo(db DBInterface) *TaskRepo {
	// Try to get concrete DB for transaction support
	dbPtr, _ := db.(*DB) //nolint:errcheck // intentionally ignored for test compatibility
	return &TaskRepo{
		db:    db,
		dbPtr: dbPtr,
	}
}

// ListStates retrieves task states based on the provided filter.
func (r *TaskRepo) ListStates(ctx context.Context, filter *task.StateFilter) ([]*task.State, error) {
	sb := squirrel.Select("*").
		From("task_states").
		PlaceholderFormat(squirrel.Dollar)

	sb = r.applyStateFilter(sb, filter)

	sql, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building query: %w", err)
	}

	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, sql, args...); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}

	var states []*task.State
	for _, stateDB := range statesDB {
		state, err := stateDB.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting state: %w", err)
		}
		states = append(states, state)
	}

	return states, nil
}

// applyStateFilter applies the state filter conditions to the query builder
func (r *TaskRepo) applyStateFilter(sb squirrel.SelectBuilder, filter *task.StateFilter) squirrel.SelectBuilder {
	if filter == nil {
		return sb
	}

	if filter.Status != nil {
		sb = sb.Where(squirrel.Eq{"status": *filter.Status})
	}
	if filter.WorkflowID != nil {
		sb = sb.Where(squirrel.Eq{"workflow_id": *filter.WorkflowID})
	}
	if filter.WorkflowExecID != nil {
		sb = sb.Where(squirrel.Eq{"workflow_exec_id": *filter.WorkflowExecID})
	}
	if filter.TaskID != nil {
		sb = sb.Where(squirrel.Eq{"task_id": *filter.TaskID})
	}
	if filter.TaskExecID != nil {
		sb = sb.Where(squirrel.Eq{"task_exec_id": *filter.TaskExecID})
	}
	if filter.ParentStateID != nil {
		sb = sb.Where(squirrel.Eq{"parent_state_id": *filter.ParentStateID})
	}
	if filter.AgentID != nil {
		sb = sb.Where(squirrel.Eq{"agent_id": *filter.AgentID})
	}
	if filter.ActionID != nil {
		sb = sb.Where(squirrel.Eq{"action_id": *filter.ActionID})
	}
	if filter.ToolID != nil {
		sb = sb.Where(squirrel.Eq{"tool_id": *filter.ToolID})
	}
	if filter.ExecutionType != nil {
		sb = sb.Where(squirrel.Eq{"execution_type": *filter.ExecutionType})
	}

	return sb
}

// buildUpsertArgs prepares the SQL query and arguments for upserting a task state
func (r *TaskRepo) buildUpsertArgs(state *task.State) (string, []any, error) {
	// Marshal common fields
	input, err := ToJSONB(state.Input)
	if err != nil {
		return "", nil, fmt.Errorf("marshaling input: %w", err)
	}
	output, err := ToJSONB(state.Output)
	if err != nil {
		return "", nil, fmt.Errorf("marshaling output: %w", err)
	}
	errJSON, err := ToJSONB(state.Error)
	if err != nil {
		return "", nil, fmt.Errorf("marshaling error: %w", err)
	}

	// Handle parent-child relationship
	var parentStateID *string
	if state.ParentStateID != nil {
		parentIDStr := string(*state.ParentStateID)
		parentStateID = &parentIDStr
	}

	query := `
		INSERT INTO task_states (
			task_exec_id, task_id, workflow_exec_id, workflow_id, component, status,
			execution_type, parent_state_id, agent_id, action_id, tool_id, input, output, error
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (task_exec_id) DO UPDATE SET
			task_id = $2,
			workflow_exec_id = $3,
			workflow_id = $4,
			component = $5,
			status = $6,
			execution_type = $7,
			parent_state_id = $8,
			agent_id = $9,
			action_id = $10,
			tool_id = $11,
			input = $12,
			output = $13,
			error = $14,
			updated_at = now()
	`

	args := []any{
		state.TaskExecID, state.TaskID, state.WorkflowExecID, state.WorkflowID,
		state.Component, state.Status, state.ExecutionType, parentStateID,
		state.AgentID, state.ActionID, state.ToolID,
		input, output, errJSON,
	}

	return query, args, nil
}

// UpsertState inserts or updates a task state (supports both basic and parallel execution).
func (r *TaskRepo) UpsertState(ctx context.Context, state *task.State) error {
	query, args, err := r.buildUpsertArgs(state)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("executing upsert: %w", err)
	}

	return nil
}

// GetState retrieves a task state by its task execution ID.
func (r *TaskRepo) GetState(ctx context.Context, taskExecID core.ID) (*task.State, error) {
	query := `
		SELECT *
		FROM task_states
		WHERE task_exec_id = $1
	`

	var stateDB task.StateDB
	err := pgxscan.Get(ctx, r.db, &stateDB, query, taskExecID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("scanning state: %w", err)
	}

	return stateDB.ToState()
}

// GetStateForUpdate retrieves a task state by its task execution ID with row-level locking.
// This method should be used within a transaction when concurrent updates are expected.
func (r *TaskRepo) GetStateForUpdate(ctx context.Context, tx pgx.Tx, taskExecID core.ID) (*task.State, error) {
	query := `
		SELECT *
		FROM task_states
		WHERE task_exec_id = $1
		FOR UPDATE
	`

	var stateDB task.StateDB
	err := pgxscan.Get(ctx, tx, &stateDB, query, taskExecID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("scanning state with lock: %w", err)
	}

	return stateDB.ToState()
}

// WithTx executes a function within a transaction
func (r *TaskRepo) WithTx(ctx context.Context, fn func(pgx.Tx) error) error {
	if r.dbPtr != nil {
		return r.dbPtr.WithTx(ctx, fn)
	}

	// Fallback for tests - use the db interface directly to begin transaction
	tx, beginErr := r.db.Begin(ctx)
	if beginErr != nil {
		return fmt.Errorf("beginning transaction: %w", beginErr)
	}

	var cbErr error
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback(ctx) //nolint:errcheck // rollback errors cannot be handled in panic recovery
			panic(p)
		} else if cbErr != nil {
			tx.Rollback(ctx) //nolint:errcheck // rollback errors cannot be handled in defer
		} else {
			cbErr = tx.Commit(ctx)
		}
	}()

	cbErr = fn(tx)
	return cbErr
}

// ListTasksInWorkflow retrieves all task states for a workflow execution.
func (r *TaskRepo) ListTasksInWorkflow(ctx context.Context, workflowExecID core.ID) (map[string]*task.State, error) {
	// Use recursive CTE to get ALL task states including nested children
	// This ensures that sibling tasks within composite parents can reference each other
	query := TaskHierarchyCTEQuery
	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, workflowExecID); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}
	result := make(map[string]*task.State)
	for _, stateDB := range statesDB {
		state, err := stateDB.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting state: %w", err)
		}
		// TODO: revisit this design
		// Note: This still keys by TaskID; if TaskIDs aren't unique per workflow,
		// we should revisit this design or make the value a slice.
		result[state.TaskID] = state
	}
	return result, nil
}

// ListTasksByStatus retrieves task states by status.
func (r *TaskRepo) ListTasksByStatus(
	ctx context.Context,
	workflowExecID core.ID,
	status core.StatusType,
) ([]*task.State, error) {
	query := `
		SELECT *
		FROM task_states
		WHERE workflow_exec_id = $1 AND status = $2
	`

	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, workflowExecID, status); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}

	var states []*task.State
	for _, stateDB := range statesDB {
		state, err := stateDB.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting state: %w", err)
		}
		states = append(states, state)
	}

	return states, nil
}

// ListTasksByAgent retrieves task states by agent ID.
func (r *TaskRepo) ListTasksByAgent(
	ctx context.Context,
	workflowExecID core.ID,
	agentID string,
) ([]*task.State, error) {
	query := `
		SELECT *
		FROM task_states
		WHERE workflow_exec_id = $1 AND agent_id = $2
	`

	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, workflowExecID, agentID); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}

	var states []*task.State
	for _, stateDB := range statesDB {
		state, err := stateDB.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting state: %w", err)
		}
		states = append(states, state)
	}

	return states, nil
}

// ListTasksByTool retrieves task states by tool ID.
func (r *TaskRepo) ListTasksByTool(ctx context.Context, workflowExecID core.ID, toolID string) ([]*task.State, error) {
	query := `
		SELECT *
		FROM task_states
		WHERE workflow_exec_id = $1 AND tool_id = $2
	`

	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, workflowExecID, toolID); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}

	var states []*task.State
	for _, stateDB := range statesDB {
		state, err := stateDB.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting state: %w", err)
		}
		states = append(states, state)
	}

	return states, nil
}

// ListChildren retrieves all child tasks for a given parent task.
func (r *TaskRepo) ListChildren(ctx context.Context, parentStateID core.ID) ([]*task.State, error) {
	query := `
		SELECT *
		FROM task_states
		WHERE parent_state_id = $1
		ORDER BY task_id
	`

	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, parentStateID); err != nil {
		return nil, fmt.Errorf("scanning child states: %w", err)
	}

	var states []*task.State
	for _, stateDB := range statesDB {
		state, err := stateDB.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting child state: %w", err)
		}
		states = append(states, state)
	}

	return states, nil
}

// ListChildrenOutputs retrieves only the outputs of child tasks for performance.
// This is more efficient than loading full task states when only outputs are needed.
func (r *TaskRepo) ListChildrenOutputs(ctx context.Context, parentStateID core.ID) (map[string]*core.Output, error) {
	query := `
		SELECT task_id, output
		FROM task_states
		WHERE parent_state_id = $1 AND output IS NOT NULL
		ORDER BY task_id
	`
	rows, err := r.db.Query(ctx, query, parentStateID)
	if err != nil {
		return nil, fmt.Errorf("querying child outputs: %w", err)
	}
	defer rows.Close()
	outputs := make(map[string]*core.Output)
	for rows.Next() {
		var taskID string
		var outputJSON []byte
		if err := rows.Scan(&taskID, &outputJSON); err != nil {
			return nil, fmt.Errorf("scanning child output: %w", err)
		}
		var output core.Output
		if err := json.Unmarshal(outputJSON, &output); err != nil {
			return nil, fmt.Errorf("unmarshaling output for task %s: %w", taskID, err)
		}
		outputs[taskID] = &output
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating child outputs: %w", err)
	}
	return outputs, nil
}

// GetChildByTaskID retrieves a specific child task state by its parent and task ID.
func (r *TaskRepo) GetChildByTaskID(ctx context.Context, parentStateID core.ID, taskID string) (*task.State, error) {
	query := `
		SELECT *
		FROM task_states
		WHERE parent_state_id = $1 AND task_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	var stateDB task.StateDB
	err := pgxscan.Get(ctx, r.db, &stateDB, query, parentStateID, taskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("scanning child state: %w", err)
	}

	return stateDB.ToState()
}

// GetTaskTree retrieves a complete task hierarchy starting from the root using PostgreSQL CTE.
func (r *TaskRepo) GetTaskTree(ctx context.Context, rootStateID core.ID) ([]*task.State, error) {
	query := fmt.Sprintf(`
        WITH RECURSIVE task_tree AS (
			-- Base case: start with the root task
			SELECT task_exec_id, task_id, workflow_exec_id, workflow_id, component,
				   status, execution_type, parent_state_id, agent_id, action_id, tool_id,
				   input, output, error, created_at, updated_at, 0 as depth
			FROM task_states
			WHERE task_exec_id = $1

			UNION ALL

			-- Recursive case: find all children
			SELECT ts.task_exec_id, ts.task_id, ts.workflow_exec_id, ts.workflow_id, ts.component,
				   ts.status, ts.execution_type, ts.parent_state_id, ts.agent_id, ts.action_id, ts.tool_id,
				   ts.input, ts.output, ts.error, ts.created_at, ts.updated_at, tt.depth + 1
			FROM task_states ts
			INNER JOIN task_tree tt ON ts.parent_state_id = tt.task_exec_id
			WHERE tt.depth < %d
		)
        SELECT task_exec_id, task_id, workflow_exec_id, workflow_id, component,
			   status, execution_type, parent_state_id, agent_id, action_id, tool_id,
			   input, output, error, created_at, updated_at
		FROM task_tree
		ORDER BY depth, created_at
	`, maxTaskTreeDepth)

	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, rootStateID); err != nil {
		return nil, fmt.Errorf("scanning task tree: %w", err)
	}

	var states []*task.State
	for _, stateDB := range statesDB {
		state, err := stateDB.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting task tree state: %w", err)
		}
		states = append(states, state)
	}

	return states, nil
}

// GetProgressInfo aggregates progress information for a parent task using SQL
func (r *TaskRepo) GetProgressInfo(ctx context.Context, parentStateID core.ID) (*task.ProgressInfo, error) {
	statusQuery := `
		SELECT status, COUNT(*) as status_count
		FROM task_states
		WHERE parent_state_id = $1
		GROUP BY status
	`

	progressInfo := &task.ProgressInfo{
		StatusCounts: make(map[core.StatusType]int),
	}

	statusRows, err := r.db.Query(ctx, statusQuery, parentStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to query status counts: %w", err)
	}
	defer statusRows.Close()

	var totalChildren int
	for statusRows.Next() {
		var status string
		var count int
		err := statusRows.Scan(&status, &count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan status row: %w", err)
		}

		statusType := core.StatusType(status)
		progressInfo.StatusCounts[statusType] = count
		totalChildren += count
	}

	if err := statusRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating status rows: %w", err)
	}

	progressInfo.TotalChildren = totalChildren

	// Derive specific counters from status counts with clear semantics
	progressInfo.SuccessCount = progressInfo.StatusCounts[core.StatusSuccess]
	progressInfo.FailedCount = progressInfo.StatusCounts[core.StatusFailed]
	progressInfo.CanceledCount = progressInfo.StatusCounts[core.StatusCanceled]
	progressInfo.TimedOutCount = progressInfo.StatusCounts[core.StatusTimedOut]
	progressInfo.RunningCount = progressInfo.StatusCounts[core.StatusRunning] +
		progressInfo.StatusCounts[core.StatusWaiting] +
		progressInfo.StatusCounts[core.StatusPaused]
	progressInfo.PendingCount = progressInfo.StatusCounts[core.StatusPending]

	// Calculate terminal count (all finished tasks regardless of outcome)
	progressInfo.TerminalCount = progressInfo.SuccessCount +
		progressInfo.FailedCount +
		progressInfo.CanceledCount +
		progressInfo.TimedOutCount

	// Calculate rates based on total children
	if progressInfo.TotalChildren > 0 {
		progressInfo.CompletionRate = float64(progressInfo.SuccessCount) / float64(progressInfo.TotalChildren)
		progressInfo.FailureRate = float64(
			progressInfo.FailedCount+progressInfo.TimedOutCount,
		) / float64(
			progressInfo.TotalChildren,
		)
	}

	return progressInfo, nil
}

// CreateChildStatesInTransaction creates multiple child states atomically
func (r *TaskRepo) CreateChildStatesInTransaction(
	ctx context.Context,
	parentStateID core.ID,
	childStates []*task.State,
) (err error) {
	log := logger.FromContext(ctx)
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				log.Error("Failed to rollback transaction after panic", "error", rollbackErr)
			}
			panic(p)
		} else if err != nil {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				log.Error("Failed to rollback transaction", "error", rollbackErr)
			}
		}
	}()

	// Validate that all child states have the correct parent ID
	for _, childState := range childStates {
		if childState.ParentStateID == nil || *childState.ParentStateID != parentStateID {
			return fmt.Errorf("child state %s does not have correct parent ID", childState.TaskID)
		}
	}

	// Insert all child states within the transaction
	for _, childState := range childStates {
		err = r.UpsertStateWithTx(ctx, tx, childState)
		if err != nil {
			return fmt.Errorf("failed to create child state %s: %w", childState.TaskID, err)
		}
	}

	// Commit the transaction and return any commit error
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// UpsertStateWithTx inserts/updates a state within an existing transaction
func (r *TaskRepo) UpsertStateWithTx(ctx context.Context, tx pgx.Tx, state *task.State) error {
	query, args, err := r.buildUpsertArgs(state)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("executing upsert: %w", err)
	}

	return nil
}
