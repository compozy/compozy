package uc

import (
	"context"
	"errors"
	"fmt"

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
	normCtx := contextBuilder.BuildContext(ctx, workflowState, input.WorkflowConfig, input.TaskConfig)
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
		ctx,
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
	normCtx, err := uc.buildTransitionContext(ctx, input)
	if err != nil {
		return nil, nil, err
	}

	successTransition, err := uc.normalizeSuccessTransition(input.TaskConfig.OnSuccess, normCtx)
	if err != nil {
		return nil, nil, err
	}

	errorTransition, err := uc.normalizeErrorTransition(input.TaskConfig.OnError, normCtx)
	if err != nil {
		return nil, nil, err
	}

	return successTransition, errorTransition, nil
}

// buildTransitionContext prepares the normalization context used for transition processing.
func (uc *HandleResponse) buildTransitionContext(
	ctx context.Context,
	input *HandleResponseInput,
) (*shared.NormalizationContext, error) {
	workflowState, err := uc.workflowRepo.GetState(ctx, input.TaskState.WorkflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow state: %w", err)
	}

	contextBuilder, err := shared.NewContextBuilderWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}

	normCtx := contextBuilder.BuildContext(ctx, workflowState, input.WorkflowConfig, input.TaskConfig)
	normCtx.CurrentInput = input.TaskState.Input
	return normCtx, nil
}

// normalizeSuccessTransition produces a normalized copy of the success transition when present.
func (uc *HandleResponse) normalizeSuccessTransition(
	transition *core.SuccessTransition,
	normCtx *shared.NormalizationContext,
) (*core.SuccessTransition, error) {
	if transition == nil {
		return nil, nil
	}

	transitionCopy := cloneSuccessTransition(transition)
	if err := uc.successTransitionNormalizer.Normalize(transitionCopy, normCtx); err != nil {
		return nil, fmt.Errorf("failed to normalize success transition: %w", err)
	}

	return transitionCopy, nil
}

// normalizeErrorTransition produces a normalized copy of the error transition when present.
func (uc *HandleResponse) normalizeErrorTransition(
	transition *core.ErrorTransition,
	normCtx *shared.NormalizationContext,
) (*core.ErrorTransition, error) {
	if transition == nil {
		return nil, nil
	}

	transitionCopy := cloneErrorTransition(transition)
	if err := uc.errorTransitionNormalizer.Normalize(transitionCopy, normCtx); err != nil {
		return nil, fmt.Errorf("failed to normalize error transition: %w", err)
	}

	return transitionCopy, nil
}

// cloneSuccessTransition creates a deep copy of a success transition definition.
func cloneSuccessTransition(transition *core.SuccessTransition) *core.SuccessTransition {
	copyTransition := &core.SuccessTransition{
		Next: transition.Next,
		With: transition.With,
	}
	if transition.With != nil {
		cloned := core.CloneMap(*transition.With)
		withCopy := core.Input(cloned)
		copyTransition.With = &withCopy
	}
	return copyTransition
}

// cloneErrorTransition creates a deep copy of an error transition definition.
func cloneErrorTransition(transition *core.ErrorTransition) *core.ErrorTransition {
	copyTransition := &core.ErrorTransition{
		Next: transition.Next,
		With: transition.With,
	}
	if transition.With != nil {
		cloned := core.CloneMap(*transition.With)
		withCopy := core.Input(cloned)
		copyTransition.With = &withCopy
	}
	return copyTransition
}

// updateParentStatusIfNeeded updates the parent task status when a child task completes
func (uc *HandleResponse) updateParentStatusIfNeeded(ctx context.Context, childState *task.State) error {
	if childState.ParentStateID == nil {
		return nil
	}

	parentState, err := uc.loadParallelParentState(ctx, *childState.ParentStateID)
	if err != nil || parentState == nil {
		return err
	}

	strategy := uc.extractParallelStrategy(ctx, parentState)
	_, err = uc.parentStatusUpdater.UpdateParentStatus(ctx, &services.UpdateParentStatusInput{
		ParentStateID: *childState.ParentStateID,
		Strategy:      strategy,
		Recursive:     true,
		ChildState:    childState,
	})
	return err
}

// loadParallelParentState retrieves the parent task state when parallel execution applies.
func (uc *HandleResponse) loadParallelParentState(
	ctx context.Context,
	parentStateID core.ID,
) (*task.State, error) {
	parentState, err := uc.taskRepo.GetState(ctx, parentStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent state %s: %w", parentStateID, err)
	}
	if parentState.ExecutionType != task.ExecutionParallel {
		return nil, nil
	}
	return parentState, nil
}

// extractParallelStrategy derives the parallel execution strategy from the parent state metadata.
func (uc *HandleResponse) extractParallelStrategy(ctx context.Context, parentState *task.State) task.ParallelStrategy {
	log := logger.FromContext(ctx)
	if parentState.Input == nil {
		return task.StrategyWaitAll
	}
	parallelConfig, ok := uc.extractParallelConfig(log, parentState)
	if !ok {
		return task.StrategyWaitAll
	}
	return uc.strategyFromConfigValue(log, parentState, parallelConfig["strategy"])
}

// extractParallelConfig loads the parallel config map and logs fallbacks when invalid.
func (uc *HandleResponse) extractParallelConfig(log logger.Logger, parentState *task.State) (map[string]any, bool) {
	value, exists := (*parentState.Input)["_parallel_config"]
	if !exists {
		log.Debug("No strategy field found in parallel config, using default wait_all",
			"parent_state_id", parentState.TaskExecID,
		)
		return nil, false
	}
	parallelConfig, ok := value.(map[string]any)
	if !ok {
		log.Error("Parallel config field is not a map, using default wait_all",
			"config_type", fmt.Sprintf("%T", value),
			"parent_state_id", parentState.TaskExecID,
		)
		return nil, false
	}
	return parallelConfig, true
}

// strategyFromConfigValue validates the strategy entry and returns a safe default when needed.
func (uc *HandleResponse) strategyFromConfigValue(
	log logger.Logger,
	parentState *task.State,
	raw any,
) task.ParallelStrategy {
	strategy := task.StrategyWaitAll
	if raw == nil {
		log.Debug("No strategy field found in parallel config, using default wait_all",
			"parent_state_id", parentState.TaskExecID,
		)
		return strategy
	}
	strategyStr, ok := raw.(string)
	if !ok {
		log.Error("Parallel strategy field is not a string, using default wait_all",
			"strategy_type", fmt.Sprintf("%T", raw),
			"strategy_value", raw,
			"parent_state_id", parentState.TaskExecID,
		)
		return strategy
	}
	if !task.ValidateStrategy(strategyStr) {
		log.Error("Invalid parallel strategy found, using default wait_all",
			"invalid_strategy", strategyStr,
			"parent_state_id", parentState.TaskExecID,
		)
		return strategy
	}
	return task.ParallelStrategy(strategyStr)
}

// logParentStatusUpdateError updates parent status and logs any errors without propagating them
// Parent status updates are non-critical operations that should not fail task completion
func (uc *HandleResponse) logParentStatusUpdateError(ctx context.Context, state *task.State) {
	log := logger.FromContext(ctx)
	if err := uc.updateParentStatusIfNeeded(ctx, state); err != nil {
		log.Debug("Failed to update parent status", "error", err)
	}
}
