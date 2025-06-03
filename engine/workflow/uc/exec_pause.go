package uc

import (
	"context"

	"github.com/compozy/compozy/engine/orchestrator"
	"github.com/compozy/compozy/engine/workflow"
)

// -----------------------------------------------------------------------------
// PauseExecution
// -----------------------------------------------------------------------------

type PauseExecution struct {
	orchestrator *orchestrator.Orchestrator
	stateID      string
}

func NewPauseExecution(orchestrator *orchestrator.Orchestrator, stateID string) *PauseExecution {
	return &PauseExecution{
		orchestrator: orchestrator,
		stateID:      stateID,
	}
}

func (uc *PauseExecution) Execute(ctx context.Context) error {
	stateID, err := workflow.StateIDFromString(uc.stateID)
	if err != nil {
		return err
	}
	return uc.orchestrator.PauseWorkflow(ctx, string(stateID.WorkflowExec))
}
