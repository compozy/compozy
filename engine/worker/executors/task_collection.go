package executors

import (
	"fmt"
	"math"
	"sync/atomic"

	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
)

// CollectionTaskExecutor handles collection task execution
type CollectionTaskExecutor struct {
	*ContainerHelpers
}

// NewCollectionTaskExecutor creates a new collection task executor
func NewCollectionTaskExecutor(helpers *ContainerHelpers) *CollectionTaskExecutor {
	return &CollectionTaskExecutor{
		ContainerHelpers: helpers,
	}
}

// Execute implements the Executor interface for collection tasks
func (e *CollectionTaskExecutor) Execute(
	ctx workflow.Context,
	taskConfig *task.Config,
	depth int,
) (task.Response, error) {
	return e.ExecuteCollectionTask(taskConfig, depth)(ctx)
}

// ExecuteCollectionTask executes a collection task with optional depth parameter
func (e *CollectionTaskExecutor) ExecuteCollectionTask(
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

// CreateCollectionState creates a collection state via activity
func (e *CollectionTaskExecutor) CreateCollectionState(
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

// GetCollectionResponse gets the final collection response via activity
func (e *CollectionTaskExecutor) GetCollectionResponse(
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

// HandleCollectionTask handles the main collection task execution logic
func (e *CollectionTaskExecutor) HandleCollectionTask(
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

// handleCollectionParallel handles parallel execution of collection items
func (e *CollectionTaskExecutor) handleCollectionParallel(
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
	e.executeChildrenInParallel(ctx, cState, childStates, func(_ *task.State) *task.Config {
		return taskConfig.Task
	}, taskConfig, depth, completed, failed)
	// Wait for tasks to complete based on strategy
	err := e.awaitStrategyCompletion(ctx, taskConfig.GetStrategy(), completed, failed, childCount)
	if err != nil {
		return fmt.Errorf("failed to await collection task: %w", err)
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

// handleCollectionSequential handles sequential execution of collection items
func (e *CollectionTaskExecutor) handleCollectionSequential(
	ctx workflow.Context,
	cState *task.State,
	taskConfig *task.Config,
	childStates []*task.State,
	completed, failed *int32,
	depth int,
) error {
	log := workflow.GetLogger(ctx)
	// For collection tasks, all children use the parent's task template
	// Collection child TaskIDs are dynamically generated (e.g., activity_analysis_item_0)
	// and don't exist as separate tasks in the workflow config
	err := e.executeChildrenSequentially(ctx, cState, childStates, func(_ *task.State) *task.Config {
		return taskConfig.Task
	}, taskConfig, taskConfig.GetStrategy(), depth, completed, failed)
	if err != nil {
		return err
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
