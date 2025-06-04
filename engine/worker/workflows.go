package worker

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
	wf "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

type WorkflowInput = wfacts.TriggerInput
type WorkflowData = wfacts.GetData

func CompozyWorkflow(ctx workflow.Context, input WorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting workflow", "workflow_id", input.WorkflowID, "exec_id", input.WorkflowExecID)

	// Get workflow data
	data, err := getWorkflowData(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to get workflow data: %w", err)
	}

	// Initial context
	projectConfig := data.ProjectConfig
	workflowConfig, err := wf.FindConfig(data.Workflows, input.WorkflowID)
	data.WorkflowConfig = workflowConfig
	if err != nil {
		return fmt.Errorf("failed to find workflow config: %w", err)
	}
	ctx = activityContext(ctx, projectConfig, workflowConfig)

	// Execute main trigger activity
	logger.Info("Executing main trigger activity...")
	wState, err := triggerWorkflow(ctx, data, &input)
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
	firstTaskState, err := dispatchFirstTask(ctx, data, pauseGate, wState, &input)
	if err != nil {
		return errorHandler(err)
	}

	// Execute tasks
	err = executeTasks(ctx, data, pauseGate, firstTaskState)
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

func activityContext(
	ctx workflow.Context,
	projectConfig *project.Config,
	workflowConfig *wf.Config,
) workflow.Context {
	resolved := core.ResolveActivityOptions(
		&projectConfig.Opts.GlobalOpts,
		&workflowConfig.Opts.GlobalOpts,
		nil,
	)
	activityOptions := resolved.ToTemporalActivityOptions()
	return workflow.WithActivityOptions(ctx, activityOptions)
}

func activityContextForTask(
	ctx workflow.Context,
	projectConfig *project.Config,
	workflowConfig *wf.Config,
	taskID string,
) workflow.Context {
	taskConfig, err := task.FindConfig(workflowConfig.Tasks, taskID)
	if err != nil {
		return ctx
	}
	resolved := core.ResolveActivityOptions(
		&projectConfig.Opts.GlobalOpts,
		&workflowConfig.Opts.GlobalOpts,
		&taskConfig.Opts.GlobalOpts,
	)
	activityOptions := resolved.ToTemporalActivityOptions()
	return workflow.WithActivityOptions(ctx, activityOptions)
}

// -----------------------------------------------------------------------------
// Workflow Functions
// -----------------------------------------------------------------------------

func getWorkflowData(ctx workflow.Context, input WorkflowInput) (*WorkflowData, error) {
	ctx = workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	})
	actLabel := wfacts.GetDataLabel
	actInput := &wfacts.GetDataInput{WorkflowID: input.WorkflowID}
	var data *wfacts.GetData
	err := workflow.ExecuteLocalActivity(ctx, actLabel, actInput).Get(ctx, &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func triggerWorkflow(
	ctx workflow.Context,
	data *WorkflowData,
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
	ctx = activityContextForTask(
		ctx,
		data.ProjectConfig,
		data.WorkflowConfig,
		input.InitialTaskID,
	)
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
	data *WorkflowData,
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
	ctx = activityContextForTask(
		ctx,
		data.ProjectConfig,
		data.WorkflowConfig,
		input.InitialTaskID,
	)
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func executeTasks(
	ctx workflow.Context,
	data *WorkflowData,
	pauseGate *PauseGate,
	firstTaskState *task.State,
) error {
	logger := workflow.GetLogger(ctx)
	currentTask := firstTaskState
	for currentTask != nil {
		// Check pause state before each task
		if err := pauseGate.Await(); err != nil {
			return err
		}

		logger.Info("Executing task", "task_id", currentTask.TaskID, "task_exec_id", currentTask.TaskExecID)
		ctx = activityContextForTask(ctx, data.ProjectConfig, data.WorkflowConfig, currentTask.TaskID)
		response, err := executeBasicTask(ctx, data, pauseGate, currentTask)
		if err != nil {
			if err := checkCancellation(ctx, err, "Task execution canceled"); err != nil {
				return err
			}
			logger.Error("Failed to execute task", "task_id", currentTask.TaskID, "error", err)
			return err
		}

		logger.Info("Task executed successfully",
			"status", response.State.Status,
			"task_id", currentTask.TaskID,
		)

		// Dispatch next task if there is one
		nextTaskID := response.NextTaskID()
		if nextTaskID != "" {
			currentTaskState := response.State
			nextTaskState, err := dispatchTask(ctx, pauseGate, currentTaskState, nextTaskID)
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

func executeBasicTask(
	ctx workflow.Context,
	data *WorkflowData,
	pauseGate *PauseGate,
	taskState *task.State,
) (*task.Response, error) {
	// Check pause state before starting execution
	if err := pauseGate.Await(); err != nil {
		return nil, err
	}
	taskConfig, err := task.FindConfig(data.WorkflowConfig.Tasks, taskState.TaskID)
	if err != nil {
		return nil, err
	}

	var response *task.Response
	actLabel := tkacts.ExecuteBasicLabel
	actInput := &tkacts.ExecuteBasicInput{
		State:  *taskState,
		Config: *taskConfig,
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
