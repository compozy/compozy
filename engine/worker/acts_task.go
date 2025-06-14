package worker

import (
	"fmt"
	"math"
	"sort"
	"sync/atomic"

	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
	"github.com/compozy/compozy/engine/task/uc"
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
	case task.TaskTypeAggregate:
		executeFn := e.ExecuteAggregateTask(taskConfig)
		response, err = executeFn(ctx)
	case task.TaskTypeComposite:
		executeFn := e.HandleCompositeTask(taskConfig)
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

func (e *TaskExecutor) ExecuteAggregateTask(taskConfig *task.Config) func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		var response *task.MainTaskResponse
		actLabel := tkacts.ExecuteAggregateLabel
		actInput := tkacts.ExecuteAggregateInput{
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

func (e *TaskExecutor) ExecuteSubtask(
	ctx workflow.Context,
	pState *task.State,
	taskExecID string,
) (*task.SubtaskResponse, error) {
	actLabel := tkacts.ExecuteSubtaskLabel
	actInput := tkacts.ExecuteSubtaskInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		ParentState:    pState,
		TaskExecID:     taskExecID,
	}
	future := workflow.ExecuteActivity(ctx, actLabel, actInput)
	var response *task.SubtaskResponse
	err := future.Get(ctx, &response)
	if err != nil {
		// Let the error propagate for Temporal to handle retries
		return nil, err
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
		finalResponse, err := e.GetCollectionResponse(ctx, cState)
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
) (task.Response, error) {
	var response *task.MainTaskResponse
	actLabel := tkacts.GetParallelResponseLabel
	actInput := tkacts.GetParallelResponseInput{
		ParentState:    pState,
		WorkflowConfig: e.WorkflowConfig,
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

func (e *TaskExecutor) GetCollectionResponse(
	ctx workflow.Context,
	cState *task.State,
) (task.Response, error) {
	var response *task.CollectionResponse
	actLabel := tkacts.GetCollectionResponseLabel
	actInput := tkacts.GetCollectionResponseInput{
		ParentState:    cState,
		WorkflowConfig: e.WorkflowConfig,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (e *TaskExecutor) sleepTask(ctx workflow.Context, taskConfig *task.Config) error {
	// Get logger from workflow context for consistency
	logger := workflow.GetLogger(ctx)
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
				_, err := e.ExecuteSubtask(gCtx, pState, childState.TaskExecID.String())
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
				// Race terminates on first result, either success or failure
				return completedCount > 0 || failedCount > 0
			default:
				return (completedCount + failedCount) >= numTasks
			}
		})
		if err != nil {
			return nil, fmt.Errorf("failed to await parallel task: %w", err)
		}
		// Process parallel response with proper transitions
		finalResponse, err := e.GetParallelResponse(ctx, pState)
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
		logger.Error("No child states found for collection task",
			"task_id", taskConfig.ID,
			"parent_state_id", cState.TaskExecID)
		return fmt.Errorf("no child states found for collection %s", taskConfig.ID)
	}

	logger.Info("Executing collection child tasks",
		"task_id", taskConfig.ID,
		"child_count", len(childStates),
		"expected_count", childCount,
		"mode", taskConfig.GetMode())

	// Branch based on collection mode
	mode := taskConfig.GetMode()
	switch mode {
	case task.CollectionModeSequential:
		return e.handleCollectionSequential(ctx, cState, taskConfig, childStates, &completed, &failed)
	case task.CollectionModeParallel:
		return e.handleCollectionParallel(ctx, cState, taskConfig, childStates, &completed, &failed, childCount)
	default:
		// Default to parallel for backward compatibility
		return e.handleCollectionParallel(ctx, cState, taskConfig, childStates, &completed, &failed, childCount)
	}
}

func (e *TaskExecutor) handleCollectionParallel(
	ctx workflow.Context,
	cState *task.State,
	taskConfig *task.Config,
	childStates []*task.State,
	completed, failed *int32,
	childCount int32,
) error {
	logger := workflow.GetLogger(ctx)
	// Execute child tasks using their TaskExecIDs
	for i := range childStates {
		childState := childStates[i]
		workflow.Go(ctx, func(gCtx workflow.Context) {
			_, err := e.ExecuteSubtask(gCtx, cState, childState.TaskExecID.String())
			if err != nil {
				logger.Error("Failed to execute collection item",
					"parent_task_id", taskConfig.ID,
					"child_task_exec_id", childState.TaskExecID,
					"error", err)
				atomic.AddInt32(failed, 1)
			} else {
				atomic.AddInt32(completed, 1)
				logger.Info("Collection item completed successfully",
					"parent_task_id", taskConfig.ID,
					"child_task_exec_id", childState.TaskExecID)
			}
		})
	}
	// Wait for tasks to complete based on strategy
	awaitErr := workflow.Await(ctx, func() bool {
		completedCount := atomic.LoadInt32(completed)
		failedCount := atomic.LoadInt32(failed)
		strategy := taskConfig.GetStrategy()
		switch strategy {
		case task.StrategyWaitAll:
			return (completedCount + failedCount) >= childCount
		case task.StrategyFailFast:
			return failedCount > 0 || completedCount >= childCount
		case task.StrategyBestEffort:
			return (completedCount + failedCount) >= childCount
		case task.StrategyRace:
			// Race terminates on first result, either success or failure
			return completedCount > 0 || failedCount > 0
		default:
			return (completedCount + failedCount) >= childCount
		}
	})
	if awaitErr != nil {
		return fmt.Errorf("failed to await collection task: %w", awaitErr)
	}
	completedCount := atomic.LoadInt32(completed)
	failedCount := atomic.LoadInt32(failed)
	logger.Info("Collection parallel execution completed",
		"task_id", taskConfig.ID,
		"completed", completedCount,
		"failed", failedCount,
		"total", childCount)
	return nil
}

func (e *TaskExecutor) handleCollectionSequential(
	ctx workflow.Context,
	cState *task.State,
	taskConfig *task.Config,
	childStates []*task.State,
	completed, failed *int32,
) error {
	logger := workflow.GetLogger(ctx)
	strategy := taskConfig.GetStrategy()
	// Process child tasks sequentially
	for i, childState := range childStates {
		// Check for cancellation between iterations
		// Note: In Temporal, we don't check cancellation explicitly in workflows
		// The framework handles it automatically
		logger.Info("Executing collection item sequentially",
			"parent_task_id", taskConfig.ID,
			"child_task_exec_id", childState.TaskExecID,
			"index", i,
			"total", len(childStates))
		_, err := e.ExecuteSubtask(ctx, cState, childState.TaskExecID.String())
		if err != nil {
			atomic.AddInt32(failed, 1)
			logger.Error("Failed to execute collection item",
				"parent_task_id", taskConfig.ID,
				"child_task_exec_id", childState.TaskExecID,
				"index", i,
				"error", err)
			// Handle strategy-based early termination
			if strategy == task.StrategyFailFast {
				logger.Info("Stopping collection execution due to FailFast strategy",
					"task_id", taskConfig.ID,
					"failed_at_index", i)
				break
			}
		} else {
			atomic.AddInt32(completed, 1)
			logger.Info("Collection item completed successfully",
				"parent_task_id", taskConfig.ID,
				"child_task_exec_id", childState.TaskExecID,
				"index", i)
			// Handle Race strategy - stop on first success
			if strategy == task.StrategyRace {
				logger.Info("Stopping collection execution due to Race strategy",
					"task_id", taskConfig.ID,
					"succeeded_at_index", i)
				break
			}
		}
	}
	completedCount := atomic.LoadInt32(completed)
	failedCount := atomic.LoadInt32(failed)
	logger.Info("Collection sequential execution completed",
		"task_id", taskConfig.ID,
		"completed", completedCount,
		"failed", failedCount,
		"total", len(childStates))
	return nil
}

func (e *TaskExecutor) HandleCompositeTask(config *task.Config) func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		logger := workflow.GetLogger(ctx)
		// Create parent state for composite task
		compositeState, err := e.CreateCompositeState(ctx, config)
		if err != nil {
			return nil, err
		}
		// Get child states that were created by CreateCompositeState
		childStates, err := e.ListChildStates(ctx, compositeState.TaskExecID)
		if err != nil {
			return nil, fmt.Errorf("failed to list child states: %w", err)
		}
		// Sort child states by task ID to ensure deterministic ordering
		// This matches the order defined in config.ParallelTask.Tasks
		sort.Slice(childStates, func(i, j int) bool {
			// Find index of each task in the config
			iIdx := findTaskIndex(config.Tasks, childStates[i].TaskID)
			jIdx := findTaskIndex(config.Tasks, childStates[j].TaskID)
			return iIdx < jIdx
		})
		// Execute subtasks sequentially
		strategy := config.GetStrategy()
		for i, childState := range childStates {
			// Execute subtask
			_, err := e.ExecuteSubtask(ctx, compositeState, childState.TaskExecID.String())
			if err != nil {
				logger.Error("Subtask failed",
					"composite_task", config.ID,
					"subtask", childState.TaskID,
					"index", i,
					"error", err)
				if strategy == task.StrategyFailFast {
					return nil, fmt.Errorf("subtask %s failed: %w", childState.TaskID, err)
				}
				// Best effort: continue to next task
				continue
			}
			logger.Info("Subtask completed",
				"composite_task", config.ID,
				"subtask", childState.TaskID,
				"index", i)
		}
		// Generate final response using standard parent task processing
		return e.GetCompositeResponse(ctx, compositeState)
	}
}

func (e *TaskExecutor) CreateCompositeState(
	ctx workflow.Context,
	config *task.Config,
) (*task.State, error) {
	var state *task.State
	actLabel := tkacts.CreateCompositeStateLabel
	actInput := tkacts.CreateCompositeStateInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		TaskConfig:     config,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (e *TaskExecutor) GetCompositeResponse(
	ctx workflow.Context,
	compositeState *task.State,
) (task.Response, error) {
	var response *task.MainTaskResponse
	actLabel := tkacts.GetCompositeResponseLabel
	actInput := tkacts.GetCompositeResponseInput{
		ParentState:    compositeState,
		WorkflowConfig: e.WorkflowConfig,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// findTaskIndex finds the index of a task ID in the task config slice
func findTaskIndex(tasks []task.Config, taskID string) int {
	for i := range tasks {
		if tasks[i].ID == taskID {
			return i
		}
	}
	return -1 // Not found, will sort to the end
}
