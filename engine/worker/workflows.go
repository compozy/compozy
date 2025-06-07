package worker

import (
	"go.temporal.io/sdk/workflow"

	wf "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

type WorkflowInput = wfacts.TriggerInput

func CompozyWorkflow(ctx workflow.Context, input WorkflowInput) (*wf.State, error) {
	// Initialize manager and get workflow data
	manager, err := InitManager(ctx, input)
	defer manager.CancelCleanup(ctx)
	if err != nil {
		return nil, err
	}

	// Setup activity context and error handler
	ctx = manager.BuildBaseContext(ctx)
	errHandler := manager.BuildErrHandler(ctx)

	// Execute main trigger activity
	triggerFn := manager.TriggerWorkflow(ctx)
	_, err = actHandler(ctx, errHandler, triggerFn)()
	if err != nil {
		return nil, err
	}

	// Dispatch first task
	dispatchFn := manager.DispatchFirstTask(ctx)
	output, err := actHandler(ctx, errHandler, dispatchFn)()
	if err != nil {
		return nil, err
	}

	// Iterate over tasks until get the final one
	currentTask := output.TaskState
	taskFn := manager.ExecuteTaskLoop(ctx, currentTask, output)
	for currentTask != nil {
		nextTask, err := actHandler(ctx, errHandler, taskFn)()
		if err != nil {
			return nil, err
		}
		if nextTask == nil {
			break
		}
		currentTask = nextTask
	}

	// Complete workflow
	completeFn := manager.CompleteWorkflow(ctx)
	wState, err := actHandler(ctx, errHandler, completeFn)()
	if err != nil {
		return nil, err
	}
	return wState, nil
}
