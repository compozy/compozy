package uc

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/worker"
)

// -----------------------------------------------------------------------------
// CancelExecution
// -----------------------------------------------------------------------------

type CancelExecution struct {
	worker         *worker.Worker
	workflowExecID core.ID
}

func NewCancelExecution(worker *worker.Worker, workflowExecID core.ID) *CancelExecution {
	return &CancelExecution{
		worker:         worker,
		workflowExecID: workflowExecID,
	}
}

func (uc *CancelExecution) Execute(ctx context.Context) error {
	return uc.worker.CancelWorkflow(ctx, uc.workflowExecID)
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

func (uc *PauseExecution) Execute(ctx context.Context) error {
	return uc.worker.PauseWorkflow(ctx, uc.workflowExecID)
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

func (uc *ResumeExecution) Execute(ctx context.Context) error {
	return uc.worker.ResumeWorkflow(ctx, uc.workflowExecID)
}
