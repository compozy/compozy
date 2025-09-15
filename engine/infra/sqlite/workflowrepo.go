package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
)

// WorkflowRepo implements workflow.Repository backed by SQLite.
type WorkflowRepo struct {
	db       *sql.DB
	taskRepo *TaskRepo
}

func NewWorkflowRepo(db *sql.DB) *WorkflowRepo {
	return &WorkflowRepo{db: db, taskRepo: NewTaskRepo(db)}
}

func scanWorkflowStateDB(rows *sql.Rows) ([]*wf.StateDB, error) {
	var out []*wf.StateDB
	for rows.Next() {
		var sdb wf.StateDB
		if err := rows.Scan(
			&sdb.WorkflowID, &sdb.WorkflowExecID, &sdb.Status, &sdb.InputRaw,
			&sdb.OutputRaw, &sdb.ErrorRaw, &sdb.CreatedAt, &sdb.UpdatedAt,
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

const (
	taskSelect = "component,status,task_id,task_exec_id,workflow_id,workflow_exec_id," +
		"parent_state_id,execution_type,agent_id,action_id,tool_id,input,output,error,created_at,updated_at"
	wfSelect = "workflow_id,workflow_exec_id,status,input,output,error,created_at,updated_at"
)

//nolint:gocyclo,funlen // filter assembly + batch fetch
func (r *WorkflowRepo) ListStates(
	ctx context.Context,
	filter *wf.StateFilter,
) ([]*wf.State, error) {
	args := make([]any, 0, 3)
	base := "SELECT " + wfSelect + " FROM workflow_states"
	// reuse simple filter builder
	where := ""
	add := func(cond string, v any) {
		if where == "" {
			where = " WHERE " + cond
		} else {
			where += " AND " + cond
		}
		args = append(args, v)
	}
	if filter != nil {
		if filter.Status != nil {
			add("status = ?", *filter.Status)
		}
		if filter.WorkflowID != nil {
			add("workflow_id = ?", *filter.WorkflowID)
		}
		if filter.WorkflowExecID != nil {
			add("workflow_exec_id = ?", *filter.WorkflowExecID)
		}
	}
	query := base + where
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query states: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanWorkflowStateDB(rows)
	if err != nil {
		return nil, fmt.Errorf("scan states: %w", err)
	}
	if len(sdbs) == 0 {
		return nil, nil
	}
	// Fetch tasks for all returned exec IDs
	execIDs := make([]any, 0, len(sdbs))
	idStrs := make([]string, 0, len(sdbs))
	for _, s := range sdbs {
		idStrs = append(idStrs, s.WorkflowExecID.String())
	}
	// Build IN clause
	placeholders := questionList(len(idStrs))
	for _, v := range idStrs {
		execIDs = append(execIDs, v)
	}
	//nolint:gosec // G202: building placeholder list; values are bound as args
	tQuery := "SELECT " + taskSelect + " FROM task_states WHERE workflow_exec_id IN (" + placeholders + ")"
	tRows, err := r.db.QueryContext(ctx, tQuery, execIDs...)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer tRows.Close()
	tdb, err := scanTaskStateDB(tRows)
	if err != nil {
		return nil, fmt.Errorf("scan tasks: %w", err)
	}
	tasksByExec := make(map[string]map[string]*task.State)
	for _, ts := range tdb {
		st, err := ts.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert task: %w", err)
		}
		key := ts.WorkflowExecID.String()
		if _, ok := tasksByExec[key]; !ok {
			tasksByExec[key] = make(map[string]*task.State)
		}
		tasksByExec[key][st.TaskID] = st
	}
	states := make([]*wf.State, 0, len(sdbs))
	for _, sdb := range sdbs {
		st, err := sdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert: %w", err)
		}
		st.Tasks = tasksByExec[sdb.WorkflowExecID.String()]
		states = append(states, st)
	}
	return states, nil
}

func (r *WorkflowRepo) UpsertState(ctx context.Context, state *wf.State) error {
	in, err := ToJSONText(state.Input)
	if err != nil {
		return fmt.Errorf("marshal input: %w", err)
	}
	out, err := ToJSONText(state.Output)
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	ej, err := ToJSONText(state.Error)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}
	q := `INSERT INTO workflow_states (workflow_exec_id,workflow_id,status,input,output,error)
          VALUES (?,?,?,?,?,?)
          ON CONFLICT(workflow_exec_id) DO UPDATE SET
            workflow_id=excluded.workflow_id,
            status=excluded.status,
            input=excluded.input,
            output=excluded.output,
            error=excluded.error,
            updated_at=CURRENT_TIMESTAMP`
	_, err = r.db.ExecContext(ctx, q, state.WorkflowExecID, state.WorkflowID, state.Status, in, out, ej)
	if err != nil {
		return fmt.Errorf("upsert: %w", err)
	}
	return nil
}

func (r *WorkflowRepo) UpdateStatus(ctx context.Context, workflowExecID string, status core.StatusType) error {
	res, err := r.db.ExecContext(
		ctx,
		"UPDATE workflow_states SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE workflow_exec_id = ?",
		status,
		workflowExecID,
	)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("rows affected: %w", raErr)
	}
	if n == 0 {
		return store.ErrWorkflowNotFound
	}
	return nil
}

func (r *WorkflowRepo) getStateTx(ctx context.Context, tx *sql.Tx, query string, args ...any) (*wf.StateDB, error) {
	row := tx.QueryRowContext(ctx, query, args...)
	var sdb wf.StateDB
	if err := row.Scan(
		&sdb.WorkflowID, &sdb.WorkflowExecID, &sdb.Status, &sdb.InputRaw,
		&sdb.OutputRaw, &sdb.ErrorRaw, &sdb.CreatedAt, &sdb.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("scan state: %w", err)
	}
	return &sdb, nil
}

func (r *WorkflowRepo) populateTasksTx(ctx context.Context, tx *sql.Tx, state *wf.State) error {
	rows, err := tx.QueryContext(
		ctx,
		"SELECT "+taskSelect+" FROM task_states WHERE workflow_exec_id = ?",
		state.WorkflowExecID,
	)
	if err != nil {
		return fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanTaskStateDB(rows)
	if err != nil {
		return fmt.Errorf("scan tasks: %w", err)
	}
	m := make(map[string]*task.State)
	for _, ts := range sdbs {
		st, err := ts.ToState()
		if err != nil {
			return fmt.Errorf("convert: %w", err)
		}
		m[st.TaskID] = st
	}
	state.Tasks = m
	return nil
}

func (r *WorkflowRepo) GetState(ctx context.Context, workflowExecID core.ID) (*wf.State, error) {
	var result *wf.State
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if result == nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				logger.FromContext(ctx).Warn("SQLite rollback failed", "error", rbErr)
			}
		} else {
			if cmErr := tx.Commit(); cmErr != nil {
				logger.FromContext(ctx).Warn("SQLite commit failed", "error", cmErr)
			}
		}
	}()
	q := "SELECT " + wfSelect + " FROM workflow_states WHERE workflow_exec_id = ?"
	sdb, err := r.getStateTx(ctx, tx, q, workflowExecID)
	if err != nil {
		return nil, err
	}
	st, err := sdb.ToState()
	if err != nil {
		return nil, err
	}
	if err := r.populateTasksTx(ctx, tx, st); err != nil {
		return nil, err
	}
	result = st
	return result, nil
}

func (r *WorkflowRepo) GetStateByID(ctx context.Context, workflowID string) (*wf.State, error) {
	var result *wf.State
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if result == nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				logger.FromContext(ctx).Warn("SQLite rollback failed", "error", rbErr)
			}
		} else {
			if cmErr := tx.Commit(); cmErr != nil {
				logger.FromContext(ctx).Warn("SQLite commit failed", "error", cmErr)
			}
		}
	}()
	q := "SELECT " + wfSelect + " FROM workflow_states WHERE workflow_id = ? LIMIT 1"
	sdb, err := r.getStateTx(ctx, tx, q, workflowID)
	if err != nil {
		return nil, err
	}
	st, err := sdb.ToState()
	if err != nil {
		return nil, err
	}
	if err := r.populateTasksTx(ctx, tx, st); err != nil {
		return nil, err
	}
	result = st
	return result, nil
}

func (r *WorkflowRepo) GetStateByTaskID(ctx context.Context, workflowID, taskID string) (*wf.State, error) {
	var result *wf.State
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if result == nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				logger.FromContext(ctx).Warn("SQLite rollback failed", "error", rbErr)
			}
		} else {
			if cmErr := tx.Commit(); cmErr != nil {
				logger.FromContext(ctx).Warn("SQLite commit failed", "error", cmErr)
			}
		}
	}()
	sdb, err := r.getStateTx(
		ctx,
		tx,
		`SELECT w.workflow_id,w.workflow_exec_id,w.status,w.input,w.output,w.error,w.created_at,w.updated_at
        FROM workflow_states w JOIN task_states t ON w.workflow_exec_id = t.workflow_exec_id
        WHERE w.workflow_id = ? AND t.task_id = ?`,
		workflowID,
		taskID,
	)
	if err != nil {
		return nil, err
	}
	st, err := sdb.ToState()
	if err != nil {
		return nil, err
	}
	if err := r.populateTasksTx(ctx, tx, st); err != nil {
		return nil, err
	}
	result = st
	return result, nil
}

func (r *WorkflowRepo) GetStateByAgentID(ctx context.Context, workflowID, agentID string) (*wf.State, error) {
	var result *wf.State
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if result == nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				logger.FromContext(ctx).Warn("SQLite rollback failed", "error", rbErr)
			}
		} else {
			if cmErr := tx.Commit(); cmErr != nil {
				logger.FromContext(ctx).Warn("SQLite commit failed", "error", cmErr)
			}
		}
	}()
	sdb, err := r.getStateTx(
		ctx,
		tx,
		`SELECT w.workflow_id,w.workflow_exec_id,w.status,w.input,w.output,w.error,w.created_at,w.updated_at
        FROM workflow_states w JOIN task_states t ON w.workflow_exec_id = t.workflow_exec_id
        WHERE w.workflow_id = ? AND t.agent_id = ?`,
		workflowID,
		agentID,
	)
	if err != nil {
		return nil, err
	}
	st, err := sdb.ToState()
	if err != nil {
		return nil, err
	}
	if err := r.populateTasksTx(ctx, tx, st); err != nil {
		return nil, err
	}
	result = st
	return result, nil
}

func (r *WorkflowRepo) GetStateByToolID(ctx context.Context, workflowID, toolID string) (*wf.State, error) {
	var result *wf.State
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if result == nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				logger.FromContext(ctx).Warn("SQLite rollback failed", "error", rbErr)
			}
		} else {
			if cmErr := tx.Commit(); cmErr != nil {
				logger.FromContext(ctx).Warn("SQLite commit failed", "error", cmErr)
			}
		}
	}()
	sdb, err := r.getStateTx(
		ctx,
		tx,
		`SELECT w.workflow_id,w.workflow_exec_id,w.status,w.input,w.output,w.error,w.created_at,w.updated_at
        FROM workflow_states w JOIN task_states t ON w.workflow_exec_id = t.workflow_exec_id
        WHERE w.workflow_id = ? AND t.tool_id = ?`,
		workflowID,
		toolID,
	)
	if err != nil {
		return nil, err
	}
	st, err := sdb.ToState()
	if err != nil {
		return nil, err
	}
	if err := r.populateTasksTx(ctx, tx, st); err != nil {
		return nil, err
	}
	result = st
	return result, nil
}

func (r *WorkflowRepo) determineFinalWorkflowStatus(tasks map[string]*task.State) core.StatusType {
	final := core.StatusSuccess
	for _, ts := range tasks {
		if ts.ParentStateID != nil {
			continue
		}
		switch ts.Status {
		case core.StatusFailed:
			final = core.StatusFailed
		case core.StatusRunning, core.StatusPending:
			final = core.StatusRunning
		}
	}
	return final
}

func (r *WorkflowRepo) createWorkflowOutputMap(tasks map[string]*task.State) map[string]any {
	out := make(map[string]any)
	ids := make([]string, 0, len(tasks))
	for id := range tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		ts := tasks[id]
		m := map[string]any{"output": ts.Output}
		if ts.ParentStateID != nil {
			m["parent_state_id"] = ts.ParentStateID.String()
		}
		if ts.ExecutionType == task.ExecutionParallel {
			m["execution_type"] = "parallel"
		}
		out[ts.TaskID] = m
	}
	return out
}

func (r *WorkflowRepo) updateWorkflowStateTx(
	ctx context.Context,
	tx *sql.Tx,
	workflowExecID core.ID,
	output map[string]any,
	finalStatus core.StatusType,
	wfErr error,
) error {
	outJSON, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	var errJSON []byte
	if wfErr != nil {
		bj, mErr := json.Marshal(core.NewError(wfErr, "OUTPUT_TRANSFORMATION_FAILED", nil))
		if mErr != nil {
			return fmt.Errorf("marshal error: %w", mErr)
		}
		errJSON = bj
	}
	uq := "UPDATE workflow_states SET output = ?, error = ?, status = ?, " +
		"updated_at = CURRENT_TIMESTAMP WHERE workflow_exec_id = ?"
	_, execErr := tx.ExecContext(ctx, uq, outJSON, errJSON, finalStatus, workflowExecID)
	if execErr != nil {
		return fmt.Errorf("update workflow state: %w", execErr)
	}
	return nil
}

func (r *WorkflowRepo) retrieveUpdatedWorkflowState(
	ctx context.Context,
	tx *sql.Tx,
	workflowExecID core.ID,
) (*wf.State, error) {
	q := "SELECT " + wfSelect + " FROM workflow_states WHERE workflow_exec_id = ?"
	row := tx.QueryRowContext(ctx, q, workflowExecID)
	var sdb wf.StateDB
	if err := row.Scan(
		&sdb.WorkflowID, &sdb.WorkflowExecID, &sdb.Status, &sdb.InputRaw,
		&sdb.OutputRaw, &sdb.ErrorRaw, &sdb.CreatedAt, &sdb.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("fetch updated: %w", err)
	}
	st, err := sdb.ToState()
	if err != nil {
		return nil, err
	}
	if err := r.populateTasksTx(ctx, tx, st); err != nil {
		return nil, err
	}
	return st, nil
}

func (r *WorkflowRepo) determineWorkflowOutput(
	ctx context.Context,
	tx *sql.Tx,
	workflowExecID core.ID,
	tasks map[string]*task.State,
	transformer wf.OutputTransformer,
	finalStatus *core.StatusType,
) (any, error) {
	if transformer == nil {
		return r.createWorkflowOutputMap(tasks), nil
	}
	q2 := "SELECT " + wfSelect + " FROM workflow_states WHERE workflow_exec_id = ?"
	row := tx.QueryRowContext(ctx, q2, workflowExecID)
	var sdb wf.StateDB
	if err := row.Scan(
		&sdb.WorkflowID, &sdb.WorkflowExecID, &sdb.Status, &sdb.InputRaw,
		&sdb.OutputRaw, &sdb.ErrorRaw, &sdb.CreatedAt, &sdb.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("get wf state: %w", err)
	}
	st, err := sdb.ToState()
	if err != nil {
		return nil, err
	}
	st.Tasks = tasks
	transformed, err := transformer(st)
	if err != nil {
		if finalStatus != nil {
			*finalStatus = core.StatusFailed
		}
		return nil, fmt.Errorf("workflow output transformation failed: %w", err)
	}
	return transformed, nil
}

func (r *WorkflowRepo) processWorkflowCompletion(
	ctx context.Context,
	tx *sql.Tx,
	workflowExecID core.ID,
	transformer wf.OutputTransformer,
) (*wf.State, error) {
	// Load all task states for the workflow
	rows, err := tx.QueryContext(
		ctx,
		"SELECT "+taskSelect+" FROM task_states WHERE workflow_exec_id = ?",
		workflowExecID,
	)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()
	sdbs, err := scanTaskStateDB(rows)
	if err != nil {
		return nil, fmt.Errorf("scan tasks: %w", err)
	}
	tasks := make(map[string]*task.State)
	for _, ts := range sdbs {
		st, err := ts.ToState()
		if err != nil {
			return nil, fmt.Errorf("convert: %w", err)
		}
		tasks[st.TaskID] = st
	}
	finalStatus := r.determineFinalWorkflowStatus(tasks)
	if finalStatus == core.StatusRunning {
		return nil, store.ErrWorkflowNotReady
	}
	out, txErr := r.determineWorkflowOutput(ctx, tx, workflowExecID, tasks, transformer, &finalStatus)
	if txErr != nil {
		out = r.createWorkflowOutputMap(tasks)
		finalStatus = core.StatusFailed
	}
	var outMap map[string]any
	switch v := out.(type) {
	case nil:
		outMap = make(map[string]any)
	case map[string]any:
		outMap = v
	case core.Output:
		outMap = map[string]any(v)
	case *core.Output:
		if v != nil {
			outMap = map[string]any(*v)
		} else {
			outMap = make(map[string]any)
		}
	default:
		return nil, fmt.Errorf("unsupported output type %T", out)
	}
	if err := r.updateWorkflowStateTx(ctx, tx, workflowExecID, outMap, finalStatus, txErr); err != nil {
		return nil, err
	}
	return r.retrieveUpdatedWorkflowState(ctx, tx, workflowExecID)
}

func (r *WorkflowRepo) CompleteWorkflow(
	ctx context.Context,
	workflowExecID core.ID,
	transformer wf.OutputTransformer,
) (*wf.State, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				logger.FromContext(ctx).Warn("SQLite rollback failed", "error", rbErr)
			}
		} else {
			if cmErr := tx.Commit(); cmErr != nil {
				logger.FromContext(ctx).Warn("SQLite commit failed", "error", cmErr)
			}
		}
	}()
	// Lock row: SQLite lacks FOR UPDATE; emulate by deferred transaction
	// Ensure the workflow exists first
	var status string
	statusQuery := "SELECT status FROM workflow_states WHERE workflow_exec_id = ?"
	if err2 := tx.QueryRowContext(ctx, statusQuery, workflowExecID).Scan(&status); err2 != nil {
		if errors.Is(err2, sql.ErrNoRows) {
			return nil, store.ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("lock workflow: %w", err2)
	}
	state, perr := r.processWorkflowCompletion(ctx, tx, workflowExecID, transformer)
	if perr != nil {
		return nil, perr
	}
	return state, nil
}
