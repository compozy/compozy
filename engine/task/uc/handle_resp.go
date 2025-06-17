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

func (uc *HandleResponse) Execute(ctx context.Context, input *HandleResponseInput) (task.Response, error) {
	// Process task execution result and determine success status
	isSuccess, executionErr := uc.processTaskResult(ctx, input)

	// Save state with context handling
	if err := uc.saveStateWithContextHandling(ctx, input.TaskState); err != nil {
		if ctx.Err() != nil {
			return &task.MainTaskResponse{State: input.TaskState}, nil
		}
		return nil, err
	}

	// Update parent status and handle context cancellation
	uc.logParentStatusUpdateError(ctx, input.TaskState)
	if ctx.Err() != nil {
		return &task.MainTaskResponse{State: input.TaskState}, nil
	}

	// Process transitions and validate error handling
	onSuccess, onError, err := uc.processTransitionsWithValidation(ctx, input, isSuccess, executionErr)
	if err != nil {
		if ctx.Err() != nil {
			return &task.MainTaskResponse{State: input.TaskState}, nil
		}
		return nil, err
	}

	// Determine next task
	nextTask := uc.selectNextTask(input, isSuccess)

	return &task.MainTaskResponse{
		OnSuccess: onSuccess,
		OnError:   onError,
		State:     input.TaskState,
		NextTask:  nextTask,
	}, nil
}

// processTaskResult handles output transformation and determines final success status
func (uc *HandleResponse) processTaskResult(ctx context.Context, input *HandleResponseInput) (bool, error) {
	state := input.TaskState
	executionErr := input.ExecutionError

	// If successful so far, try to apply output transformation.
	// If transformation fails, it becomes a task failure.
	isSuccess := executionErr == nil && state.Status != core.StatusFailed
	if isSuccess {
		state.UpdateStatus(core.StatusSuccess)
		if input.TaskConfig.GetOutputs() != nil && state.Output != nil {
			if err := uc.applyOutputTransformation(ctx, input); err != nil {
				executionErr = err // Transition to failure
				isSuccess = false
			}
		}
	}

	// Handle final state (success or failure)
	if !isSuccess {
		state.UpdateStatus(core.StatusFailed)
		uc.setErrorState(state, executionErr)
	}

	return isSuccess, executionErr
}

// saveStateWithContextHandling saves state and handles context cancellation
func (uc *HandleResponse) saveStateWithContextHandling(ctx context.Context, state *task.State) error {
	if err := uc.taskRepo.UpsertState(ctx, state); err != nil {
		return fmt.Errorf("failed to update task state: %w", err)
	}
	return nil
}

// processTransitionsWithValidation normalizes transitions and validates error handling
func (uc *HandleResponse) processTransitionsWithValidation(
	ctx context.Context,
	input *HandleResponseInput,
	isSuccess bool,
	executionErr error,
) (*core.SuccessTransition, *core.ErrorTransition, error) {
	onSuccess, onError, err := uc.normalizeTransitions(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to normalize transitions: %w", err)
	}

	if !isSuccess && (onError == nil || onError.Next == nil) {
		if executionErr != nil {
			return nil, nil, fmt.Errorf("task failed with no error transition defined: %w", executionErr)
		}
		return nil, nil, errors.New("task failed with no error transition defined")
	}

	return onSuccess, onError, nil
}

// selectNextTask determines the next task based on override or workflow configuration
func (uc *HandleResponse) selectNextTask(input *HandleResponseInput, isSuccess bool) *task.Config {
	if input.NextTaskOverride != nil {
		return input.NextTaskOverride
	}
	return input.WorkflowConfig.DetermineNextTask(input.TaskConfig, isSuccess)
}

func (uc *HandleResponse) applyOutputTransformation(ctx context.Context, input *HandleResponseInput) error {
	workflowState, err := uc.workflowRepo.GetState(ctx, input.TaskState.WorkflowExecID)
	if err != nil {
		return fmt.Errorf("failed to get workflow state for output transformation: %w", err)
	}
	output, err := uc.normalizer.NormalizeTaskOutput(
		input.TaskState.Output,
		input.TaskConfig.GetOutputs(),
		workflowState,
		input.WorkflowConfig,
		input.TaskConfig,
	)
	if err != nil {
		return fmt.Errorf("failed to apply output transformation: %w", err)
	}
	input.TaskState.Output = output
	return nil
}

func (uc *HandleResponse) setErrorState(state *task.State, executionErr error) {
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
	log := logger.FromContext(ctx)
	// Only proceed if this is a child task (has a parent)
	if childState.ParentStateID == nil {
		return nil
	}
	parentStateID := *childState.ParentStateID
	parentState, err := uc.taskRepo.GetState(ctx, parentStateID)
	if err != nil {
		return fmt.Errorf("failed to get parent state %s: %w", parentStateID, err)
	}
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
						log.Error("Invalid parallel strategy found, using default wait_all",
							"invalid_strategy", strategyStr,
							"parent_state_id", parentStateID,
						)
					}
				} else {
					// Strategy exists but is not a string - log the type mismatch
					log.Error("Parallel strategy field is not a string, using default wait_all",
						"strategy_type", fmt.Sprintf("%T", strategyValue),
						"strategy_value", strategyValue,
						"parent_state_id", parentStateID,
					)
				}
			} else {
				// Strategy field is missing from parallel config
				log.Debug("No strategy field found in parallel config, using default wait_all",
					"parent_state_id", parentStateID,
				)
			}
		} else {
			// _parallel_config exists but is not the expected map type
			if _, exists := (*parentState.Input)["_parallel_config"]; exists {
				log.Error("Parallel config field is not a map, using default wait_all",
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
	log := logger.FromContext(ctx)
	if err := uc.updateParentStatusIfNeeded(ctx, state); err != nil {
		log.Debug("failed to update parent status", "error", err)
	}
}
