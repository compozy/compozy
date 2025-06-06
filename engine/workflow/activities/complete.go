package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
)

const CompleteWorkflowLabel = "CompleteWorkflow"

type CompleteWorkflowInput struct {
	WorkflowExecID core.ID `json:"workflow_exec_id"`
}

type CompleteWorkflow struct {
	workflowRepo workflow.Repository
}

func NewCompleteWorkflow(workflowRepo workflow.Repository) *CompleteWorkflow {
	return &CompleteWorkflow{
		workflowRepo: workflowRepo,
	}
}

func (a *CompleteWorkflow) Run(ctx context.Context, input *CompleteWorkflowInput) (*workflow.State, error) {
	state, err := a.workflowRepo.CompleteWorkflow(ctx, input.WorkflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to complete workflow %s: %w", input.WorkflowExecID, err)
	}
	return state, nil
}
