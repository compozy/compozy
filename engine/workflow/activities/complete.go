package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
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
	workflows    map[string]*wf.Config
	normalizer   *normalizer.ConfigNormalizer
}

func NewCompleteWorkflow(workflowRepo wf.Repository, workflows []*wf.Config) *CompleteWorkflow {
	workflowMap := make(map[string]*wf.Config, len(workflows))
	for _, wf := range workflows {
		workflowMap[wf.ID] = wf
	}
	return &CompleteWorkflow{
		workflowRepo: workflowRepo,
		workflows:    workflowMap,
		normalizer:   normalizer.NewConfigNormalizer(),
	}
}

func (a *CompleteWorkflow) Run(ctx context.Context, input *CompleteWorkflowInput) (*wf.State, error) {
	// Add heartbeat to ensure activity stays alive during retries
	activity.RecordHeartbeat(ctx, "Attempting to complete workflow")
	log := logger.FromContext(ctx)

	// Find the workflow config
	config, exists := a.workflows[input.WorkflowID]
	if !exists {
		return nil, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("unknown workflow ID: %s", input.WorkflowID),
			"unknown_workflow_id",
			nil,
		)
	}

	// Create transformer if outputs are defined
	var transformer wf.OutputTransformer
	if config.GetOutputs() != nil {
		transformer = func(state *wf.State) (*core.Output, error) {
			output, err := a.normalizer.NormalizeWorkflowOutput(ctx, state, config, config.GetOutputs())
			if err != nil {
				log.Error("Output transformation failed",
					"workflow_id", state.WorkflowID,
					"workflow_exec_id", state.WorkflowExecID,
					"error", err)
				return nil, temporal.NewNonRetryableApplicationError(
					fmt.Sprintf("failed to normalize workflow output: %v", err),
					"normalization_failed",
					err,
				)
			}
			return output, nil
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
