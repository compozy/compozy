package worker

import (
	"fmt"
	"math"
	"sync/atomic"

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

func (e *TaskExecutor) ExecuteFirstTask() func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
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

func (e *TaskExecutor) ExecuteTasks(response task.Response) func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		logger := workflow.GetLogger(ctx)
		taskConfig := response.GetNextTask()
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
		if taskResponse.GetNextTask() == nil {
			logger.Info("No more tasks to execute", "task_id", taskID)
			return nil, nil
		}
		// Ensure NextTask has a valid ID
		nextTaskID := taskResponse.GetNextTask().ID
		if nextTaskID == "" {
			logger.Error("NextTask has empty ID", "current_task", taskID)
			return nil, fmt.Errorf("next task has empty ID for current task: %s", taskID)
		}
		return taskResponse, nil
	}
}

func (e *TaskExecutor) HandleExecution(ctx workflow.Context, taskConfig *task.Config) (task.Response, error) {
	logger := workflow.GetLogger(ctx)
	taskID := taskConfig.ID
	taskType := taskConfig.Type
	var response task.Response
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
	case task.TaskTypeCollection:
		executeFn := e.ExecuteCollectionTask(taskConfig)
		response, err = executeFn(ctx)
	default:
		return nil, fmt.Errorf("unsupported execution type: %s", taskType)
	}
	if err != nil {
		logger.Error("Failed to execute task", "task_id", taskID, "error", err)
		return nil, err
	}
	logger.Info("Task executed successfully",
		"status", response.GetState().Status,
		"task_id", taskID,
	)
	return response, nil
}

func (e *TaskExecutor) ExecuteBasicTask(taskConfig *task.Config) func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		var response *task.MainTaskResponse
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

func (e *TaskExecutor) ExecuteRouterTask(taskConfig *task.Config) func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		var response *task.MainTaskResponse
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

func (e *TaskExecutor) ListChildStates(
	ctx workflow.Context,
	parentTaskExecID core.ID,
) ([]*task.State, error) {
	actLabel := tkacts.ListChildStatesLabel
	actInput := tkacts.ListChildStatesInput{
		ParentTaskExecID: parentTaskExecID,
	}
	var childStates []*task.State
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &childStates)
	if err != nil {
		return nil, err
	}
	return childStates, nil
}

func (e *TaskExecutor) ExecuteParallelTask(
	ctx workflow.Context,
	pState *task.State,
	taskExecID string,
) (*task.SubtaskResponse, error) {
	actLabel := tkacts.ExecuteParallelTaskLabel
	actInput := tkacts.ExecuteParallelTaskInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		ParentState:    pState,
		TaskExecID:     taskExecID,
	}
	future := workflow.ExecuteActivity(ctx, actLabel, actInput)
	var response *task.SubtaskResponse
	err := future.Get(ctx, &response)
	if err != nil {
		subtaskResponse := &task.SubtaskResponse{
			TaskID: taskExecID, // Using taskExecID as identifier
			Output: nil,
			Status: core.StatusFailed,
			Error:  core.NewError(err, "subtask_execution_failed", nil),
		}
		return subtaskResponse, err
	}
	return response, nil
}

func (e *TaskExecutor) ExecuteCollectionTask(
	taskConfig *task.Config,
) func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		logger := workflow.GetLogger(ctx)
		cState, err := e.CreateCollectionState(ctx, taskConfig)
		if err != nil {
			return nil, err
		}
		err = e.HandleCollectionTask(ctx, cState, taskConfig)
		if err != nil {
			return nil, err
		}
		finalResponse, err := e.GetCollectionResponse(ctx, cState, taskConfig)
		if err != nil {
			return nil, err
		}
		logger.Info("Collection task execution completed",
			"task_id", taskConfig.ID,
			"final_status", finalResponse.GetState().Status)

		return finalResponse, nil
	}
}

func (e *TaskExecutor) GetParallelResponse(
	ctx workflow.Context,
	pState *task.State,
	pConfig *task.Config,
) (task.Response, error) {
	var response *task.MainTaskResponse
	actLabel := tkacts.GetParallelResponseLabel
	actInput := tkacts.GetParallelResponseInput{
		ParentState:    pState,
		WorkflowConfig: e.WorkflowConfig,
		TaskConfig:     pConfig,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (e *TaskExecutor) CreateCollectionState(
	ctx workflow.Context,
	taskConfig *task.Config,
) (*task.State, error) {
	var state *task.State
	actLabel := tkacts.CreateCollectionStateLabel
	actInput := tkacts.CreateCollectionStateInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		TaskConfig:     taskConfig,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (e *TaskExecutor) HandleCollectionTask(
	ctx workflow.Context,
	cState *task.State,
	taskConfig *task.Config,
) error {
	// Get child states that were created by CreateCollectionState
	childStates, err := e.ListChildStates(ctx, cState.TaskExecID)
	if err != nil {
		return fmt.Errorf("failed to list child states: %w", err)
	}
	// Check for overflow before converting
	childStatesLen := len(childStates)
	if childStatesLen > math.MaxInt32 {
		return fmt.Errorf("too many child states: %d exceeds maximum of %d", childStatesLen, math.MaxInt32)
	}
	childCount := int32(childStatesLen)

	// Use the same atomic counters approach as parallel tasks
	var completed, failed int32
	logger := workflow.GetLogger(ctx)

	if len(childStates) == 0 {
		logger.Warn("No child states found for collection task",
			"task_id", taskConfig.ID,
			"parent_state_id", cState.TaskExecID)
		return fmt.Errorf("no child states found for collection %s", taskConfig.ID)
	}

	logger.Info("Executing collection child tasks",
		"task_id", taskConfig.ID,
		"child_count", len(childStates),
		"expected_count", childCount)

	// Execute child tasks using their TaskExecIDs
	for i := range childStates {
		childState := childStates[i]
		workflow.Go(ctx, func(gCtx workflow.Context) {
			_, err := e.ExecuteParallelTask(gCtx, cState, childState.TaskExecID.String())
			if err != nil {
				logger.Error("Failed to execute collection item",
					"parent_task_id", taskConfig.ID,
					"child_task_exec_id", childState.TaskExecID,
					"error", err)
				atomic.AddInt32(&failed, 1)
			} else {
				atomic.AddInt32(&completed, 1)
				logger.Info("Collection item completed successfully",
					"parent_task_id", taskConfig.ID,
					"child_task_exec_id", childState.TaskExecID)
			}
		})
	}

	// Wait for tasks to complete based on strategy
	awaitErr := workflow.Await(ctx, func() bool {
		completedCount := atomic.LoadInt32(&completed)
		failedCount := atomic.LoadInt32(&failed)
		strategy := taskConfig.GetStrategy()
		switch strategy {
		case task.StrategyWaitAll:
			return (completedCount + failedCount) >= childCount
		case task.StrategyFailFast:
			return failedCount > 0 || completedCount >= childCount
		case task.StrategyBestEffort:
			return (completedCount + failedCount) >= childCount
		case task.StrategyRace:
			return completedCount > 0 || failedCount >= childCount
		default:
			return (completedCount + failedCount) >= childCount
		}
	})
	if awaitErr != nil {
		return fmt.Errorf("failed to await collection task: %w", awaitErr)
	}

	completedCount := atomic.LoadInt32(&completed)
	failedCount := atomic.LoadInt32(&failed)
	logger.Info("Collection execution completed",
		"task_id", taskConfig.ID,
		"completed", completedCount,
		"failed", failedCount,
		"total", childCount)

	return nil
}

func (e *TaskExecutor) GetCollectionResponse(
	ctx workflow.Context,
	cState *task.State,
	taskConfig *task.Config,
) (task.Response, error) {
	var response *task.CollectionResponse
	actLabel := tkacts.GetCollectionResponseLabel
	actInput := tkacts.GetCollectionResponseInput{
		ParentState:    cState,
		WorkflowConfig: e.WorkflowConfig,
		TaskConfig:     taskConfig,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (e *TaskExecutor) HandleParallelTask(pConfig *task.Config) func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		logger := workflow.GetLogger(ctx)
		var completed, failed int32
		pState, err := e.CreateParallelState(ctx, pConfig)
		if err != nil {
			return nil, err
		}

		// Get child states that were created by CreateParallelState
		childStates, err := e.ListChildStates(ctx, pState.TaskExecID)
		if err != nil {
			return nil, fmt.Errorf("failed to list child states: %w", err)
		}

		tasksLen := len(childStates)
		if tasksLen > math.MaxInt32 {
			return nil, fmt.Errorf("too many tasks: %d exceeds maximum of %d", tasksLen, math.MaxInt32)
		}
		numTasks := int32(tasksLen)

		// Execute subtasks in parallel using their TaskExecIDs
		for i := range childStates {
			childState := childStates[i]
			workflow.Go(ctx, func(gCtx workflow.Context) {
				_, err := e.ExecuteParallelTask(gCtx, pState, childState.TaskExecID.String())
				if err != nil {
					logger.Error("Failed to execute sub task",
						"parent_task_id", pConfig.ID,
						"sub_task_exec_id", childState.TaskExecID,
						"error", err)
					atomic.AddInt32(&failed, 1)
				} else {
					atomic.AddInt32(&completed, 1)
					logger.Info("Subtask completed successfully",
						"parent_task_id", pConfig.ID,
						"sub_task_exec_id", childState.TaskExecID)
				}
			})
		}

		// Wait for tasks to complete based on strategy
		err = workflow.Await(ctx, func() bool {
			completedCount := atomic.LoadInt32(&completed)
			failedCount := atomic.LoadInt32(&failed)
			strategy := pConfig.GetStrategy()
			switch strategy {
			case task.StrategyWaitAll:
				return (completedCount + failedCount) >= numTasks
			case task.StrategyFailFast:
				return failedCount > 0 || completedCount >= numTasks
			case task.StrategyBestEffort:
				return (completedCount + failedCount) >= numTasks
			case task.StrategyRace:
				return completedCount > 0 || failedCount >= numTasks
			default:
				return (completedCount + failedCount) >= numTasks
			}
		})
		if err != nil {
			return nil, fmt.Errorf("failed to await parallel task: %w", err)
		}
		// Process parallel response with proper transitions
		finalResponse, err := e.GetParallelResponse(ctx, pState, pConfig)
		if err != nil {
			return nil, err
		}
		completedCount := atomic.LoadInt32(&completed)
		failedCount := atomic.LoadInt32(&failed)
		logger.Info("Parallel task execution completed",
			"task_id", pConfig.ID,
			"completed", completedCount,
			"failed", failedCount,
			"total", numTasks,
			"final_status", finalResponse.GetState().Status)
		return finalResponse, nil
	}
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
