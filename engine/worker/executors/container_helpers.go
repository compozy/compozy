package executors

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
		// Recurse for container tasks - bump depth
		response, err := h.ExecutionHandler(ctx, cfg, depth+1)
		if err != nil {
			return err
		}
		// Update child state record with final status from nested task execution
		// This ensures parent tasks can see the completion status
		if response != nil && response.GetState() != nil {
			finalState := response.GetState()
			// Create a simple activity to update the child state status
			updateInput := tkacts.UpdateChildStateInput{
				TaskExecID: childState.TaskExecID.String(),
				Status:     string(finalState.Status),
				Output:     finalState.Output,
			}
			// Add retry policy for this critical update
			// We MUST wait for this to complete to ensure database consistency
			activityOpts := workflow.ActivityOptions{
				StartToCloseTimeout: 30 * time.Second,
				RetryPolicy: &temporal.RetryPolicy{
					InitialInterval:    time.Second,
					BackoffCoefficient: 2.0,
					MaximumInterval:    10 * time.Second,
					MaximumAttempts:    5, // Increased attempts for critical operation
				},
			}
			activityCtx := workflow.WithActivityOptions(ctx, activityOpts)
			// CRITICAL: We must wait for this activity to complete or fail definitively
			// This ensures the database state is updated before the parent queries it
			err = workflow.ExecuteActivity(activityCtx, tkacts.UpdateChildStateLabel, updateInput).Get(activityCtx, nil)
			if err != nil {
				log.Error("Failed to update child state after nested task completion",
					"child_task_id", childState.TaskID, "final_status", finalState.Status, "error", err)
				// Return the error to ensure proper state synchronization
				// The parent needs to know that the state update failed
				return fmt.Errorf("failed to update child state %s: %w", childState.TaskID, err)
			}
		}
		return nil
	}
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
		// Let the error propagate for Temporal to handle retries
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
		// Race terminates on first result, either success or failure
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
) {
	log := workflow.GetLogger(ctx)
	// Sort child states by TaskID to ensure deterministic replay
	sort.Slice(childStates, func(i, j int) bool {
		return childStates[i].TaskID < childStates[j].TaskID
	})
	// Create semaphore to limit concurrent executions
	sem := workflow.NewSemaphore(ctx, MaxConcurrentChildTasks)
	for i := range childStates {
		childState := childStates[i]
		// Capture variables by value to avoid race conditions in goroutines
		cs := childState
		workflow.Go(ctx, func(gCtx workflow.Context) {
			// Acquire semaphore slot
			if err := sem.Acquire(gCtx, 1); err != nil {
				log.Error("Failed to acquire semaphore for child task",
					"child_task_id", cs.TaskID,
					"error", err)
				return
			}
			defer sem.Release(1)
			if gCtx.Err() != nil {
				return // canceled before start
			}
			cfg := getChildConfig(cs)
			err := h.executeChild(gCtx, parentState.TaskExecID, cs, cfg, depth)
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

// awaitStrategyCompletion waits for tasks to complete based on strategy
// It ensures all goroutines complete even after strategy is satisfied
func (h *ContainerHelpers) awaitStrategyCompletion(
	ctx workflow.Context,
	strategy task.ParallelStrategy,
	completed, failed *int32,
	total int32,
) error {
	// First wait for the strategy condition to be met
	err := workflow.Await(ctx, func() bool {
		completedCount := atomic.LoadInt32(completed)
		failedCount := atomic.LoadInt32(failed)
		return h.shouldCompleteParallelTask(strategy, completedCount, failedCount, total)
	})
	if err != nil {
		return err
	}
	// Then ensure all goroutines have finished to avoid database sync issues
	// This is critical for ensuring UpdateChildState activities complete
	// before GetCollectionResponse queries the database
	return workflow.Await(ctx, func() bool {
		completedCount := atomic.LoadInt32(completed)
		failedCount := atomic.LoadInt32(failed)
		// All tasks must have reached a terminal state
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
	// Sort child states by TaskID to ensure deterministic replay
	sort.Slice(childStates, func(i, j int) bool {
		return childStates[i].TaskID < childStates[j].TaskID
	})
	// Process child tasks sequentially
	for i, childState := range childStates {
		log.Debug("Executing child task sequentially",
			"parent_task_id", taskConfig.ID,
			"child_task_id", childState.TaskID,
			"index", i,
			"depth", depth+1,
			"total", len(childStates))
		cfg := getChildConfig(childState)
		err := h.executeChild(ctx, parentState.TaskExecID, childState, cfg, depth)
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
				log.Debug("Stopping execution due to FailFast strategy",
					"task_id", taskConfig.ID,
					"failed_at_index", i)
				// Mark remaining tasks as skipped to ensure awaitStrategyCompletion works correctly
				remainingInt := len(childStates) - i - 1
				if remainingInt > 0 && remainingInt <= math.MaxInt32 {
					remaining := int32(remainingInt)
					atomic.AddInt32(failed, remaining)
				}
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
				log.Debug("Stopping execution due to Race strategy",
					"task_id", taskConfig.ID,
					"succeeded_at_index", i)
				// Mark remaining tasks as completed to ensure awaitStrategyCompletion works correctly
				remainingInt := len(childStates) - i - 1
				if remainingInt > 0 && remainingInt <= math.MaxInt32 {
					remaining := int32(remainingInt)
					atomic.AddInt32(completed, remaining)
				}
				break
			}
		}
	}
	return nil
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
