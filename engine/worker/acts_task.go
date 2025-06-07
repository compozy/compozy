package worker

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/pkg/logger"
)

type TaskExecutor struct {
	*ContextBuilder
}

func NewTaskExecutor(contextBuilder *ContextBuilder) *TaskExecutor {
	return &TaskExecutor{ContextBuilder: contextBuilder}
}

func (e *TaskExecutor) ExecuteFirstTask() func(ctx workflow.Context) (*task.Response, error) {
	return func(ctx workflow.Context) (*task.Response, error) {
		ctx = e.BuildTaskContext(ctx, e.InitialTaskID)
		loadTaskUC := uc.NewLoadTaskConfig(e.Workflows)
		taskConfig, err := loadTaskUC.Execute(ctx, &uc.LoadTaskConfigInput{
			WorkflowConfig: e.WorkflowConfig,
			TaskID:         e.InitialTaskID,
		})
		if err != nil {
			return nil, err
		}
		return e.HandleExecution(ctx, taskConfig)
	}
}

func (e *TaskExecutor) ExecuteTasks(response *task.Response) func(ctx workflow.Context) (*task.Response, error) {
	return func(ctx workflow.Context) (*task.Response, error) {
		logger := workflow.GetLogger(ctx)
		taskConfig := response.NextTask
		taskID := taskConfig.ID
		ctx = e.BuildTaskContext(ctx, taskID)
		// Sleep if needed
		if err := e.sleepTask(ctx, taskConfig); err != nil {
			return nil, err
		}
		// Execute task
		taskResponse, err := e.HandleExecution(ctx, taskConfig)
		if err != nil {
			return nil, err
		}
		// Dispatch next task if there is one
		if taskResponse.NextTask == nil {
			logger.Info("No more tasks to execute", "task_id", taskID)
			return nil, nil
		}
		// Ensure NextTask has a valid ID
		nextTaskID := taskResponse.NextTask.ID
		if nextTaskID == "" {
			logger.Error("NextTask has empty ID", "current_task", taskID)
			return nil, fmt.Errorf("next task has empty ID for current task: %s", taskID)
		}
		return taskResponse, nil
	}
}

func (e *TaskExecutor) HandleExecution(ctx workflow.Context, taskConfig *task.Config) (*task.Response, error) {
	taskID := taskConfig.ID
	taskType := taskConfig.Type
	var response *task.Response
	var err error
	switch taskType {
	case task.TaskTypeBasic:
		executeFn := e.ExecuteBasicTask(taskConfig)
		response, err = executeFn(ctx)
	case task.TaskTypeRouter:
		executeFn := e.ExecuteRouterTask(taskConfig)
		response, err = executeFn(ctx)
	case task.TaskTypeParallel:
		executeFn := e.HandleParallelTask(taskConfig)
		response, err = executeFn(ctx)
	default:
		return nil, fmt.Errorf("unsupported execution type: %s", taskType)
	}
	if err != nil {
		logger.Error("Failed to execute task", "task_id", taskID, "error", err)
		return nil, err
	}
	logger.Info("Task executed successfully",
		"status", response.State.Status,
		"task_id", taskID,
	)
	return response, nil
}

func (e *TaskExecutor) ExecuteBasicTask(taskConfig *task.Config) func(ctx workflow.Context) (*task.Response, error) {
	return func(ctx workflow.Context) (*task.Response, error) {
		var response *task.Response
		actLabel := tkacts.ExecuteBasicLabel
		actInput := tkacts.ExecuteBasicInput{
			WorkflowID:     e.WorkflowID,
			WorkflowExecID: e.WorkflowExecID,
			TaskConfig:     taskConfig,
		}
		err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
		if err != nil {
			return nil, err
		}
		return response, nil
	}
}

func (e *TaskExecutor) ExecuteRouterTask(taskConfig *task.Config) func(ctx workflow.Context) (*task.Response, error) {
	return func(ctx workflow.Context) (*task.Response, error) {
		var response *task.Response
		actLabel := tkacts.ExecuteRouterLabel
		actInput := tkacts.ExecuteRouterInput{
			WorkflowID:     e.WorkflowID,
			WorkflowExecID: e.WorkflowExecID,
			TaskConfig:     taskConfig,
		}
		err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
		if err != nil {
			return nil, err
		}
		return response, nil
	}
}

func (e *TaskExecutor) HandleParallelTask(pConfig *task.Config) func(ctx workflow.Context) (*task.Response, error) {
	return func(ctx workflow.Context) (*task.Response, error) {
		logger := workflow.GetLogger(ctx)
		tasks := pConfig.Tasks
		numTasks := len(tasks)
		results := make([]*task.SubtaskResponse, numTasks)
		completed, failed := 0, 0
		pState, err := e.CreateParallelState(ctx, pConfig)
		if err != nil {
			return nil, err
		}
		// Execute subtasks in parallel
		for i := range tasks {
			taskConfig := tasks[i]
			workflow.Go(ctx, func(gCtx workflow.Context) {
				response, err := e.ExecuteParallelTask(gCtx, pState, &taskConfig)
				if err != nil {
					logger.Error("Failed to execute sub task",
						"parent_task_id", pConfig.ID,
						"sub_task_id", taskConfig.ID,
						"error", err)
					failed++
				} else {
					completed++
					logger.Info("Subtask completed successfully",
						"parent_task_id", pConfig.ID,
						"sub_task_id", taskConfig.ID)
				}
				results[i] = response
			})
		}

		// Wait for tasks to complete based on strategy
		err = workflow.Await(ctx, func() bool {
			strategy := pConfig.GetStrategy()
			switch strategy {
			case task.StrategyWaitAll:
				return (completed + failed) >= numTasks
			case task.StrategyFailFast:
				return failed > 0 || completed >= numTasks
			case task.StrategyBestEffort:
				return (completed + failed) >= numTasks
			case task.StrategyRace:
				return completed > 0 || failed >= numTasks
			default:
				return (completed + failed) >= numTasks
			}
		})
		if err != nil {
			return nil, fmt.Errorf("failed to await parallel task: %w", err)
		}
		// Process parallel response with proper transitions
		finalResponse, err := e.GetParallelResponse(ctx, pState, results, pConfig)
		if err != nil {
			return nil, err
		}
		logger.Info("Parallel task execution completed",
			"task_id", pConfig.ID,
			"completed", completed,
			"failed", failed,
			"total", numTasks,
			"final_status", finalResponse.State.Status)
		return finalResponse, nil
	}
}

func (e *TaskExecutor) CreateParallelState(
	ctx workflow.Context,
	pConfig *task.Config,
) (*task.State, error) {
	var state *task.State
	actLabel := tkacts.CreateParallelStateLabel
	actInput := tkacts.CreateParallelStateInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		TaskConfig:     pConfig,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (e *TaskExecutor) ExecuteParallelTask(
	ctx workflow.Context,
	pState *task.State,
	taskConfig *task.Config,
) (*task.SubtaskResponse, error) {
	actLabel := tkacts.ExecuteParallelTaskLabel
	actInput := tkacts.ExecuteParallelTaskInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		ParentState:    pState,
		TaskConfig:     taskConfig,
	}
	future := workflow.ExecuteActivity(ctx, actLabel, actInput)
	var response *task.SubtaskResponse
	err := future.Get(ctx, &response)
	if err != nil && response == nil {
		response = &task.SubtaskResponse{
			TaskID: taskConfig.ID,
			Output: nil,
			Status: core.StatusFailed,
			Error:  core.NewError(err, "subtask_execution_failed", nil),
		}
	}
	return response, err
}

func (e *TaskExecutor) GetParallelResponse(
	ctx workflow.Context,
	pState *task.State,
	results []*task.SubtaskResponse,
	pConfig *task.Config,
) (*task.Response, error) {
	var response *task.Response
	actLabel := tkacts.GetParallelResponseLabel
	actInput := tkacts.GetParallelResponseInput{
		ParentState:    pState,
		Results:        results,
		WorkflowConfig: e.WorkflowConfig,
		TaskConfig:     pConfig,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (e *TaskExecutor) sleepTask(ctx workflow.Context, taskConfig *task.Config) error {
	// Check if task has sleep configuration
	taskID := taskConfig.ID
	sleepDuration, err := taskConfig.GetSleepDuration()
	if err != nil {
		logger.Error("Invalid sleep duration format", "task_id", taskID, "sleep", taskConfig.Sleep, "error", err)
		return err
	}
	if sleepDuration != 0 {
		if err := SleepWithPause(ctx, sleepDuration); err != nil {
			if err == workflow.ErrCanceled {
				return nil
			}
			logger.Error("Error during task sleep", "task_id", taskID, "error", err)
			return err
		}
		logger.Info("Task sleep completed", "task_id", taskID)
	}
	return nil
}
