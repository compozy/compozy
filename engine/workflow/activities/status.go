package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
)

const UpdateWorkflowStatusLabel = "UpdateWorkflowStatus"

type UpdateStatusInput struct {
	WorkflowID     string
	WorkflowExecID core.ID
	Status         core.StatusType
}

type UpdateStatus struct {
	workflowRepo workflow.Repository
}

func NewUpdateStatus(workflowRepo workflow.Repository) *UpdateStatus {
	return &UpdateStatus{
		workflowRepo: workflowRepo,
	}
}

func (a *UpdateStatus) Run(ctx context.Context, input *UpdateStatusInput) error {
	workflowExecID := input.WorkflowExecID.String()
	if err := a.workflowRepo.UpdateStatus(ctx, workflowExecID, input.Status); err != nil {
		return fmt.Errorf("failed to update workflow %s with new status %s: %w", workflowExecID, input.Status, err)
	}
	return nil
}
