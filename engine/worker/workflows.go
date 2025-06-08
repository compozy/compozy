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
	logger := workflow.GetLogger(ctx)
	logger.Info("About to execute first task")
	
	execFn := manager.ExecuteFirstTask()
	logger.Info("ExecuteFirstTask function created")
	
	output, err := actHandler(errHandler, execFn)(ctx)
	logger.Info("ActHandler call completed", "has_error", err != nil)
	
	if err != nil {
		logger.Error("ExecuteFirstTask failed", "error", err)
		return nil, err
	}

	logger.Info("ExecuteFirstTask completed", "output_is_nil", output == nil)
	if output != nil {
		logger.Info("Output details", "next_task_is_nil", output.NextTask == nil, "state_is_nil", output.State == nil)
	}

	// Iterate over tasks until get the final one
	logger.Info("Task iteration starting", "has_next_task", output != nil && output.NextTask != nil)
	
	for output.NextTask != nil {
		logger.Info("Starting next task iteration", "current_next_task", output.NextTask.ID)

		// Create a new task execution function for each iteration
		taskFn := manager.ExecuteTasks(output)
		nextTask, err := actHandler(errHandler, taskFn)(ctx)
		if err != nil {
			logger.Error("Task execution failed", "error", err)
			return nil, err
		}
		if nextTask == nil {
			logger.Info("ExecuteTasks returned nil, breaking loop")
			break
		}

		// Add nil checks to prevent dereferencing nil pointers
		if nextTask.State == nil {
			logger.Error("NextTask has nil State, breaking loop")
			break
		}

		logger.Info("Task execution completed", "task_status", nextTask.State.Status, "has_next_task", nextTask.NextTask != nil)
		// Check if the returned task response has a valid NextTask before continuing
		if nextTask.NextTask == nil {
			logger.Info("No more tasks to execute, setting final output and breaking")
			output = nextTask
			break
		}
		output = nextTask
	}

	logger.Info("Task iteration complete, proceeding to CompleteWorkflow")
	
	// Complete workflow
	completeFn := manager.CompleteWorkflow()
	logger.Info("About to call CompleteWorkflow activity")
	wState, err := actHandler(errHandler, completeFn)(ctx)
	if err != nil {
		logger.Error("CompleteWorkflow failed", "error", err)
		return nil, err
	}
	logger.Info("CompleteWorkflow succeeded")
	return wState, nil
}
