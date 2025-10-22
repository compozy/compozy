package worker

import (
	"go.temporal.io/sdk/workflow"

	wf "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

// CompozyWorkflowName is the Temporal workflow type name for all Compozy workflows
const CompozyWorkflowName = "CompozyWorkflow"

type WorkflowInput = wfacts.TriggerInput

func CompozyWorkflow(ctx workflow.Context, input WorkflowInput) (res *wf.State, err error) {
	tracker, err := newWorkflowStreamTracker(ctx, input)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tracker.Fail(ctx, err)
			return
		}
		tracker.Success(ctx, res)
	}()
	manager, err := InitManager(ctx, input)
	if err != nil {
		return nil, err
	}
	defer manager.CancelCleanup(ctx)
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
		nextTask, nextErr := actHandler(errHandler, taskFn)(ctx)
		if nextErr != nil {
			err = nextErr
			return nil, err
		}
		if nextTask == nil {
			break
		}
		output = nextTask
	}
	completeFn := manager.CompleteWorkflow()
	res, err = actHandler(errHandler, completeFn)(ctx)
	return res, err
}
