package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const taskStateSelectColumns = `component, status, task_exec_id, task_id, workflow_id, workflow_exec_id, execution_type, usage, agent_id, tool_id, action_id, parent_state_id, input, output, error, created_at, updated_at`

const maxTaskTreeDepth = 100

const taskStateUpsertQuery = `
INSERT INTO task_states (
    component,
    status,
    task_exec_id,
    task_id,
    workflow_exec_id,
    workflow_id,
    execution_type,
    usage,
    agent_id,
    tool_id,
    action_id,
    parent_state_id,
    input,
    output,
    error,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (task_exec_id) DO UPDATE SET
    component = excluded.component,
    status = excluded.status,
    task_id = excluded.task_id,
    workflow_exec_id = excluded.workflow_exec_id,
    workflow_id = excluded.workflow_id,
    execution_type = excluded.execution_type,
    usage = excluded.usage,
    agent_id = excluded.agent_id,
    tool_id = excluded.tool_id,
    action_id = excluded.action_id,
    parent_state_id = excluded.parent_state_id,
    input = excluded.input,
    output = excluded.output,
    error = excluded.error,
    updated_at = excluded.updated_at
`

// TaskRepo implements task.Repository backed by SQLite.
type TaskRepo struct {
	db *sql.DB
}

// NewTaskRepo creates a SQLite-backed task repository.
func NewTaskRepo(db *sql.DB) task.Repository {
	return &TaskRepo{db: db}
}

type execRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func maxDepthFromConfig(ctx context.Context) int {
	cfg := config.FromContext(ctx)
	if cfg != nil && cfg.Limits.MaxTaskContextDepth > 0 {
		return cfg.Limits.MaxTaskContextDepth
	}
	return maxTaskTreeDepth
}

// ListStates returns task states matching the supplied filter.
func (r *TaskRepo) ListStates(ctx context.Context, filter *task.StateFilter) ([]*task.State, error) {
	return r.listStatesWith(ctx, r.db, filter)
}

func (r *TaskRepo) listStatesWith(
	ctx context.Context,
	runner execRunner,
	filter *task.StateFilter,
) ([]*task.State, error) {
	query, args := buildTaskStateFilterQuery(filter)
	rows, err := runner.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite task: query states: %w", err)
	}
	defer rows.Close()

	var states []*task.State
	for rows.Next() {
		row, scanErr := scanTaskStateRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		state, convErr := row.toState()
		if convErr != nil {
			return nil, convErr
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite task: iterate states: %w", err)
	}
	return states, nil
}

func buildTaskStateFilterQuery(filter *task.StateFilter) (string, []any) {
	sb := strings.Builder{}
	sb.WriteString("SELECT ")
	sb.WriteString(taskStateSelectColumns)
	sb.WriteString(" FROM task_states WHERE 1=1")
	var args []any

	appendEq := func(column string, value any) {
		sb.WriteString(" AND ")
		sb.WriteString(column)
		sb.WriteString(" = ?")
		args = append(args, value)
	}

	if filter != nil {
		if filter.Status != nil {
			appendEq("status", string(*filter.Status))
		}
		if filter.WorkflowID != nil {
			appendEq("workflow_id", *filter.WorkflowID)
		}
		if filter.WorkflowExecID != nil {
			appendEq("workflow_exec_id", filter.WorkflowExecID.String())
		}
		if filter.TaskID != nil {
			appendEq("task_id", *filter.TaskID)
		}
		if filter.TaskExecID != nil {
			appendEq("task_exec_id", filter.TaskExecID.String())
		}
		if filter.ParentStateID != nil {
			appendEq("parent_state_id", filter.ParentStateID.String())
		}
		if filter.AgentID != nil {
			appendEq("agent_id", *filter.AgentID)
		}
		if filter.ActionID != nil {
			appendEq("action_id", *filter.ActionID)
		}
		if filter.ToolID != nil {
			appendEq("tool_id", *filter.ToolID)
		}
		if filter.ExecutionType != nil {
			appendEq("execution_type", string(*filter.ExecutionType))
		}
	}
	sb.WriteString(" ORDER BY created_at ASC")
	return sb.String(), args
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	str := strings.TrimSpace(*value)
	if str == "" {
		return nil
	}
	return str
}

func nullableID(value *core.ID) any {
	if value == nil {
		return nil
	}
	return value.String()
}

func fetchTaskStates(ctx context.Context, runner execRunner, query string, args ...any) ([]*task.State, error) {
	rows, err := runner.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite task: query states: %w", err)
	}
	defer rows.Close()

	var states []*task.State
	for rows.Next() {
		row, scanErr := scanTaskStateRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		state, convErr := row.toState()
		if convErr != nil {
			return nil, convErr
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite task: iterate states: %w", err)
	}
	return states, nil
}

func fetchSingleState(ctx context.Context, runner execRunner, query string, args ...any) (*task.State, error) {
	row := runner.QueryRowContext(ctx, query, args...)
	stateRow, err := scanTaskStateRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrTaskNotFound
		}
		return nil, err
	}
	state, convErr := stateRow.toState()
	if convErr != nil {
		return nil, convErr
	}
	return state, nil
}

// GetState fetches a task state by execution identifier.
func (r *TaskRepo) GetState(ctx context.Context, taskExecID core.ID) (*task.State, error) {
	return fetchSingleState(ctx, r.db,
		"SELECT "+taskStateSelectColumns+" FROM task_states WHERE task_exec_id = ?",
		taskExecID.String(),
	)
}

// GetUsageSummary loads only the usage summary for the provided task exec identifier.
func (r *TaskRepo) GetUsageSummary(ctx context.Context, taskExecID core.ID) (*usage.Summary, error) {
	var usageJSON sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT usage FROM task_states WHERE task_exec_id = ?",
		taskExecID.String(),
	).Scan(&usageJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrTaskNotFound
		}
		return nil, fmt.Errorf("sqlite task: scan usage: %w", err)
	}
	if !usageJSON.Valid || strings.TrimSpace(usageJSON.String) == "" {
		return nil, nil
	}
	var summary usage.Summary
	if err := json.Unmarshal([]byte(usageJSON.String), &summary); err != nil {
		return nil, fmt.Errorf("sqlite task: decode usage summary: %w", err)
	}
	if err := summary.Validate(); err != nil {
		return nil, fmt.Errorf("sqlite task: validate usage summary: %w", err)
	}
	summary.Sort()
	return summary.Clone(), nil
}

// GetStateForUpdate is only available on transactional repositories in SQLite.
func (r *TaskRepo) GetStateForUpdate(context.Context, core.ID) (*task.State, error) {
	return nil, fmt.Errorf("sqlite task: GetStateForUpdate requires transactional context")
}

// MergeUsage merges usage summaries atomically using a transaction.
func (r *TaskRepo) MergeUsage(ctx context.Context, taskExecID core.ID, summary *usage.Summary) error {
	if summary == nil || len(summary.Entries) == 0 {
		return nil
	}
	return r.WithTransaction(ctx, func(repo task.Repository) error {
		return repo.MergeUsage(ctx, taskExecID, summary)
	})
}

// WithTransaction executes fn inside a SQLite transaction and exposes a tx-scoped repository.
func (r *TaskRepo) WithTransaction(ctx context.Context, fn func(task.Repository) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite task: begin transaction: %w", err)
	}
	log := logger.FromContext(ctx)
	repoTx := &taskRepoTx{parent: r, tx: tx}
	var cbErr error
	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Error("sqlite task: rollback after panic failed", "err", rbErr)
			}
			panic(p)
		}
		if cbErr != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Error("sqlite task: rollback failed", "err", rbErr)
			}
			return
		}
		if commitErr := tx.Commit(); commitErr != nil {
			log.Error("sqlite task: commit failed", "err", commitErr)
			cbErr = fmt.Errorf("sqlite task: commit transaction: %w", commitErr)
		}
	}()
	cbErr = fn(repoTx)
	return cbErr
}

// ListTasksInWorkflow returns the most recent state for each task within a workflow execution.
func (r *TaskRepo) ListTasksInWorkflow(ctx context.Context, workflowExecID core.ID) (map[string]*task.State, error) {
	query := `SELECT ` + taskStateSelectColumns + ` FROM (
        SELECT ` + taskStateSelectColumns + `,
               ROW_NUMBER() OVER (PARTITION BY task_id ORDER BY created_at DESC, updated_at DESC) AS rn
        FROM task_states
        WHERE workflow_exec_id = ?
    ) WHERE rn = 1`
	states, err := fetchTaskStates(ctx, r.db, query, workflowExecID.String())
	if err != nil {
		return nil, err
	}
	result := make(map[string]*task.State, len(states))
	for _, state := range states {
		result[state.TaskID] = state
	}
	return result, nil
}

// ListTasksByStatus returns states in a workflow filtered by status.
func (r *TaskRepo) ListTasksByStatus(
	ctx context.Context,
	workflowExecID core.ID,
	status core.StatusType,
) ([]*task.State, error) {
	query := "SELECT " + taskStateSelectColumns +
		" FROM task_states WHERE workflow_exec_id = ? AND status = ? " +
		"ORDER BY created_at ASC"
	return fetchTaskStates(ctx, r.db, query, workflowExecID.String(), string(status))
}

// ListTasksByAgent returns states executed by a specific agent within a workflow.
func (r *TaskRepo) ListTasksByAgent(
	ctx context.Context,
	workflowExecID core.ID,
	agentID string,
) ([]*task.State, error) {
	query := "SELECT " + taskStateSelectColumns +
		" FROM task_states WHERE workflow_exec_id = ? AND agent_id = ? " +
		"ORDER BY created_at ASC"
	return fetchTaskStates(ctx, r.db, query, workflowExecID.String(), agentID)
}

// ListTasksByTool returns states executed by a specific tool within a workflow.
func (r *TaskRepo) ListTasksByTool(
	ctx context.Context,
	workflowExecID core.ID,
	toolID string,
) ([]*task.State, error) {
	query := "SELECT " + taskStateSelectColumns +
		" FROM task_states WHERE workflow_exec_id = ? AND tool_id = ? " +
		"ORDER BY created_at ASC"
	return fetchTaskStates(ctx, r.db, query, workflowExecID.String(), toolID)
}

// ListChildren returns child states for a parent task ordered by creation time.
func (r *TaskRepo) ListChildren(ctx context.Context, parentStateID core.ID) ([]*task.State, error) {
	query := "SELECT " + taskStateSelectColumns +
		" FROM task_states WHERE parent_state_id = ? " +
		"ORDER BY created_at ASC"
	return fetchTaskStates(ctx, r.db, query, parentStateID.String())
}

// ListChildrenOutputs returns decoded outputs for child tasks of the provided parent.
func (r *TaskRepo) ListChildrenOutputs(ctx context.Context, parentStateID core.ID) (map[string]*core.Output, error) {
	return listChildrenOutputsWith(ctx, r.db, parentStateID)
}

// GetChildByTaskID returns the latest child state for the given parent and task ID.
func (r *TaskRepo) GetChildByTaskID(ctx context.Context, parentStateID core.ID, taskID string) (*task.State, error) {
	query := "SELECT " + taskStateSelectColumns +
		" FROM task_states WHERE parent_state_id = ? AND task_id = ? " +
		"ORDER BY created_at DESC LIMIT 1"
	return fetchSingleState(ctx, r.db, query, parentStateID.String(), taskID)
}

// GetTaskTree retrieves a task state and its descendants up to the configured depth.
func (r *TaskRepo) GetTaskTree(ctx context.Context, rootStateID core.ID) ([]*task.State, error) {
	return r.getTaskTreeWith(ctx, r.db, rootStateID)
}

func (r *TaskRepo) getTaskTreeWith(ctx context.Context, runner execRunner, rootStateID core.ID) ([]*task.State, error) {
	depth := maxDepthFromConfig(ctx)
	query := `WITH RECURSIVE task_tree AS (
        SELECT ` + taskStateSelectColumns + `, 0 AS depth
        FROM task_states
        WHERE task_exec_id = ?
        UNION ALL
        SELECT ` + taskStateSelectColumns + `, tt.depth + 1
        FROM task_states ts
        JOIN task_tree tt ON ts.parent_state_id = tt.task_exec_id
        WHERE tt.depth < ?
    )
    SELECT ` + taskStateSelectColumns + ` FROM task_tree ORDER BY depth, created_at`
	return fetchTaskStates(ctx, runner, query, rootStateID.String(), depth)
}

// ListByIDs retrieves states matching the provided execution identifiers.
func (r *TaskRepo) ListByIDs(ctx context.Context, ids []core.ID) ([]*task.State, error) {
	if len(ids) == 0 {
		return []*task.State{}, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id.String()
	}
	query := `SELECT ` + taskStateSelectColumns + ` FROM task_states WHERE task_exec_id IN (` + strings.Join(
		placeholders,
		",",
	) + `) ORDER BY created_at ASC`
	return fetchTaskStates(ctx, r.db, query, args...)
}

func buildUpsertArgs(state *task.State, now time.Time) ([]any, error) {
	usageJSON, err := marshalJSON(state.Usage)
	if err != nil {
		return nil, fmt.Errorf("sqlite task: marshal usage: %w", err)
	}
	inputJSON, err := marshalJSON(state.Input)
	if err != nil {
		return nil, fmt.Errorf("sqlite task: marshal input: %w", err)
	}
	outputJSON, err := marshalJSON(state.Output)
	if err != nil {
		return nil, fmt.Errorf("sqlite task: marshal output: %w", err)
	}
	errorJSON, err := marshalJSON(state.Error)
	if err != nil {
		return nil, fmt.Errorf("sqlite task: marshal error: %w", err)
	}
	createdAt := state.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	return []any{
		string(state.Component),
		string(state.Status),
		state.TaskExecID.String(),
		state.TaskID,
		state.WorkflowExecID.String(),
		state.WorkflowID,
		string(state.ExecutionType),
		jsonArg(usageJSON),
		nullableString(state.AgentID),
		nullableString(state.ToolID),
		nullableString(state.ActionID),
		nullableID(state.ParentStateID),
		jsonArg(inputJSON),
		jsonArg(outputJSON),
		jsonArg(errorJSON),
		createdAt.Format(time.RFC3339Nano),
		now.Format(time.RFC3339Nano),
	}, nil
}

func (r *TaskRepo) upsertStateWithTx(ctx context.Context, tx *sql.Tx, state *task.State) error {
	now := time.Now().UTC()
	args, err := buildUpsertArgs(state, now)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, taskStateUpsertQuery, args...); err != nil {
		if isForeignKeyConstraint(err) {
			return fmt.Errorf("sqlite task: tx upsert state foreign key: %w", err)
		}
		return fmt.Errorf("sqlite task: tx upsert state: %w", err)
	}
	return nil
}

func listChildrenOutputsWith(
	ctx context.Context,
	runner execRunner,
	parentStateID core.ID,
) (map[string]*core.Output, error) {
	rows, err := runner.QueryContext(ctx,
		`SELECT task_id, output FROM task_states WHERE parent_state_id = ? AND output IS NOT NULL ORDER BY task_id`,
		parentStateID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite task: query child outputs: %w", err)
	}
	defer rows.Close()

	outputs := make(map[string]*core.Output)
	for rows.Next() {
		var taskID string
		var raw sql.NullString
		if err := rows.Scan(&taskID, &raw); err != nil {
			return nil, fmt.Errorf("sqlite task: scan child output: %w", err)
		}
		if !raw.Valid {
			continue
		}
		var out core.Output
		if err := json.Unmarshal([]byte(raw.String), &out); err != nil {
			return nil, fmt.Errorf("sqlite task: decode child output: %w", err)
		}
		cloned := out
		outputs[taskID] = &cloned
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite task: iterate child outputs: %w", err)
	}
	return outputs, nil
}

func getProgressInfoWith(ctx context.Context, runner execRunner, parentStateID core.ID) (*task.ProgressInfo, error) {
	rows, err := runner.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM task_states WHERE parent_state_id = ? GROUP BY status`,
		parentStateID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite task: query progress info: %w", err)
	}
	defer rows.Close()

	info := &task.ProgressInfo{StatusCounts: make(map[core.StatusType]int)}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("sqlite task: scan progress row: %w", err)
		}
		info.StatusCounts[core.StatusType(status)] = count
		info.TotalChildren += count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite task: iterate progress rows: %w", err)
	}
	info.SuccessCount = info.StatusCounts[core.StatusSuccess]
	info.FailedCount = info.StatusCounts[core.StatusFailed]
	info.CanceledCount = info.StatusCounts[core.StatusCanceled]
	info.TimedOutCount = info.StatusCounts[core.StatusTimedOut]
	info.RunningCount = info.StatusCounts[core.StatusRunning] +
		info.StatusCounts[core.StatusWaiting] +
		info.StatusCounts[core.StatusPaused]
	info.PendingCount = info.StatusCounts[core.StatusPending]
	info.TerminalCount = info.SuccessCount + info.FailedCount + info.CanceledCount + info.TimedOutCount
	if info.TotalChildren > 0 {
		total := float64(info.TotalChildren)
		info.CompletionRate = float64(info.SuccessCount) / total
		info.FailureRate = float64(info.FailedCount+info.TimedOutCount) / total
	}
	return info, nil
}

type taskRepoTx struct {
	parent *TaskRepo
	tx     *sql.Tx
}

func (t *taskRepoTx) ListStates(ctx context.Context, filter *task.StateFilter) ([]*task.State, error) {
	return t.parent.listStatesWith(ctx, t.tx, filter)
}

func (t *taskRepoTx) UpsertState(ctx context.Context, state *task.State) error {
	return t.parent.upsertStateWithTx(ctx, t.tx, state)
}

func (t *taskRepoTx) GetState(ctx context.Context, taskExecID core.ID) (*task.State, error) {
	return fetchSingleState(ctx, t.tx,
		"SELECT "+taskStateSelectColumns+" FROM task_states WHERE task_exec_id = ?",
		taskExecID.String(),
	)
}

func (t *taskRepoTx) GetUsageSummary(ctx context.Context, taskExecID core.ID) (*usage.Summary, error) {
	var usageJSON sql.NullString
	err := t.tx.QueryRowContext(ctx,
		"SELECT usage FROM task_states WHERE task_exec_id = ?",
		taskExecID.String(),
	).Scan(&usageJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrTaskNotFound
		}
		return nil, fmt.Errorf("sqlite task: tx scan usage: %w", err)
	}
	if !usageJSON.Valid || strings.TrimSpace(usageJSON.String) == "" {
		return nil, nil
	}
	var summary usage.Summary
	if err := json.Unmarshal([]byte(usageJSON.String), &summary); err != nil {
		return nil, fmt.Errorf("sqlite task: tx decode usage summary: %w", err)
	}
	if err := summary.Validate(); err != nil {
		return nil, fmt.Errorf("sqlite task: tx validate usage summary: %w", err)
	}
	summary.Sort()
	return summary.Clone(), nil
}

func (t *taskRepoTx) WithTransaction(_ context.Context, fn func(task.Repository) error) error {
	return fn(t)
}

func (t *taskRepoTx) GetStateForUpdate(ctx context.Context, taskExecID core.ID) (*task.State, error) {
	return t.GetState(ctx, taskExecID)
}

func (t *taskRepoTx) MergeUsage(ctx context.Context, taskExecID core.ID, summary *usage.Summary) error {
	if summary == nil || len(summary.Entries) == 0 {
		return nil
	}
	state, err := t.GetState(ctx, taskExecID)
	if err != nil {
		return err
	}
	var base *usage.Summary
	if state.Usage != nil {
		base = state.Usage.Clone()
	} else {
		base = &usage.Summary{}
	}
	delta := summary.Clone()
	if delta == nil {
		delta = &usage.Summary{}
	}
	delta.Sort()
	base.MergeAll(delta)
	base.Sort()
	updated := *state
	updated.Usage = base
	return t.parent.upsertStateWithTx(ctx, t.tx, &updated)
}

func (t *taskRepoTx) ListTasksInWorkflow(ctx context.Context, workflowExecID core.ID) (map[string]*task.State, error) {
	query := `SELECT ` + taskStateSelectColumns + ` FROM (
        SELECT ` + taskStateSelectColumns + `,
               ROW_NUMBER() OVER (PARTITION BY task_id ORDER BY created_at DESC, updated_at DESC) AS rn
        FROM task_states
        WHERE workflow_exec_id = ?
    ) WHERE rn = 1`
	states, err := fetchTaskStates(ctx, t.tx, query, workflowExecID.String())
	if err != nil {
		return nil, err
	}
	result := make(map[string]*task.State, len(states))
	for _, state := range states {
		result[state.TaskID] = state
	}
	return result, nil
}

func (t *taskRepoTx) ListTasksByStatus(
	ctx context.Context,
	workflowExecID core.ID,
	status core.StatusType,
) ([]*task.State, error) {
	query := "SELECT " + taskStateSelectColumns +
		" FROM task_states WHERE workflow_exec_id = ? AND status = ? " +
		"ORDER BY created_at ASC"
	return fetchTaskStates(ctx, t.tx, query, workflowExecID.String(), string(status))
}

func (t *taskRepoTx) ListTasksByAgent(
	ctx context.Context,
	workflowExecID core.ID,
	agentID string,
) ([]*task.State, error) {
	query := "SELECT " + taskStateSelectColumns +
		" FROM task_states WHERE workflow_exec_id = ? AND agent_id = ? " +
		"ORDER BY created_at ASC"
	return fetchTaskStates(ctx, t.tx, query, workflowExecID.String(), agentID)
}

func (t *taskRepoTx) ListTasksByTool(
	ctx context.Context,
	workflowExecID core.ID,
	toolID string,
) ([]*task.State, error) {
	query := "SELECT " + taskStateSelectColumns +
		" FROM task_states WHERE workflow_exec_id = ? AND tool_id = ? " +
		"ORDER BY created_at ASC"
	return fetchTaskStates(ctx, t.tx, query, workflowExecID.String(), toolID)
}

func (t *taskRepoTx) ListChildren(ctx context.Context, parentStateID core.ID) ([]*task.State, error) {
	query := "SELECT " + taskStateSelectColumns +
		" FROM task_states WHERE parent_state_id = ? " +
		"ORDER BY created_at ASC"
	return fetchTaskStates(ctx, t.tx, query, parentStateID.String())
}

func (t *taskRepoTx) ListChildrenOutputs(ctx context.Context, parentStateID core.ID) (map[string]*core.Output, error) {
	return listChildrenOutputsWith(ctx, t.tx, parentStateID)
}

func (t *taskRepoTx) GetChildByTaskID(ctx context.Context, parentStateID core.ID, taskID string) (*task.State, error) {
	query := "SELECT " + taskStateSelectColumns +
		" FROM task_states WHERE parent_state_id = ? AND task_id = ? " +
		"ORDER BY created_at DESC LIMIT 1"
	return fetchSingleState(ctx, t.tx, query, parentStateID.String(), taskID)
}

func (t *taskRepoTx) GetTaskTree(ctx context.Context, rootStateID core.ID) ([]*task.State, error) {
	return t.parent.getTaskTreeWith(ctx, t.tx, rootStateID)
}

func (t *taskRepoTx) GetProgressInfo(ctx context.Context, parentStateID core.ID) (*task.ProgressInfo, error) {
	return getProgressInfoWith(ctx, t.tx, parentStateID)
}

func (t *taskRepoTx) ListByIDs(ctx context.Context, ids []core.ID) ([]*task.State, error) {
	if len(ids) == 0 {
		return []*task.State{}, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id.String()
	}
	query := `SELECT ` + taskStateSelectColumns + ` FROM task_states WHERE task_exec_id IN (` + strings.Join(
		placeholders,
		",",
	) + `) ORDER BY created_at ASC`
	return fetchTaskStates(ctx, t.tx, query, args...)
}

var _ task.Repository = (*TaskRepo)(nil)
var _ task.Repository = (*taskRepoTx)(nil)

// GetProgressInfo aggregates child statuses into a progress summary for the parent.
func (r *TaskRepo) GetProgressInfo(ctx context.Context, parentStateID core.ID) (*task.ProgressInfo, error) {
	return getProgressInfoWith(ctx, r.db, parentStateID)
}

// UpsertState inserts or updates a task state record.
func (r *TaskRepo) UpsertState(ctx context.Context, state *task.State) error {
	if state == nil {
		return fmt.Errorf("sqlite task: nil state provided")
	}
	now := time.Now().UTC()
	args, err := buildUpsertArgs(state, now)
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, taskStateUpsertQuery, args...); err != nil {
		if isForeignKeyConstraint(err) {
			return fmt.Errorf("sqlite task: upsert state foreign key: %w", err)
		}
		return fmt.Errorf("sqlite task: upsert state: %w", err)
	}
	return nil
}
