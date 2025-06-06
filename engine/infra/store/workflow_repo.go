package store

import (
	"context"
	"errors"
	"fmt"

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
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			err := tx.Rollback(ctx)
			if err != nil {
				logger.Error("error rolling back transaction", "error", err)
			}
			panic(p)
		} else if err != nil {
			err := tx.Rollback(ctx)
			if err != nil {
				logger.Error("error rolling back transaction", "error", err)
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

// CompleteWorkflow collects all task outputs and saves them as workflow output
func (r *WorkflowRepo) CompleteWorkflow(ctx context.Context, workflowExecID core.ID) (*workflow.State, error) {
	var result *workflow.State

	err := r.withTransaction(ctx, func(tx pgx.Tx) error {
		// Get all task states for this workflow execution
		tasks, err := r.listTasksInWorkflowWithTx(ctx, tx, workflowExecID)
		if err != nil {
			return fmt.Errorf("failed to get task states: %w", err)
		}

		// Create output map: task_id -> Output
		outputMap := make(map[string]any)
		for taskID, taskState := range tasks {
			if taskState.Output != nil {
				outputMap[taskID] = map[string]any{
					"output": taskState.Output,
				}
			}
		}

		// Convert output map to JSONB
		outputJSON, err := ToJSONB(outputMap)
		if err != nil {
			return fmt.Errorf("marshaling workflow output: %w", err)
		}

		// Update the workflow state with the collected outputs and success status
		query := `
			UPDATE workflow_states
			SET output = $1, status = $2, updated_at = now()
			WHERE workflow_exec_id = $3
		`

		cmdTag, err := tx.Exec(ctx, query, outputJSON, core.StatusSuccess, workflowExecID)
		if err != nil {
			return fmt.Errorf("updating workflow output: %w", err)
		}

		if cmdTag.RowsAffected() == 0 {
			return ErrWorkflowNotFound
		}

		// Get the updated workflow state
		getQuery := `
			SELECT workflow_exec_id, workflow_id, status, input, output, error
			FROM workflow_states
			WHERE workflow_exec_id = $1
		`

		stateDB, err := r.getStateDBWithTx(ctx, tx, getQuery, workflowExecID)
		if err != nil {
			return fmt.Errorf("fetching updated workflow state: %w", err)
		}

		state, err := stateDB.ToState()
		if err != nil {
			return fmt.Errorf("converting updated workflow state: %w", err)
		}

		// Populate child task states within the same transaction
		if err := r.populateTaskStatesWithTx(ctx, tx, state); err != nil {
			return fmt.Errorf("populating task states: %w", err)
		}

		result = state
		return nil
	})

	return result, err
}
