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

func CompozyWorkflow(ctx workflow.Context, input WorkflowInput) (*wf.State, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting workflow", "workflow_id", input.WorkflowID, "exec_id", input.WorkflowExecID)
	var wState *wf.State
	defer cancelCleanup(ctx, &input)

	// Get workflow data
	data, err := getWorkflowData(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow data: %w", err)
	}

	// Initial context
	projectConfig := data.ProjectConfig
	workflowConfig, err := wf.FindConfig(data.Workflows, input.WorkflowID)
	data.WorkflowConfig = workflowConfig
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}
	ctx = activityContext(ctx, projectConfig, workflowConfig)
	errHandler := BuildErrHandler(ctx, input)

	// Execute main trigger activity
	logger.Info("Executing main trigger activity...")
	triggerFn := triggerWorkflow(ctx, data, &input)
	wState, err = handleAct(ctx, errHandler, triggerFn)()
	if err != nil {
		return nil, err
	}

	// Dispatch first task
	dispatchFn := dispatchFirstTask(ctx, data, wState, &input)
	output, err := handleAct(ctx, errHandler, dispatchFn)()
	if err != nil {
		return nil, err
	}

	// Iterate over tasks until get the final one
	currentTask := output.State
	taskFn := executeTask(ctx, currentTask, data, output)
	for currentTask != nil {
		nextTask, err := handleAct(ctx, errHandler, taskFn)()
		if err != nil {
			return nil, err
		}
		if nextTask == nil {
			break
		}
		currentTask = nextTask
	}

	// Complete workflow
	completeFn := completeWorkflow(ctx, wState)
	wState, err = handleAct(ctx, errHandler, completeFn)()
	if err != nil {
		return nil, err
	}
	logger.Info("Workflow completed",
		"workflow_id", input.WorkflowID,
		"exec_id", input.WorkflowExecID,
	)
	return wState, nil
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
	activityOptions.WaitForCancellation = true
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
	// Set WaitForCancellation to true to have the Workflow wait to return
	// until all in progress Activities have completed, failed, or accepted the Cancellation.
	activityOptions.WaitForCancellation = true
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
) func() (*wf.State, error) {
	return func() (*wf.State, error) {
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
}

func completeWorkflow(ctx workflow.Context, wState *wf.State) func() (*wf.State, error) {
	return func() (*wf.State, error) {
		actLabel := wfacts.CompleteWorkflowLabel
		actInput := &wfacts.CompleteWorkflowInput{
			WorkflowExecID: wState.WorkflowExecID,
		}
		var output *wf.State
		err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &output)
		if err != nil {
			return nil, err
		}
		return output, nil
	}
}

// -----------------------------------------------------------------------------
// Task Functions
// -----------------------------------------------------------------------------

func dispatchFirstTask(
	ctx workflow.Context,
	data *WorkflowData,
	wState *wf.State,
	input *wfacts.TriggerInput,
) func() (*tkacts.DispatchOutput, error) {
	return func() (*tkacts.DispatchOutput, error) {
		var output *tkacts.DispatchOutput
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
		err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &output)
		if err != nil {
			return nil, err
		}
		return output, nil
	}
}

func executeTask(
	ctx workflow.Context,
	currentTask *task.State,
	data *WorkflowData,
	output *tkacts.DispatchOutput,
) func() (*task.State, error) {
	logger := workflow.GetLogger(ctx)
	return func() (*task.State, error) {
		taskID := currentTask.TaskID
		taskExecID := currentTask.TaskExecID
		logger.Info(fmt.Sprintf("Executing Task: %s", taskID), "task_exec_id", taskExecID)
		ctx = activityContextForTask(ctx, data.ProjectConfig, data.WorkflowConfig, taskID)

		// Check if task has sleep configuration
		sleepDuration, err := output.Config.GetSleepDuration()
		if err != nil {
			logger.Error("Invalid sleep duration format", "task_id", taskID, "sleep", output.Config.Sleep, "error", err)
			return nil, err
		}
		if sleepDuration != 0 {
			if err := SleepWithPause(ctx, sleepDuration); err != nil {
				if err == workflow.ErrCanceled {
					return nil, nil
				}
				logger.Error("Error during task sleep", "task_id", taskID, "error", err)
				return nil, err
			}
			logger.Info("Task sleep completed", "task_id", taskID)
		}

		// Execute the task
		executeFn := executeBasicTask(ctx, output)
		response, err := executeFn()
		if err != nil {
			logger.Error("Failed to execute task", "task_id", currentTask.TaskID, "error", err)
			return nil, err
		}
		logger.Info("Task executed successfully",
			"status", response.State.Status,
			"task_id", currentTask.TaskID,
		)

		// Dispatch next task if there is one
		if response.NextTask == nil {
			// No more tasks to execute
			logger.Info("No more tasks to execute", "current_task", currentTask.TaskID)
			return nil, nil
		}
		// Ensure NextTask has a valid ID
		nextTaskID := response.NextTask.ID
		if nextTaskID == "" {
			logger.Error("NextTask has empty ID", "current_task", currentTask.TaskID)
			return nil, fmt.Errorf("next task has empty ID for current task: %s", currentTask.TaskID)
		}
		currentTaskState := response.State
		dispatchFn := dispatchTask(ctx, currentTaskState, nextTaskID)
		nextTaskOutput, err := dispatchFn()
		if err != nil {
			logger.Error("Failed to dispatch next task", "next_task", nextTaskID, "error", err)
			return nil, err
		}
		currentTask = nextTaskOutput.State
		output = nextTaskOutput
		return currentTask, nil
	}
}

func executeBasicTask(ctx workflow.Context, output *tkacts.DispatchOutput) func() (*task.Response, error) {
	return func() (*task.Response, error) {
		var response *task.Response
		actLabel := tkacts.ExecuteBasicLabel
		err := workflow.ExecuteActivity(ctx, actLabel, output).Get(ctx, &response)
		if err != nil {
			return nil, err
		}
		return response, nil
	}
}

func dispatchTask(
	ctx workflow.Context,
	currentTaskState *task.State,
	nextTaskID string,
) func() (*tkacts.DispatchOutput, error) {
	return func() (*tkacts.DispatchOutput, error) {
		var output *tkacts.DispatchOutput
		actLabel := tkacts.DispatchLabel
		actInput := &tkacts.DispatchInput{
			WorkflowID:     currentTaskState.WorkflowID,
			WorkflowExecID: currentTaskState.WorkflowExecID,
			TaskID:         nextTaskID,
		}
		err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &output)
		if err != nil {
			return nil, err
		}
		return output, nil
	}
}
