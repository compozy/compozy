package worker

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
)

type TaskExecutor struct {
	*ContextBuilder
}

func NewTaskExecutor(contextBuilder *ContextBuilder) *TaskExecutor {
	return &TaskExecutor{ContextBuilder: contextBuilder}
}

func (e *TaskExecutor) DispatchFirstTask(ctx workflow.Context) func() (*tkacts.DispatchOutput, error) {
	return func() (*tkacts.DispatchOutput, error) {
		var output *tkacts.DispatchOutput
		actLabel := tkacts.DispatchLabel
		actInput := &tkacts.DispatchInput{
			WorkflowID:     e.WorkflowID,
			WorkflowExecID: e.WorkflowExecID,
			TaskID:         e.InitialTaskID,
		}
		ctx = e.BuildTaskContext(ctx, e.InitialTaskID)
		err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &output)
		if err != nil {
			return nil, err
		}
		return output, nil
	}
}

func (e *TaskExecutor) ExecuteBasicTask(
	ctx workflow.Context,
	output *tkacts.DispatchOutput,
) func() (*task.Response, error) {
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

func (e *TaskExecutor) DispatchTask(
	ctx workflow.Context,
	nextTaskID string,
) func() (*tkacts.DispatchOutput, error) {
	return func() (*tkacts.DispatchOutput, error) {
		var output *tkacts.DispatchOutput
		actLabel := tkacts.DispatchLabel
		actInput := &tkacts.DispatchInput{
			WorkflowID:     e.WorkflowID,
			WorkflowExecID: e.WorkflowExecID,
			TaskID:         nextTaskID,
		}
		err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &output)
		if err != nil {
			return nil, err
		}
		return output, nil
	}
}

func (e *TaskExecutor) ExecuteTaskLoop(
	ctx workflow.Context,
	currentTask *task.State,
	output *tkacts.DispatchOutput,
) func() (*task.State, error) {
	logger := workflow.GetLogger(ctx)
	return func() (*task.State, error) {
		taskID := currentTask.TaskID
		taskExecID := currentTask.TaskExecID
		logger.Info(fmt.Sprintf("Executing Task: %s", taskID), "task_exec_id", taskExecID)
		ctx = e.BuildTaskContext(ctx, taskID)

		// Check if task has sleep configuration
		sleepDuration, err := output.TaskConfig.GetSleepDuration()
		if err != nil {
			logger.Error("Invalid sleep duration format", "task_id", taskID, "sleep", output.TaskConfig.Sleep, "error", err)
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
		executeFn := e.ExecuteBasicTask(ctx, output)
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
			logger.Info("No more tasks to execute", "current_task", currentTask.TaskID)
			return nil, nil
		}

		// Ensure NextTask has a valid ID
		nextTaskID := response.NextTask.ID
		if nextTaskID == "" {
			logger.Error("NextTask has empty ID", "current_task", currentTask.TaskID)
			return nil, fmt.Errorf("next task has empty ID for current task: %s", currentTask.TaskID)
		}

		dispatchFn := e.DispatchTask(ctx, nextTaskID)
		nextTaskOutput, err := dispatchFn()
		if err != nil {
			logger.Error("Failed to dispatch next task", "next_task", nextTaskID, "error", err)
			return nil, err
		}
		currentTask = nextTaskOutput.TaskState
		output = nextTaskOutput
		return currentTask, nil
	}
}
