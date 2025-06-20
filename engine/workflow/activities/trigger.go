package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
)

const TriggerLabel = "TriggerWorkflow"

type TriggerInput struct {
	WorkflowID     string      `json:"workflow_id"`
	WorkflowExecID core.ID     `json:"workflow_exec_id"`
	OrgID          core.ID     `json:"org_id"`
	Input          *core.Input `json:"input"`
	InitialTaskID  string
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
	repo := a.workflowRepo
	// Validate OrgID is not empty
	if input.OrgID == "" {
		return nil, fmt.Errorf("org_id cannot be empty")
	}
	wfState := workflow.NewState(
		input.WorkflowID,
		input.WorkflowExecID,
		input.OrgID,
		input.Input,
	)
	if err := repo.UpsertState(ctx, wfState); err != nil {
		return nil, err
	}
	return wfState, nil
}
