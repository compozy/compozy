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
	triggerFn := manager.TriggerWorkflow()
	_, err = actHandler(errHandler, triggerFn)(ctx)
	if err != nil {
		return nil, err
	}

	// Dispatch first task
	execFn := manager.ExecuteFirstTask()
	output, err := actHandler(errHandler, execFn)(ctx)
	if err != nil {
		return nil, err
	}

	// Iterate over tasks until get the final one
	for output.NextTask != nil {
		// Create a new task execution function for each iteration
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

	// Complete workflow
	completeFn := manager.CompleteWorkflow()
	wState, err := actHandler(errHandler, completeFn)(ctx)
	if err != nil {
		return nil, err
	}
	return wState, nil
}
