package uc

import (
	"context"
	"errors"
	"fmt"

	"maps"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
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
	workflowRepo        workflow.Repository
	taskRepo            task.Repository
	normalizer          *normalizer.ConfigNormalizer
	parentStatusUpdater *services.ParentStatusUpdater
}

func NewHandleResponse(workflowRepo workflow.Repository, taskRepo task.Repository) *HandleResponse {
	return &HandleResponse{
		workflowRepo:        workflowRepo,
		taskRepo:            taskRepo,
		normalizer:          normalizer.NewConfigNormalizer(),
		parentStatusUpdater: services.NewParentStatusUpdater(taskRepo),
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

	// Update parent task status if this is a child task (non-critical operation)
	uc.logParentStatusUpdateError(ctx, state)
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
		if state.IsParallelExecution() {
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

	// Update parent task status if this is a child task (non-critical operation)
	uc.logParentStatusUpdateError(ctx, state)
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
			if strategyValue, exists := parallelConfig["strategy"]; exists {
				// Explicit type validation for strategy value
				if strategyStr, ok := strategyValue.(string); ok {
					if task.ValidateStrategy(strategyStr) {
						strategy = task.ParallelStrategy(strategyStr)
					} else {
						logger.Error("Invalid parallel strategy found, using default wait_all",
							"invalid_strategy", strategyStr,
							"parent_state_id", parentStateID,
						)
					}
				} else {
					// Strategy exists but is not a string - log the type mismatch
					logger.Error("Parallel strategy field is not a string, using default wait_all",
						"strategy_type", fmt.Sprintf("%T", strategyValue),
						"strategy_value", strategyValue,
						"parent_state_id", parentStateID,
					)
				}
			} else {
				// Strategy field is missing from parallel config
				logger.Debug("No strategy field found in parallel config, using default wait_all",
					"parent_state_id", parentStateID,
				)
			}
		} else {
			// _parallel_config exists but is not the expected map type
			if _, exists := (*parentState.Input)["_parallel_config"]; exists {
				logger.Error("Parallel config field is not a map, using default wait_all",
					"config_type", fmt.Sprintf("%T", (*parentState.Input)["_parallel_config"]),
					"parent_state_id", parentStateID,
				)
			}
		}
	}

	// Use the shared service to update parent status
	_, err = uc.parentStatusUpdater.UpdateParentStatus(ctx, &services.UpdateParentStatusInput{
		ParentStateID: parentStateID,
		Strategy:      strategy,
		Recursive:     true,
		ChildState:    childState,
	})

	return err
}

// logParentStatusUpdateError updates parent status and logs any errors without propagating them
// Parent status updates are non-critical operations that should not fail task completion
func (uc *HandleResponse) logParentStatusUpdateError(ctx context.Context, state *task.State) {
	if err := uc.updateParentStatusIfNeeded(ctx, state); err != nil {
		logger.Debug("failed to update parent status", "error", err)
	}
}
