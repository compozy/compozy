package uc

import (
	"context"

	"github.com/compozy/compozy/engine/orchestrator"
	"github.com/compozy/compozy/engine/workflow"
)

// -----------------------------------------------------------------------------
// CancelExecution
// -----------------------------------------------------------------------------

type CancelExecution struct {
	orchestrator *orchestrator.Orchestrator
	stateID      string
}

func NewCancelExecution(orchestrator *orchestrator.Orchestrator, stateID string) *CancelExecution {
	return &CancelExecution{
		orchestrator: orchestrator,
		stateID:      stateID,
	}
}

func (uc *CancelExecution) Execute(ctx context.Context) error {
	stateID, err := workflow.StateIDFromString(uc.stateID)
	if err != nil {
		return err
	}
	return uc.orchestrator.CancelWorkflow(ctx, string(stateID.WorkflowExec))
}
