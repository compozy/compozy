package uc

import (
	"context"

	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
)

// -----------------------------------------------------------------------------
// PauseExecution
// -----------------------------------------------------------------------------

type PauseExecution struct {
	worker  *worker.Worker
	stateID string
}

func NewPauseExecution(worker *worker.Worker, stateID string) *PauseExecution {
	return &PauseExecution{
		worker:  worker,
		stateID: stateID,
	}
}

func (uc *PauseExecution) Execute(ctx context.Context) error {
	stateID, err := workflow.StateIDFromString(uc.stateID)
	if err != nil {
		return err
	}
	return uc.worker.PauseWorkflow(ctx, string(stateID.WorkflowExec))
}
