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

func (e *CollectionTaskExecutor) loadCollectionChildren(
	ctx workflow.Context,
	parentState *task.State,
) ([]*task.State, map[string]*task.Config, int32, error) {
	childStates, err := e.ListChildStates(ctx, parentState.TaskExecID)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to list child states: %w", err)
	}
	if len(childStates) > math.MaxInt32 {
		return nil, nil, 0, fmt.Errorf(
			"too many child states: %d exceeds maximum of %d",
			len(childStates),
			math.MaxInt32,
		)
	}
	if len(childStates) == 0 {
		return childStates, nil, 0, nil
	}
	childIDs := make([]string, len(childStates))
	for i, st := range childStates {
		childIDs[i] = st.TaskID
	}
	var childCfgs map[string]*task.Config
	if err := workflow.ExecuteActivity(
		ctx,
		tkacts.LoadCollectionConfigsLabel,
		&tkacts.LoadCollectionConfigsInput{
			ParentTaskExecID: parentState.TaskExecID,
			TaskIDs:          childIDs,
		},
	).Get(ctx, &childCfgs); err != nil {
		return nil, nil, 0, fmt.Errorf("failed to load child configs: %w", err)
	}
	childCount := int32(len(childStates)) // #nosec G115: len bounded by check above
	return childStates, childCfgs, childCount, nil
}

// HandleCollectionTask handles the main collection task execution logic
func (e *CollectionTaskExecutor) HandleCollectionTask(
	ctx workflow.Context,
	cState *task.State,
	taskConfig *task.Config,
	depth int,
) error {
	childStates, childCfgs, childCount, err := e.loadCollectionChildren(ctx, cState)
	if err != nil {
		return err
	}
	var completed, failed int32
	log := workflow.GetLogger(ctx)
	if childCount == 0 {
		log.Debug("Collection task has no items, completing successfully",
			"task_id", taskConfig.ID,
			"parent_state_id", cState.TaskExecID)
		return nil
	}
	rawConcurrency := collectionConcurrencyLimit(taskConfig)
	effectiveConcurrency := rawConcurrency
	// Default to max concurrency when no explicit limit is set
	if effectiveConcurrency <= 0 {
		effectiveConcurrency = MaxConcurrentChildTasks
	} else if effectiveConcurrency > MaxConcurrentChildTasks {
		// Clamp to system maximum
		effectiveConcurrency = MaxConcurrentChildTasks
	}
	log.Debug("Executing collection child tasks",
		"task_id", taskConfig.ID,
		"child_count", len(childStates),
		"expected_count", childCount,
		"mode", taskConfig.GetMode(),
		"max_workers", taskConfig.GetMaxWorkers(),
		"batch_limit", taskConfig.Batch,
		"effective_concurrency", effectiveConcurrency)
	return e.executeCollectionByMode(
		ctx,
		cState,
		taskConfig,
		childStates,
		childCfgs,
		&completed,
		&failed,
		childCount,
		depth,
		rawConcurrency,
	)
}

// executeCollectionByMode routes collection execution based on the configured mode.
func (e *CollectionTaskExecutor) executeCollectionByMode(
	ctx workflow.Context,
	cState *task.State,
	taskConfig *task.Config,
	childStates []*task.State,
	childCfgs map[string]*task.Config,
	completed, failed *int32,
	childCount int32,
	depth int,
	rawConcurrency int,
) error {
	switch taskConfig.GetMode() {
	case task.CollectionModeSequential:
		return e.handleCollectionSequential(ctx, cState, taskConfig, childStates, childCfgs, completed, failed, depth)
	case task.CollectionModeParallel:
		return e.handleCollectionParallel(
			ctx,
			cState,
			taskConfig,
			childStates,
			childCfgs,
			completed,
			failed,
			childCount,
			depth,
			rawConcurrency,
		)
	default:
		return e.handleCollectionParallel(
			ctx,
			cState,
			taskConfig,
			childStates,
			childCfgs,
			completed,
			failed,
			childCount,
			depth,
			rawConcurrency,
		)
	}
}

// handleCollectionParallel handles parallel execution of collection items
func (e *CollectionTaskExecutor) handleCollectionParallel(
	ctx workflow.Context,
	cState *task.State,
	taskConfig *task.Config,
	childStates []*task.State,
	childCfgs map[string]*task.Config,
	completed, failed *int32,
	childCount int32,
	depth int,
	maxConcurrency int,
) error {
	log := workflow.GetLogger(ctx)
	// Use the loaded child configs instead of the template
	e.executeChildrenInParallel(ctx, cState, childStates, func(cs *task.State) *task.Config {
		return childCfgs[cs.TaskID]
	}, taskConfig, depth, completed, failed, maxConcurrency)
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
	childCfgs map[string]*task.Config,
	completed, failed *int32,
	depth int,
) error {
	log := workflow.GetLogger(ctx)
	// Use the loaded child configs instead of the template
	err := e.executeChildrenSequentially(ctx, cState, childStates, func(cs *task.State) *task.Config {
		return childCfgs[cs.TaskID]
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

func collectionConcurrencyLimit(cfg *task.Config) int {
	if cfg == nil {
		return 0
	}
	limit := cfg.GetMaxWorkers()
	if cfg.Batch > 0 && (limit == 0 || cfg.Batch < limit) {
		limit = cfg.Batch
	}
	if limit < 0 {
		return 0
	}
	return limit
}
