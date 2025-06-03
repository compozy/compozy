package uc

import (
	"context"

	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
)

// -----------------------------------------------------------------------------
// CancelExecution
// -----------------------------------------------------------------------------

type CancelExecution struct {
	worker  *worker.Worker
	stateID string
}

func NewCancelExecution(worker *worker.Worker, stateID string) *CancelExecution {
	return &CancelExecution{
		worker:  worker,
		stateID: stateID,
	}
}

func (uc *CancelExecution) Execute(ctx context.Context) error {
	stateID, err := workflow.StateIDFromString(uc.stateID)
	if err != nil {
		return err
	}
	return uc.worker.CancelWorkflow(ctx, string(stateID.WorkflowExec))
}
