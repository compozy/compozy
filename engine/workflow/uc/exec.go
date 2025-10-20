package uc

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
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

// -----------------------------------------------------------------------------
// SendSignalToExecution
// -----------------------------------------------------------------------------

type SendSignalToExecution struct {
	worker         *worker.Worker
	workflowRepo   workflow.Repository
	workflowExecID core.ID
	signalName     string
	payload        core.Input
}

func NewSendSignalToExecution(
	worker *worker.Worker,
	workflowExecID core.ID,
	signalName string,
	payload core.Input,
) *SendSignalToExecution {
	return &SendSignalToExecution{
		worker:         worker,
		workflowRepo:   worker.WorkflowRepo(),
		workflowExecID: workflowExecID,
		signalName:     signalName,
		payload:        payload,
	}
}

func (uc *SendSignalToExecution) Execute(ctx context.Context) error {
	state, err := uc.workflowRepo.GetState(ctx, uc.workflowExecID)
	if err != nil {
		return fmt.Errorf("failed to get workflow state: %w", err)
	}
	if state == nil {
		return fmt.Errorf("workflow state not found for execution %s", uc.workflowExecID)
	}
	signal := &task.SignalEnvelope{
		Metadata: task.SignalMetadata{
			SignalID:      core.MustNewID().String(),
			ReceivedAtUTC: time.Now().UTC(),
			WorkflowID:    state.WorkflowID,
			Source:        "api",
		},
		Payload: uc.payload,
	}
	// NOTE: Temporal workflow IDs combine definition + exec for uniqueness.
	temporalWorkflowID := fmt.Sprintf("%s-%s", state.WorkflowID, uc.workflowExecID.String())
	// NOTE: Signal via the Temporal client to deliver payloads to the running workflow.
	return uc.worker.GetClient().SignalWorkflow(
		ctx,
		temporalWorkflowID,
		"", // empty run ID means current run
		uc.signalName,
		signal,
	)
}
