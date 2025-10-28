package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	sqliteLib "modernc.org/sqlite/lib"
)

const (
	selectWorkflowStateByExecID = `SELECT workflow_exec_id, workflow_id, status, usage, input, output, error, created_at, updated_at FROM workflow_states WHERE workflow_exec_id = ?`
	selectWorkflowStateByID     = `SELECT workflow_exec_id, workflow_id, status, usage, input, output, error, created_at, updated_at FROM workflow_states WHERE workflow_id = ? LIMIT 1`
	selectWorkflowByJoinBase    = `SELECT w.workflow_exec_id, w.workflow_id, w.status, w.usage, w.input, w.output, w.error, w.created_at, w.updated_at FROM workflow_states w JOIN task_states t ON w.workflow_exec_id = t.workflow_exec_id`
)

const taskHierarchyQuery = `
WITH RECURSIVE task_hierarchy AS (
    SELECT *
    FROM task_states
    WHERE workflow_exec_id = ? AND parent_state_id IS NULL

    UNION ALL

    SELECT ts.*
    FROM task_states ts
    INNER JOIN task_hierarchy th ON ts.parent_state_id = th.task_exec_id
    WHERE ts.workflow_exec_id = ?
)
SELECT * FROM task_hierarchy
`

// WorkflowRepo implements workflow.Repository using SQLite as backend.
type WorkflowRepo struct{ db *sql.DB }

// NewWorkflowRepo creates a SQLite-backed workflow repository.
func NewWorkflowRepo(db *sql.DB) workflow.Repository {
	return &WorkflowRepo{db: db}
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type workflowStateRow struct {
	workflowExecID string
	workflowID     string
	status         string
	usageJSON      sql.NullString
	inputJSON      sql.NullString
	outputJSON     sql.NullString
	errorJSON      sql.NullString
	createdAt      string
	updatedAt      string
}

func (r *WorkflowRepo) ListStates(ctx context.Context, filter *workflow.StateFilter) ([]*workflow.State, error) {
	query, args := buildListStatesQuery(filter)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite workflow: list states: %w", err)
	}
	defer rows.Close()

	var (
		states  []*workflow.State
		execIDs []core.ID
	)
	for rows.Next() {
		row, scanErr := scanWorkflowState(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		state, convErr := row.toState()
		if convErr != nil {
			return nil, convErr
		}
		states = append(states, state)
		execIDs = append(execIDs, state.WorkflowExecID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite workflow: iterate states: %w", err)
	}
	if len(states) == 0 {
		return []*workflow.State{}, nil
	}
	tasksByExec, err := r.fetchTaskStatesForExec(ctx, r.db, execIDs)
	if err != nil {
		return nil, err
	}
	for _, state := range states {
		if tasks := tasksByExec[state.WorkflowExecID.String()]; tasks != nil {
			state.Tasks = tasks
			continue
		}
		state.Tasks = make(map[string]*task.State)
	}
	return states, nil
}

func buildListStatesQuery(filter *workflow.StateFilter) (string, []any) {
	sb := strings.Builder{}
	sb.WriteString(
		`SELECT workflow_exec_id, workflow_id, status, usage, input, output, error, created_at, updated_at FROM workflow_states WHERE 1=1`,
	)
	var args []any
	if filter != nil {
		if filter.Status != nil {
			sb.WriteString(` AND status = ?`)
			args = append(args, string(*filter.Status))
		}
		if filter.WorkflowID != nil {
			sb.WriteString(` AND workflow_id = ?`)
			args = append(args, *filter.WorkflowID)
		}
		if filter.WorkflowExecID != nil {
			sb.WriteString(` AND workflow_exec_id = ?`)
			args = append(args, filter.WorkflowExecID.String())
		}
	}
	sb.WriteString(` ORDER BY created_at DESC`)
	return sb.String(), args
}

func (r *WorkflowRepo) UpsertState(ctx context.Context, state *workflow.State) error {
	if state == nil {
		return fmt.Errorf("sqlite workflow: nil state provided")
	}
	usageJSON, inputJSON, outputJSON, errorJSON, err := marshalStateJSONFields(state)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	const query = `
	        INSERT INTO workflow_states (
	            workflow_exec_id,
	            workflow_id,
	            status,
	            usage,
	            input,
	            output,
	            error,
	            created_at,
	            updated_at
	        )
	        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	        ON CONFLICT (workflow_exec_id) DO UPDATE SET
	            workflow_id = excluded.workflow_id,
	            status = excluded.status,
	            usage = excluded.usage,
	            input = excluded.input,
	            output = excluded.output,
	            error = excluded.error,
	            updated_at = excluded.updated_at
	    `
	_, err = r.db.ExecContext(
		ctx,
		query,
		state.WorkflowExecID.String(),
		state.WorkflowID,
		string(state.Status),
		jsonArg(usageJSON),
		jsonArg(inputJSON),
		jsonArg(outputJSON),
		jsonArg(errorJSON),
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("sqlite workflow: upsert state: %w", err)
	}
	return nil
}

func (r *WorkflowRepo) UpdateStatus(ctx context.Context, workflowExecID core.ID, status core.StatusType) error {
	const query = `UPDATE workflow_states SET status = ?, updated_at = ? WHERE workflow_exec_id = ?`
	result, err := r.db.ExecContext(
		ctx,
		query,
		string(status),
		time.Now().UTC().Format(time.RFC3339Nano),
		workflowExecID.String(),
	)
	if err != nil {
		return fmt.Errorf("sqlite workflow: update status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite workflow: status rows affected: %w", err)
	}
	if rows == 0 {
		return store.ErrWorkflowNotFound
	}
	return nil
}

func (r *WorkflowRepo) GetState(ctx context.Context, workflowExecID core.ID) (*workflow.State, error) {
	var state *workflow.State
	err := r.withTransaction(ctx, func(tx *sql.Tx) error {
		row, getErr := r.fetchStateRow(ctx, tx, selectWorkflowStateByExecID, workflowExecID.String())
		if getErr != nil {
			return getErr
		}
		converted, convErr := row.toState()
		if convErr != nil {
			return convErr
		}
		tasks, tasksErr := r.fetchTaskStatesForExec(ctx, tx, []core.ID{workflowExecID})
		if tasksErr != nil {
			return tasksErr
		}
		converted.Tasks = tasks[workflowExecID.String()]
		if converted.Tasks == nil {
			converted.Tasks = make(map[string]*task.State)
		}
		state = converted
		return nil
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (r *WorkflowRepo) GetStateByID(ctx context.Context, workflowID string) (*workflow.State, error) {
	var state *workflow.State
	err := r.withTransaction(ctx, func(tx *sql.Tx) error {
		row, getErr := r.fetchStateRow(ctx, tx, selectWorkflowStateByID, workflowID)
		if getErr != nil {
			return getErr
		}
		converted, convErr := row.toState()
		if convErr != nil {
			return convErr
		}
		tasks, tasksErr := r.fetchTaskStatesForExec(ctx, tx, []core.ID{converted.WorkflowExecID})
		if tasksErr != nil {
			return tasksErr
		}
		converted.Tasks = tasks[converted.WorkflowExecID.String()]
		if converted.Tasks == nil {
			converted.Tasks = make(map[string]*task.State)
		}
		state = converted
		return nil
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (r *WorkflowRepo) GetStateByTaskID(ctx context.Context, workflowID, taskID string) (*workflow.State, error) {
	query := selectWorkflowByJoinBase + ` WHERE w.workflow_id = ? AND t.task_id = ? LIMIT 1`
	return r.getStateWithJoin(ctx, query, workflowID, taskID)
}

func (r *WorkflowRepo) GetStateByAgentID(ctx context.Context, workflowID, agentID string) (*workflow.State, error) {
	query := selectWorkflowByJoinBase + ` WHERE w.workflow_id = ? AND t.agent_id = ? LIMIT 1`
	return r.getStateWithJoin(ctx, query, workflowID, agentID)
}

func (r *WorkflowRepo) GetStateByToolID(ctx context.Context, workflowID, toolID string) (*workflow.State, error) {
	query := selectWorkflowByJoinBase + ` WHERE w.workflow_id = ? AND t.tool_id = ? LIMIT 1`
	return r.getStateWithJoin(ctx, query, workflowID, toolID)
}

func (r *WorkflowRepo) getStateWithJoin(ctx context.Context, query string, args ...any) (*workflow.State, error) {
	var state *workflow.State
	err := r.withTransaction(ctx, func(tx *sql.Tx) error {
		row, getErr := r.fetchStateRow(ctx, tx, query, args...)
		if getErr != nil {
			return getErr
		}
		converted, convErr := row.toState()
		if convErr != nil {
			return convErr
		}
		tasks, tasksErr := r.fetchTaskStatesForExec(ctx, tx, []core.ID{converted.WorkflowExecID})
		if tasksErr != nil {
			return tasksErr
		}
		converted.Tasks = tasks[converted.WorkflowExecID.String()]
		if converted.Tasks == nil {
			converted.Tasks = make(map[string]*task.State)
		}
		state = converted
		return nil
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (r *WorkflowRepo) CompleteWorkflow(
	ctx context.Context,
	workflowExecID core.ID,
	outputTransformer workflow.OutputTransformer,
) (*workflow.State, error) {
	var result *workflow.State
	err := r.withTransaction(ctx, func(tx *sql.Tx) error {
		status, lockErr := r.lockWorkflowStatus(ctx, tx, workflowExecID)
		if lockErr != nil {
			return lockErr
		}
		if status == string(core.StatusSuccess) || status == string(core.StatusFailed) {
			return nil
		}
		state, procErr := r.processWorkflowCompletion(ctx, tx, workflowExecID, outputTransformer)
		if procErr != nil {
			return procErr
		}
		result = state
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return r.GetState(ctx, workflowExecID)
	}
	return result, nil
}

func (r *WorkflowRepo) MergeUsage(ctx context.Context, workflowExecID core.ID, summary *usage.Summary) error {
	if summary == nil || len(summary.Entries) == 0 {
		return nil
	}
	return r.withTransaction(ctx, func(tx *sql.Tx) error {
		row, err := r.fetchStateRow(ctx, tx, selectWorkflowStateByExecID, workflowExecID.String())
		if err != nil {
			return err
		}
		state, convErr := row.toState()
		if convErr != nil {
			return convErr
		}
		base := &usage.Summary{}
		if state.Usage != nil {
			base = state.Usage.Clone()
		}
		delta := summary.Clone()
		if delta == nil {
			delta = &usage.Summary{}
		}
		delta.Sort()
		base.MergeAll(delta)
		base.Sort()
		usageJSON, marshalErr := marshalJSON(base)
		if marshalErr != nil {
			return fmt.Errorf("sqlite workflow: marshal merged usage: %w", marshalErr)
		}
		const query = `UPDATE workflow_states SET usage = ?, updated_at = ? WHERE workflow_exec_id = ?`
		_, execErr := tx.ExecContext(
			ctx,
			query,
			jsonArg(usageJSON),
			time.Now().UTC().Format(time.RFC3339Nano),
			workflowExecID.String(),
		)
		if execErr != nil {
			return fmt.Errorf("sqlite workflow: update usage: %w", execErr)
		}
		return nil
	})
}

func (r *WorkflowRepo) fetchStateRow(
	ctx context.Context,
	exec sqlExecutor,
	query string,
	args ...any,
) (*workflowStateRow, error) {
	row := exec.QueryRowContext(ctx, query, args...)
	var result workflowStateRow
	if err := row.Scan(
		&result.workflowExecID,
		&result.workflowID,
		&result.status,
		&result.usageJSON,
		&result.inputJSON,
		&result.outputJSON,
		&result.errorJSON,
		&result.createdAt,
		&result.updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("sqlite workflow: scan workflow state: %w", err)
	}
	return &result, nil
}

func (r *WorkflowRepo) withTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	const maxAttempts = 50
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			if isBusyError(err) {
				lastErr = err
				time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
				continue
			}
			return fmt.Errorf("sqlite workflow: begin tx: %w", err)
		}
		if err := r.runTransaction(ctx, tx, fn); err != nil {
			if isBusyError(err) {
				lastErr = err
				time.Sleep(time.Duration(attempt+1) * 25 * time.Millisecond)
				continue
			}
			return err
		}
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("sqlite workflow: transaction retries exhausted: %w", lastErr)
	}
	return fmt.Errorf("sqlite workflow: transaction retries exhausted")
}

func (r *WorkflowRepo) runTransaction(ctx context.Context, tx *sql.Tx, fn func(*sql.Tx) error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
				logger.FromContext(ctx).Warn("sqlite workflow: rollback after panic failed", "error", rbErr)
			}
			panic(p)
		}
	}()
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			logger.FromContext(ctx).Warn("sqlite workflow: rollback failed", "error", rbErr)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			logger.FromContext(ctx).Warn("sqlite workflow: rollback after commit failure", "error", rbErr)
		}
		return err
	}
	return nil
}

func isBusyError(err error) bool {
	if err == nil {
		return false
	}
	code, ok := sqliteErrorCode(err)
	if !ok {
		return strings.Contains(strings.ToLower(err.Error()), "database is locked")
	}
	return code == sqliteLib.SQLITE_BUSY || code == sqliteLib.SQLITE_LOCKED
}

func (r *WorkflowRepo) fetchTaskStatesForExec(
	ctx context.Context,
	exec sqlExecutor,
	execIDs []core.ID,
) (map[string]map[string]*task.State, error) {
	if len(execIDs) == 0 {
		return map[string]map[string]*task.State{}, nil
	}
	placeholders := make([]string, len(execIDs))
	args := make([]any, len(execIDs))
	for i, id := range execIDs {
		placeholders[i] = "?"
		args[i] = id.String()
	}
	query := fmt.Sprintf(
		`SELECT component, status, task_exec_id, task_id, workflow_id, workflow_exec_id, execution_type, usage, agent_id, tool_id, action_id, parent_state_id, input, output, error, created_at, updated_at FROM task_states WHERE workflow_exec_id IN (%s)`,
		strings.Join(placeholders, ","),
	)
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite workflow: list task states: %w", err)
	}
	defer rows.Close()

	result := make(map[string]map[string]*task.State)
	for rows.Next() {
		row, scanErr := scanTaskStateRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		state, convErr := row.toState()
		if convErr != nil {
			return nil, convErr
		}
		key := row.workflowExecID
		if result[key] == nil {
			result[key] = make(map[string]*task.State)
		}
		result[key][state.TaskID] = state
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite workflow: iterate task states: %w", err)
	}
	for _, id := range execIDs {
		key := id.String()
		if result[key] == nil {
			result[key] = make(map[string]*task.State)
		}
	}
	return result, nil
}

func (r *WorkflowRepo) lockWorkflowStatus(ctx context.Context, tx *sql.Tx, workflowExecID core.ID) (string, error) {
	// SQLite acquires a RESERVED lock once a write occurs inside the transaction.
	// We perform a dummy update to ensure we hold the lock without changing data.
	const touchQuery = `UPDATE workflow_states SET updated_at = updated_at WHERE workflow_exec_id = ?`
	if _, err := tx.ExecContext(ctx, touchQuery, workflowExecID.String()); err != nil {
		return "", fmt.Errorf("sqlite workflow: lock workflow state: %w", err)
	}
	row, err := r.fetchStateRow(ctx, tx, selectWorkflowStateByExecID, workflowExecID.String())
	if err != nil {
		return "", err
	}
	return row.status, nil
}

func (r *WorkflowRepo) processWorkflowCompletion(
	ctx context.Context,
	tx *sql.Tx,
	workflowExecID core.ID,
	outputTransformer workflow.OutputTransformer,
) (*workflow.State, error) {
	tasks, err := r.loadWorkflowTasks(ctx, tx, workflowExecID)
	if err != nil {
		return nil, err
	}
	finalStatus := determineFinalWorkflowStatus(tasks)
	if finalStatus == core.StatusRunning {
		return nil, store.ErrWorkflowNotReady
	}
	if err := r.applyWorkflowOutput(ctx, tx, workflowExecID, tasks, outputTransformer, &finalStatus); err != nil {
		return nil, err
	}
	return r.reloadWorkflowState(ctx, tx, workflowExecID)
}

func (r *WorkflowRepo) loadWorkflowTasks(
	ctx context.Context,
	tx *sql.Tx,
	workflowExecID core.ID,
) (map[string]*task.State, error) {
	rows, err := tx.QueryContext(ctx, taskHierarchyQuery, workflowExecID.String(), workflowExecID.String())
	if err != nil {
		return nil, fmt.Errorf("sqlite workflow: load workflow tasks: %w", err)
	}
	defer rows.Close()

	tasks := make(map[string]*task.State)
	for rows.Next() {
		row, scanErr := scanTaskStateRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		state, convErr := row.toState()
		if convErr != nil {
			return nil, convErr
		}
		tasks[state.TaskID] = state
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite workflow: iterate workflow tasks: %w", err)
	}
	return tasks, nil
}

func (r *WorkflowRepo) applyWorkflowOutput(
	ctx context.Context,
	tx *sql.Tx,
	workflowExecID core.ID,
	tasks map[string]*task.State,
	outputTransformer workflow.OutputTransformer,
	finalStatus *core.StatusType,
) error {
	output, transformErr := r.resolveWorkflowOutput(ctx, tx, workflowExecID, tasks, outputTransformer, finalStatus)
	outputMap, convErr := convertOutputToMap(output)
	if convErr != nil {
		return convErr
	}
	return r.updateWorkflowOutput(ctx, tx, workflowExecID, outputMap, *finalStatus, transformErr)
}

func (r *WorkflowRepo) resolveWorkflowOutput(
	ctx context.Context,
	tx *sql.Tx,
	workflowExecID core.ID,
	tasks map[string]*task.State,
	outputTransformer workflow.OutputTransformer,
	finalStatus *core.StatusType,
) (any, error) {
	if outputTransformer == nil {
		return createWorkflowOutputMap(tasks), nil
	}
	row, err := r.fetchStateRow(ctx, tx, selectWorkflowStateByExecID, workflowExecID.String())
	if err != nil {
		return nil, fmt.Errorf("sqlite workflow: load workflow for transformer: %w", err)
	}
	state, convErr := row.toState()
	if convErr != nil {
		return nil, convErr
	}
	state.Tasks = tasks
	transformed, transformErr := outputTransformer(state)
	if transformErr != nil {
		if finalStatus != nil {
			*finalStatus = core.StatusFailed
		}
		return createWorkflowOutputMap(tasks), transformErr
	}
	return transformed, nil
}

func (r *WorkflowRepo) updateWorkflowOutput(
	ctx context.Context,
	tx *sql.Tx,
	workflowExecID core.ID,
	output map[string]any,
	finalStatus core.StatusType,
	transformErr error,
) error {
	outputJSON, err := marshalJSON(output)
	if err != nil {
		return fmt.Errorf("sqlite workflow: marshal workflow output: %w", err)
	}
	var errorJSON sql.NullString
	if transformErr != nil {
		errorJSON, err = marshalJSON(core.NewError(transformErr, "OUTPUT_TRANSFORMATION_FAILED", nil))
		if err != nil {
			return fmt.Errorf("sqlite workflow: marshal workflow error: %w", err)
		}
	}
	const query = `UPDATE workflow_states SET output = ?, status = ?, error = ?, updated_at = ? WHERE workflow_exec_id = ?`
	result, execErr := tx.ExecContext(
		ctx,
		query,
		jsonArg(outputJSON),
		string(finalStatus),
		jsonArg(errorJSON),
		time.Now().UTC().Format(time.RFC3339Nano),
		workflowExecID.String(),
	)
	if execErr != nil {
		return fmt.Errorf("sqlite workflow: update workflow output: %w", execErr)
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("sqlite workflow: output rows affected: %w", rowsErr)
	}
	if rows == 0 {
		return store.ErrWorkflowNotFound
	}
	return nil
}

func (r *WorkflowRepo) reloadWorkflowState(
	ctx context.Context,
	tx *sql.Tx,
	workflowExecID core.ID,
) (*workflow.State, error) {
	row, err := r.fetchStateRow(ctx, tx, selectWorkflowStateByExecID, workflowExecID.String())
	if err != nil {
		return nil, err
	}
	state, convErr := row.toState()
	if convErr != nil {
		return nil, convErr
	}
	tasks, tasksErr := r.loadWorkflowTasks(ctx, tx, workflowExecID)
	if tasksErr != nil {
		return nil, tasksErr
	}
	state.Tasks = tasks
	return state, nil
}

func scanWorkflowState(rows *sql.Rows) (*workflowStateRow, error) {
	var row workflowStateRow
	if err := rows.Scan(
		&row.workflowExecID,
		&row.workflowID,
		&row.status,
		&row.usageJSON,
		&row.inputJSON,
		&row.outputJSON,
		&row.errorJSON,
		&row.createdAt,
		&row.updatedAt,
	); err != nil {
		return nil, fmt.Errorf("sqlite workflow: scan state row: %w", err)
	}
	return &row, nil
}

func (r *workflowStateRow) toState() (*workflow.State, error) {
	state := &workflow.State{
		WorkflowID:     r.workflowID,
		WorkflowExecID: core.ID(r.workflowExecID),
		Status:         core.StatusType(r.status),
		Tasks:          make(map[string]*task.State),
	}
	if err := decodeUsage(r.usageJSON, &state.Usage); err != nil {
		return nil, err
	}
	if err := decodeInput(r.inputJSON, &state.Input); err != nil {
		return nil, err
	}
	if err := decodeOutput(r.outputJSON, &state.Output); err != nil {
		return nil, err
	}
	if err := decodeError(r.errorJSON, &state.Error); err != nil {
		return nil, err
	}
	return state, nil
}

func determineFinalWorkflowStatus(tasks map[string]*task.State) core.StatusType {
	hasRunning := false
	hasFailed := false
	for _, taskState := range tasks {
		if taskState.ParentStateID != nil {
			continue
		}
		switch taskState.Status {
		case core.StatusRunning, core.StatusPending, core.StatusWaiting:
			hasRunning = true
		case core.StatusFailed, core.StatusTimedOut, core.StatusCanceled:
			hasFailed = true
		}
	}
	if hasRunning {
		return core.StatusRunning
	}
	if hasFailed {
		return core.StatusFailed
	}
	return core.StatusSuccess
}

func createWorkflowOutputMap(tasks map[string]*task.State) map[string]any {
	outputs := make(map[string]any, len(tasks))
	taskIDs := make([]string, 0, len(tasks))
	for taskID := range tasks {
		taskIDs = append(taskIDs, taskID)
	}
	sort.Strings(taskIDs)
	for _, taskID := range taskIDs {
		taskState := tasks[taskID]
		data := map[string]any{"output": taskState.Output}
		if taskState.ParentStateID != nil {
			data["parent_state_id"] = taskState.ParentStateID.String()
		}
		if taskState.ExecutionType == task.ExecutionParallel {
			data["execution_type"] = "parallel"
		}
		outputs[taskID] = data
	}
	return outputs
}

func convertOutputToMap(output any) (map[string]any, error) {
	switch v := output.(type) {
	case nil:
		return map[string]any{}, nil
	case map[string]any:
		return v, nil
	case core.Output:
		return map[string]any(v), nil
	case *core.Output:
		if v == nil {
			return map[string]any{}, nil
		}
		return map[string]any(*v), nil
	default:
		return nil, fmt.Errorf("sqlite workflow: unsupported output type %T", output)
	}
}

func decodeUsage(src sql.NullString, dest **usage.Summary) error {
	if !src.Valid || strings.TrimSpace(src.String) == "" {
		*dest = nil
		return nil
	}
	var summary usage.Summary
	if err := json.Unmarshal([]byte(src.String), &summary); err != nil {
		return fmt.Errorf("sqlite workflow: unmarshal usage: %w", err)
	}
	if err := summary.Validate(); err != nil {
		return fmt.Errorf("sqlite workflow: validate usage: %w", err)
	}
	summary.Sort()
	*dest = &summary
	return nil
}

func decodeInput(src sql.NullString, dest **core.Input) error {
	if !src.Valid || strings.TrimSpace(src.String) == "" {
		*dest = nil
		return nil
	}
	var input core.Input
	if err := json.Unmarshal([]byte(src.String), &input); err != nil {
		return fmt.Errorf("sqlite workflow: unmarshal input: %w", err)
	}
	*dest = &input
	return nil
}

func decodeOutput(src sql.NullString, dest **core.Output) error {
	if !src.Valid || strings.TrimSpace(src.String) == "" {
		*dest = nil
		return nil
	}
	var output core.Output
	if err := json.Unmarshal([]byte(src.String), &output); err != nil {
		return fmt.Errorf("sqlite workflow: unmarshal output: %w", err)
	}
	*dest = &output
	return nil
}

func decodeError(src sql.NullString, dest **core.Error) error {
	if !src.Valid || strings.TrimSpace(src.String) == "" {
		*dest = nil
		return nil
	}
	var errObj core.Error
	if err := json.Unmarshal([]byte(src.String), &errObj); err != nil {
		return fmt.Errorf("sqlite workflow: unmarshal error: %w", err)
	}
	*dest = &errObj
	return nil
}

func marshalJSON(value any) (sql.NullString, error) {
	if value == nil {
		return sql.NullString{}, nil
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map:
		if rv.IsNil() {
			return sql.NullString{}, nil
		}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(data), Valid: true}, nil
}

func marshalStateJSONFields(
	state *workflow.State,
) (
	usageJSON sql.NullString,
	inputJSON sql.NullString,
	outputJSON sql.NullString,
	errorJSON sql.NullString,
	err error,
) {
	usageJSON, err = marshalJSON(state.Usage)
	if err != nil {
		err = fmt.Errorf("sqlite workflow: marshal usage: %w", err)
		return
	}
	inputJSON, err = marshalJSON(state.Input)
	if err != nil {
		err = fmt.Errorf("sqlite workflow: marshal input: %w", err)
		return
	}
	outputJSON, err = marshalJSON(state.Output)
	if err != nil {
		err = fmt.Errorf("sqlite workflow: marshal output: %w", err)
		return
	}
	errorJSON, err = marshalJSON(state.Error)
	if err != nil {
		err = fmt.Errorf("sqlite workflow: marshal error: %w", err)
		return
	}
	return
}

func jsonArg(ns sql.NullString) any {
	if !ns.Valid {
		return nil
	}
	return ns.String
}

var _ workflow.Repository = (*WorkflowRepo)(nil)
