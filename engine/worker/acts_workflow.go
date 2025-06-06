package worker

import (
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

func (e *WorkflowExecutor) TriggerWorkflow(ctx workflow.Context) func() (*wf.State, error) {
	logger := workflow.GetLogger(ctx)
	return func() (*wf.State, error) {
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
		logger.Info("Workflow trigger",
			"workflow_id", e.WorkflowID,
			"exec_id", e.WorkflowExecID,
		)
		return state, nil
	}
}

func (e *WorkflowExecutor) CompleteWorkflow(ctx workflow.Context) func() (*wf.State, error) {
	logger := workflow.GetLogger(ctx)
	return func() (*wf.State, error) {
		actLabel := wfacts.CompleteWorkflowLabel
		actInput := &wfacts.CompleteWorkflowInput{
			WorkflowExecID: e.WorkflowExecID,
		}
		var output *wf.State
		err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &output)
		if err != nil {
			return nil, err
		}
		logger.Info("Workflow completed",
			"workflow_id", e.WorkflowID,
			"exec_id", e.WorkflowExecID,
		)
		return output, nil
	}
}
