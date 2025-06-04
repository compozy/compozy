package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
)

const UpdateStateLabel = "UpdateWorkflowState"

type UpdateStateInput struct {
	WorkflowID     string          `json:"workflow_id"`
	WorkflowExecID core.ID         `json:"workflow_exec_id"`
	Status         core.StatusType `json:"status"`
	Error          *core.Error     `json:"error"`
	Output         *core.Output    `json:"output"`
}

type UpdateState struct {
	workflowRepo workflow.Repository
}

func NewUpdateState(workflowRepo workflow.Repository) *UpdateState {
	return &UpdateState{
		workflowRepo: workflowRepo,
	}
}

func (a *UpdateState) Run(ctx context.Context, input *UpdateStateInput) error {
	workflowExecID := input.WorkflowExecID
	state, err := a.workflowRepo.GetState(ctx, workflowExecID)
	if err != nil {
		return fmt.Errorf("failed to get workflow %s: %w", input.WorkflowExecID, err)
	}
	state.WithStatus(input.Status)
	if input.Error != nil {
		state.WithError(input.Error)
	}
	if input.Output != nil {
		state.WithOutput(input.Output)
	}
	if err := a.workflowRepo.UpsertState(ctx, state); err != nil {
		return fmt.Errorf("failed to update workflow %s: %w", input.WorkflowExecID, err)
	}
	return nil
}
