package shared

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/jackc/pgx/v5"
)

// BaseResponseHandler provides common response handling logic for all task types
type BaseResponseHandler struct {
	templateEngine      *tplengine.TemplateEngine
	contextBuilder      *ContextBuilder
	parentStatusManager ParentStatusManager
	workflowRepo        workflow.Repository
	taskRepo            task.Repository
	outputTransformer   OutputTransformer
}

// OutputTransformer defines the interface for output transformation
type OutputTransformer interface {
	TransformOutput(
		ctx context.Context,
		state *task.State,
		config *task.Config,
		workflowConfig *workflow.Config,
	) (map[string]any, error)
}

// NewBaseResponseHandler creates a new base response handler
func NewBaseResponseHandler(
	templateEngine *tplengine.TemplateEngine,
	contextBuilder *ContextBuilder,
	parentStatusManager ParentStatusManager,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	outputTransformer OutputTransformer,
) *BaseResponseHandler {
	return &BaseResponseHandler{
		templateEngine:      templateEngine,
		contextBuilder:      contextBuilder,
		parentStatusManager: parentStatusManager,
		workflowRepo:        workflowRepo,
		taskRepo:            taskRepo,
		outputTransformer:   outputTransformer,
	}
}

// ProcessMainTaskResponse handles the main task execution response processing
// This method mirrors TaskResponder.HandleMainTask exactly to ensure behavior compatibility
func (h *BaseResponseHandler) ProcessMainTaskResponse(
	ctx context.Context,
	input *ResponseInput,
) (*ResponseOutput, error) {
	// Process task execution result
	isSuccess, executionErr := h.processTaskExecutionResult(ctx, input)

	// Save state and handle context cancellation
	if err := h.saveTaskState(ctx, input.TaskState); err != nil {
		if ctx.Err() != nil {
			return &ResponseOutput{State: input.TaskState}, nil
		}
		return nil, err
	}

	// Update parent status and handle context cancellation
	h.logParentStatusUpdateError(ctx, input.TaskState)
	if ctx.Err() != nil {
		return &ResponseOutput{State: input.TaskState}, nil
	}

	// Process transitions and validate error handling
	onSuccess, onError, err := h.processTransitions(ctx, input, isSuccess, executionErr)
	if err != nil {
		return nil, err
	}

	// Handle context cancellation after transition processing
	if ctx.Err() != nil {
		return &ResponseOutput{State: input.TaskState}, nil
	}

	// Determine next task
	nextTask := h.determineNextTask(input, isSuccess)

	response := &task.MainTaskResponse{
		OnSuccess: onSuccess,
		OnError:   onError,
		State:     input.TaskState,
		NextTask:  nextTask,
	}

	return &ResponseOutput{
		Response: response,
		State:    input.TaskState,
	}, nil
}

// processTaskExecutionResult handles output transformation and determines success status
// Extracted from TaskResponder.processTaskExecutionResult
func (h *BaseResponseHandler) processTaskExecutionResult(
	ctx context.Context,
	input *ResponseInput,
) (bool, error) {
	state := input.TaskState
	executionErr := input.ExecutionError

	// Determine if task is successful so far
	isSuccess := executionErr == nil && state.Status != core.StatusFailed

	// Apply output transformation if needed
	// Skip for collection/parallel tasks as they need children data first
	if isSuccess && !h.shouldDeferOutputTransformation(input.TaskConfig) {
		state.UpdateStatus(core.StatusSuccess)
		if input.TaskConfig.GetOutputs() != nil && state.Output != nil {
			if err := h.applyOutputTransformation(ctx, input); err != nil {
				executionErr = err
				isSuccess = false
			}
		}
	}

	// Handle final state
	if !isSuccess {
		state.UpdateStatus(core.StatusFailed)
		h.setErrorState(state, executionErr)
	}

	return isSuccess, executionErr
}

// shouldDeferOutputTransformation determines if output transformation should be deferred
// Extracted from TaskResponder.shouldDeferOutputTransformation
func (h *BaseResponseHandler) shouldDeferOutputTransformation(taskConfig *task.Config) bool {
	return taskConfig.Type == task.TaskTypeCollection || taskConfig.Type == task.TaskTypeParallel
}

// processTransitions normalizes transitions and validates error handling requirements
// Extracted from TaskResponder.processTransitions
func (h *BaseResponseHandler) processTransitions(
	ctx context.Context,
	input *ResponseInput,
	isSuccess bool,
	executionErr error,
) (*core.SuccessTransition, *core.ErrorTransition, error) {
	// Normalize transitions
	onSuccess, onError, err := h.normalizeTransitions(ctx, input)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, nil // Will be handled by caller
		}
		return nil, nil, fmt.Errorf("failed to normalize transitions: %w", err)
	}

	// Check for error transition requirement
	if !isSuccess && (onError == nil || onError.Next == nil) {
		if executionErr != nil {
			return nil, nil, fmt.Errorf("task failed with no error transition defined: %w", executionErr)
		}
		return nil, nil, errors.New("task failed with no error transition defined")
	}

	return onSuccess, onError, nil
}

// determineNextTask selects the next task based on override or workflow configuration
// Extracted from TaskResponder.determineNextTask
func (h *BaseResponseHandler) determineNextTask(input *ResponseInput, isSuccess bool) *task.Config {
	if input.NextTaskOverride != nil {
		return input.NextTaskOverride
	}
	return input.WorkflowConfig.DetermineNextTask(input.TaskConfig, isSuccess)
}

// applyOutputTransformation applies output transformation to the task state
func (h *BaseResponseHandler) applyOutputTransformation(ctx context.Context, input *ResponseInput) error {
	if h.outputTransformer == nil {
		return fmt.Errorf("output transformer not configured for deferred transformation")
	}
	transformedOutput, err := h.outputTransformer.TransformOutput(
		ctx,
		input.TaskState,
		input.TaskConfig,
		input.WorkflowConfig,
	)
	if err != nil {
		return fmt.Errorf("output transformation failed: %w", err)
	}
	output := core.Output(transformedOutput)
	input.TaskState.Output = &output
	return nil
}

// setErrorState sets the error state for a task
// Extracted from TaskResponder.setErrorState
func (h *BaseResponseHandler) setErrorState(state *task.State, executionErr error) {
	if executionErr != nil {
		errorText := executionErr.Error()
		state.Error = &core.Error{Message: errorText}
	} else {
		state.Error = &core.Error{Message: "Task failed without specific error"}
	}
}

// logParentStatusUpdateError logs parent status update errors without failing the main flow
// Extracted from TaskResponder.logParentStatusUpdateError
func (h *BaseResponseHandler) logParentStatusUpdateError(ctx context.Context, state *task.State) {
	if err := h.updateParentStatusIfNeeded(ctx, state); err != nil {
		// Log the error but don't fail the main flow to match TaskResponder behavior
		log := logger.FromContext(ctx).With(
			"task_exec_id", state.TaskExecID,
			"task_id", state.TaskID,
			"error", err,
		)
		if state.ParentStateID != nil {
			log = log.With("parent_state_id", *state.ParentStateID)
		}
		log.Error("Failed to update parent task status")
	}
}

// normalizeTransitions normalizes task transitions for processing
func (h *BaseResponseHandler) normalizeTransitions(
	_ context.Context,
	input *ResponseInput,
) (*core.SuccessTransition, *core.ErrorTransition, error) {
	if input.TaskConfig == nil {
		return nil, nil, fmt.Errorf("task config cannot be nil for transition normalization")
	}

	// Build normalization context
	normCtx := h.contextBuilder.BuildContext(input.WorkflowState, input.WorkflowConfig, input.TaskConfig)
	normCtx.CurrentInput = input.TaskState.Input

	// Build template context for normalization
	templateContext := normCtx.BuildTemplateContext()

	// Normalize success transition
	var normalizedSuccess *core.SuccessTransition
	if input.TaskConfig.OnSuccess != nil {
		normalizedSuccess = &core.SuccessTransition{}
		*normalizedSuccess = *input.TaskConfig.OnSuccess

		// Set current input if not already set
		if normCtx.CurrentInput == nil && normalizedSuccess.With != nil {
			normCtx.CurrentInput = normalizedSuccess.With
		}

		// Convert to map for template processing
		configMap, err := normalizedSuccess.AsMap()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to convert success transition to map: %w", err)
		}

		// Apply template processing
		parsed, err := h.templateEngine.ParseAny(configMap, templateContext)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to normalize success transition: %w", err)
		}

		// Update from normalized map
		if err := normalizedSuccess.FromMap(parsed); err != nil {
			return nil, nil, fmt.Errorf("failed to update success transition from normalized map: %w", err)
		}
	}

	// Normalize error transition
	var normalizedError *core.ErrorTransition
	if input.TaskConfig.OnError != nil {
		normalizedError = &core.ErrorTransition{}
		*normalizedError = *input.TaskConfig.OnError

		// Set current input if not already set
		if normCtx.CurrentInput == nil && normalizedError.With != nil {
			normCtx.CurrentInput = normalizedError.With
		}

		// Convert to map for template processing
		configMap, err := normalizedError.AsMap()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to convert error transition to map: %w", err)
		}

		// Apply template processing
		parsed, err := h.templateEngine.ParseAny(configMap, templateContext)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to normalize error transition: %w", err)
		}

		// Update from normalized map
		if err := normalizedError.FromMap(parsed); err != nil {
			return nil, nil, fmt.Errorf("failed to update error transition from normalized map: %w", err)
		}
	}

	return normalizedSuccess, normalizedError, nil
}

// updateParentStatusIfNeeded updates parent task status if this is a child task
// Extracted from TaskResponder.updateParentStatusIfNeeded logic
func (h *BaseResponseHandler) updateParentStatusIfNeeded(ctx context.Context, childState *task.State) error {
	// Only proceed if this is a child task (has a parent)
	if childState.ParentStateID == nil {
		return nil
	}

	if h.parentStatusManager == nil {
		return nil // No parent status manager configured
	}

	// Get parent state to extract strategy
	parentState, err := h.taskRepo.GetState(ctx, *childState.ParentStateID)
	if err != nil {
		return fmt.Errorf("failed to get parent state: %w", err)
	}

	// Extract strategy from parent state
	strategy := h.extractParentStrategy(parentState)

	// Use the parent status manager to update status
	return h.parentStatusManager.UpdateParentStatus(ctx, *childState.ParentStateID, strategy)
}

// extractParentStrategy extracts the parallel strategy from parent state
func (h *BaseResponseHandler) extractParentStrategy(parentState *task.State) task.ParallelStrategy {
	// Default strategy if not specified
	defaultStrategy := task.StrategyWaitAll

	if parentState.Input == nil {
		return defaultStrategy
	}

	// Check for strategy in input
	if strategyVal, exists := (*parentState.Input)["strategy"]; exists {
		if strategyStr, ok := strategyVal.(string); ok {
			switch task.ParallelStrategy(strategyStr) {
			case task.StrategyWaitAll:
				return task.StrategyWaitAll
			case task.StrategyFailFast:
				return task.StrategyFailFast
			case task.StrategyBestEffort:
				return task.StrategyBestEffort
			case task.StrategyRace:
				return task.StrategyRace
			}
		}
	}

	return defaultStrategy
}

// ShouldDeferOutputTransformation determines if output transformation should be deferred
// Public interface for external callers needing to check deferral logic
func (h *BaseResponseHandler) ShouldDeferOutputTransformation(config *task.Config) bool {
	return config.Type == task.TaskTypeCollection || config.Type == task.TaskTypeParallel
}

// CreateDeferredOutputConfig creates configuration for deferred output processing
func (h *BaseResponseHandler) CreateDeferredOutputConfig(taskType task.Type, reason string) *DeferredOutputConfig {
	shouldDefer := taskType == task.TaskTypeCollection || taskType == task.TaskTypeParallel
	return &DeferredOutputConfig{
		ShouldDefer: shouldDefer,
		Reason:      reason,
	}
}

// ValidateInput validates the response input structure
func (h *BaseResponseHandler) ValidateInput(input *ResponseInput) error {
	if input == nil {
		return &ValidationError{Field: "input", Message: "input cannot be nil"}
	}
	if input.TaskConfig == nil {
		return &ValidationError{Field: "task_config", Message: "task config cannot be nil"}
	}
	if input.TaskState == nil {
		return &ValidationError{Field: "task_state", Message: "task state cannot be nil"}
	}
	if input.WorkflowConfig == nil {
		return &ValidationError{Field: "workflow_config", Message: "workflow config cannot be nil"}
	}
	if input.WorkflowState == nil {
		return &ValidationError{Field: "workflow_state", Message: "workflow state cannot be nil"}
	}
	return nil
}

// CreateResponseContext creates response context for the given input
func (h *BaseResponseHandler) CreateResponseContext(input *ResponseInput) *ResponseContext {
	context := &ResponseContext{
		IsParentTask: input.TaskState.ParentStateID != nil,
	}

	if input.TaskState.ParentStateID != nil {
		context.ParentTaskID = input.TaskState.ParentStateID.String()
	}

	context.DeferredConfig = h.CreateDeferredOutputConfig(
		input.TaskConfig.Type,
		fmt.Sprintf("Output transformation deferred for %s tasks", input.TaskConfig.Type),
	)

	return context
}

// ProcessTransitions handles task transitions after completion
// Base implementation provides no-op behavior - override in specific task handlers
func (h *BaseResponseHandler) ProcessTransitions(_ context.Context, _ *ResponseInput) error {
	// Base implementation does nothing - specific task handlers will override
	// this method to implement their transition logic
	return nil
}

// saveTaskState saves the task state with transaction safety when available
// Extracted from TaskResponder.saveTaskState
func (h *BaseResponseHandler) saveTaskState(ctx context.Context, state *task.State) error {
	// Check if the task repository supports transactions for row-level locking
	type txRepo interface {
		WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error
		GetStateForUpdate(ctx context.Context, tx pgx.Tx, taskExecID core.ID) (*task.State, error)
		UpsertStateWithTx(ctx context.Context, tx pgx.Tx, state *task.State) error
	}

	if txTaskRepo, ok := h.taskRepo.(txRepo); ok {
		return txTaskRepo.WithTx(ctx, func(tx pgx.Tx) error {
			// Get latest state with row-level lock to prevent concurrent modifications
			latestState, err := txTaskRepo.GetStateForUpdate(ctx, tx, state.TaskExecID)
			if err != nil {
				return fmt.Errorf("failed to lock state for update: %w", err)
			}

			// Apply changes to latest state to prevent overwrites
			latestState.Status = state.Status
			latestState.Output = state.Output
			latestState.Error = state.Error

			// Save with transaction safety
			if err := txTaskRepo.UpsertStateWithTx(ctx, tx, latestState); err != nil {
				return fmt.Errorf("failed to update task state: %w", err)
			}
			return nil
		})
	}

	// Fallback to regular save if transactions are not supported
	if err := h.taskRepo.UpsertState(ctx, state); err != nil {
		return fmt.Errorf("failed to update task state: %w", err)
	}
	return nil
}

// ApplyDeferredOutputTransformation applies output transformation for parent tasks after children are processed
// Extracted from TaskResponder.applyDeferredOutputTransformation with transaction safety
func (h *BaseResponseHandler) ApplyDeferredOutputTransformation(
	ctx context.Context,
	input *ResponseInput,
) error {
	// Only apply if no execution error and task is not failed
	if input.ExecutionError != nil || input.TaskState.Status == core.StatusFailed {
		return nil
	}

	// Check if the task repository supports transactions
	type txRepo interface {
		WithTx(ctx context.Context, fn func(pgx.Tx) error) error
		GetStateForUpdate(ctx context.Context, tx pgx.Tx, taskExecID core.ID) (*task.State, error)
		UpsertStateWithTx(ctx context.Context, tx pgx.Tx, state *task.State) error
	}

	if txTaskRepo, ok := h.taskRepo.(txRepo); ok {
		// Use a transaction to prevent race conditions and ensure atomicity
		var transformErr error
		txErr := txTaskRepo.WithTx(ctx, func(tx pgx.Tx) error {
			// Get the latest state with lock to prevent concurrent modifications
			latestState, err := txTaskRepo.GetStateForUpdate(ctx, tx, input.TaskState.TaskExecID)
			if err != nil {
				return fmt.Errorf("failed to get latest state for update: %w", err)
			}

			// Use the fresh state for the transformation
			// Create a new input for the transformation function to avoid side effects
			transformInput := &ResponseInput{
				TaskConfig:     input.TaskConfig,
				TaskState:      latestState,
				WorkflowConfig: input.WorkflowConfig,
				WorkflowState:  input.WorkflowState,
			}

			// Ensure the task is marked as successful for the transformation context
			transformInput.TaskState.UpdateStatus(core.StatusSuccess)

			// Apply output transformation if needed
			if transformInput.TaskConfig.GetOutputs() != nil && transformInput.TaskState.Output != nil {
				transformErr = h.applyOutputTransformation(ctx, transformInput)
			}

			if transformErr != nil {
				// On transformation failure, mark task as failed and set error
				transformInput.TaskState.UpdateStatus(core.StatusFailed)
				h.setErrorState(transformInput.TaskState, transformErr)
			}

			// Save the updated state (either with transformed output or with failure status)
			if err := txTaskRepo.UpsertStateWithTx(ctx, tx, transformInput.TaskState); err != nil {
				if transformErr != nil {
					return fmt.Errorf(
						"failed to save state after transformation error: %w (original error: %w)",
						err,
						transformErr,
					)
				}
				return fmt.Errorf("failed to save transformed state: %w", err)
			}

			// Always commit the transaction - the state has been properly saved
			return nil
		})

		// If transaction failed, return the transaction error
		if txErr != nil {
			return txErr
		}

		// If transformation failed, return the transformation error after successful state save
		return transformErr
	}

	// Fallback to direct transformation if transactions are not supported
	if input.TaskConfig.GetOutputs() != nil && input.TaskState.Output != nil {
		if err := h.applyOutputTransformation(ctx, input); err != nil {
			// On transformation failure, mark task as failed and set error
			input.TaskState.UpdateStatus(core.StatusFailed)
			h.setErrorState(input.TaskState, err)
			// Save the failed state
			if saveErr := h.taskRepo.UpsertState(ctx, input.TaskState); saveErr != nil {
				return fmt.Errorf(
					"failed to save state after transformation error: %w (original error: %w)",
					saveErr,
					err,
				)
			}
			return err
		}
	}
	return nil
}
