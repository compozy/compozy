package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
)

// -----------------------------------------------------------------------------
// CancelExecution
// -----------------------------------------------------------------------------

type CancelExecution struct {
	worker         *worker.Worker
	workflowRepo   workflow.Repository
	workflowExecID core.ID
}

func NewCancelExecution(worker *worker.Worker, workflowExecID core.ID) *CancelExecution {
	return &CancelExecution{
		worker:         worker,
		workflowRepo:   worker.WorkflowRepo(),
		workflowExecID: workflowExecID,
	}
}

func (uc *CancelExecution) Execute(ctx context.Context) error {
	// Get workflow state to retrieve workflowID
	state, err := uc.workflowRepo.GetState(ctx, uc.workflowExecID)
	if err != nil {
		return fmt.Errorf("failed to get workflow state: %w", err)
	}
	if state == nil {
		return fmt.Errorf("workflow state not found for execution %s", uc.workflowExecID)
	}
	return uc.worker.CancelWorkflow(ctx, state.WorkflowID, uc.workflowExecID)
}

// -----------------------------------------------------------------------------
// PauseExecution
// -----------------------------------------------------------------------------

type PauseExecution struct {
	worker         *worker.Worker
	workflowExecID core.ID
}

func NewPauseExecution(worker *worker.Worker, workflowExecID core.ID) *PauseExecution {
	return &PauseExecution{
		worker:         worker,
		workflowExecID: workflowExecID,
	}
}

func (uc *PauseExecution) Execute(_ context.Context) error {
	return fmt.Errorf("not implemented")
}

// -----------------------------------------------------------------------------
// ResumeExecution
// -----------------------------------------------------------------------------

type ResumeExecution struct {
	worker         *worker.Worker
	workflowExecID core.ID
}

func NewResumeExecution(worker *worker.Worker, workflowExecID core.ID) *ResumeExecution {
	return &ResumeExecution{
		worker:         worker,
		workflowExecID: workflowExecID,
	}
}

func (uc *ResumeExecution) Execute(_ context.Context) error {
	return fmt.Errorf("not implemented")
}
