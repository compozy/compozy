package uc

import (
	"context"
	"errors"
	"fmt"

	"maps"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task2"
	task2core "github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
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
	workflowRepo                workflow.Repository
	taskRepo                    task.Repository
	parentStatusUpdater         *services.ParentStatusUpdater
	successTransitionNormalizer *task2core.SuccessTransitionNormalizer
	errorTransitionNormalizer   *task2core.ErrorTransitionNormalizer
	outputTransformer           *task2core.OutputTransformer
}

func NewHandleResponse(workflowRepo workflow.Repository, taskRepo task.Repository) *HandleResponse {
	// Create template engine for task2 normalizers
	tplEngine := tplengine.NewEngine(tplengine.FormatJSON)
	return &HandleResponse{
		workflowRepo:                workflowRepo,
		taskRepo:                    taskRepo,
		parentStatusUpdater:         services.NewParentStatusUpdater(taskRepo),
		successTransitionNormalizer: task2core.NewSuccessTransitionNormalizer(tplEngine),
		errorTransitionNormalizer:   task2core.NewErrorTransitionNormalizer(tplEngine),
		outputTransformer:           task2core.NewOutputTransformer(tplEngine),
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
	// Build task configs map for context
	taskConfigs := task2.BuildTaskConfigsMap(input.WorkflowConfig.Tasks)

	// Create normalization context with proper Variables
	contextBuilder, err := shared.NewContextBuilderWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to create context builder: %w", err)
	}
	normCtx := contextBuilder.BuildContext(workflowState, input.WorkflowConfig, input.TaskConfig)
	normCtx.TaskConfigs = taskConfigs
	normCtx.CurrentInput = input.TaskConfig.With
	normCtx.MergedEnv = input.TaskConfig.Env

	// For collection child tasks, we need to add the item context
	// Check if this task has a parent that is a collection
	if input.TaskState.ParentStateID != nil {
		parentState, err := uc.taskRepo.GetState(ctx, *input.TaskState.ParentStateID)
		if err == nil && parentState.ExecutionType == task.ExecutionCollection {
			// Extract item and index from the task's input
			if input.TaskState.Input != nil {
				if item, hasItem := (*input.TaskState.Input)[shared.ItemKey]; hasItem {
					normCtx.Variables[shared.ItemKey] = item
				}
				if index, hasIndex := (*input.TaskState.Input)[shared.IndexKey]; hasIndex {
					normCtx.Variables[shared.IndexKey] = index
				}
			}
		}
	}

	output, err := uc.outputTransformer.TransformOutput(
		input.TaskState.Output,
		input.TaskConfig.GetOutputs(),
		normCtx,
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
	workflowState, err := uc.workflowRepo.GetState(ctx, input.TaskState.WorkflowExecID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get workflow state: %w", err)
	}

	// Create normalization context for task2 with proper Variables
	contextBuilder, err := shared.NewContextBuilder()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	normCtx := contextBuilder.BuildContext(workflowState, input.WorkflowConfig, input.TaskConfig)
	normCtx.CurrentInput = input.TaskState.Input

	// Normalize success transition
	var normalizedOnSuccess *core.SuccessTransition
	if input.TaskConfig.OnSuccess != nil {
		// Create a copy to avoid mutating the original
		successCopy := &core.SuccessTransition{
			Next: input.TaskConfig.OnSuccess.Next,
			With: input.TaskConfig.OnSuccess.With,
		}
		if input.TaskConfig.OnSuccess.With != nil {
			withCopy := make(core.Input)
			maps.Copy(withCopy, *input.TaskConfig.OnSuccess.With)
			successCopy.With = &withCopy
		}

		err = uc.successTransitionNormalizer.Normalize(successCopy, normCtx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to normalize success transition: %w", err)
		}
		normalizedOnSuccess = successCopy
	}

	// Normalize error transition
	var normalizedOnError *core.ErrorTransition
	if input.TaskConfig.OnError != nil {
		// Create a copy to avoid mutating the original
		errorCopy := &core.ErrorTransition{
			Next: input.TaskConfig.OnError.Next,
			With: input.TaskConfig.OnError.With,
		}
		if input.TaskConfig.OnError.With != nil {
			withCopy := make(core.Input)
			maps.Copy(withCopy, *input.TaskConfig.OnError.With)
			errorCopy.With = &withCopy
		}

		err = uc.errorTransitionNormalizer.Normalize(errorCopy, normCtx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to normalize error transition: %w", err)
		}
		normalizedOnError = errorCopy
	}

	return normalizedOnSuccess, normalizedOnError, nil
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
		log.Debug("Failed to update parent status", "error", err)
	}
}
