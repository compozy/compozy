package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const maxTaskTreeDepth = 100

var taskStateColumns = []string{
	"task_exec_id",
	"task_id",
	"workflow_exec_id",
	"workflow_id",
	"component",
	"status",
	"execution_type",
	"parent_state_id",
	"agent_id",
	"action_id",
	"tool_id",
	"input",
	"output",
	"error",
	"created_at",
	"updated_at",
}

const taskStateColumnsSQL = "task_exec_id, task_id, workflow_exec_id, workflow_id, " +
	"component, status, execution_type, parent_state_id, agent_id, action_id, " +
	"tool_id, input, output, error, created_at, updated_at"

func maxDepthFromConfig(ctx context.Context) int {
	cfg := config.FromContext(ctx)
	if cfg != nil && cfg.Limits.MaxTaskContextDepth > 0 {
		return cfg.Limits.MaxTaskContextDepth
	}
	return maxTaskTreeDepth
}

// DB is the minimal database interface TaskRepo depends on (pgxpool or pgxmock).
type DB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// TaskRepo implements the task.Repository interface backed by pgx-compatible pool.
type TaskRepo struct {
	db DB
}

func NewTaskRepo(db DB) *TaskRepo {
	return &TaskRepo{db: db}
}

// ListStates retrieves task states based on the provided filter.
func (r *TaskRepo) ListStates(ctx context.Context, filter *task.StateFilter) ([]*task.State, error) {
	return r.listStatesWith(ctx, r.db, filter)
}

func (r *TaskRepo) listStatesWith(
	ctx context.Context,
	runner pgxscan.Querier,
	filter *task.StateFilter,
) ([]*task.State, error) {
	sb := squirrel.Select(taskStateColumns...).From("task_states").PlaceholderFormat(squirrel.Dollar)
	sb = r.applyStateFilter(sb, filter)
	sql, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building query: %w", err)
	}
	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, runner, &statesDB, sql, args...); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}
	return convertStates(statesDB)
}

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

func convertStates(statesDB []*task.StateDB) ([]*task.State, error) {
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

// buildUpsertArgs prepares the SQL query and arguments for upserting a task state
func (r *TaskRepo) buildUpsertArgs(state *task.State) (string, []any, error) {
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
	var parentStateID *string
	if state.ParentStateID != nil {
		s := string(*state.ParentStateID)
		parentStateID = &s
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

// UpsertState inserts or updates a task state.
func (r *TaskRepo) UpsertState(ctx context.Context, state *task.State) error {
	query, args, err := r.buildUpsertArgs(state)
	if err != nil {
		return err
	}
	if _, err = r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("executing upsert: %w", err)
	}
	return nil
}

// GetState retrieves a task state by its task execution ID.
func (r *TaskRepo) GetState(ctx context.Context, taskExecID core.ID) (*task.State, error) {
	query := fmt.Sprintf("SELECT %s FROM task_states WHERE task_exec_id = $1", taskStateColumnsSQL)
	var stateDB task.StateDB
	if err := pgxscan.Get(ctx, r.db, &stateDB, query, taskExecID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrTaskNotFound
		}
		return nil, fmt.Errorf("scanning state: %w", err)
	}
	return stateDB.ToState()
}

// GetStateForUpdate is not allowed outside of a transaction on the non-tx repo.
func (r *TaskRepo) GetStateForUpdate(_ context.Context, _ core.ID) (*task.State, error) {
	return nil, fmt.Errorf("GetStateForUpdate requires transactional context")
}

func (r *TaskRepo) getStateForUpdateTx(ctx context.Context, tx pgx.Tx, taskExecID core.ID) (*task.State, error) {
	query := fmt.Sprintf("SELECT %s FROM task_states WHERE task_exec_id = $1 FOR UPDATE", taskStateColumnsSQL)
	var stateDB task.StateDB
	if err := pgxscan.Get(ctx, tx, &stateDB, query, taskExecID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrTaskNotFound
		}
		return nil, fmt.Errorf("scanning state with lock: %w", err)
	}
	return stateDB.ToState()
}

// WithTransaction provides a tx-scoped repository to the callback.
func (r *TaskRepo) WithTransaction(ctx context.Context, fn func(task.Repository) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	log := logger.FromContext(ctx)
	repoTx := &taskRepoTx{parent: r, tx: tx}
	var cbErr error
	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				log.Error("Failed to rollback transaction", "error", rbErr)
			}
			panic(p)
		} else if cbErr != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				log.Error("Failed to rollback transaction", "error", rbErr)
			}
		} else {
			if err := tx.Commit(ctx); err != nil {
				log.Error("Failed to commit transaction", "error", err)
				cbErr = fmt.Errorf("commit transaction: %w", err)
			}
		}
	}()
	cbErr = fn(repoTx)
	return cbErr
}

type taskRepoTx struct {
	parent *TaskRepo
	tx     pgx.Tx
}

func (t *taskRepoTx) ListStates(ctx context.Context, filter *task.StateFilter) ([]*task.State, error) {
	return t.parent.listStatesWith(ctx, t.tx, filter)
}

func (t *taskRepoTx) WithTransaction(_ context.Context, fn func(task.Repository) error) error {
	return fn(t)
}

func (t *taskRepoTx) UpsertState(ctx context.Context, state *task.State) error {
	return t.parent.UpsertStateWithTx(ctx, t.tx, state)
}

func (t *taskRepoTx) GetState(ctx context.Context, taskExecID core.ID) (*task.State, error) {
	query := fmt.Sprintf("SELECT %s FROM task_states WHERE task_exec_id = $1", taskStateColumnsSQL)
	var stateDB task.StateDB
	if err := pgxscan.Get(ctx, t.tx, &stateDB, query, taskExecID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrTaskNotFound
		}
		return nil, fmt.Errorf("scanning state: %w", err)
	}
	return stateDB.ToState()
}

func (t *taskRepoTx) GetStateForUpdate(ctx context.Context, taskExecID core.ID) (*task.State, error) {
	return t.parent.getStateForUpdateTx(ctx, t.tx, taskExecID)
}

func (t *taskRepoTx) ListTasksInWorkflow(ctx context.Context, workflowExecID core.ID) (map[string]*task.State, error) {
	query := fmt.Sprintf(`
		SELECT DISTINCT ON (task_id) %s
		FROM task_states
		WHERE workflow_exec_id = $1
		ORDER BY task_id, created_at DESC
	`, taskStateColumnsSQL)
	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, t.tx, &statesDB, query, workflowExecID); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}
	result := make(map[string]*task.State, len(statesDB))
	for _, sdb := range statesDB {
		s, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting state: %w", err)
		}
		result[s.TaskID] = s
	}
	return result, nil
}

func (t *taskRepoTx) ListTasksByStatus(
	ctx context.Context,
	workflowExecID core.ID,
	status core.StatusType,
) ([]*task.State, error) {
	query := fmt.Sprintf("SELECT %s FROM task_states WHERE workflow_exec_id = $1 AND status = $2", taskStateColumnsSQL)
	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, t.tx, &statesDB, query, workflowExecID, status); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}
	return convertStates(statesDB)
}

func (t *taskRepoTx) ListTasksByAgent(
	ctx context.Context,
	workflowExecID core.ID,
	agentID string,
) ([]*task.State, error) {
	query := fmt.Sprintf(
		"SELECT %s FROM task_states WHERE workflow_exec_id = $1 AND agent_id = $2",
		taskStateColumnsSQL,
	)
	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, t.tx, &statesDB, query, workflowExecID, agentID); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}
	return convertStates(statesDB)
}

func (t *taskRepoTx) ListTasksByTool(
	ctx context.Context,
	workflowExecID core.ID,
	toolID string,
) ([]*task.State, error) {
	query := fmt.Sprintf("SELECT %s FROM task_states WHERE workflow_exec_id = $1 AND tool_id = $2", taskStateColumnsSQL)
	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, t.tx, &statesDB, query, workflowExecID, toolID); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}
	return convertStates(statesDB)
}

func (t *taskRepoTx) ListChildren(ctx context.Context, parentStateID core.ID) ([]*task.State, error) {
	query := fmt.Sprintf("SELECT %s FROM task_states WHERE parent_state_id = $1 ORDER BY task_id", taskStateColumnsSQL)
	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, t.tx, &statesDB, query, parentStateID); err != nil {
		return nil, fmt.Errorf("scanning child states: %w", err)
	}
	return convertStates(statesDB)
}

func (t *taskRepoTx) ListChildrenOutputs(ctx context.Context, parentStateID core.ID) (map[string]*core.Output, error) {
	query := `SELECT task_id, output FROM task_states WHERE parent_state_id = $1 AND output IS NOT NULL ORDER BY task_id`
	rows, err := t.tx.Query(ctx, query, parentStateID)
	if err != nil {
		return nil, fmt.Errorf("querying child outputs: %w", err)
	}
	defer rows.Close()
	outs := make(map[string]*core.Output)
	for rows.Next() {
		var taskID string
		var outputJSON []byte
		if err := rows.Scan(&taskID, &outputJSON); err != nil {
			return nil, fmt.Errorf("scanning child output: %w", err)
		}
		var out core.Output
		if err := json.Unmarshal(outputJSON, &out); err != nil {
			return nil, fmt.Errorf("unmarshaling output for task %s: %w", taskID, err)
		}
		outs[taskID] = &out
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating child outputs: %w", err)
	}
	return outs, nil
}

func (t *taskRepoTx) GetChildByTaskID(ctx context.Context, parentStateID core.ID, taskID string) (*task.State, error) {
	query := fmt.Sprintf(
		"SELECT %s FROM task_states WHERE parent_state_id = $1 AND task_id = $2 "+
			"ORDER BY created_at DESC LIMIT 1",
		taskStateColumnsSQL,
	)
	var stateDB task.StateDB
	if err := pgxscan.Get(ctx, t.tx, &stateDB, query, parentStateID, taskID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrTaskNotFound
		}
		return nil, fmt.Errorf("scanning child state: %w", err)
	}
	return stateDB.ToState()
}

func (t *taskRepoTx) GetTaskTree(ctx context.Context, rootStateID core.ID) ([]*task.State, error) {
	return t.parent.getTaskTreeWith(ctx, t.tx, rootStateID)
}

func (t *taskRepoTx) GetProgressInfo(ctx context.Context, parentStateID core.ID) (*task.ProgressInfo, error) {
	statusQuery := `SELECT status, COUNT(*) as status_count FROM task_states WHERE parent_state_id = $1 GROUP BY status`
	progressInfo := &task.ProgressInfo{StatusCounts: make(map[core.StatusType]int)}
	statusRows, err := t.tx.Query(ctx, statusQuery, parentStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to query status counts: %w", err)
	}
	defer statusRows.Close()
	var totalChildren int
	for statusRows.Next() {
		var status string
		var count int
		if err := statusRows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan status row: %w", err)
		}
		progressInfo.StatusCounts[core.StatusType(status)] = count
		totalChildren += count
	}
	if err := statusRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating status rows: %w", err)
	}
	progressInfo.TotalChildren = totalChildren
	progressInfo.SuccessCount = progressInfo.StatusCounts[core.StatusSuccess]
	progressInfo.FailedCount = progressInfo.StatusCounts[core.StatusFailed]
	progressInfo.CanceledCount = progressInfo.StatusCounts[core.StatusCanceled]
	progressInfo.TimedOutCount = progressInfo.StatusCounts[core.StatusTimedOut]
	progressInfo.RunningCount = progressInfo.StatusCounts[core.StatusRunning] +
		progressInfo.StatusCounts[core.StatusWaiting] +
		progressInfo.StatusCounts[core.StatusPaused]
	progressInfo.PendingCount = progressInfo.StatusCounts[core.StatusPending]
	progressInfo.TerminalCount = progressInfo.SuccessCount +
		progressInfo.FailedCount +
		progressInfo.CanceledCount +
		progressInfo.TimedOutCount
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

// ListTasksInWorkflow retrieves task states by workflow exec id.
func (r *TaskRepo) ListTasksInWorkflow(ctx context.Context, workflowExecID core.ID) (map[string]*task.State, error) {
	query := fmt.Sprintf(`
		SELECT DISTINCT ON (task_id) %s
		FROM task_states
		WHERE workflow_exec_id = $1
		ORDER BY task_id, created_at DESC
	`, taskStateColumnsSQL)
	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, workflowExecID); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}
	states, err := convertStates(statesDB)
	if err != nil {
		return nil, err
	}
	result := make(map[string]*task.State)
	for _, state := range states {
		result[state.TaskID] = state
	}
	return result, nil
}

func (r *TaskRepo) ListTasksByStatus(
	ctx context.Context,
	workflowExecID core.ID,
	status core.StatusType,
) ([]*task.State, error) {
	query := fmt.Sprintf("SELECT %s FROM task_states WHERE workflow_exec_id = $1 AND status = $2", taskStateColumnsSQL)
	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, workflowExecID, status); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}
	return convertStates(statesDB)
}

func (r *TaskRepo) ListTasksByAgent(
	ctx context.Context,
	workflowExecID core.ID,
	agentID string,
) ([]*task.State, error) {
	query := fmt.Sprintf(
		"SELECT %s FROM task_states WHERE workflow_exec_id = $1 AND agent_id = $2",
		taskStateColumnsSQL,
	)
	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, workflowExecID, agentID); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}
	return convertStates(statesDB)
}

func (r *TaskRepo) ListTasksByTool(ctx context.Context, workflowExecID core.ID, toolID string) ([]*task.State, error) {
	query := fmt.Sprintf("SELECT %s FROM task_states WHERE workflow_exec_id = $1 AND tool_id = $2", taskStateColumnsSQL)
	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, workflowExecID, toolID); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}
	return convertStates(statesDB)
}

func (r *TaskRepo) ListChildren(ctx context.Context, parentStateID core.ID) ([]*task.State, error) {
	query := fmt.Sprintf("SELECT %s FROM task_states WHERE parent_state_id = $1 ORDER BY task_id", taskStateColumnsSQL)
	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, parentStateID); err != nil {
		return nil, fmt.Errorf("scanning child states: %w", err)
	}
	return convertStates(statesDB)
}

func (r *TaskRepo) ListChildrenOutputs(ctx context.Context, parentStateID core.ID) (map[string]*core.Output, error) {
	query := `SELECT task_id, output FROM task_states WHERE parent_state_id = $1 AND output IS NOT NULL ORDER BY task_id`
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

func (r *TaskRepo) GetChildByTaskID(ctx context.Context, parentStateID core.ID, taskID string) (*task.State, error) {
	query := fmt.Sprintf(
		"SELECT %s FROM task_states WHERE parent_state_id = $1 AND task_id = $2 "+
			"ORDER BY created_at DESC LIMIT 1",
		taskStateColumnsSQL,
	)
	var stateDB task.StateDB
	if err := pgxscan.Get(ctx, r.db, &stateDB, query, parentStateID, taskID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrTaskNotFound
		}
		return nil, fmt.Errorf("scanning child state: %w", err)
	}
	return stateDB.ToState()
}

func (r *TaskRepo) GetTaskTree(ctx context.Context, rootStateID core.ID) ([]*task.State, error) {
	query := `
        WITH RECURSIVE task_tree AS (
            SELECT task_exec_id, task_id, workflow_exec_id, workflow_id, component,
                   status, execution_type, parent_state_id, agent_id, action_id, tool_id,
                   input, output, error, created_at, updated_at, 0 as depth
            FROM task_states
            WHERE task_exec_id = $1

            UNION ALL

            SELECT ts.task_exec_id, ts.task_id, ts.workflow_exec_id, ts.workflow_id, ts.component,
                   ts.status, ts.execution_type, ts.parent_state_id, ts.agent_id, ts.action_id, ts.tool_id,
                   ts.input, ts.output, ts.error, ts.created_at, ts.updated_at, tt.depth + 1
            FROM task_states ts
            INNER JOIN task_tree tt ON ts.parent_state_id = tt.task_exec_id
            WHERE tt.depth < $2
        )
        SELECT task_exec_id, task_id, workflow_exec_id, workflow_id, component,
               status, execution_type, parent_state_id, agent_id, action_id, tool_id,
               input, output, error, created_at, updated_at
        FROM task_tree
        ORDER BY depth, created_at
    `
	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, rootStateID, maxDepthFromConfig(ctx)); err != nil {
		return nil, fmt.Errorf("scanning task tree: %w", err)
	}
	states, err := convertStates(statesDB)
	if err != nil {
		return nil, fmt.Errorf("converting task tree state: %w", err)
	}
	return states, nil
}

func (r *TaskRepo) GetProgressInfo(ctx context.Context, parentStateID core.ID) (*task.ProgressInfo, error) {
	statusQuery := `SELECT status, COUNT(*) as status_count FROM task_states WHERE parent_state_id = $1 GROUP BY status`
	progressInfo := &task.ProgressInfo{StatusCounts: make(map[core.StatusType]int)}
	statusRows, err := r.db.Query(ctx, statusQuery, parentStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to query status counts: %w", err)
	}
	defer statusRows.Close()
	var totalChildren int
	for statusRows.Next() {
		var status string
		var count int
		if err := statusRows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan status row: %w", err)
		}
		progressInfo.StatusCounts[core.StatusType(status)] = count
		totalChildren += count
	}
	if err := statusRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating status rows: %w", err)
	}
	progressInfo.TotalChildren = totalChildren
	progressInfo.SuccessCount = progressInfo.StatusCounts[core.StatusSuccess]
	progressInfo.FailedCount = progressInfo.StatusCounts[core.StatusFailed]
	progressInfo.CanceledCount = progressInfo.StatusCounts[core.StatusCanceled]
	progressInfo.TimedOutCount = progressInfo.StatusCounts[core.StatusTimedOut]
	progressInfo.RunningCount = progressInfo.StatusCounts[core.StatusRunning] +
		progressInfo.StatusCounts[core.StatusWaiting] +
		progressInfo.StatusCounts[core.StatusPaused]
	progressInfo.PendingCount = progressInfo.StatusCounts[core.StatusPending]
	progressInfo.TerminalCount = progressInfo.SuccessCount +
		progressInfo.FailedCount +
		progressInfo.CanceledCount +
		progressInfo.TimedOutCount
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

// getTaskTreeWith executes GetTaskTree against any pgxscan.Querier (pool or tx).
func (r *TaskRepo) getTaskTreeWith(ctx context.Context, q pgxscan.Querier, rootStateID core.ID) ([]*task.State, error) {
	query := `
        WITH RECURSIVE task_tree AS (
            SELECT task_exec_id, task_id, workflow_exec_id, workflow_id, component,
                   status, execution_type, parent_state_id, agent_id, action_id, tool_id,
                   input, output, error, created_at, updated_at, 0 as depth
            FROM task_states
            WHERE task_exec_id = $1
            UNION ALL
            SELECT ts.task_exec_id, ts.task_id, ts.workflow_exec_id, ts.workflow_id, ts.component,
                   ts.status, ts.execution_type, ts.parent_state_id, ts.agent_id, ts.action_id, ts.tool_id,
                   ts.input, ts.output, ts.error, ts.created_at, ts.updated_at, tt.depth + 1
            FROM task_states ts
            INNER JOIN task_tree tt ON ts.parent_state_id = tt.task_exec_id
            WHERE tt.depth < $2
        )
        SELECT task_exec_id, task_id, workflow_exec_id, workflow_id, component,
               status, execution_type, parent_state_id, agent_id, action_id, tool_id,
               input, output, error, created_at, updated_at
        FROM task_tree
        ORDER BY depth, created_at
    `
	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, q, &statesDB, query, rootStateID, maxDepthFromConfig(ctx)); err != nil {
		return nil, fmt.Errorf("scanning task tree: %w", err)
	}
	states := make([]*task.State, 0, len(statesDB))
	for _, sdb := range statesDB {
		s, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting task tree state: %w", err)
		}
		states = append(states, s)
	}
	return states, nil
}

func (r *TaskRepo) UpsertStateWithTx(ctx context.Context, tx pgx.Tx, state *task.State) error {
	query, args, err := r.buildUpsertArgs(state)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("executing upsert: %w", err)
	}
	return nil
}
