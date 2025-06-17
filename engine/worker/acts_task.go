package worker

import (
	"fmt"
	"math"
	"sort"
	"sync/atomic"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
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

		// Load task config via activity (deterministic)
		var taskConfig *task.Config
		actInput := &tkacts.LoadTaskConfigInput{
			WorkflowConfig: e.WorkflowConfig,
			TaskID:         e.InitialTaskID,
		}
		err := workflow.ExecuteActivity(ctx, tkacts.LoadTaskConfigLabel, actInput).Get(ctx, &taskConfig)
		if err != nil {
			return nil, err
		}
		return e.HandleExecution(ctx, taskConfig, 0)
	}
}

func (e *TaskExecutor) ExecuteTasks(response task.Response) func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		log := workflow.GetLogger(ctx)
		taskConfig := response.GetNextTask()
		taskID := taskConfig.ID
		ctx = e.BuildTaskContext(ctx, taskID)
		// Sleep if needed
		if err := e.sleepTask(ctx, taskConfig); err != nil {
			return nil, err
		}
		// Execute task
		taskResponse, err := e.HandleExecution(ctx, taskConfig, 0)
		if err != nil {
			return nil, err
		}
		// Dispatch next task if there is one
		if taskResponse.GetNextTask() == nil {
			log.Info("No more tasks to execute", "task_id", taskID)
			return nil, nil
		}
		// Ensure NextTask has a valid ID
		nextTaskID := taskResponse.GetNextTask().ID
		if nextTaskID == "" {
			log.Error("NextTask has empty ID", "current_task", taskID)
			return nil, fmt.Errorf("next task has empty ID for current task: %s", taskID)
		}
		return taskResponse, nil
	}
}

func (e *TaskExecutor) HandleExecution(
	ctx workflow.Context,
	taskConfig *task.Config,
	depth ...int,
) (task.Response, error) {
	log := workflow.GetLogger(ctx)
	taskID := taskConfig.ID
	taskType := taskConfig.Type
	currentDepth := 0
	if len(depth) > 0 {
		currentDepth = depth[0]
	}
	if currentDepth > 20 { // max_nesting_depth from config
		return nil, fmt.Errorf("maximum nesting depth exceeded: %d", currentDepth)
	}
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
		executeFn := e.HandleParallelTask(taskConfig, currentDepth)
		response, err = executeFn(ctx)
	case task.TaskTypeCollection:
		executeFn := e.ExecuteCollectionTask(taskConfig, currentDepth)
		response, err = executeFn(ctx)
	case task.TaskTypeAggregate:
		executeFn := e.ExecuteAggregateTask(taskConfig)
		response, err = executeFn(ctx)
	case task.TaskTypeComposite:
		executeFn := e.HandleCompositeTask(taskConfig, currentDepth)
		response, err = executeFn(ctx)
	case task.TaskTypeSignal:
		executeFn := e.ExecuteSignalTask(taskConfig)
		response, err = executeFn(ctx)
	default:
		return nil, fmt.Errorf("unsupported execution type: %s", taskType)
	}
	if err != nil {
		log.Error("Failed to execute task", "task_id", taskID, "depth", currentDepth, "error", err)
		return nil, err
	}
	log.Debug("Task executed successfully",
		"status", response.GetState().Status,
		"task_id", taskID,
		"depth", currentDepth,
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

func (e *TaskExecutor) ExecuteSignalTask(taskConfig *task.Config) func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		var response *task.MainTaskResponse
		actLabel := tkacts.ExecuteSignalLabel
		actInput := tkacts.ExecuteSignalInput{
			WorkflowID:     e.WorkflowID,
			WorkflowExecID: e.WorkflowExecID,
			TaskConfig:     taskConfig,
			ProjectName:    e.ProjectConfig.Name,
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
	parentStateID core.ID,
	taskExecID string,
) (*task.SubtaskResponse, error) {
	actLabel := tkacts.ExecuteSubtaskLabel
	actInput := tkacts.ExecuteSubtaskInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		ParentStateID:  parentStateID,
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
	depth ...int,
) func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		log := workflow.GetLogger(ctx)
		currentDepth := 0
		if len(depth) > 0 {
			currentDepth = depth[0]
		}
		cState, err := e.CreateCollectionState(ctx, taskConfig)
		if err != nil {
			return nil, err
		}
		err = e.HandleCollectionTask(ctx, cState, taskConfig, currentDepth)
		if err != nil {
			return nil, err
		}
		finalResponse, err := e.GetCollectionResponse(ctx, cState)
		if err != nil {
			return nil, err
		}
		log.Debug("Collection task execution completed",
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

// executeChild executes a child task, handling both Basic tasks (using ExecuteSubtask)
// and container tasks (recursively calling HandleExecution)
func (e *TaskExecutor) executeChild(
	ctx workflow.Context,
	parentStateID core.ID,
	childState *task.State,
	cfg *task.Config,
	depth int,
) error {
	log := workflow.GetLogger(ctx)
	log.Debug("Executing child", "task", cfg.ID, "depth", depth)

	switch cfg.Type {
	case task.TaskTypeBasic:
		_, err := e.ExecuteSubtask(ctx, parentStateID, childState.TaskExecID.String())
		return err
	default:
		// Recurse for container tasks - bump depth
		response, err := e.HandleExecution(ctx, cfg, depth+1)
		if err != nil {
			return err
		}
		// Update child state record with final status from nested task execution
		// This ensures parent tasks can see the completion status
		if response != nil && response.GetState() != nil {
			finalState := response.GetState()
			// Create a simple activity to update the child state status
			updateInput := map[string]any{
				"task_exec_id": childState.TaskExecID.String(),
				"status":       string(finalState.Status),
				"output":       finalState.Output,
			}

			// Add retry policy for this non-critical but important update
			// This makes the update more resilient to transient network or database issues
			activityOpts := workflow.ActivityOptions{
				StartToCloseTimeout: 30 * time.Second,
				RetryPolicy: &temporal.RetryPolicy{
					InitialInterval:    time.Second,
					BackoffCoefficient: 2.0,
					MaximumInterval:    10 * time.Second,
					MaximumAttempts:    3,
				},
			}
			activityCtx := workflow.WithActivityOptions(ctx, activityOpts)

			err = workflow.ExecuteActivity(activityCtx, "UpdateChildState", updateInput).Get(activityCtx, nil)
			if err != nil {
				log.Error("Failed to update child state after nested task completion",
					"child_task_id", childState.TaskID, "final_status", finalState.Status, "error", err)
				// Don't fail the execution - this is for tracking purposes only
			}
		}
		return nil
	}
}

func (e *TaskExecutor) sleepTask(ctx workflow.Context, taskConfig *task.Config) error {
	// Get log from workflow context for consistency
	log := workflow.GetLogger(ctx)
	// Check if task has sleep configuration
	taskID := taskConfig.ID
	sleepDuration, err := taskConfig.GetSleepDuration()
	if err != nil {
		log.Error("Invalid sleep duration format", "task_id", taskID, "sleep", taskConfig.Sleep, "error", err)
		return err
	}
	if sleepDuration != 0 {
		if err := SleepWithPause(ctx, sleepDuration); err != nil {
			if err == workflow.ErrCanceled {
				return nil
			}
			log.Error("Error during task sleep", "task_id", taskID, "error", err)
			return err
		}
		log.Debug("Task sleep completed", "task_id", taskID)
	}
	return nil
}

func (e *TaskExecutor) HandleParallelTask(
	pConfig *task.Config,
	depth ...int,
) func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		log := workflow.GetLogger(ctx)
		currentDepth := 0
		if len(depth) > 0 {
			currentDepth = depth[0]
		}

		// TODO: ContinueAsNew guard (history safety) - implement when needed
		// if historyLength > 25000 { return continueAsNew() }

		var completed, failed int32
		pState, childStates, childCfgs, numTasks, err := e.setupParallelExecution(ctx, pConfig)
		if err != nil {
			return nil, err
		}

		// Execute subtasks in parallel using executeChild helper
		e.executeChildrenInParallel(ctx, pState, childStates, childCfgs, pConfig, currentDepth, &completed, &failed)

		// Wait for tasks to complete based on strategy
		err = workflow.Await(ctx, func() bool {
			completedCount := atomic.LoadInt32(&completed)
			failedCount := atomic.LoadInt32(&failed)
			return e.shouldCompleteParallelTask(pConfig.GetStrategy(), completedCount, failedCount, numTasks)
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
		log.Debug("Parallel task execution completed",
			"task_id", pConfig.ID,
			"completed", completedCount,
			"failed", failedCount,
			"total", numTasks,
			"final_status", finalResponse.GetState().Status)
		return finalResponse, nil
	}
}

// shouldCompleteParallelTask determines if parallel task execution should complete based on strategy
func (e *TaskExecutor) shouldCompleteParallelTask(strategy task.ParallelStrategy, completed, failed, total int32) bool {
	switch strategy {
	case task.StrategyWaitAll:
		return (completed + failed) >= total
	case task.StrategyFailFast:
		return failed > 0 || completed >= total
	case task.StrategyBestEffort:
		return (completed + failed) >= total
	case task.StrategyRace:
		// Race terminates on first result, either success or failure
		return completed > 0 || failed > 0
	default:
		return (completed + failed) >= total
	}
}

// executeChildrenInParallel executes child tasks in parallel using goroutines
func (e *TaskExecutor) executeChildrenInParallel(
	ctx workflow.Context,
	parentState *task.State,
	childStates []*task.State,
	childCfgs map[string]*task.Config,
	taskConfig *task.Config,
	depth int,
	completed, failed *int32,
) {
	log := workflow.GetLogger(ctx)
	for i := range childStates {
		childState := childStates[i]
		childConfig := childCfgs[childState.TaskID]
		// Capture variables by value to avoid race conditions in goroutines
		cs := childState
		cfg := childConfig
		workflow.Go(ctx, func(gCtx workflow.Context) {
			if gCtx.Err() != nil {
				return // canceled before start
			}
			err := e.executeChild(gCtx, parentState.TaskExecID, cs, cfg, depth)
			if gCtx.Err() != nil {
				return // canceled during work
			}
			if err != nil {
				log.Error("Failed to execute child task",
					"parent_task_id", taskConfig.ID,
					"child_task_id", cs.TaskID,
					"depth", depth+1,
					"error", err)
				atomic.AddInt32(failed, 1)
			} else {
				atomic.AddInt32(completed, 1)
				log.Debug("Child task completed successfully",
					"parent_task_id", taskConfig.ID,
					"child_task_id", cs.TaskID,
					"depth", depth+1)
			}
		})
	}
}

// setupParallelExecution sets up the parallel task execution
func (e *TaskExecutor) setupParallelExecution(
	ctx workflow.Context,
	pConfig *task.Config,
) (*task.State, []*task.State, map[string]*task.Config, int32, error) {
	pState, err := e.CreateParallelState(ctx, pConfig)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	// Get child states that were created by CreateParallelState
	childStates, err := e.ListChildStates(ctx, pState.TaskExecID)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to list child states: %w", err)
	}
	// For nested parallel tasks, use the task's own configs; for root tasks, load from workflow
	var childCfgs map[string]*task.Config
	if len(pConfig.Tasks) > 0 {
		// Nested parallel task - use configs from the task itself
		childCfgs = make(map[string]*task.Config)
		for i := range pConfig.Tasks {
			cfg := &pConfig.Tasks[i]
			childCfgs[cfg.ID] = cfg
		}
	} else {
		// Root parallel task - load configs from workflow
		childIDs := make([]string, len(childStates))
		for i, st := range childStates {
			childIDs[i] = st.TaskID
		}
		err = workflow.ExecuteActivity(ctx, tkacts.LoadBatchConfigsLabel, &tkacts.LoadBatchConfigsInput{
			WorkflowConfig: e.WorkflowConfig,
			TaskIDs:        childIDs,
		}).Get(ctx, &childCfgs)
		if err != nil {
			return nil, nil, nil, 0, fmt.Errorf("failed to load child configs: %w", err)
		}
	}
	tasksLen := len(childStates)
	if tasksLen > math.MaxInt32 {
		return nil, nil, nil, 0, fmt.Errorf("too many tasks: %d exceeds maximum of %d", tasksLen, math.MaxInt32)
	}
	numTasks := int32(tasksLen)
	return pState, childStates, childCfgs, numTasks, nil
}

func (e *TaskExecutor) HandleCollectionTask(
	ctx workflow.Context,
	cState *task.State,
	taskConfig *task.Config,
	depth int,
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
	log := workflow.GetLogger(ctx)

	// Empty collections are valid - they complete successfully with no child tasks
	if len(childStates) == 0 {
		log.Debug("Collection task has no items, completing successfully",
			"task_id", taskConfig.ID,
			"parent_state_id", cState.TaskExecID)
		// Collection completes successfully even with no items
		return nil
	}

	log.Debug("Executing collection child tasks",
		"task_id", taskConfig.ID,
		"child_count", len(childStates),
		"expected_count", childCount,
		"mode", taskConfig.GetMode())

	// Branch based on collection mode
	mode := taskConfig.GetMode()
	switch mode {
	case task.CollectionModeSequential:
		return e.handleCollectionSequential(ctx, cState, taskConfig, childStates, &completed, &failed, depth)
	case task.CollectionModeParallel:
		return e.handleCollectionParallel(ctx, cState, taskConfig, childStates, &completed, &failed, childCount, depth)
	default:
		// Default to parallel for backward compatibility
		return e.handleCollectionParallel(ctx, cState, taskConfig, childStates, &completed, &failed, childCount, depth)
	}
}

func (e *TaskExecutor) handleCollectionParallel(
	ctx workflow.Context,
	cState *task.State,
	taskConfig *task.Config,
	childStates []*task.State,
	completed, failed *int32,
	childCount int32,
	depth int,
) error {
	log := workflow.GetLogger(ctx)

	// For collection tasks, all children use the parent's task template
	// Collection child TaskIDs are dynamically generated (e.g., activity_analysis_item_0)
	// and don't exist as separate tasks in the workflow config

	// Execute child tasks using executeChild helper
	for i := range childStates {
		childState := childStates[i]
		// Capture variables by value to avoid race conditions in goroutines
		cs := childState
		workflow.Go(ctx, func(gCtx workflow.Context) {
			if gCtx.Err() != nil {
				return // canceled before start
			}
			err := e.executeChild(gCtx, cState.TaskExecID, cs, taskConfig.Task, depth)
			if gCtx.Err() != nil {
				return // canceled during work
			}
			if err != nil {
				log.Error("Failed to execute child task",
					"parent_task_id", taskConfig.ID,
					"child_task_id", cs.TaskID,
					"depth", depth+1,
					"error", err)
				atomic.AddInt32(failed, 1)
			} else {
				atomic.AddInt32(completed, 1)
				log.Debug("Child task completed successfully",
					"parent_task_id", taskConfig.ID,
					"child_task_id", cs.TaskID,
					"depth", depth+1)
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
	log.Debug("Collection parallel execution completed",
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
	depth int,
) error {
	log := workflow.GetLogger(ctx)
	strategy := taskConfig.GetStrategy()

	// For collection tasks, all children use the parent's task template
	// Collection child TaskIDs are dynamically generated (e.g., activity_analysis_item_0)
	// and don't exist as separate tasks in the workflow config

	// Process child tasks sequentially
	for i, childState := range childStates {
		log.Debug("Executing child task sequentially",
			"parent_task_id", taskConfig.ID,
			"child_task_id", childState.TaskID,
			"index", i,
			"depth", depth+1,
			"total", len(childStates))
		err := e.executeChild(ctx, cState.TaskExecID, childState, taskConfig.Task, depth)
		if err != nil {
			atomic.AddInt32(failed, 1)
			log.Error("Failed to execute child task",
				"parent_task_id", taskConfig.ID,
				"child_task_id", childState.TaskID,
				"index", i,
				"depth", depth+1,
				"error", err)
			// Handle strategy-based early termination
			if strategy == task.StrategyFailFast {
				log.Debug("Stopping collection execution due to FailFast strategy",
					"task_id", taskConfig.ID,
					"failed_at_index", i)
				break
			}
		} else {
			atomic.AddInt32(completed, 1)
			log.Debug("Child task completed successfully",
				"parent_task_id", taskConfig.ID,
				"child_task_id", childState.TaskID,
				"index", i,
				"depth", depth+1)
			// Handle Race strategy - stop on first success
			if strategy == task.StrategyRace {
				log.Debug("Stopping collection execution due to Race strategy",
					"task_id", taskConfig.ID,
					"succeeded_at_index", i)
				break
			}
		}
	}
	completedCount := atomic.LoadInt32(completed)
	failedCount := atomic.LoadInt32(failed)
	log.Debug("Collection sequential execution completed",
		"task_id", taskConfig.ID,
		"completed", completedCount,
		"failed", failedCount,
		"total", len(childStates))
	return nil
}

func (e *TaskExecutor) HandleCompositeTask(
	config *task.Config,
	depth ...int,
) func(ctx workflow.Context) (task.Response, error) {
	return func(ctx workflow.Context) (task.Response, error) {
		log := workflow.GetLogger(ctx)
		currentDepth := 0
		if len(depth) > 0 {
			currentDepth = depth[0]
		}

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

		// Load child configs from composite metadata
		childIDs := make([]string, len(childStates))
		for i, st := range childStates {
			childIDs[i] = st.TaskID
		}
		var childCfgs map[string]*task.Config
		err = workflow.ExecuteActivity(ctx, tkacts.LoadCompositeConfigsLabel, &tkacts.LoadCompositeConfigsInput{
			ParentTaskExecID: compositeState.TaskExecID,
			TaskIDs:          childIDs,
		}).Get(ctx, &childCfgs)
		if err != nil {
			return nil, fmt.Errorf("failed to load child configs: %w", err)
		}

		// Sort child states by task ID to ensure deterministic ordering
		// This matches the order defined in config.Tasks
		sort.Slice(childStates, func(i, j int) bool {
			// Find index of each task in the config
			iIdx := findTaskIndex(config.Tasks, childStates[i].TaskID)
			jIdx := findTaskIndex(config.Tasks, childStates[j].TaskID)
			return iIdx < jIdx
		})
		// Execute subtasks sequentially (composite tasks are always sequential)
		for i, childState := range childStates {
			childConfig := childCfgs[childState.TaskID]
			// Execute child task
			err := e.executeChild(ctx, compositeState.TaskExecID, childState, childConfig, currentDepth)
			if err != nil {
				log.Error("Child task failed",
					"composite_task", config.ID,
					"child_task", childState.TaskID,
					"index", i,
					"depth", currentDepth+1,
					"error", err)
				// Composite tasks always fail immediately on any child failure
				return nil, fmt.Errorf("child task %s failed: %w", childState.TaskID, err)
			}
			log.Debug("Child task completed",
				"composite_task", config.ID,
				"child_task", childState.TaskID,
				"index", i,
				"depth", currentDepth+1)
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
