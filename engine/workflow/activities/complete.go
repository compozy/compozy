package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	wf "github.com/compozy/compozy/engine/workflow"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

const CompleteWorkflowLabel = "CompleteWorkflow"

type CompleteWorkflowInput struct {
	WorkflowExecID core.ID `json:"workflow_exec_id"`
}

type CompleteWorkflow struct {
	workflowRepo wf.Repository
}

func NewCompleteWorkflow(workflowRepo wf.Repository) *CompleteWorkflow {
	return &CompleteWorkflow{
		workflowRepo: workflowRepo,
	}
}

func (a *CompleteWorkflow) Run(ctx context.Context, input *CompleteWorkflowInput) (*wf.State, error) {
	// Add heartbeat to ensure activity stays alive during retries
	activity.RecordHeartbeat(ctx, "Attempting to complete workflow")
	state, err := a.workflowRepo.CompleteWorkflow(ctx, input.WorkflowExecID)
	if err != nil {
		// Check if this is the specific error indicating tasks are not ready for completion
		if err == store.ErrWorkflowNotFound {
			// Create a retryable application error to trigger Temporal's retry mechanism
			return nil, temporal.NewApplicationError(
				fmt.Sprintf("workflow %s not ready for completion, tasks still running", input.WorkflowExecID),
				"workflow_not_ready",
				err,
			)
		}
		return nil, fmt.Errorf("failed to complete workflow %s: %w", input.WorkflowExecID, err)
	}
	return state, nil
}
