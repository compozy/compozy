package store

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/jackc/pgx/v5"
)

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
		// Default behavior
		return r.createWorkflowOutputMap(tasks), nil
	}
	// Get current workflow state for transformation
	query := `
		SELECT workflow_exec_id, workflow_id, status, input, output, error
		FROM workflow_states
		WHERE workflow_exec_id = $1
	`
	stateDB, err := r.getStateDBWithTx(ctx, tx, query, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow state: %w", err)
	}
	state, err := stateDB.ToState()
	if err != nil {
		return nil, fmt.Errorf("failed to convert workflow state: %w", err)
	}
	// Populate tasks for the state
	state.Tasks = tasks
	// Apply output transformation
	transformedOutput, err := outputTransformer(state)
	if err != nil {
		// Create error object but don't update DB - let caller handle it
		*finalStatus = core.StatusFailed
		return nil, fmt.Errorf("workflow output transformation failed: %w", err)
	}
	return transformedOutput, nil
}
