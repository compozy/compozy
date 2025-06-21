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
	db          DBInterface
	taskRepo    *TaskRepo
	queryFilter *QueryFilters
}

func NewWorkflowRepo(db DBInterface) *WorkflowRepo {
	taskRepo := NewTaskRepo(db)
	return &WorkflowRepo{
		db:          db,
		taskRepo:    taskRepo,
		queryFilter: NewQueryFilters(),
	}
}

// getOrganizationID gets the organization ID for a workflow without applying organization filtering.
//
// SECURITY WARNING: This method intentionally bypasses tenant isolation and should ONLY be used by
// trusted internal services (e.g., workflow schedulers, activity workers) to establish an
// organization context from a workflow execution ID. It MUST NOT be exposed to end-user-facing APIs,
// as it could leak information about the existence of workflows in other organizations.
//
// This method exists solely for internal context establishment and should be used with extreme caution.
// Any caller must ensure the workflow execution ID comes from a trusted source and not from user input.
//
// This method is unexported (private) to prevent misuse outside the store package.
func (r *WorkflowRepo) getOrganizationID(ctx context.Context, workflowExecID core.ID) (core.ID, error) {
	query := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Select("org_id").
		From("workflow_states").
		Where(squirrel.Eq{"workflow_exec_id": workflowExecID})

	sql, args, err := query.ToSql()
	if err != nil {
		return "", fmt.Errorf("building query: %w", err)
	}

	var orgID core.ID
	err = r.db.QueryRow(ctx, sql, args...).Scan(&orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrWorkflowNotFound
		}
		return "", fmt.Errorf("querying workflow organization ID: %w", err)
	}

	return orgID, nil
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
	sb := squirrel.Select("*").
		From("task_states").
		Where(squirrel.Eq{"workflow_exec_id": workflowExecID}).
		PlaceholderFormat(squirrel.Dollar)

	// Apply organization filtering
	sb, err := r.queryFilter.ApplyOrgFilter(ctx, sb)
	if err != nil {
		return nil, fmt.Errorf("applying organization filter: %w", err)
	}

	query, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building query: %w", err)
	}

	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, tx, &statesDB, query, args...); err != nil {
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
		"workflow_exec_id", "workflow_id", "org_id", "status", "input", "output", "error",
	).
		From("workflow_states").
		PlaceholderFormat(squirrel.Dollar)

	// Apply organization filtering first
	sb, err := r.queryFilter.ApplyOrgFilter(ctx, sb)
	if err != nil {
		return nil, fmt.Errorf("applying organization filter: %w", err)
	}

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
	// SECURITY: Enforce organization ID from context, not from input
	// This prevents cross-tenant data writes if caller forgets to validate
	contextOrgID := MustGetOrganizationID(ctx)
	state.OrgID = contextOrgID // Overwrite with trusted value from context

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
			workflow_exec_id, workflow_id, org_id, status, input, output, error
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (workflow_exec_id, org_id) 
		DO UPDATE SET
			workflow_id = EXCLUDED.workflow_id,
			status = EXCLUDED.status,
			input = EXCLUDED.input,
			output = EXCLUDED.output,
			error = EXCLUDED.error,
			updated_at = now()
	`

	_, err = r.db.Exec(ctx, query,
		state.WorkflowExecID, state.WorkflowID, state.OrgID, state.Status,
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
	ub := squirrel.Update("workflow_states").
		Set("status", status).
		SetMap(squirrel.Eq{"updated_at": squirrel.Expr("now()")}).
		Where(squirrel.Eq{"workflow_exec_id": workflowExecID}).
		PlaceholderFormat(squirrel.Dollar)

	// Apply organization filtering to prevent cross-org updates
	cmdTag, err := r.queryFilter.ExecuteOrgFilteredUpdate(ctx, r.db, ub)
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
		sb := squirrel.Select("workflow_exec_id", "workflow_id", "org_id", "status", "input", "output", "error").
			From("workflow_states").
			Where(squirrel.Eq{"workflow_exec_id": workflowExecID}).
			PlaceholderFormat(squirrel.Dollar)

		// Apply organization filtering
		sb, err := r.queryFilter.ApplyOrgFilter(ctx, sb)
		if err != nil {
			return fmt.Errorf("applying organization filter: %w", err)
		}

		query, args, err := sb.ToSql()
		if err != nil {
			return fmt.Errorf("building query: %w", err)
		}

		stateDB, err := r.getStateDBWithTx(
			ctx,
			tx,
			query,
			args...,
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
		sb := squirrel.Select("workflow_exec_id", "workflow_id", "org_id", "status", "input", "output", "error").
			From("workflow_states").
			Where(squirrel.Eq{"workflow_id": workflowID}).
			Limit(1).
			PlaceholderFormat(squirrel.Dollar)

		// Apply organization filtering
		sb, err := r.queryFilter.ApplyOrgFilter(ctx, sb)
		if err != nil {
			return fmt.Errorf("applying organization filter: %w", err)
		}

		query, args, err := sb.ToSql()
		if err != nil {
			return fmt.Errorf("building query: %w", err)
		}

		stateDB, err := r.getStateDBWithTx(ctx, tx, query, args...)
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
		sb := squirrel.Select(
			"w.workflow_exec_id", "w.workflow_id", "w.org_id",
			"w.status", "w.input", "w.output", "w.error",
		).
			From("workflow_states w").
			Join("task_states t ON w.workflow_exec_id = t.workflow_exec_id").
			Where(squirrel.Eq{"w.workflow_id": workflowID}).
			Where(squirrel.Eq{"t.task_id": taskID}).
			PlaceholderFormat(squirrel.Dollar)

		// Apply organization filtering on both workflow_states and task_states tables
		orgID, err := r.queryFilter.GetOrgID(ctx)
		if err != nil {
			return fmt.Errorf("getting organization ID: %w", err)
		}
		sb = sb.Where(squirrel.Eq{"w.org_id": orgID}).Where(squirrel.Eq{"t.org_id": orgID})

		query, args, err := sb.ToSql()
		if err != nil {
			return fmt.Errorf("building query: %w", err)
		}

		stateDB, err := r.getStateDBWithTx(ctx, tx, query, args...)
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
		sb := squirrel.Select(
			"w.workflow_exec_id", "w.workflow_id", "w.org_id",
			"w.status", "w.input", "w.output", "w.error",
		).
			From("workflow_states w").
			Join("task_states t ON w.workflow_exec_id = t.workflow_exec_id").
			Where(squirrel.Eq{"w.workflow_id": workflowID}).
			Where(squirrel.Eq{"t.agent_id": agentID}).
			PlaceholderFormat(squirrel.Dollar)

		// Apply organization filtering on both workflow_states and task_states tables
		orgID, err := r.queryFilter.GetOrgID(ctx)
		if err != nil {
			return fmt.Errorf("getting organization ID: %w", err)
		}
		sb = sb.Where(squirrel.Eq{"w.org_id": orgID}).Where(squirrel.Eq{"t.org_id": orgID})

		query, args, err := sb.ToSql()
		if err != nil {
			return fmt.Errorf("building query: %w", err)
		}

		stateDB, err := r.getStateDBWithTx(
			ctx,
			tx,
			query,
			args...,
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
		sb := squirrel.Select(
			"w.workflow_exec_id", "w.workflow_id", "w.org_id",
			"w.status", "w.input", "w.output", "w.error",
		).
			From("workflow_states w").
			Join("task_states t ON w.workflow_exec_id = t.workflow_exec_id").
			Where(squirrel.Eq{"w.workflow_id": workflowID}).
			Where(squirrel.Eq{"t.tool_id": toolID}).
			PlaceholderFormat(squirrel.Dollar)

		// Apply organization filtering on both workflow_states and task_states tables
		orgID, err := r.queryFilter.GetOrgID(ctx)
		if err != nil {
			return fmt.Errorf("getting organization ID: %w", err)
		}
		sb = sb.Where(squirrel.Eq{"w.org_id": orgID}).Where(squirrel.Eq{"t.org_id": orgID})

		query, args, err := sb.ToSql()
		if err != nil {
			return fmt.Errorf("building query: %w", err)
		}

		stateDB, err := r.getStateDBWithTx(
			ctx,
			tx,
			query,
			args...,
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

	// Use squirrel query builder with organization filtering
	ub := squirrel.Update("workflow_states").
		Set("output", outputJSON).
		Set("status", finalStatus).
		Set("error", errorJSON).
		Set("updated_at", squirrel.Expr("NOW()")).
		Where(squirrel.Eq{"workflow_exec_id": workflowExecID}).
		PlaceholderFormat(squirrel.Dollar)

	// Apply organization filtering to prevent cross-org updates
	orgID, err := r.queryFilter.GetOrgID(ctx)
	if err != nil {
		return fmt.Errorf("getting organization ID for update: %w", err)
	}
	ub = ub.Where(squirrel.Eq{"org_id": orgID})

	query, args, err := ub.ToSql()
	if err != nil {
		return fmt.Errorf("building update query: %w", err)
	}

	cmdTag, err := tx.Exec(ctx, query, args...)
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
	sb := squirrel.Select("workflow_exec_id", "status").
		From("workflow_states").
		Where(squirrel.Eq{"workflow_exec_id": workflowExecID}).
		Suffix("FOR UPDATE").
		PlaceholderFormat(squirrel.Dollar)

	// Apply organization filtering to prevent cross-org locking
	sb, err := r.queryFilter.ApplyOrgFilter(ctx, sb)
	if err != nil {
		return "", fmt.Errorf("applying organization filter: %w", err)
	}

	lockQuery, args, err := sb.ToSql()
	if err != nil {
		return "", fmt.Errorf("building lock query: %w", err)
	}

	var status string
	err = tx.QueryRow(ctx, lockQuery, args...).Scan(&workflowExecID, &status)
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
		log.Error(
			"Output transformation failed - workflow will be marked as failed",
			"workflow_exec_id",
			workflowExecID,
			"error",
			transformErr,
		)
		// Use default output when transformation fails
		finalOutput = r.createWorkflowOutputMap(tasks)
		*finalStatus = core.StatusFailed
	}
	// Convert output to map
	outputMap, err := r.convertOutputToMap(finalOutput)
	if err != nil {
		return err
	}
	// Update workflow state with error if transformation failed
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
	sb := squirrel.Select("workflow_exec_id", "workflow_id", "org_id", "status", "input", "output", "error").
		From("workflow_states").
		Where(squirrel.Eq{"workflow_exec_id": workflowExecID}).
		PlaceholderFormat(squirrel.Dollar)

	// Apply organization filtering
	sb, err := r.queryFilter.ApplyOrgFilter(ctx, sb)
	if err != nil {
		return nil, fmt.Errorf("applying organization filter: %w", err)
	}

	getQuery, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building get query: %w", err)
	}

	stateDB, err := r.getStateDBWithTx(ctx, tx, getQuery, args...)
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
