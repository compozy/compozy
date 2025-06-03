package uc

import (
	"context"

	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
)

// -----------------------------------------------------------------------------
// ResumeExecution
// -----------------------------------------------------------------------------

type ResumeExecution struct {
	worker  *worker.Worker
	stateID string
}

func NewResumeExecution(worker *worker.Worker, stateID string) *ResumeExecution {
	return &ResumeExecution{
		worker:  worker,
		stateID: stateID,
	}
}

func (uc *ResumeExecution) Execute(ctx context.Context) error {
	stateID, err := workflow.StateIDFromString(uc.stateID)
	if err != nil {
		return err
	}
	return uc.worker.ResumeWorkflow(ctx, string(stateID.WorkflowExec))
}
