package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/Masterminds/squirrel"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
)

const selectWorkflowStateByExecID = "SELECT " +
	"workflow_exec_id, workflow_id, status, input, output, error " +
	"FROM workflow_states WHERE workflow_exec_id = $1"

// WorkflowRepo implements the workflow.Repository interface.
type WorkflowRepo struct {
	db       DB
	taskRepo *TaskRepo
}

func NewWorkflowRepo(db DB) *WorkflowRepo {
	return &WorkflowRepo{db: db, taskRepo: NewTaskRepo(db)}
}

func (r *WorkflowRepo) withTransaction(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				logger.FromContext(ctx).Warn("Transaction rollback failed after panic", "error", rbErr)
			}
			panic(p)
		} else if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				logger.FromContext(ctx).Warn("Transaction rollback failed", "error", rbErr)
			}
		} else {
			err = tx.Commit(ctx)
		}
	}()
	err = fn(tx)
	return err
}

func (r *WorkflowRepo) getStateDBWithTx(
	ctx context.Context,
	tx pgx.Tx,
	query string,
	args ...any,
) (*workflow.StateDB, error) {
	var stateDB workflow.StateDB
	if err := pgxscan.Get(ctx, tx, &stateDB, query, args...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("scanning state: %w", err)
	}
	return &stateDB, nil
}

func (r *WorkflowRepo) listTasksInWorkflowWithTx(
	ctx context.Context,
	tx pgx.Tx,
	workflowExecID core.ID,
) (map[string]*task.State, error) {
	query := TaskHierarchyCTEQuery
	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, tx, &statesDB, query, workflowExecID); err != nil {
		return nil, fmt.Errorf("scanning task states: %w", err)
	}
	result := make(map[string]*task.State)
	for _, stateDB := range statesDB {
		state, err := stateDB.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting task state: %w", err)
		}
		result[state.TaskID] = state
	}
	return result, nil
}

func (r *WorkflowRepo) populateTaskStatesWithTx(ctx context.Context, tx pgx.Tx, state *workflow.State) error {
	if state == nil {
		return nil
	}
	query := TaskHierarchyCTEQuery
	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, tx, &statesDB, query, state.WorkflowExecID); err != nil {
		return fmt.Errorf("scanning task states: %w", err)
	}
	result := make(map[string]*task.State)
	for _, stateDB := range statesDB {
		taskState, err := stateDB.ToState()
		if err != nil {
			return fmt.Errorf("converting task state: %w", err)
		}
		result[taskState.TaskID] = taskState
	}
	state.Tasks = result
	return nil
}

func (r *WorkflowRepo) ListStates(ctx context.Context, filter *workflow.StateFilter) ([]*workflow.State, error) {
	sb := squirrel.Select("workflow_exec_id", "workflow_id", "status", "input", "output", "error").
		From("workflow_states").
		PlaceholderFormat(squirrel.Dollar)
	if filter != nil {
		if filter.Status != nil {
			sb = sb.Where("status = ?", *filter.Status)
		}
		if filter.WorkflowID != nil {
			sb = sb.Where("workflow_id = ?", *filter.WorkflowID)
		}
		if filter.WorkflowExecID != nil {
			sb = sb.Where("workflow_exec_id = ?", *filter.WorkflowExecID)
		}
	}
	sql, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building query: %w", err)
	}
	var statesDB []*workflow.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, sql, args...); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}
	if len(statesDB) == 0 {
		return nil, nil
	}
	execIDs := make([]string, 0, len(statesDB))
	for _, sdb := range statesDB {
		execIDs = append(execIDs, sdb.WorkflowExecID.String())
	}
	// Fetch all tasks for the returned workflow executions in a single query
	var taskStatesDB []*task.StateDB
	// Using ANY($1) with text[] to avoid N+1 queries
	if err := pgxscan.Select(ctx, r.db, &taskStatesDB, `SELECT * FROM task_states WHERE workflow_exec_id = ANY($1)`, execIDs); err != nil {
		return nil, fmt.Errorf("scanning task states: %w", err)
	}
	tasksByExec := make(map[string]map[string]*task.State)
	for _, tsdb := range taskStatesDB {
		st, err := tsdb.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting task state: %w", err)
		}
		key := tsdb.WorkflowExecID.String()
		if _, ok := tasksByExec[key]; !ok {
			tasksByExec[key] = make(map[string]*task.State)
		}
		tasksByExec[key][st.TaskID] = st
	}
	var states []*workflow.State
	for _, stateDB := range statesDB {
		state, err := stateDB.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting state: %w", err)
		}
		state.Tasks = tasksByExec[state.WorkflowExecID.String()]
		states = append(states, state)
	}
	return states, nil
}

func (r *WorkflowRepo) UpsertState(ctx context.Context, state *workflow.State) error {
	input, err := ToJSONB(state.Input)
	if err != nil {
		return fmt.Errorf("marshaling input: %w", err)
	}
	output, err := ToJSONB(state.Output)
	if err != nil {
		return fmt.Errorf("marshaling output: %w", err)
	}
	errJSON, err := ToJSONB(state.Error)
	if err != nil {
		return fmt.Errorf("marshaling error: %w", err)
	}
	query := `
        INSERT INTO workflow_states (workflow_exec_id, workflow_id, status, input, output, error)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (workflow_exec_id) DO UPDATE SET
            workflow_id = $2, status = $3, input = $4, output = $5, error = $6, updated_at = now()
    `
	if _, err := r.db.Exec(
		ctx,
		query,
		state.WorkflowExecID,
		state.WorkflowID,
		state.Status,
		input,
		output,
		errJSON,
	); err != nil {
		return fmt.Errorf("executing upsert: %w", err)
	}
	return nil
}

func (r *WorkflowRepo) UpdateStatus(ctx context.Context, workflowExecID core.ID, status core.StatusType) error {
	query := `UPDATE workflow_states SET status = $1, updated_at = now() WHERE workflow_exec_id = $2`
	tag, err := r.db.Exec(ctx, query, status, workflowExecID)
	if err != nil {
		return fmt.Errorf("updating workflow status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.ErrWorkflowNotFound
	}
	return nil
}

func (r *WorkflowRepo) GetState(ctx context.Context, workflowExecID core.ID) (*workflow.State, error) {
	var result *workflow.State
	err := r.withTransaction(ctx, func(tx pgx.Tx) error {
		stateDB, err := r.getStateDBWithTx(ctx, tx, selectWorkflowStateByExecID, workflowExecID)
		if err != nil {
			return err
		}
		state, err := stateDB.ToState()
		if err != nil {
			return err
		}
		if err := r.populateTaskStatesWithTx(ctx, tx, state); err != nil {
			return err
		}
		result = state
		return nil
	})
	return result, err
}

func (r *WorkflowRepo) GetStateByID(ctx context.Context, workflowID string) (*workflow.State, error) {
	var result *workflow.State
	err := r.withTransaction(ctx, func(tx pgx.Tx) error {
		query := `SELECT workflow_exec_id, workflow_id, status, input, output, error FROM workflow_states WHERE workflow_id = $1 LIMIT 1`
		stateDB, err := r.getStateDBWithTx(ctx, tx, query, workflowID)
		if err != nil {
			return err
		}
		state, err := stateDB.ToState()
		if err != nil {
			return err
		}
		if err := r.populateTaskStatesWithTx(ctx, tx, state); err != nil {
			return err
		}
		result = state
		return nil
	})
	return result, err
}

func (r *WorkflowRepo) GetStateByTaskID(ctx context.Context, workflowID, taskID string) (*workflow.State, error) {
	var result *workflow.State
	err := r.withTransaction(ctx, func(tx pgx.Tx) error {
		query := `
            SELECT w.workflow_exec_id, w.workflow_id, w.status, w.input, w.output, w.error
            FROM workflow_states w JOIN task_states t ON w.workflow_exec_id = t.workflow_exec_id
            WHERE w.workflow_id = $1 AND t.task_id = $2`
		stateDB, err := r.getStateDBWithTx(ctx, tx, query, workflowID, taskID)
		if err != nil {
			return err
		}
		state, err := stateDB.ToState()
		if err != nil {
			return err
		}
		if err := r.populateTaskStatesWithTx(ctx, tx, state); err != nil {
			return err
		}
		result = state
		return nil
	})
	return result, err
}

func (r *WorkflowRepo) GetStateByAgentID(ctx context.Context, workflowID, agentID string) (*workflow.State, error) {
	var result *workflow.State
	err := r.withTransaction(ctx, func(tx pgx.Tx) error {
		query := `
            SELECT w.workflow_exec_id, w.workflow_id, w.status, w.input, w.output, w.error
            FROM workflow_states w JOIN task_states t ON w.workflow_exec_id = t.workflow_exec_id
            WHERE w.workflow_id = $1 AND t.agent_id = $2`
		stateDB, err := r.getStateDBWithTx(ctx, tx, query, workflowID, agentID)
		if err != nil {
			return err
		}
		state, err := stateDB.ToState()
		if err != nil {
			return err
		}
		if err := r.populateTaskStatesWithTx(ctx, tx, state); err != nil {
			return err
		}
		result = state
		return nil
	})
	return result, err
}

func (r *WorkflowRepo) GetStateByToolID(ctx context.Context, workflowID, toolID string) (*workflow.State, error) {
	var result *workflow.State
	err := r.withTransaction(ctx, func(tx pgx.Tx) error {
		query := `
            SELECT w.workflow_exec_id, w.workflow_id, w.status, w.input, w.output, w.error
            FROM workflow_states w JOIN task_states t ON w.workflow_exec_id = t.workflow_exec_id
            WHERE w.workflow_id = $1 AND t.tool_id = $2`
		stateDB, err := r.getStateDBWithTx(ctx, tx, query, workflowID, toolID)
		if err != nil {
			return err
		}
		state, err := stateDB.ToState()
		if err != nil {
			return err
		}
		if err := r.populateTaskStatesWithTx(ctx, tx, state); err != nil {
			return err
		}
		result = state
		return nil
	})
	return result, err
}

func (r *WorkflowRepo) determineFinalWorkflowStatus(tasks map[string]*task.State) core.StatusType {
	hasRunning := false
	hasFailed := false
	for _, taskState := range tasks {
		if taskState.ParentStateID != nil {
			continue
		}
		switch taskState.Status {
		case core.StatusRunning, core.StatusPending:
			hasRunning = true
		case core.StatusFailed:
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

func (r *WorkflowRepo) createWorkflowOutputMap(tasks map[string]*task.State) map[string]any {
	outputMap := make(map[string]any)
	taskIDs := make([]string, 0, len(tasks))
	for taskID := range tasks {
		taskIDs = append(taskIDs, taskID)
	}
	sort.Strings(taskIDs)
	for _, taskID := range taskIDs {
		taskState := tasks[taskID]
		outputData := map[string]any{"output": taskState.Output}
		if taskState.ParentStateID != nil {
			outputData["parent_state_id"] = taskState.ParentStateID.String()
		}
		if taskState.ExecutionType == task.ExecutionParallel {
			outputData["execution_type"] = "parallel"
		}
		outputMap[taskID] = outputData
	}
	return outputMap
}

func (r *WorkflowRepo) updateWorkflowStateWithTx(
	ctx context.Context,
	tx pgx.Tx,
	workflowExecID core.ID,
	outputMap map[string]any,
	finalStatus core.StatusType,
	workflowError error,
) error {
	outputJSON, err := ToJSONB(outputMap)
	if err != nil {
		return fmt.Errorf("marshaling workflow output: %w", err)
	}
	var errorJSON any
	if workflowError != nil {
		errorJSON, err = ToJSONB(core.NewError(workflowError, "OUTPUT_TRANSFORMATION_FAILED", nil))
		if err != nil {
			return fmt.Errorf("marshaling workflow error: %w", err)
		}
	}
	query := `UPDATE workflow_states SET output = $1, status = $2, error = $3, updated_at = now() WHERE workflow_exec_id = $4`
	tag, err := tx.Exec(ctx, query, outputJSON, finalStatus, errorJSON, workflowExecID)
	if err != nil {
		return fmt.Errorf("updating workflow output: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.ErrWorkflowNotFound
	}
	return nil
}

func (r *WorkflowRepo) CompleteWorkflow(
	ctx context.Context,
	workflowExecID core.ID,
	outputTransformer workflow.OutputTransformer,
) (*workflow.State, error) {
	var result *workflow.State
	err := r.withTransaction(ctx, func(tx pgx.Tx) error {
		status, err := r.lockAndCheckWorkflowStatus(ctx, tx, workflowExecID)
		if err != nil {
			return err
		}
		if status == string(core.StatusSuccess) || status == string(core.StatusFailed) {
			return nil
		}
		state, err := r.processWorkflowCompletion(ctx, tx, workflowExecID, outputTransformer)
		if err != nil {
			return err
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

func (r *WorkflowRepo) lockAndCheckWorkflowStatus(
	ctx context.Context,
	tx pgx.Tx,
	workflowExecID core.ID,
) (string, error) {
	lockQuery := `SELECT status FROM workflow_states WHERE workflow_exec_id = $1 FOR UPDATE`
	var status string
	if err := tx.QueryRow(ctx, lockQuery, workflowExecID).Scan(&status); err != nil {
		if err == pgx.ErrNoRows {
			return "", store.ErrWorkflowNotFound
		}
		return "", fmt.Errorf("failed to lock workflow state: %w", err)
	}
	return status, nil
}

func (r *WorkflowRepo) processWorkflowCompletion(
	ctx context.Context,
	tx pgx.Tx,
	workflowExecID core.ID,
	outputTransformer workflow.OutputTransformer,
) (*workflow.State, error) {
	tasks, err := r.listTasksInWorkflowWithTx(ctx, tx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task states: %w", err)
	}
	finalStatus := r.determineFinalWorkflowStatus(tasks)
	if finalStatus == core.StatusRunning {
		return nil, store.ErrWorkflowNotReady
	}
	if err := r.processOutputAndUpdateState(
		ctx, tx, workflowExecID, tasks, outputTransformer, &finalStatus,
	); err != nil {
		return nil, err
	}
	return r.retrieveUpdatedWorkflowState(ctx, tx, workflowExecID)
}

func (r *WorkflowRepo) processOutputAndUpdateState(
	ctx context.Context,
	tx pgx.Tx,
	workflowExecID core.ID,
	tasks map[string]*task.State,
	outputTransformer workflow.OutputTransformer,
	finalStatus *core.StatusType,
) error {
	finalOutput, transformErr := r.determineWorkflowOutput(
		ctx,
		tx,
		workflowExecID,
		tasks,
		outputTransformer,
		finalStatus,
	)
	if transformErr != nil {
		finalOutput = r.createWorkflowOutputMap(tasks)
		*finalStatus = core.StatusFailed
	}
	outputMap, err := r.convertOutputToMap(finalOutput)
	if err != nil {
		return err
	}
	return r.updateWorkflowStateWithTx(ctx, tx, workflowExecID, outputMap, *finalStatus, transformErr)
}

// determineWorkflowOutput determines the final workflow output based on transformer
func (r *WorkflowRepo) determineWorkflowOutput(
	ctx context.Context,
	tx pgx.Tx,
	workflowExecID core.ID,
	tasks map[string]*task.State,
	outputTransformer workflow.OutputTransformer,
	finalStatus *core.StatusType,
) (any, error) {
	if outputTransformer == nil {
		return r.createWorkflowOutputMap(tasks), nil
	}
	query := `SELECT workflow_exec_id, workflow_id, status, input, output, error FROM workflow_states WHERE workflow_exec_id = $1`
	stateDB, err := r.getStateDBWithTx(ctx, tx, query, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow state: %w", err)
	}
	state, err := stateDB.ToState()
	if err != nil {
		return nil, fmt.Errorf("failed to convert workflow state: %w", err)
	}
	state.Tasks = tasks
	transformedOutput, err := outputTransformer(state)
	if err != nil {
		if finalStatus != nil {
			*finalStatus = core.StatusFailed
		}
		return nil, fmt.Errorf("workflow output transformation failed: %w", err)
	}
	return transformedOutput, nil
}

func (r *WorkflowRepo) convertOutputToMap(output any) (map[string]any, error) {
	switch v := output.(type) {
	case nil:
		return make(map[string]any), nil
	case map[string]any:
		return v, nil
	case core.Output:
		return map[string]any(v), nil
	case *core.Output:
		if v != nil {
			return map[string]any(*v), nil
		}
		return make(map[string]any), nil
	default:
		return nil, fmt.Errorf("unsupported output type %T", output)
	}
}

func (r *WorkflowRepo) retrieveUpdatedWorkflowState(
	ctx context.Context,
	tx pgx.Tx,
	workflowExecID core.ID,
) (*workflow.State, error) {
	getQuery := `SELECT workflow_exec_id, workflow_id, status, input, output, error FROM workflow_states WHERE workflow_exec_id = $1`
	stateDB, err := r.getStateDBWithTx(ctx, tx, getQuery, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("fetching updated workflow state: %w", err)
	}
	state, err := stateDB.ToState()
	if err != nil {
		return nil, fmt.Errorf("converting updated workflow state: %w", err)
	}
	if err := r.populateTaskStatesWithTx(ctx, tx, state); err != nil {
		return nil, fmt.Errorf("populating task states: %w", err)
	}
	return state, nil
}
