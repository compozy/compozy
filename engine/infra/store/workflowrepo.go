package store

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/Masterminds/squirrel"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
)

// ErrWorkflowNotFound is returned when a workflow state is not found.
var ErrWorkflowNotFound = fmt.Errorf("workflow state not found")

// ErrWorkflowNotReady is returned when top-level tasks are still running.
var ErrWorkflowNotReady = fmt.Errorf("workflow not ready for completion")

// WorkflowRepo implements the workflow.Repository interface.
type WorkflowRepo struct {
	db       DBInterface
	taskRepo *TaskRepo
}

func NewWorkflowRepo(db DBInterface) *WorkflowRepo {
	taskRepo := NewTaskRepo(db)
	return &WorkflowRepo{db: db, taskRepo: taskRepo}
}

// withTransaction executes a function within a database transaction
func (r *WorkflowRepo) withTransaction(ctx context.Context, fn func(pgx.Tx) error) error {
	log := logger.FromContext(ctx)
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			err := tx.Rollback(ctx)
			if err != nil {
				log.Error("Failed to rollback transaction", "error", err)
			}
			panic(p)
		} else if err != nil {
			err := tx.Rollback(ctx)
			if err != nil {
				log.Error("Failed to rollback transaction", "error", err)
			}
		} else {
			err = tx.Commit(ctx)
		}
	}()

	err = fn(tx)
	return err
}

// getStateDBWithTx retrieves a workflow StateDB using the provided transaction
func (r *WorkflowRepo) getStateDBWithTx(
	ctx context.Context,
	tx pgx.Tx,
	query string,
	args ...any,
) (*workflow.StateDB, error) {
	var stateDB workflow.StateDB
	err := pgxscan.Get(ctx, tx, &stateDB, query, args...)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("scanning state: %w", err)
	}
	return &stateDB, nil
}

// listTasksInWorkflowWithTx gets task states within a transaction
func (r *WorkflowRepo) listTasksInWorkflowWithTx(
	ctx context.Context,
	tx pgx.Tx,
	workflowExecID core.ID,
) (map[string]*task.State, error) {
	query := `
		SELECT *
		FROM task_states
		WHERE workflow_exec_id = $1
	`

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

// populateTaskStatesWithTx fetches and populates the Tasks map within a transaction
func (r *WorkflowRepo) populateTaskStatesWithTx(
	ctx context.Context,
	tx pgx.Tx,
	state *workflow.State,
) error {
	if state == nil {
		return nil
	}

	// Get all task states for this workflow execution within the transaction
	tasks, err := r.listTasksInWorkflowWithTx(
		ctx,
		tx,
		state.WorkflowExecID,
	)
	if err != nil {
		return fmt.Errorf("failed to load task states: %w", err)
	}

	state.Tasks = tasks
	return nil
}

// ListStates retrieves workflow states based on the provided filter.
func (r *WorkflowRepo) ListStates(
	ctx context.Context,
	filter *workflow.StateFilter,
) ([]*workflow.State, error) {
	sb := squirrel.Select(
		"workflow_exec_id", "workflow_id", "status", "input", "output", "error",
	).
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

	var states []*workflow.State
	for _, stateDB := range statesDB {
		state, err := stateDB.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting state: %w", err)
		}

		// Populate child task states using existing task repo method
		tasks, err := r.taskRepo.ListTasksInWorkflow(
			ctx,
			state.WorkflowExecID,
		)
		if err != nil {
			return nil, fmt.Errorf("populating task states: %w", err)
		}
		state.Tasks = tasks

		states = append(states, state)
	}

	return states, nil
}

// UpsertState inserts or updates a workflow state.
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
		INSERT INTO workflow_states (
			workflow_exec_id, workflow_id, status, input, output, error
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (workflow_exec_id) DO UPDATE SET
			workflow_id = $2,
			status = $3,
			input = $4,
			output = $5,
			error = $6,
			updated_at = now()
	`

	_, err = r.db.Exec(ctx, query,
		state.WorkflowExecID, state.WorkflowID, state.Status,
		input, output, errJSON,
	)
	if err != nil {
		return fmt.Errorf("executing upsert: %w", err)
	}

	return nil
}

// UpdateStatus updates the status of a workflow state by execution ID.
func (r *WorkflowRepo) UpdateStatus(
	ctx context.Context,
	workflowExecID string,
	status core.StatusType,
) error {
	query := `
		UPDATE workflow_states
		SET status = $1, updated_at = now()
		WHERE workflow_exec_id = $2
	`

	cmdTag, err := r.db.Exec(ctx, query, status, workflowExecID)
	if err != nil {
		return fmt.Errorf("updating workflow status: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return ErrWorkflowNotFound
	}

	return nil
}

// GetState retrieves a workflow state by its StateID.
func (r *WorkflowRepo) GetState(ctx context.Context, workflowExecID core.ID) (*workflow.State, error) {
	var result *workflow.State

	err := r.withTransaction(ctx, func(tx pgx.Tx) error {
		query := `
			SELECT workflow_exec_id, workflow_id, status, input, output, error
			FROM workflow_states
			WHERE workflow_exec_id = $1
		`
		stateDB, err := r.getStateDBWithTx(
			ctx,
			tx,
			query,
			workflowExecID,
		)
		if err != nil {
			return err
		}

		state, err := stateDB.ToState()
		if err != nil {
			return err
		}

		// Populate child task states within the same transaction
		if err := r.populateTaskStatesWithTx(ctx, tx, state); err != nil {
			return err
		}

		result = state
		return nil
	})

	return result, err
}

// GetStateByID retrieves a workflow state by workflow ID.
func (r *WorkflowRepo) GetStateByID(ctx context.Context, workflowID string) (*workflow.State, error) {
	var result *workflow.State

	err := r.withTransaction(ctx, func(tx pgx.Tx) error {
		query := `
			SELECT workflow_exec_id, workflow_id, status, input, output, error
			FROM workflow_states
			WHERE workflow_id = $1
			LIMIT 1
		`

		stateDB, err := r.getStateDBWithTx(ctx, tx, query, workflowID)
		if err != nil {
			return err
		}

		state, err := stateDB.ToState()
		if err != nil {
			return err
		}

		// Populate child task states within the same transaction
		if err := r.populateTaskStatesWithTx(ctx, tx, state); err != nil {
			return err
		}

		result = state
		return nil
	})

	return result, err
}

// GetStateByTaskID retrieves a workflow state associated with a task ID.
func (r *WorkflowRepo) GetStateByTaskID(
	ctx context.Context,
	workflowID, taskID string,
) (*workflow.State, error) {
	var result *workflow.State
	err := r.withTransaction(ctx, func(tx pgx.Tx) error {
		query := `
			SELECT w.workflow_exec_id, w.workflow_id, w.status, w.input, w.output, w.error
			FROM workflow_states w
			JOIN task_states t ON w.workflow_exec_id = t.workflow_exec_id
			WHERE w.workflow_id = $1 AND t.task_id = $2
		`

		stateDB, err := r.getStateDBWithTx(ctx, tx, query, workflowID, taskID)
		if err != nil {
			return err
		}

		state, err := stateDB.ToState()
		if err != nil {
			return err
		}

		// Populate child task states within the same transaction
		if err := r.populateTaskStatesWithTx(ctx, tx, state); err != nil {
			return err
		}

		result = state
		return nil
	})
	return result, err
}

// GetStateByAgentID retrieves a workflow state associated with an agent ID.
func (r *WorkflowRepo) GetStateByAgentID(
	ctx context.Context,
	workflowID, agentID string,
) (*workflow.State, error) {
	var result *workflow.State

	err := r.withTransaction(ctx, func(tx pgx.Tx) error {
		query := `
			SELECT w.workflow_exec_id, w.workflow_id, w.status, w.input, w.output, w.error
			FROM workflow_states w
			JOIN task_states t ON w.workflow_exec_id = t.workflow_exec_id
			WHERE w.workflow_id = $1 AND t.agent_id = $2
		`

		stateDB, err := r.getStateDBWithTx(
			ctx,
			tx,
			query,
			workflowID,
			agentID,
		)
		if err != nil {
			return err
		}

		state, err := stateDB.ToState()
		if err != nil {
			return err
		}

		// Populate child task states within the same transaction
		if err := r.populateTaskStatesWithTx(ctx, tx, state); err != nil {
			return err
		}

		result = state
		return nil
	})

	return result, err
}

// GetStateByToolID retrieves a workflow state associated with a tool ID.
func (r *WorkflowRepo) GetStateByToolID(
	ctx context.Context,
	workflowID, toolID string,
) (*workflow.State, error) {
	var result *workflow.State

	err := r.withTransaction(ctx, func(tx pgx.Tx) error {
		query := `
			SELECT w.workflow_exec_id, w.workflow_id, w.status, w.input, w.output, w.error
			FROM workflow_states w
			JOIN task_states t ON w.workflow_exec_id = t.workflow_exec_id
			WHERE w.workflow_id = $1 AND t.tool_id = $2
		`

		stateDB, err := r.getStateDBWithTx(
			ctx,
			tx,
			query,
			workflowID,
			toolID,
		)
		if err != nil {
			return err
		}

		state, err := stateDB.ToState()
		if err != nil {
			return err
		}

		// Populate child task states within the same transaction
		if err := r.populateTaskStatesWithTx(ctx, tx, state); err != nil {
			return err
		}

		result = state
		return nil
	})

	return result, err
}

// determineFinalWorkflowStatus calculates the final workflow status based on top-level task states
func (r *WorkflowRepo) determineFinalWorkflowStatus(tasks map[string]*task.State) core.StatusType {
	finalStatus := core.StatusSuccess
	for _, taskState := range tasks {
		// Skip child tasks - only consider top-level tasks for workflow status
		if taskState.ParentStateID != nil {
			continue
		}

		switch taskState.Status {
		case core.StatusFailed:
			finalStatus = core.StatusFailed
		case core.StatusRunning, core.StatusPending:
			// At least one top-level task still active → workflow is not done yet.
			finalStatus = core.StatusRunning
		}
	}
	return finalStatus
}

// createWorkflowOutputMap builds the output map from all task states
func (r *WorkflowRepo) createWorkflowOutputMap(tasks map[string]*task.State) map[string]any {
	outputMap := make(map[string]any)
	// Sort task IDs for deterministic output
	taskIDs := make([]string, 0, len(tasks))
	for taskID := range tasks {
		taskIDs = append(taskIDs, taskID)
	}
	sort.Strings(taskIDs)
	// Process tasks in sorted order
	for _, taskID := range taskIDs {
		taskState := tasks[taskID]
		// Include progress_info for parent tasks to show completion details
		outputData := map[string]any{
			"output": taskState.Output,
		}
		// Add parent-child relationship info for debugging
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

// updateWorkflowStateWithTx updates the workflow state with output and status within a transaction
func (r *WorkflowRepo) updateWorkflowStateWithTx(
	ctx context.Context,
	tx pgx.Tx,
	workflowExecID core.ID,
	outputMap map[string]any,
	finalStatus core.StatusType,
	workflowError error,
) error {
	// Convert output map to JSONB
	outputJSON, err := ToJSONB(outputMap)
	if err != nil {
		return fmt.Errorf("marshaling workflow output: %w", err)
	}
	// Convert error to JSONB if present
	var errorJSON any
	if workflowError != nil {
		errorJSON, err = ToJSONB(core.NewError(workflowError, "OUTPUT_TRANSFORMATION_FAILED", nil))
		if err != nil {
			return fmt.Errorf("marshaling workflow error: %w", err)
		}
	}
	// Update the workflow state with the collected outputs and determined status
	query := `
		UPDATE workflow_states
		SET output = $1, status = $2, error = $3, updated_at = now()
		WHERE workflow_exec_id = $4
	`
	cmdTag, err := tx.Exec(ctx, query, outputJSON, finalStatus, errorJSON, workflowExecID)
	if err != nil {
		return fmt.Errorf("updating workflow output: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrWorkflowNotFound
	}
	return nil
}

// CompleteWorkflow collects all task outputs and saves them as workflow output
func (r *WorkflowRepo) CompleteWorkflow(
	ctx context.Context,
	workflowExecID core.ID,
	outputTransformer workflow.OutputTransformer,
) (*workflow.State, error) {
	var result *workflow.State

	err := r.withTransaction(ctx, func(tx pgx.Tx) error {
		// Lock workflow and check if already completed
		status, err := r.lockAndCheckWorkflowStatus(ctx, tx, workflowExecID)
		if err != nil {
			return err
		}
		if status == string(core.StatusSuccess) || status == string(core.StatusFailed) {
			return nil // Already completed
		}
		// Process workflow completion
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
	// If result is nil, workflow was already completed - fetch current state
	if result == nil {
		return r.GetState(ctx, workflowExecID)
	}
	return result, nil
}

// lockAndCheckWorkflowStatus locks the workflow row and returns its status
func (r *WorkflowRepo) lockAndCheckWorkflowStatus(
	ctx context.Context, tx pgx.Tx, workflowExecID core.ID,
) (string, error) {
	lockQuery := `
		SELECT workflow_exec_id, status
		FROM workflow_states
		WHERE workflow_exec_id = $1
		FOR UPDATE
	`
	var status string
	err := tx.QueryRow(ctx, lockQuery, workflowExecID.String()).Scan(&workflowExecID, &status)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", ErrWorkflowNotFound
		}
		return "", fmt.Errorf("failed to lock workflow state: %w", err)
	}
	return status, nil
}

// processWorkflowCompletion handles the main completion logic
func (r *WorkflowRepo) processWorkflowCompletion(
	ctx context.Context,
	tx pgx.Tx,
	workflowExecID core.ID,
	outputTransformer workflow.OutputTransformer,
) (*workflow.State, error) {
	// Get all task states
	tasks, err := r.listTasksInWorkflowWithTx(ctx, tx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task states: %w", err)
	}
	// Check if ready to complete
	finalStatus := r.determineFinalWorkflowStatus(tasks)
	if finalStatus == core.StatusRunning {
		return nil, ErrWorkflowNotReady
	}
	// Process output and update state
	if err := r.processOutputAndUpdateState(ctx, tx, workflowExecID, tasks, outputTransformer, &finalStatus); err != nil {
		return nil, err
	}
	// Retrieve updated state
	return r.retrieveUpdatedWorkflowState(ctx, tx, workflowExecID)
}

// processOutputAndUpdateState handles output transformation and state update
func (r *WorkflowRepo) processOutputAndUpdateState(
	ctx context.Context,
	tx pgx.Tx,
	workflowExecID core.ID,
	tasks map[string]*task.State,
	outputTransformer workflow.OutputTransformer,
	finalStatus *core.StatusType,
) error {
	// Determine output
	finalOutput, transformErr := r.determineWorkflowOutput(
		ctx, tx, workflowExecID, tasks, outputTransformer, finalStatus,
	)
	if transformErr != nil {
		log := logger.FromContext(ctx)
		log.Warn(
			"Output transformation failed, using default output",
			"workflow_exec_id",
			workflowExecID,
			"error",
			transformErr,
		)
		finalOutput = r.createWorkflowOutputMap(tasks)
	}
	// Convert output to map
	outputMap, err := r.convertOutputToMap(finalOutput)
	if err != nil {
		return err
	}
	// Update workflow state
	return r.updateWorkflowStateWithTx(
		ctx, tx, workflowExecID, outputMap, *finalStatus, transformErr,
	)
}

// convertOutputToMap converts various output types to map[string]any
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

// retrieveUpdatedWorkflowState gets the updated workflow state after completion
func (r *WorkflowRepo) retrieveUpdatedWorkflowState(
	ctx context.Context,
	tx pgx.Tx,
	workflowExecID core.ID,
) (*workflow.State, error) {
	getQuery := `
		SELECT workflow_exec_id, workflow_id, status, input, output, error
		FROM workflow_states
		WHERE workflow_exec_id = $1
	`
	stateDB, err := r.getStateDBWithTx(ctx, tx, getQuery, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("fetching updated workflow state: %w", err)
	}
	state, err := stateDB.ToState()
	if err != nil {
		return nil, fmt.Errorf("converting updated workflow state: %w", err)
	}
	// Populate child task states
	if err := r.populateTaskStatesWithTx(ctx, tx, state); err != nil {
		return nil, fmt.Errorf("populating task states: %w", err)
	}
	return state, nil
}
