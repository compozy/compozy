package worker

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	wf "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

type WorkflowExecutor struct {
	*ContextBuilder
}

func NewWorkflowExecutor(contextBuilder *ContextBuilder) *WorkflowExecutor {
	return &WorkflowExecutor{ContextBuilder: contextBuilder}
}

func (e *WorkflowExecutor) TriggerWorkflow() func(ctx workflow.Context) (*wf.State, error) {
	return func(ctx workflow.Context) (*wf.State, error) {
		log := workflow.GetLogger(ctx)
		var state *wf.State
		actLabel := wfacts.TriggerLabel
		actInput := &wfacts.TriggerInput{
			WorkflowID:     e.WorkflowID,
			WorkflowExecID: e.WorkflowExecID,
			Input:          e.Input,
			InitialTaskID:  e.InitialTaskID,
		}
		ctx = e.BuildTaskContext(ctx, e.InitialTaskID)
		err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &state)
		if err != nil {
			return nil, err
		}
		log.Info("Workflow trigger",
			"workflow_id", e.WorkflowID,
			"exec_id", e.WorkflowExecID,
		)
		return state, nil
	}
}

func (e *WorkflowExecutor) CompleteWorkflow() func(ctx workflow.Context) (*wf.State, error) {
	return func(ctx workflow.Context) (*wf.State, error) {
		log := workflow.GetLogger(ctx)

		// Configure specific activity options for CompleteWorkflow to handle race conditions
		activityOptions := workflow.ActivityOptions{
			StartToCloseTimeout: 2 * time.Minute,
			HeartbeatTimeout:    10 * time.Second,
			RetryPolicy: &temporal.RetryPolicy{
				InitialInterval:    500 * time.Millisecond, // Start retrying quickly
				BackoffCoefficient: 1.5,                    // Moderate backoff
				MaximumInterval:    5 * time.Second,        // Cap retry interval
				MaximumAttempts:    10,                     // Fit within StartToCloseTimeout window
			},
		}

		// Override the context with specific options for CompleteWorkflow
		ctx = workflow.WithActivityOptions(ctx, activityOptions)

		actLabel := wfacts.CompleteWorkflowLabel
		actInput := &wfacts.CompleteWorkflowInput{
			WorkflowExecID: e.WorkflowExecID,
		}
		var output *wf.State
		err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &output)
		if err != nil {
			return nil, err
		}
		log.Info("Workflow completed",
			"workflow_id", e.WorkflowID,
			"exec_id", e.WorkflowExecID,
		)
		return output, nil
	}
}
