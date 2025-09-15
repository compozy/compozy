package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
)

// TaskRepo implements task.Repository backed by SQLite (*sql.DB).
type TaskRepo struct{ db *sql.DB }

func NewTaskRepo(db *sql.DB) *TaskRepo { return &TaskRepo{db: db} }

// buildFilter applies WHERE clauses based on filter.
func buildFilter(base string, filter *task.StateFilter, args *[]any) string {
	if filter == nil {
		return base
	}
	where := ""
	add := func(cond string, v any) {
		if where == "" {
			where = " WHERE " + cond
		} else {
			where += " AND " + cond
		}
		*args = append(*args, v)
	}
	if filter.Status != nil {
		add("status = ?", *filter.Status)
	}
	if filter.WorkflowID != nil {
		add("workflow_id = ?", *filter.WorkflowID)
	}
	if filter.WorkflowExecID != nil {
		add("workflow_exec_id = ?", *filter.WorkflowExecID)
	}
	if filter.TaskID != nil {
		add("task_id = ?", *filter.TaskID)
	}
	if filter.TaskExecID != nil {
		add("task_exec_id = ?", *filter.TaskExecID)
	}
	if filter.ParentStateID != nil {
		add("parent_state_id = ?", *filter.ParentStateID)
	}
	if filter.AgentID != nil {
		add("agent_id = ?", *filter.AgentID)
	}
	if filter.ActionID != nil {
		add("action_id = ?", *filter.ActionID)
	}
	if filter.ToolID != nil {
		add("tool_id = ?", *filter.ToolID)
	}
	if filter.ExecutionType != nil {
		add("execution_type = ?", *filter.ExecutionType)
	}
	return base + where
}

func scanTaskStateDB(rows *sql.Rows) ([]*task.StateDB, error) {
	var out []*task.StateDB
	for rows.Next() {
		var sdb task.StateDB
		if err := rows.Scan(
			&sdb.Component,
			&sdb.Status,
			&sdb.TaskID,
			&sdb.TaskExecID,
			&sdb.WorkflowID,
			&sdb.WorkflowExecID,
			&sdb.ParentStateID,
			&sdb.ExecutionType,
			&sdb.AgentIDRaw,
			&sdb.ActionIDRaw,
			&sdb.ToolIDRaw,
			&sdb.InputRaw,
			&sdb.OutputRaw,
			&sdb.ErrorRaw,
			&sdb.CreatedAt,
			&sdb.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, &sdb)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

const taskSelectCols = "component,status,task_id,task_exec_id,workflow_id,workflow_exec_id," +
	"parent_state_id,execution_type,agent_id,action_id,tool_id,input,output,error,created_at,updated_at"

func (r *TaskRepo) ListStates(ctx context.Context, filter *task.StateFilter) ([]*task.State, error) {
	args := make([]any, 0, 10)
	base := "SELECT " + taskSelectCols + " FROM task_states"
	query := buildFilter(base, filter, &args)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query states: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanTaskStateDB(rows)
	if err != nil {
		return nil, fmt.Errorf("scan states: %w", err)
	}
	states := make([]*task.State, 0, len(sdbs))
	for _, sdb := range sdbs {
		st, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert state: %w", err)
		}
		states = append(states, st)
	}
	return states, nil
}

func (r *TaskRepo) buildUpsertArgs(state *task.State) (string, []any, error) {
	input, err := ToJSONText(state.Input)
	if err != nil {
		return "", nil, fmt.Errorf("marshal input: %w", err)
	}
	output, err := ToJSONText(state.Output)
	if err != nil {
		return "", nil, fmt.Errorf("marshal output: %w", err)
	}
	errJSON, err := ToJSONText(state.Error)
	if err != nil {
		return "", nil, fmt.Errorf("marshal error: %w", err)
	}
	var parentID any
	if state.ParentStateID != nil {
		parentID = state.ParentStateID.String()
	} else {
		parentID = nil
	}
	query := `INSERT INTO task_states (
        task_exec_id, task_id, workflow_exec_id, workflow_id, component, status,
        execution_type, parent_state_id, agent_id, action_id, tool_id, input, output, error
    ) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
    ON CONFLICT(task_exec_id) DO UPDATE SET
        task_id=excluded.task_id,
        workflow_exec_id=excluded.workflow_exec_id,
        workflow_id=excluded.workflow_id,
        component=excluded.component,
        status=excluded.status,
        execution_type=excluded.execution_type,
        parent_state_id=excluded.parent_state_id,
        agent_id=excluded.agent_id,
        action_id=excluded.action_id,
        tool_id=excluded.tool_id,
        input=excluded.input,
        output=excluded.output,
        error=excluded.error,
        updated_at=CURRENT_TIMESTAMP`
	args := []any{
		state.TaskExecID, state.TaskID, state.WorkflowExecID, state.WorkflowID,
		state.Component, state.Status, state.ExecutionType, parentID,
		state.AgentID, state.ActionID, state.ToolID, input, output, errJSON,
	}
	return query, args, nil
}

func (r *TaskRepo) UpsertState(ctx context.Context, state *task.State) error {
	q, args, err := r.buildUpsertArgs(state)
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("upsert: %w", err)
	}
	return nil
}

func (r *TaskRepo) GetState(ctx context.Context, taskExecID core.ID) (*task.State, error) {
	query := "SELECT " + taskSelectCols + " FROM task_states WHERE task_exec_id = ?"
	row := r.db.QueryRowContext(ctx, query, taskExecID)
	var sdb task.StateDB
	if err := row.Scan(
		&sdb.Component,
		&sdb.Status,
		&sdb.TaskID,
		&sdb.TaskExecID,
		&sdb.WorkflowID,
		&sdb.WorkflowExecID,
		&sdb.ParentStateID,
		&sdb.ExecutionType,
		&sdb.AgentIDRaw,
		&sdb.ActionIDRaw,
		&sdb.ToolIDRaw,
		&sdb.InputRaw,
		&sdb.OutputRaw,
		&sdb.ErrorRaw,
		&sdb.CreatedAt,
		&sdb.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrTaskNotFound
		}
		return nil, fmt.Errorf("scan state: %w", err)
	}
	return sdb.ToState()
}

func (r *TaskRepo) GetStateForUpdate(_ context.Context, _ core.ID) (*task.State, error) {
	return nil, fmt.Errorf("GetStateForUpdate requires transactional context")
}

// Non-transactional helpers to satisfy the interface
func (r *TaskRepo) ListTasksInWorkflow(ctx context.Context, workflowExecID core.ID) (map[string]*task.State, error) {
	query := "SELECT " + taskSelectCols + " FROM task_states WHERE workflow_exec_id = ?"
	rows, err := r.db.QueryContext(ctx, query, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanTaskStateDB(rows)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	result := make(map[string]*task.State)
	for _, sdb := range sdbs {
		st, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert: %w", err)
		}
		result[st.TaskID] = st
	}
	return result, nil
}

func (r *TaskRepo) ListTasksByStatus(
	ctx context.Context,
	workflowExecID core.ID,
	status core.StatusType,
) ([]*task.State, error) {
	query := "SELECT " + taskSelectCols + " FROM task_states WHERE workflow_exec_id = ? AND status = ?"
	rows, err := r.db.QueryContext(ctx, query, workflowExecID, status)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanTaskStateDB(rows)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	states := make([]*task.State, 0, len(sdbs))
	for _, sdb := range sdbs {
		st, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert: %w", err)
		}
		states = append(states, st)
	}
	return states, nil
}

func (r *TaskRepo) ListTasksByAgent(
	ctx context.Context,
	workflowExecID core.ID,
	agentID string,
) ([]*task.State, error) {
	query := "SELECT " + taskSelectCols + " FROM task_states WHERE workflow_exec_id = ? AND agent_id = ?"
	rows, err := r.db.QueryContext(ctx, query, workflowExecID, agentID)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanTaskStateDB(rows)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	states := make([]*task.State, 0, len(sdbs))
	for _, sdb := range sdbs {
		st, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert: %w", err)
		}
		states = append(states, st)
	}
	return states, nil
}

func (r *TaskRepo) ListTasksByTool(ctx context.Context, workflowExecID core.ID, toolID string) ([]*task.State, error) {
	query := "SELECT " + taskSelectCols + " FROM task_states WHERE workflow_exec_id = ? AND tool_id = ?"
	rows, err := r.db.QueryContext(ctx, query, workflowExecID, toolID)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanTaskStateDB(rows)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	states := make([]*task.State, 0, len(sdbs))
	for _, sdb := range sdbs {
		st, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert: %w", err)
		}
		states = append(states, st)
	}
	return states, nil
}

func (r *TaskRepo) ListChildren(ctx context.Context, parentStateID core.ID) ([]*task.State, error) {
	query := "SELECT " + taskSelectCols + " FROM task_states WHERE parent_state_id = ? ORDER BY task_id"
	rows, err := r.db.QueryContext(ctx, query, parentStateID)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanTaskStateDB(rows)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	states := make([]*task.State, 0, len(sdbs))
	for _, sdb := range sdbs {
		st, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert: %w", err)
		}
		states = append(states, st)
	}
	return states, nil
}

func (r *TaskRepo) ListChildrenOutputs(ctx context.Context, parentStateID core.ID) (map[string]*core.Output, error) {
	query := "SELECT task_id, output FROM task_states WHERE parent_state_id = ? AND output IS NOT NULL"
	rows, err := r.db.QueryContext(ctx, query, parentStateID)
	if err != nil {
		return nil, fmt.Errorf("query outputs: %w", err)
	}
	defer rows.Close()
	outputs := make(map[string]*core.Output)
	for rows.Next() {
		var taskID string
		var outputRaw []byte
		if err := rows.Scan(&taskID, &outputRaw); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		var out core.Output
		if err := json.Unmarshal(outputRaw, &out); err != nil {
			return nil, fmt.Errorf("unmarshal output: %w", err)
		}
		outputs[taskID] = &out
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return outputs, nil
}

func (r *TaskRepo) GetChildByTaskID(ctx context.Context, parentStateID core.ID, taskID string) (*task.State, error) {
	query := "SELECT " + taskSelectCols + " FROM task_states WHERE parent_state_id = ? AND task_id = ? " +
		"ORDER BY created_at DESC LIMIT 1"
	row := r.db.QueryRowContext(ctx, query, parentStateID, taskID)
	var sdb task.StateDB
	if err := row.Scan(
		&sdb.Component,
		&sdb.Status,
		&sdb.TaskID,
		&sdb.TaskExecID,
		&sdb.WorkflowID,
		&sdb.WorkflowExecID,
		&sdb.ParentStateID,
		&sdb.ExecutionType,
		&sdb.AgentIDRaw,
		&sdb.ActionIDRaw,
		&sdb.ToolIDRaw,
		&sdb.InputRaw,
		&sdb.OutputRaw,
		&sdb.ErrorRaw,
		&sdb.CreatedAt,
		&sdb.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrTaskNotFound
		}
		return nil, fmt.Errorf("scan child: %w", err)
	}
	return sdb.ToState()
}

func (r *TaskRepo) GetTaskTree(ctx context.Context, rootStateID core.ID) ([]*task.State, error) {
	query := "WITH RECURSIVE task_tree AS (" +
		"SELECT task_exec_id, task_id, workflow_exec_id, workflow_id, component, " +
		"status, execution_type, parent_state_id, agent_id, action_id, tool_id, " +
		"input, output, error, created_at, updated_at, 0 AS depth " +
		"FROM task_states WHERE task_exec_id = ? " +
		"UNION ALL " +
		"SELECT ts.task_exec_id, ts.task_id, ts.workflow_exec_id, ts.workflow_id, ts.component, " +
		"ts.status, ts.execution_type, ts.parent_state_id, ts.agent_id, ts.action_id, ts.tool_id, " +
		"ts.input, ts.output, ts.error, ts.created_at, ts.updated_at, tt.depth + 1 " +
		"FROM task_states ts JOIN task_tree tt ON ts.parent_state_id = tt.task_exec_id " +
		"WHERE tt.depth < 100) " +
		"SELECT " + taskSelectCols + " FROM task_tree ORDER BY depth, created_at"
	rows, err := r.db.QueryContext(ctx, query, rootStateID)
	if err != nil {
		return nil, fmt.Errorf("query tree: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanTaskStateDB(rows)
	if err != nil {
		return nil, fmt.Errorf("scan tree: %w", err)
	}
	states := make([]*task.State, 0, len(sdbs))
	for _, sdb := range sdbs {
		st, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert: %w", err)
		}
		states = append(states, st)
	}
	return states, nil
}

func (r *TaskRepo) GetProgressInfo(ctx context.Context, parentStateID core.ID) (*task.ProgressInfo, error) {
	query := "SELECT status, COUNT(*) FROM task_states WHERE parent_state_id = ? GROUP BY status"
	rows, err := r.db.QueryContext(ctx, query, parentStateID)
	if err != nil {
		return nil, fmt.Errorf("query status counts: %w", err)
	}
	defer rows.Close()
	pi := &task.ProgressInfo{StatusCounts: make(map[core.StatusType]int)}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		pi.StatusCounts[core.StatusType(status)] = count
		pi.TotalChildren += count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	pi.SuccessCount = pi.StatusCounts[core.StatusSuccess]
	pi.FailedCount = pi.StatusCounts[core.StatusFailed]
	pi.CanceledCount = pi.StatusCounts[core.StatusCanceled]
	pi.TimedOutCount = pi.StatusCounts[core.StatusTimedOut]
	pi.RunningCount = pi.StatusCounts[core.StatusRunning] +
		pi.StatusCounts[core.StatusWaiting] +
		pi.StatusCounts[core.StatusPaused]
	pi.PendingCount = pi.StatusCounts[core.StatusPending]
	pi.TerminalCount = pi.SuccessCount + pi.FailedCount + pi.CanceledCount + pi.TimedOutCount
	if pi.TotalChildren > 0 {
		pi.CompletionRate = float64(pi.SuccessCount) / float64(pi.TotalChildren)
		pi.FailureRate = float64(pi.FailedCount+pi.TimedOutCount) / float64(pi.TotalChildren)
	}
	return pi, nil
}

// tx repo
type taskRepoTx struct {
	parent *TaskRepo
	tx     *sql.Tx
}

func (t *taskRepoTx) ListStates(ctx context.Context, filter *task.StateFilter) ([]*task.State, error) {
	args := make([]any, 0, 10)
	base := "SELECT " + taskSelectCols + " FROM task_states"
	query := buildFilter(base, filter, &args)
	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query states: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanTaskStateDB(rows)
	if err != nil {
		return nil, fmt.Errorf("scan states: %w", err)
	}
	states := make([]*task.State, 0, len(sdbs))
	for _, sdb := range sdbs {
		st, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert state: %w", err)
		}
		states = append(states, st)
	}
	return states, nil
}

func (t *taskRepoTx) WithTransaction(_ context.Context, fn func(task.Repository) error) error {
	return fn(t)
}

func (t *taskRepoTx) UpsertState(ctx context.Context, state *task.State) error {
	q, args, err := t.parent.buildUpsertArgs(state)
	if err != nil {
		return err
	}
	if _, err := t.tx.ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("upsert: %w", err)
	}
	return nil
}

func (t *taskRepoTx) GetState(ctx context.Context, taskExecID core.ID) (*task.State, error) {
	return t.parent.GetState(ctx, taskExecID)
}

func (t *taskRepoTx) GetStateForUpdate(ctx context.Context, taskExecID core.ID) (*task.State, error) {
	// SQLite has no FOR UPDATE; transaction provides sufficient isolation in standalone.
	return t.parent.GetState(ctx, taskExecID)
}

func (t *taskRepoTx) ListTasksInWorkflow(ctx context.Context, workflowExecID core.ID) (map[string]*task.State, error) {
	query := "SELECT " + taskSelectCols + " FROM task_states WHERE workflow_exec_id = ?"
	rows, err := t.tx.QueryContext(ctx, query, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanTaskStateDB(rows)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	result := make(map[string]*task.State)
	for _, sdb := range sdbs {
		st, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert: %w", err)
		}
		result[st.TaskID] = st
	}
	return result, nil
}

func (t *taskRepoTx) ListTasksByStatus(
	ctx context.Context,
	workflowExecID core.ID,
	status core.StatusType,
) ([]*task.State, error) {
	query := "SELECT " + taskSelectCols + " FROM task_states WHERE workflow_exec_id = ? AND status = ?"
	rows, err := t.tx.QueryContext(ctx, query, workflowExecID, status)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanTaskStateDB(rows)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	states := make([]*task.State, 0, len(sdbs))
	for _, sdb := range sdbs {
		st, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert: %w", err)
		}
		states = append(states, st)
	}
	return states, nil
}

func (t *taskRepoTx) ListTasksByAgent(
	ctx context.Context,
	workflowExecID core.ID,
	agentID string,
) ([]*task.State, error) {
	query := "SELECT " + taskSelectCols + " FROM task_states WHERE workflow_exec_id = ? AND agent_id = ?"
	rows, err := t.tx.QueryContext(ctx, query, workflowExecID, agentID)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanTaskStateDB(rows)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	states := make([]*task.State, 0, len(sdbs))
	for _, sdb := range sdbs {
		st, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert: %w", err)
		}
		states = append(states, st)
	}
	return states, nil
}

func (t *taskRepoTx) ListTasksByTool(
	ctx context.Context,
	workflowExecID core.ID,
	toolID string,
) ([]*task.State, error) {
	query := "SELECT " + taskSelectCols + " FROM task_states WHERE workflow_exec_id = ? AND tool_id = ?"
	rows, err := t.tx.QueryContext(ctx, query, workflowExecID, toolID)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanTaskStateDB(rows)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	states := make([]*task.State, 0, len(sdbs))
	for _, sdb := range sdbs {
		st, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert: %w", err)
		}
		states = append(states, st)
	}
	return states, nil
}

func (t *taskRepoTx) ListChildren(ctx context.Context, parentStateID core.ID) ([]*task.State, error) {
	query := "SELECT " + taskSelectCols + " FROM task_states WHERE parent_state_id = ? ORDER BY task_id"
	rows, err := t.tx.QueryContext(ctx, query, parentStateID)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanTaskStateDB(rows)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	states := make([]*task.State, 0, len(sdbs))
	for _, sdb := range sdbs {
		st, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert: %w", err)
		}
		states = append(states, st)
	}
	return states, nil
}

func (t *taskRepoTx) ListChildrenOutputs(ctx context.Context, parentStateID core.ID) (map[string]*core.Output, error) {
	query := "SELECT task_id, output FROM task_states WHERE parent_state_id = ? AND output IS NOT NULL"
	rows, err := t.tx.QueryContext(ctx, query, parentStateID)
	if err != nil {
		return nil, fmt.Errorf("query outputs: %w", err)
	}
	defer rows.Close()
	outputs := make(map[string]*core.Output)
	for rows.Next() {
		var taskID string
		var outputRaw []byte
		if err := rows.Scan(&taskID, &outputRaw); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		var out core.Output
		if err := json.Unmarshal(outputRaw, &out); err != nil {
			return nil, fmt.Errorf("unmarshal output: %w", err)
		}
		outputs[taskID] = &out
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return outputs, nil
}

func (t *taskRepoTx) GetChildByTaskID(ctx context.Context, parentStateID core.ID, taskID string) (*task.State, error) {
	query := "SELECT " + taskSelectCols + " FROM task_states WHERE parent_state_id = ? AND task_id = ? " +
		"ORDER BY created_at DESC LIMIT 1"
	row := t.tx.QueryRowContext(ctx, query, parentStateID, taskID)
	var sdb task.StateDB
	if err := row.Scan(
		&sdb.Component,
		&sdb.Status,
		&sdb.TaskID,
		&sdb.TaskExecID,
		&sdb.WorkflowID,
		&sdb.WorkflowExecID,
		&sdb.ParentStateID,
		&sdb.ExecutionType,
		&sdb.AgentIDRaw,
		&sdb.ActionIDRaw,
		&sdb.ToolIDRaw,
		&sdb.InputRaw,
		&sdb.OutputRaw,
		&sdb.ErrorRaw,
		&sdb.CreatedAt,
		&sdb.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrTaskNotFound
		}
		return nil, fmt.Errorf("scan child: %w", err)
	}
	return sdb.ToState()
}

func (t *taskRepoTx) GetTaskTree(ctx context.Context, rootStateID core.ID) ([]*task.State, error) {
	query := "WITH RECURSIVE task_tree AS (" +
		"SELECT task_exec_id, task_id, workflow_exec_id, workflow_id, component, " +
		"status, execution_type, parent_state_id, agent_id, action_id, tool_id, " +
		"input, output, error, created_at, updated_at, 0 AS depth " +
		"FROM task_states WHERE task_exec_id = ? " +
		"UNION ALL " +
		"SELECT ts.task_exec_id, ts.task_id, ts.workflow_exec_id, ts.workflow_id, ts.component, " +
		"ts.status, ts.execution_type, ts.parent_state_id, ts.agent_id, ts.action_id, ts.tool_id, " +
		"ts.input, ts.output, ts.error, ts.created_at, ts.updated_at, tt.depth + 1 " +
		"FROM task_states ts JOIN task_tree tt ON ts.parent_state_id = tt.task_exec_id " +
		"WHERE tt.depth < 100) " +
		"SELECT " + taskSelectCols + " FROM task_tree ORDER BY depth, created_at"
	rows, err := t.tx.QueryContext(ctx, query, rootStateID)
	if err != nil {
		return nil, fmt.Errorf("query tree: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanTaskStateDB(rows)
	if err != nil {
		return nil, fmt.Errorf("scan tree: %w", err)
	}
	states := make([]*task.State, 0, len(sdbs))
	for _, sdb := range sdbs {
		st, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert: %w", err)
		}
		states = append(states, st)
	}
	return states, nil
}

func (t *taskRepoTx) GetProgressInfo(ctx context.Context, parentStateID core.ID) (*task.ProgressInfo, error) {
	query := "SELECT status, COUNT(*) FROM task_states WHERE parent_state_id = ? GROUP BY status"
	rows, err := t.tx.QueryContext(ctx, query, parentStateID)
	if err != nil {
		return nil, fmt.Errorf("query status counts: %w", err)
	}
	defer rows.Close()
	pi := &task.ProgressInfo{StatusCounts: make(map[core.StatusType]int)}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		pi.StatusCounts[core.StatusType(status)] = count
		pi.TotalChildren += count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	pi.SuccessCount = pi.StatusCounts[core.StatusSuccess]
	pi.FailedCount = pi.StatusCounts[core.StatusFailed]
	pi.CanceledCount = pi.StatusCounts[core.StatusCanceled]
	pi.TimedOutCount = pi.StatusCounts[core.StatusTimedOut]
	pi.RunningCount = pi.StatusCounts[core.StatusRunning] +
		pi.StatusCounts[core.StatusWaiting] +
		pi.StatusCounts[core.StatusPaused]
	pi.PendingCount = pi.StatusCounts[core.StatusPending]
	pi.TerminalCount = pi.SuccessCount + pi.FailedCount + pi.CanceledCount + pi.TimedOutCount
	if pi.TotalChildren > 0 {
		pi.CompletionRate = float64(pi.SuccessCount) / float64(pi.TotalChildren)
		pi.FailureRate = float64(pi.FailedCount+pi.TimedOutCount) / float64(pi.TotalChildren)
	}
	return pi, nil
}

func (r *TaskRepo) WithTransaction(ctx context.Context, fn func(task.Repository) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	log := logger.FromContext(ctx)
	var cbErr error
	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Warn("SQLite rollback after panic failed", "error", rbErr)
			}
			panic(p)
		} else if cbErr != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Warn("SQLite rollback failed", "error", rbErr)
			}
		} else {
			cbErr = tx.Commit()
		}
	}()
	repoTx := &taskRepoTx{parent: r, tx: tx}
	cbErr = fn(repoTx)
	return cbErr
}
