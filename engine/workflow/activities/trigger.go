package activities

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
)

const TriggerLabel = "TriggerWorkflow"

type TriggerInput struct {
	WorkflowID     string      `json:"workflow_id"`
	WorkflowExecID core.ID     `json:"workflow_exec_id"`
	Input          *core.Input `json:"input"`
	InitialTaskID  string
	// ErrorHandlerTimeout bounds error-handling activities; zero uses worker defaults.
	ErrorHandlerTimeout time.Duration `json:"error_handler_timeout"`
	// ErrorHandlerMaxRetries limits retries for error-handling activities; zero uses worker defaults.
	ErrorHandlerMaxRetries int `json:"error_handler_max_retries"`
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
	wfState := workflow.NewState(
		input.WorkflowID,
		input.WorkflowExecID,
		input.Input,
	)
	if err := repo.UpsertState(ctx, wfState); err != nil {
		return nil, err
	}
	return wfState, nil
}
