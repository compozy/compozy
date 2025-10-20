package executors

import (
	"fmt"
	"math"
	"sort"
	"sync/atomic"
	"time"

	temporalLog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
)

const (
	// MaxConcurrentChildTasks limits the number of concurrent child workflow goroutines
	// to prevent exhausting the deterministic scheduler
	MaxConcurrentChildTasks = 100
)

// ContainerHelpers provides shared functionality for container tasks (parallel, collection, composite)
type ContainerHelpers struct {
	*ContextBuilder
	// ExecutionHandler allows recursive calls to HandleExecution for nested tasks
	ExecutionHandler func(ctx workflow.Context, taskConfig *task.Config, depth ...int) (task.Response, error)
}

// NewContainerHelpers creates a new ContainerHelpers instance
func NewContainerHelpers(
	cb *ContextBuilder,
	handler func(ctx workflow.Context, taskConfig *task.Config, depth ...int) (task.Response, error),
) *ContainerHelpers {
	return &ContainerHelpers{
		ContextBuilder:   cb,
		ExecutionHandler: handler,
	}
}

// executeChild executes a child task, handling both Basic tasks (using ExecuteSubtask)
// and container tasks (recursively calling HandleExecution)
func (h *ContainerHelpers) executeChild(
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
		_, err := h.ExecuteSubtask(ctx, parentStateID, childState.TaskExecID.String())
		return err
	default:
		response, err := h.ExecutionHandler(ctx, cfg, depth+1)
		if err != nil {
			return err
		}
		return h.updateChildStateAfterExecution(ctx, log, childState, response, depth)
	}
}

// updateChildStateAfterExecution persists nested task state updates when required.
func (h *ContainerHelpers) updateChildStateAfterExecution(
	ctx workflow.Context,
	log temporalLog.Logger,
	childState *task.State,
	response task.Response,
	depth int,
) error {
	if response == nil || response.GetState() == nil {
		return nil
	}
	finalState := response.GetState()
	updateInput := tkacts.UpdateChildStateInput{
		TaskExecID: childState.TaskExecID.String(),
		Status:     string(finalState.Status),
		Output:     finalState.Output,
	}
	activityOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    10 * time.Second,
			MaximumAttempts:    5,
		},
	}
	activityCtx := workflow.WithActivityOptions(ctx, activityOpts)
	future := workflow.ExecuteActivity(activityCtx, tkacts.UpdateChildStateLabel, updateInput)
	if err := future.Get(activityCtx, nil); err != nil {
		log.Error("Failed to update child state after nested task completion",
			"child_task_id", childState.TaskID,
			"final_status", finalState.Status,
			"depth", depth+1,
			"error", err)
		return fmt.Errorf("failed to update child state %s: %w", childState.TaskID, err)
	}
	return nil
}

// ListChildStates fetches child states for a parent task
func (h *ContainerHelpers) ListChildStates(
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

// ExecuteSubtask executes a subtask
func (h *ContainerHelpers) ExecuteSubtask(
	ctx workflow.Context,
	parentStateID core.ID,
	taskExecID string,
) (*task.SubtaskResponse, error) {
	actLabel := tkacts.ExecuteSubtaskLabel
	actInput := tkacts.ExecuteSubtaskInput{
		WorkflowID:     h.WorkflowID,
		WorkflowExecID: h.WorkflowExecID,
		ParentStateID:  parentStateID,
		TaskExecID:     taskExecID,
	}
	future := workflow.ExecuteActivity(ctx, actLabel, actInput)
	var response *task.SubtaskResponse
	err := future.Get(ctx, &response)
	if err != nil {
		// NOTE: Surface activity errors so Temporal can apply retry/backoff policies.
		return nil, err
	}
	return response, nil
}

// shouldCompleteParallelTask determines if parallel task execution should complete based on strategy
func (h *ContainerHelpers) shouldCompleteParallelTask(
	strategy task.ParallelStrategy,
	completed, failed, total int32,
) bool {
	switch strategy {
	case task.StrategyWaitAll, task.StrategyBestEffort:
		return (completed + failed) >= total
	case task.StrategyFailFast:
		return failed > 0 || completed >= total
	case task.StrategyRace:
		return completed > 0 || failed > 0
	default:
		return (completed + failed) >= total
	}
}

// executeChildrenInParallel executes child tasks in parallel using goroutines
func (h *ContainerHelpers) executeChildrenInParallel(
	ctx workflow.Context,
	parentState *task.State,
	childStates []*task.State,
	getChildConfig func(*task.State) *task.Config,
	taskConfig *task.Config,
	depth int,
	completed, failed *int32,
	maxConcurrency int,
) {
	log := workflow.GetLogger(ctx)
	sort.Slice(childStates, func(i, j int) bool {
		return childStates[i].TaskID < childStates[j].TaskID
	})
	limit := determineParallelLimit(maxConcurrency)
	sem := workflow.NewSemaphore(ctx, int64(limit))
	for i := range childStates {
		child := childStates[i]
		workflow.Go(ctx, func(gCtx workflow.Context) {
			h.runParallelChild(
				gCtx,
				sem,
				log,
				parentState,
				taskConfig,
				child,
				getChildConfig,
				depth,
				completed,
				failed,
			)
		})
	}
}

// determineParallelLimit calculates the concurrency cap respecting defaults.
func determineParallelLimit(maxConcurrency int) int {
	limit := MaxConcurrentChildTasks
	if maxConcurrency > 0 && maxConcurrency < limit {
		limit = maxConcurrency
	}
	if limit <= 0 {
		return 1
	}
	return limit
}

// runParallelChild executes a child task respecting the semaphore limit and metrics.
func (h *ContainerHelpers) runParallelChild(
	ctx workflow.Context,
	sem workflow.Semaphore,
	log temporalLog.Logger,
	parentState *task.State,
	taskConfig *task.Config,
	childState *task.State,
	getChildConfig func(*task.State) *task.Config,
	depth int,
	completed, failed *int32,
) {
	if err := sem.Acquire(ctx, 1); err != nil {
		log.Error("Failed to acquire semaphore for child task",
			"child_task_id", childState.TaskID,
			"error", err)
		return
	}
	defer sem.Release(1)
	if ctx.Err() != nil {
		return
	}
	cfg := getChildConfig(childState)
	err := h.executeChild(ctx, parentState.TaskExecID, childState, cfg, depth)
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		log.Error("Failed to execute child task",
			"parent_task_id", taskConfig.ID,
			"child_task_id", childState.TaskID,
			"depth", depth+1,
			"error", err)
		atomic.AddInt32(failed, 1)
		return
	}
	atomic.AddInt32(completed, 1)
	log.Debug("Child task completed successfully",
		"parent_task_id", taskConfig.ID,
		"child_task_id", childState.TaskID,
		"depth", depth+1)
}

// awaitStrategyCompletion waits for tasks to complete based on strategy
// It ensures all goroutines complete even after strategy is satisfied
func (h *ContainerHelpers) awaitStrategyCompletion(
	ctx workflow.Context,
	strategy task.ParallelStrategy,
	completed, failed *int32,
	total int32,
) error {
	err := workflow.Await(ctx, func() bool {
		completedCount := atomic.LoadInt32(completed)
		failedCount := atomic.LoadInt32(failed)
		return h.shouldCompleteParallelTask(strategy, completedCount, failedCount, total)
	})
	if err != nil {
		return err
	}
	// NOTE: Ensure every subtask reaches a terminal state before querying aggregated responses.
	return workflow.Await(ctx, func() bool {
		completedCount := atomic.LoadInt32(completed)
		failedCount := atomic.LoadInt32(failed)
		return (completedCount + failedCount) >= total
	})
}

// executeChildrenSequentially executes child tasks sequentially
func (h *ContainerHelpers) executeChildrenSequentially(
	ctx workflow.Context,
	parentState *task.State,
	childStates []*task.State,
	getChildConfig func(*task.State) *task.Config,
	taskConfig *task.Config,
	strategy task.ParallelStrategy,
	depth int,
	completed, failed *int32,
) error {
	log := workflow.GetLogger(ctx)
	sort.Slice(childStates, func(i, j int) bool {
		return childStates[i].TaskID < childStates[j].TaskID
	})
	total := len(childStates)
	for index, childState := range childStates {
		log.Debug("Executing child task sequentially",
			"parent_task_id", taskConfig.ID,
			"child_task_id", childState.TaskID,
			"index", index,
			"depth", depth+1,
			"total", total)
		cfg := getChildConfig(childState)
		execErr := h.executeChild(ctx, parentState.TaskExecID, childState, cfg, depth)
		if stop := h.handleSequentialOutcome(
			log,
			taskConfig,
			childState,
			strategy,
			depth,
			index,
			total,
			execErr,
			completed,
			failed,
		); stop {
			break
		}
	}
	return nil
}

// handleSequentialOutcome updates counters and determines whether to stop iterating.
func (h *ContainerHelpers) handleSequentialOutcome(
	log temporalLog.Logger,
	taskConfig *task.Config,
	childState *task.State,
	strategy task.ParallelStrategy,
	depth int,
	index int,
	total int,
	execErr error,
	completed, failed *int32,
) bool {
	if execErr != nil {
		atomic.AddInt32(failed, 1)
		log.Error("Failed to execute child task",
			"parent_task_id", taskConfig.ID,
			"child_task_id", childState.TaskID,
			"index", index,
			"depth", depth+1,
			"error", execErr)
		if strategy == task.StrategyFailFast {
			log.Debug("Stopping execution due to FailFast strategy",
				"task_id", taskConfig.ID,
				"failed_at_index", index)
			markRemainingAs(failed, total-index-1)
			return true
		}
		return false
	}
	atomic.AddInt32(completed, 1)
	log.Debug("Child task completed successfully",
		"parent_task_id", taskConfig.ID,
		"child_task_id", childState.TaskID,
		"index", index,
		"depth", depth+1)
	if strategy == task.StrategyRace {
		log.Debug("Stopping execution due to Race strategy",
			"task_id", taskConfig.ID,
			"succeeded_at_index", index)
		markRemainingAs(completed, total-index-1)
		return true
	}
	return false
}

// markRemainingAs atomically adjusts counters for remaining tasks when terminating early.
func markRemainingAs(counter *int32, remainingInt int) {
	if remainingInt <= 0 || remainingInt > math.MaxInt32 {
		return
	}
	atomic.AddInt32(counter, int32(remainingInt))
}

// findTaskIndex finds the index of a task ID in the task config slice
func (h *ContainerHelpers) findTaskIndex(tasks []task.Config, taskID string) int {
	for i := range tasks {
		if tasks[i].ID == taskID {
			return i
		}
	}
	return -1 // Not found, will sort to the end
}
