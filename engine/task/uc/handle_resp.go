package uc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"maps"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/normalizer"
)

// -----------------------------------------------------------------------------
// HandleResponse
// -----------------------------------------------------------------------------

type HandleResponseInput struct {
	WorkflowConfig   *workflow.Config `json:"workflow_config"`
	TaskState        *task.State      `json:"task_state"`
	TaskConfig       *task.Config     `json:"task_config"`
	ExecutionError   error            `json:"execution_error"`
	NextTaskOverride *task.Config     `json:"next_task_override,omitempty"`
}

type HandleResponse struct {
	workflowRepo workflow.Repository
	taskRepo     task.Repository
	normalizer   *normalizer.ConfigNormalizer
}

func NewHandleResponse(workflowRepo workflow.Repository, taskRepo task.Repository) *HandleResponse {
	return &HandleResponse{
		workflowRepo: workflowRepo,
		taskRepo:     taskRepo,
		normalizer:   normalizer.NewConfigNormalizer(),
	}
}

func (uc *HandleResponse) Execute(ctx context.Context, input *HandleResponseInput) (*task.Response, error) {
	// Check if there's an execution error OR if the task state indicates failure
	hasExecutionError := input.ExecutionError != nil
	hasTaskFailure := input.TaskState.Status == core.StatusFailed
	if hasExecutionError || hasTaskFailure {
		return uc.handleErrorFlow(ctx, input)
	}
	return uc.handleSuccessFlow(ctx, input)
}

func (uc *HandleResponse) handleSuccessFlow(
	ctx context.Context,
	input *HandleResponseInput,
) (*task.Response, error) {
	state := input.TaskState
	state.UpdateStatus(core.StatusSuccess)
	if input.TaskConfig.GetOutputs() != nil && state.Output != nil {
		workflowState, err := uc.workflowRepo.GetState(ctx, input.TaskState.WorkflowExecID)
		if err != nil {
			return nil, fmt.Errorf("failed to get workflow state for output transformation: %w", err)
		}
		output, err := uc.normalizer.NormalizeTaskOutput(
			state.Output,
			input.TaskConfig.GetOutputs(),
			workflowState,
			input.WorkflowConfig,
			input.TaskConfig,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to apply output transformation: %w", err)
		}
		state.Output = output
	}
	if err := uc.taskRepo.UpsertState(ctx, state); err != nil {
		if ctx.Err() != nil {
			return &task.Response{State: state}, nil
		}
		return nil, fmt.Errorf("failed to update task state: %w", err)
	}

	// Update parent task status if this is a child task
	// Parent status updates are non-critical, so we ignore errors to avoid failing task completion
	if err := uc.updateParentStatusIfNeeded(ctx, state); err != nil {
		if ctx.Err() != nil {
			return &task.Response{State: state}, nil
		}
		return nil, fmt.Errorf("failed to update parent status: %w", err)
	}
	if ctx.Err() != nil {
		return &task.Response{State: state}, nil
	}
	onSuccess, onError, err := uc.normalizeTransitions(ctx, input)
	if err != nil {
		if ctx.Err() != nil {
			return &task.Response{State: state}, nil
		}
		return nil, fmt.Errorf("failed to normalize transitions: %w", err)
	}
	var nextTask *task.Config
	if input.NextTaskOverride != nil {
		nextTask = input.NextTaskOverride
	} else {
		nextTask = input.WorkflowConfig.DetermineNextTask(input.TaskConfig, true)
	}
	return &task.Response{
		OnSuccess: onSuccess,
		OnError:   onError,
		State:     state,
		NextTask:  nextTask,
	}, nil
}

func (uc *HandleResponse) handleErrorFlow(
	ctx context.Context,
	input *HandleResponseInput,
) (*task.Response, error) {
	state := input.TaskState
	executionErr := input.ExecutionError
	state.UpdateStatus(core.StatusFailed)

	// Handle case where task failed but there's no execution error (e.g., parent task with failed child tasks)
	if executionErr == nil {
		var errorMessage string
		if state.IsParentTask() {
			errorMessage = "parent task execution failed due to child task failures"
		} else {
			errorMessage = "task execution failed"
		}
		state.Error = core.NewError(errors.New(errorMessage), "execution_error", nil)
	} else {
		state.Error = core.NewError(executionErr, "execution_error", nil)
	}

	if updateErr := uc.taskRepo.UpsertState(ctx, state); updateErr != nil {
		if ctx.Err() != nil {
			return &task.Response{State: state}, nil
		}
		return nil, fmt.Errorf("failed to update task state after error: %w", updateErr)
	}

	// Update parent task status if this is a child task (for error case)
	// Parent status updates are non-critical, so we ignore errors to avoid failing task completion
	if err := uc.updateParentStatusIfNeeded(ctx, state); err != nil {
		if ctx.Err() != nil {
			return &task.Response{State: state}, nil
		}
		return nil, fmt.Errorf("failed to update parent status: %w", err)
	}
	if ctx.Err() != nil {
		return &task.Response{State: state}, nil
	}
	if input.TaskConfig.OnError == nil || input.TaskConfig.OnError.Next == nil {
		// For cases where execution error is nil, we shouldn't propagate it
		if executionErr != nil {
			return nil, fmt.Errorf("task failed with no error transition defined: %w", executionErr)
		}
		return nil, fmt.Errorf("task failed with no error transition defined")
	}
	onSuccess, onError, err := uc.normalizeTransitions(ctx, input)
	if err != nil {
		if ctx.Err() != nil {
			return &task.Response{State: state}, nil
		}
		return nil, fmt.Errorf("failed to normalize transitions: %w", err)
	}
	nextTask := input.WorkflowConfig.DetermineNextTask(input.TaskConfig, false)
	return &task.Response{
		OnSuccess: onSuccess,
		OnError:   onError,
		State:     state,
		NextTask:  nextTask,
	}, nil
}

func (uc *HandleResponse) normalizeTransitions(
	ctx context.Context,
	input *HandleResponseInput,
) (*core.SuccessTransition, *core.ErrorTransition, error) {
	workflowExecID := input.TaskState.WorkflowExecID
	workflowConfig := input.WorkflowConfig
	taskConfig := input.TaskConfig
	tasks := workflowConfig.Tasks
	allTaskConfigs := normalizer.BuildTaskConfigsMap(tasks)
	workflowState, err := uc.workflowRepo.GetState(ctx, workflowExecID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get workflow state: %w", err)
	}
	err = uc.normalizer.NormalizeTaskEnvironment(workflowConfig, taskConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to normalize base environment: %w", err)
	}
	normalizedOnSuccess, err := uc.normalizeSuccessTransition(
		taskConfig.OnSuccess,
		workflowState,
		workflowConfig,
		allTaskConfigs,
		taskConfig.Env,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to normalize success transition: %w", err)
	}
	normalizedOnError, err := uc.normalizeErrorTransition(
		taskConfig.OnError,
		workflowState,
		workflowConfig,
		allTaskConfigs,
		taskConfig.Env,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to normalize error transition: %w", err)
	}
	return normalizedOnSuccess, normalizedOnError, nil
}

func (uc *HandleResponse) normalizeSuccessTransition(
	transition *core.SuccessTransition,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	allTaskConfigs map[string]*task.Config,
	baseEnv *core.EnvMap,
) (*core.SuccessTransition, error) {
	if transition == nil {
		return nil, nil
	}
	normalizedTransition := &core.SuccessTransition{
		Next: transition.Next,
		With: transition.With,
	}
	if transition.With != nil {
		withCopy := make(core.Input)
		maps.Copy(withCopy, *transition.With)
		normalizedTransition.With = &withCopy
	}
	if err := uc.normalizer.NormalizeSuccessTransition(
		normalizedTransition,
		workflowState,
		workflowConfig,
		allTaskConfigs,
		baseEnv,
	); err != nil {
		return nil, err
	}
	return normalizedTransition, nil
}

func (uc *HandleResponse) normalizeErrorTransition(
	transition *core.ErrorTransition,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	allTaskConfigs map[string]*task.Config,
	baseEnv *core.EnvMap,
) (*core.ErrorTransition, error) {
	if transition == nil {
		return nil, nil
	}
	normalizedTransition := &core.ErrorTransition{
		Next: transition.Next,
		With: transition.With,
	}
	if transition.With != nil {
		withCopy := make(core.Input)
		maps.Copy(withCopy, *transition.With)
		normalizedTransition.With = &withCopy
	}
	if err := uc.normalizer.NormalizeErrorTransition(
		normalizedTransition,
		workflowState,
		workflowConfig,
		allTaskConfigs,
		baseEnv,
	); err != nil {
		return nil, err
	}
	return normalizedTransition, nil
}

// updateParentStatusIfNeeded updates the parent task status when a child task completes
func (uc *HandleResponse) updateParentStatusIfNeeded(ctx context.Context, childState *task.State) error {
	// Only proceed if this is a child task (has a parent)
	if childState.ParentStateID == nil {
		return nil
	}

	parentStateID := *childState.ParentStateID

	// Get the parent task to determine the parallel strategy
	parentState, err := uc.taskRepo.GetState(ctx, parentStateID)
	if err != nil {
		return fmt.Errorf("failed to get parent state %s: %w", parentStateID, err)
	}

	// Only update parent status for parallel tasks
	if parentState.ExecutionType != task.ExecutionParallel {
		return nil
	}

	// Get the parallel configuration to determine strategy
	// The parallel strategy should be stored in the parent task's input metadata
	strategy := task.StrategyWaitAll // Default strategy
	if parentState.Input != nil {
		if parallelConfig, ok := (*parentState.Input)["_parallel_config"].(map[string]any); ok {
			if strategyStr, ok := parallelConfig["strategy"].(string); ok {
				strategy = task.ParallelStrategy(strategyStr)
			}
		}
	}

	// Get progress information from child tasks
	progressInfo, err := uc.taskRepo.GetProgressInfo(ctx, parentStateID)
	if err != nil {
		return fmt.Errorf("failed to get progress info for parent %s: %w", parentStateID, err)
	}

	// Calculate new status based on strategy and child progress
	newStatus := progressInfo.CalculateOverallStatus(strategy)

	// Only update if status has changed and should be updated
	if parentState.Status != newStatus && uc.shouldUpdateParentStatus(parentState.Status, newStatus) {
		parentState.Status = newStatus

		// Add progress metadata to parent task output
		if parentState.Output == nil {
			parentState.Output = &core.Output{}
		}

		progressOutput := map[string]any{
			"completion_rate": progressInfo.CompletionRate,
			"failure_rate":    progressInfo.FailureRate,
			"total_children":  progressInfo.TotalChildren,
			"completed_count": progressInfo.CompletedCount,
			"failed_count":    progressInfo.FailedCount,
			"running_count":   progressInfo.RunningCount,
			"pending_count":   progressInfo.PendingCount,
			"strategy":        string(strategy),
			"last_updated":    fmt.Sprintf("%d", time.Now().Unix()),
		}
		(*parentState.Output)["progress_info"] = progressOutput

		// Set error if parent task failed due to child failures
		if newStatus == core.StatusFailed && progressInfo.HasFailures() {
			parentState.Error = core.NewError(
				fmt.Errorf("parent task failed due to child task failures"),
				"child_task_failure",
				map[string]any{
					"failed_count":    progressInfo.FailedCount,
					"completed_count": progressInfo.CompletedCount,
					"total_children":  progressInfo.TotalChildren,
					"child_task_id":   childState.TaskID,
					"child_status":    string(childState.Status),
				},
			)
		}

		// Update parent state in database
		if err := uc.taskRepo.UpsertState(ctx, parentState); err != nil {
			return fmt.Errorf("failed to update parent state %s: %w", parentStateID, err)
		}

		// Recursively update grandparent if needed
		if parentState.ParentStateID != nil {
			return uc.updateParentStatusIfNeeded(ctx, parentState)
		}
	}

	return nil
}

// shouldUpdateParentStatus determines if parent status should be updated
func (uc *HandleResponse) shouldUpdateParentStatus(currentStatus, newStatus core.StatusType) bool {
	// Don't update if status hasn't changed
	if currentStatus == newStatus {
		return false
	}

	// Allow transitions to terminal states
	if newStatus == core.StatusSuccess || newStatus == core.StatusFailed {
		return true
	}

	// Allow transitions from pending/running to other active states
	if currentStatus == core.StatusPending || currentStatus == core.StatusRunning {
		return true
	}

	// Don't update from terminal states unless moving to another terminal state
	if currentStatus == core.StatusSuccess || currentStatus == core.StatusFailed {
		return newStatus == core.StatusSuccess || newStatus == core.StatusFailed
	}

	return false
}
