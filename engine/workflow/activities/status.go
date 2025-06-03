package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
)

const UpdateWorkflowStatusLabel = "UpdateWorkflowStatus"

type UpdateWorkflowStatusInput struct {
	WorkflowID     string
	WorkflowExecID core.ID
	NewStatus      core.StatusType
}

type UpdateWorkflowStatusActivity struct {
	workflowRepo workflow.Repository
}

func NewUpdateWorkflowStatusActivity(workflowRepo workflow.Repository) *UpdateWorkflowStatusActivity {
	return &UpdateWorkflowStatusActivity{
		workflowRepo: workflowRepo,
	}
}

func (a *UpdateWorkflowStatusActivity) Run(ctx context.Context, input *UpdateWorkflowStatusInput) error {
	stateID := workflow.StateID{
		WorkflowID:   input.WorkflowID,
		WorkflowExec: input.WorkflowExecID,
	}

	state, err := a.workflowRepo.GetState(ctx, stateID)
	if err != nil {
		return fmt.Errorf("failed to get workflow state %s: %w", stateID.String(), err)
	}

	state.UpdateStatus(input.NewStatus)

	if err := a.workflowRepo.UpsertState(ctx, state); err != nil {
		return fmt.Errorf("failed to upsert workflow state %s with new status %s: %w", stateID.String(), input.NewStatus, err)
	}
	return nil
}
