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
)

// BaseResponseHandler provides common response handling logic for all task types
type BaseResponseHandler struct {
	templateEngine      *tplengine.TemplateEngine
	contextBuilder      *ContextBuilder
	parentStatusManager ParentStatusManager
	workflowRepo        workflow.Repository
	taskRepo            task.Repository
	outputTransformer   OutputTransformer
	transactionService  *TransactionService
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
		transactionService:  NewTransactionService(taskRepo),
	}
}

// ProcessMainTaskResponse handles the main task execution response processing
// This method mirrors TaskResponder.HandleMainTask exactly to ensure behavior compatibility
func (h *BaseResponseHandler) ProcessMainTaskResponse(
	ctx context.Context,
	input *ResponseInput,
) (*ResponseOutput, error) {
	// Process execution and save state
	isSuccess, executionErr, err := h.processAndSaveTaskResult(ctx, input)
	if err != nil {
		return h.handleProcessingError(ctx, input, err)
	}
	// Update parent status if needed
	if err := h.updateParentIfNeeded(ctx, input.TaskState); err != nil {
		return h.handleProcessingError(ctx, input, err)
	}
	// Build final response
	return h.buildTaskResponse(ctx, input, isSuccess, executionErr)
}

// processAndSaveTaskResult processes execution result and saves state
func (h *BaseResponseHandler) processAndSaveTaskResult(
	ctx context.Context,
	input *ResponseInput,
) (bool, error, error) {
	// Process task execution result
	isSuccess, executionErr := h.processTaskExecutionResult(ctx, input)
	// Save state with proper error handling
	if err := h.saveTaskState(ctx, input.TaskState); err != nil {
		return false, nil, err
	}
	return isSuccess, executionErr, nil
}

// updateParentIfNeeded updates parent status with proper error handling
func (h *BaseResponseHandler) updateParentIfNeeded(
	ctx context.Context,
	state *task.State,
) error {
	if err := h.updateParentStatusIfNeeded(ctx, state); err != nil {
		// Log detailed error internally
		log := logger.FromContext(ctx).With(
			"task_exec_id", state.TaskExecID,
			"parent_state_id", state.ParentStateID,
		)
		log.Error("Failed to update parent task status", "error", err)
		// Return generic error to prevent information disclosure
		return errors.New("parent task update failed")
	}
	return nil
}

// buildTaskResponse builds the final response with transitions
func (h *BaseResponseHandler) buildTaskResponse(
	ctx context.Context,
	input *ResponseInput,
	isSuccess bool,
	executionErr error,
) (*ResponseOutput, error) {
	// Process transitions
	onSuccess, onError, err := h.processTransitions(ctx, input, isSuccess, executionErr)
	if err != nil {
		return nil, err
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

// handleProcessingError handles errors with context cancellation check
func (h *BaseResponseHandler) handleProcessingError(
	ctx context.Context,
	input *ResponseInput,
	err error,
) (*ResponseOutput, error) {
	if ctx.Err() != nil {
		return &ResponseOutput{State: input.TaskState}, nil
	}
	return nil, err
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

// normalizeTransitions normalizes task transitions for processing
func (h *BaseResponseHandler) normalizeTransitions(
	_ context.Context,
	input *ResponseInput,
) (*core.SuccessTransition, *core.ErrorTransition, error) {
	if input.TaskConfig == nil {
		return nil, nil, fmt.Errorf("task config cannot be nil for transition normalization")
	}
	// Build contexts
	normCtx, templateContext := h.buildNormalizationContexts(input)
	// Normalize both transitions
	normalizedSuccess, err := h.normalizeSuccessTransition(input.TaskConfig.OnSuccess, normCtx, templateContext)
	if err != nil {
		return nil, nil, err
	}
	normalizedError, err := h.normalizeErrorTransition(input.TaskConfig.OnError, normCtx, templateContext)
	if err != nil {
		return nil, nil, err
	}
	return normalizedSuccess, normalizedError, nil
}

// buildNormalizationContexts builds contexts for transition normalization
func (h *BaseResponseHandler) buildNormalizationContexts(
	input *ResponseInput,
) (*NormalizationContext, map[string]any) {
	normCtx := h.contextBuilder.BuildContext(input.WorkflowState, input.WorkflowConfig, input.TaskConfig)
	normCtx.CurrentInput = input.TaskState.Input
	templateContext := normCtx.BuildTemplateContext()
	return normCtx, templateContext
}

// normalizeSuccessTransition normalizes success transition
func (h *BaseResponseHandler) normalizeSuccessTransition(
	transition *core.SuccessTransition,
	normCtx *NormalizationContext,
	templateContext map[string]any,
) (*core.SuccessTransition, error) {
	if transition == nil {
		return nil, nil
	}
	normalized := &core.SuccessTransition{}
	*normalized = *transition
	// Set current input if not already set
	if normCtx.CurrentInput == nil && normalized.With != nil {
		normCtx.CurrentInput = normalized.With
	}
	// Apply template processing
	if err := h.applyTransitionTemplates(normalized, templateContext, "success"); err != nil {
		return nil, err
	}
	return normalized, nil
}

// normalizeErrorTransition normalizes error transition
func (h *BaseResponseHandler) normalizeErrorTransition(
	transition *core.ErrorTransition,
	normCtx *NormalizationContext,
	templateContext map[string]any,
) (*core.ErrorTransition, error) {
	if transition == nil {
		return nil, nil
	}
	normalized := &core.ErrorTransition{}
	*normalized = *transition
	// Set current input if not already set
	if normCtx.CurrentInput == nil && normalized.With != nil {
		normCtx.CurrentInput = normalized.With
	}
	// Apply template processing
	if err := h.applyTransitionTemplates(normalized, templateContext, "error"); err != nil {
		return nil, err
	}
	return normalized, nil
}

// applyTransitionTemplates applies template processing to transitions
func (h *BaseResponseHandler) applyTransitionTemplates(
	transition any,
	templateContext map[string]any,
	transitionType string,
) error {
	// Convert to map for template processing
	var configMap map[string]any
	var err error
	switch t := transition.(type) {
	case *core.SuccessTransition:
		configMap, err = t.AsMap()
	case *core.ErrorTransition:
		configMap, err = t.AsMap()
	default:
		return fmt.Errorf("unsupported transition type: %T", transition)
	}
	if err != nil {
		return fmt.Errorf("failed to convert %s transition to map: %w", transitionType, err)
	}
	// Apply template processing
	parsed, err := h.templateEngine.ParseAny(configMap, templateContext)
	if err != nil {
		return fmt.Errorf("failed to normalize %s transition: %w", transitionType, err)
	}
	// Update from normalized map
	switch t := transition.(type) {
	case *core.SuccessTransition:
		err = t.FromMap(parsed)
	case *core.ErrorTransition:
		err = t.FromMap(parsed)
	}
	if err != nil {
		return fmt.Errorf("failed to update %s transition from normalized map: %w", transitionType, err)
	}
	return nil
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
	if strategyVal, exists := (*parentState.Input)[FieldStrategy]; exists {
		if strategyStr, ok := strategyVal.(string); ok {
			return h.parseStrategy(strategyStr)
		}
	}
	// Check for parallel config in input
	if parallelConfig, exists := (*parentState.Input)[FieldParallelConfig]; exists {
		switch v := parallelConfig.(type) {
		case map[string]any:
			if strategy, ok := v[FieldStrategy].(string); ok {
				return h.parseStrategy(strategy)
			}
		case string:
			return h.parseStrategy(v)
		}
	}
	return defaultStrategy
}

// parseStrategy converts string to ParallelStrategy type
func (h *BaseResponseHandler) parseStrategy(strategy string) task.ParallelStrategy {
	switch task.ParallelStrategy(strategy) {
	case task.StrategyWaitAll:
		return task.StrategyWaitAll
	case task.StrategyFailFast:
		return task.StrategyFailFast
	case task.StrategyBestEffort:
		return task.StrategyBestEffort
	case task.StrategyRace:
		return task.StrategyRace
	default:
		return task.StrategyWaitAll
	}
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
	return h.transactionService.SaveStateWithLocking(ctx, state)
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
	// Apply transformation with transaction support
	transformer := h.createOutputTransformationFunction(ctx, input)
	if err := h.transactionService.ApplyTransformation(ctx, input.TaskState.TaskExecID, transformer); err != nil {
		return err
	}
	// Verify transformation persistence and update in-memory state
	return h.verifyTransformationPersistence(ctx, input)
}

// createOutputTransformationFunction creates the transformation logic for deferred output processing
func (h *BaseResponseHandler) createOutputTransformationFunction(
	ctx context.Context,
	input *ResponseInput,
) func(*task.State) error {
	return func(state *task.State) error {
		transformInput := &ResponseInput{
			TaskConfig:     input.TaskConfig,
			TaskState:      state,
			WorkflowConfig: input.WorkflowConfig,
			WorkflowState:  input.WorkflowState,
		}
		transformInput.TaskState.UpdateStatus(core.StatusSuccess)
		return h.applyOutputTransformationIfNeeded(ctx, transformInput)
	}
}

// applyOutputTransformationIfNeeded applies output transformation if the task has outputs configured
func (h *BaseResponseHandler) applyOutputTransformationIfNeeded(
	ctx context.Context,
	transformInput *ResponseInput,
) error {
	if transformInput.TaskConfig.GetOutputs() == nil || transformInput.TaskState.Output == nil {
		return nil
	}
	if err := h.applyOutputTransformation(ctx, transformInput); err != nil {
		transformInput.TaskState.UpdateStatus(core.StatusFailed)
		h.setErrorState(transformInput.TaskState, err)
		return err
	}
	return nil
}

// verifyTransformationPersistence ensures the transformation was persisted and updates in-memory state
func (h *BaseResponseHandler) verifyTransformationPersistence(
	ctx context.Context,
	input *ResponseInput,
) error {
	verifiedState, err := h.taskRepo.GetState(ctx, input.TaskState.TaskExecID)
	if err != nil {
		log := logger.FromContext(ctx).With(
			"task_exec_id", input.TaskState.TaskExecID,
			"task_id", input.TaskConfig.ID,
		)
		log.Warn("Failed to verify transformation persistence", "error", err)
		return nil
	}
	input.TaskState.Status = verifiedState.Status
	input.TaskState.Output = verifiedState.Output
	input.TaskState.Error = verifiedState.Error
	return nil
}
