package worker

import (
	"go.temporal.io/sdk/workflow"

	wf "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

// CompozyWorkflowName is the Temporal workflow type name for all Compozy workflows
const CompozyWorkflowName = "CompozyWorkflow"

type WorkflowInput = wfacts.TriggerInput

func CompozyWorkflow(ctx workflow.Context, input WorkflowInput) (*wf.State, error) {
	manager, err := InitManager(ctx, input)
	defer manager.CancelCleanup(ctx)
	if err != nil {
		return nil, err
	}
	ctx = manager.BuildBaseContext(ctx)
	errHandler := manager.BuildErrHandler(ctx)
	triggerFn := manager.TriggerWorkflow()
	_, err = actHandler(errHandler, triggerFn)(ctx)
	if err != nil {
		return nil, err
	}
	execFn := manager.ExecuteFirstTask()
	output, err := actHandler(errHandler, execFn)(ctx)
	if err != nil {
		return nil, err
	}
	for output.GetNextTask() != nil {
		taskFn := manager.ExecuteTasks(output)
		nextTask, err := actHandler(errHandler, taskFn)(ctx)
		if err != nil {
			return nil, err
		}
		if nextTask == nil {
			break
		}
		output = nextTask
	}
	completeFn := manager.CompleteWorkflow()
	wState, err := actHandler(errHandler, completeFn)(ctx)
	if err != nil {
		return nil, err
	}
	return wState, nil
}
