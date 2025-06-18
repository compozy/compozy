package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/normalizer"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

const CompleteWorkflowLabel = "CompleteWorkflow"

type CompleteWorkflowInput struct {
	WorkflowExecID core.ID `json:"workflow_exec_id"`
	WorkflowID     string  `json:"workflow_id"`
}

type CompleteWorkflow struct {
	workflowRepo wf.Repository
	workflows    []*wf.Config
	normalizer   *normalizer.ConfigNormalizer
}

func NewCompleteWorkflow(workflowRepo wf.Repository, workflows []*wf.Config) *CompleteWorkflow {
	return &CompleteWorkflow{
		workflowRepo: workflowRepo,
		workflows:    workflows,
		normalizer:   normalizer.NewConfigNormalizer(),
	}
}

func (a *CompleteWorkflow) Run(ctx context.Context, input *CompleteWorkflowInput) (*wf.State, error) {
	// Add heartbeat to ensure activity stays alive during retries
	activity.RecordHeartbeat(ctx, "Attempting to complete workflow")

	// Find the workflow config
	var config *wf.Config
	for _, wfConfig := range a.workflows {
		if wfConfig.ID == input.WorkflowID {
			config = wfConfig
			break
		}
	}

	// Create transformer if outputs are defined
	var transformer wf.OutputTransformer
	if config != nil && config.GetOutputs() != nil {
		transformer = func(state *wf.State) (*core.Output, error) {
			return a.normalizer.NormalizeWorkflowOutput(state, config.GetOutputs())
		}
	}

	// Complete workflow with optional transformer
	state, err := a.workflowRepo.CompleteWorkflow(ctx, input.WorkflowExecID, transformer)
	if err != nil {
		// Check if this is the specific error indicating tasks are not ready for completion
		if err == store.ErrWorkflowNotReady {
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
