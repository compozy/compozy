package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
)

const TriggerLabel = "WorkflowTrigger"

type TriggerInput struct {
	WorkflowID     string
	WorkflowExecID core.ID
	Input          *core.Input
}

type Trigger struct {
	workflows    []*workflow.Config
	workflowRepo workflow.Repository
}

func NewTrigger(workflows []*workflow.Config, workflowRepo workflow.Repository) *Trigger {
	return &Trigger{
		workflows:    workflows,
		workflowRepo: workflowRepo,
	}
}

func (a *Trigger) Run(ctx context.Context, input *TriggerInput) (*workflow.State, error) {
	workflowID := input.WorkflowID
	_, err := workflow.FindConfig(a.workflows, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}

	repo := a.workflowRepo
	workflowState := workflow.NewState(
		input.WorkflowID,
		input.WorkflowExecID,
		input.Input,
	)
	if err := repo.UpsertState(ctx, workflowState); err != nil {
		return nil, fmt.Errorf("failed to create workflow state: %w", err)
	}

	return workflowState, nil
}
