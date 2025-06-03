package uc

import (
	"context"

	"github.com/compozy/compozy/engine/orchestrator"
	"github.com/compozy/compozy/engine/workflow"
)

// -----------------------------------------------------------------------------
// ResumeExecution
// -----------------------------------------------------------------------------

type ResumeExecution struct {
	orchestrator *orchestrator.Orchestrator
	stateID      string
}

func NewResumeExecution(orchestrator *orchestrator.Orchestrator, stateID string) *ResumeExecution {
	return &ResumeExecution{
		orchestrator: orchestrator,
		stateID:      stateID,
	}
}

func (uc *ResumeExecution) Execute(ctx context.Context) error {
	stateID, err := workflow.StateIDFromString(uc.stateID)
	if err != nil {
		return err
	}
	return uc.orchestrator.ResumeWorkflow(ctx, string(stateID.WorkflowExec))
}
