package worker

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
	wf "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

type WorkflowInput = wfacts.TriggerInput

// -----------------------------------------------------------------------------
// Workflow Definition
// -----------------------------------------------------------------------------

func CompozyWorkflow(ctx workflow.Context, input WorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting workflow", "workflow_id", input.WorkflowID, "exec_id", input.WorkflowExecID)
	ctx = initialContext(ctx)

	// Execute main trigger activity
	logger.Info("Executing main trigger activity...")
	wState, err := triggerWorkflow(ctx, &input)
	if err != nil {
		logger.Error("Failed to execute trigger activity", "error", err)
		return err
	}

	// Setup signals for PAUSE/RESUME/CANCEL
	ctx, cancel := workflow.WithCancel(ctx)
	errorHandler := BuildErrorHandler(ctx, &input)
	pauseGate, err := RegisterSignals(ctx, cancel, &input)
	if err != nil {
		return errorHandler(err)
	}

	// Dispatch and execute first task
	taskState, err := dispatchFirstTask(ctx, pauseGate, wState, &input)
	if err != nil {
		return errorHandler(err)
	}

	// Execute tasks
	err = executeTasks(ctx, pauseGate, taskState)
	if err != nil {
		return errorHandler(err)
	}

	// Complete workflow
	err = completeWorkflow(ctx, pauseGate, wState)
	if err != nil {
		return errorHandler(err)
	}
	logger.Info("Workflow completed",
		"workflow_id", input.WorkflowID,
		"exec_id", input.WorkflowExecID,
	)
	return nil
}

// -----------------------------------------------------------------------------
// Context
// -----------------------------------------------------------------------------

func initialContext(ctx workflow.Context) workflow.Context {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	})
	return ctx
}

// -----------------------------------------------------------------------------
// Workflow Functions
// -----------------------------------------------------------------------------

func triggerWorkflow(
	ctx workflow.Context,
	input *wfacts.TriggerInput,
) (*wf.State, error) {
	var state *wf.State
	actLabel := wfacts.TriggerLabel
	actInput := &wfacts.TriggerInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		Input:          input.Input,
		InitialTaskID:  input.InitialTaskID,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func completeWorkflow(
	ctx workflow.Context,
	pauseGate *PauseGate,
	wState *wf.State,
) error {
	if err := pauseGate.Await(); err != nil {
		return err
	}
	actLabel := wfacts.UpdateStateLabel
	actInput := &wfacts.UpdateStateInput{
		WorkflowID:     wState.WorkflowID,
		WorkflowExecID: wState.WorkflowExecID,
		Status:         core.StatusSuccess,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

// -----------------------------------------------------------------------------
// Task Functions
// -----------------------------------------------------------------------------

func dispatchFirstTask(
	ctx workflow.Context,
	pauseGate *PauseGate,
	wState *wf.State,
	input *wfacts.TriggerInput,
) (*task.State, error) {
	if err := pauseGate.Await(); err != nil {
		return nil, err
	}
	var state *task.State
	actLabel := tkacts.DispatchLabel
	actInput := &tkacts.DispatchInput{
		WorkflowID:     wState.WorkflowID,
		WorkflowExecID: wState.WorkflowExecID,
		TaskID:         input.InitialTaskID,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func executeBasicTask(
	ctx workflow.Context,
	pauseGate *PauseGate,
	taskState *task.State,
) (*task.Response, error) {
	// Check pause state before starting execution
	if err := pauseGate.Await(); err != nil {
		return nil, err
	}

	var response *task.Response
	actLabel := tkacts.ExecuteBasicLabel
	actInput := &tkacts.ExecuteBasicInput{
		State: *taskState,
	}

	// Use a selector to handle both task completion and cancellation
	future := workflow.ExecuteActivity(ctx, actLabel, actInput)
	selector := workflow.NewSelector(ctx)

	taskCompleted := false
	var taskError error

	selector.AddFuture(future, func(f workflow.Future) {
		taskError = f.Get(ctx, &response)
		taskCompleted = true
	})

	// Run the selector until task completes or context is canceled
	for !taskCompleted {
		selector.Select(ctx)
		if ctx.Err() == workflow.ErrCanceled {
			return nil, workflow.ErrCanceled
		}

		// Check if paused (this will block until resumed)
		if pauseGate.IsPaused() {
			if err := pauseGate.Await(); err != nil {
				return nil, err
			}
		}
	}
	if taskError != nil {
		return nil, taskError
	}
	return response, nil
}

func dispatchTask(
	ctx workflow.Context,
	pauseGate *PauseGate,
	currentTaskState *task.State,
	nextTaskID string,
) (*task.State, error) {
	if err := pauseGate.Await(); err != nil {
		return nil, err
	}
	var state *task.State
	actLabel := tkacts.DispatchLabel
	actInput := &tkacts.DispatchInput{
		WorkflowID:     currentTaskState.WorkflowID,
		WorkflowExecID: currentTaskState.WorkflowExecID,
		TaskID:         nextTaskID,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func executeTasks(
	ctx workflow.Context,
	pauseGate *PauseGate,
	taskState *task.State,
) error {
	logger := workflow.GetLogger(ctx)
	currentTask := taskState
	for currentTask != nil {
		// Check pause state before each task
		if err := pauseGate.Await(); err != nil {
			return err
		}

		logger.Info("Executing task", "task_id", currentTask.TaskID, "task_exec_id", currentTask.TaskExecID)
		response, err := executeBasicTask(ctx, pauseGate, currentTask)
		if err != nil {
			if err := checkCancellation(ctx, err, "Task execution canceled"); err != nil {
				return err
			}
			logger.Error("Failed to execute task", "task_id", currentTask.TaskID, "error", err)
			return err
		}

		logger.Info("Task executed successfully",
			"task_id", currentTask.TaskID,
			"status", response.State.Status,
		)

		// Handle task transitions and determine next task
		var nextTaskID string
		switch {
		case response.State.Status == core.StatusSuccess && response.OnSuccess != nil && response.OnSuccess.Next != nil:
			nextTaskID = *response.OnSuccess.Next
			logger.Info("Task succeeded, transitioning to next task",
				"current_task", currentTask.TaskID,
				"next_task", nextTaskID,
			)
		case response.State.Status == core.StatusFailed && response.OnError != nil && response.OnError.Next != nil:
			nextTaskID = *response.OnError.Next
			logger.Info("Task failed, transitioning to error task",
				"current_task", currentTask.TaskID,
				"next_task", nextTaskID,
			)
		default:
			logger.Info("No more tasks to execute", "current_task", currentTask.TaskID)
		}

		// Dispatch next task if there is one
		if nextTaskID != "" {
			nextTaskState, err := dispatchTask(ctx, pauseGate, response.State, nextTaskID)
			if err != nil {
				if err := checkCancellation(ctx, err, "Task dispatch canceled"); err != nil {
					return err
				}
				logger.Error("Failed to dispatch next task", "next_task", nextTaskID, "error", err)
				return err
			}
			currentTask = nextTaskState
		} else {
			currentTask = nil
		}
	}

	return nil
}
