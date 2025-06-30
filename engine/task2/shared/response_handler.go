package shared

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

// BaseResponseHandler provides common response handling logic for all task types
type BaseResponseHandler struct {
	templateEngine      *tplengine.TemplateEngine
	contextBuilder      *ContextBuilder
	parentStatusManager ParentStatusManager
	workflowRepo        workflow.Repository
	taskRepo            task.Repository
}

// NewBaseResponseHandler creates a new base response handler
func NewBaseResponseHandler(
	templateEngine *tplengine.TemplateEngine,
	contextBuilder *ContextBuilder,
	parentStatusManager ParentStatusManager,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *BaseResponseHandler {
	return &BaseResponseHandler{
		templateEngine:      templateEngine,
		contextBuilder:      contextBuilder,
		parentStatusManager: parentStatusManager,
		workflowRepo:        workflowRepo,
		taskRepo:            taskRepo,
	}
}

// ProcessMainTaskResponse handles the main task execution response processing
func (h *BaseResponseHandler) ProcessMainTaskResponse(
	ctx context.Context,
	input *ResponseInput,
) (*ResponseOutput, error) {
	// Validate input first
	if err := h.ValidateInput(input); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return &ResponseOutput{State: input.TaskState}, nil
	}
	state := input.TaskState
	// Process execution result
	if input.ExecutionError != nil {
		state.Status = core.StatusFailed
		errorText := input.ExecutionError.Error()
		state.Error = &core.Error{Message: errorText}
	} else {
		state.Status = core.StatusSuccess
	}
	// Save state with error handling for context cancellation
	if err := h.saveTaskState(ctx, state); err != nil {
		if ctx.Err() != nil {
			return &ResponseOutput{State: state}, nil
		}
		return nil, fmt.Errorf("failed to save task state: %w", err)
	}
	// Update parent status if this is a child task
	if state.ParentStateID != nil {
		if err := h.updateParentStatus(ctx, *state.ParentStateID); err != nil {
			if ctx.Err() != nil {
				return &ResponseOutput{State: state}, nil
			}
			return nil, fmt.Errorf("failed to update parent status: %w", err)
		}
	}
	return &ResponseOutput{
		Response: h.createMainTaskResponse(state),
		State:    state,
	}, nil
}

// ShouldDeferOutputTransformation determines if output transformation should be deferred
func (h *BaseResponseHandler) ShouldDeferOutputTransformation(config *task.Config) bool {
	switch config.Type {
	case task.TaskTypeCollection, task.TaskTypeParallel:
		return true
	default:
		return false
	}
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
func (h *BaseResponseHandler) ProcessTransitions(ctx context.Context, input *ResponseInput) error {
	if input.TaskConfig.OnSuccess != nil && input.TaskState.Status == core.StatusSuccess {
		return h.processSuccessTransition(ctx, input)
	}
	if input.TaskConfig.OnError != nil && input.TaskState.Status == core.StatusFailed {
		return h.processErrorTransition(ctx, input)
	}
	return nil
}

// saveTaskState saves the task state with transaction safety
func (h *BaseResponseHandler) saveTaskState(ctx context.Context, state *task.State) error {
	return h.taskRepo.UpsertState(ctx, state)
}

// updateParentStatus updates the parent task status
func (h *BaseResponseHandler) updateParentStatus(
	ctx context.Context,
	parentStateID core.ID,
) error {
	if h.parentStatusManager == nil {
		return nil
	}
	// TODO: Strategy should be provided by the specific task response handler
	// For now, using default wait_all strategy
	return h.parentStatusManager.UpdateParentStatus(ctx, parentStateID, task.StrategyWaitAll)
}

// createMainTaskResponse creates the main task response object
func (h *BaseResponseHandler) createMainTaskResponse(state *task.State) *task.MainTaskResponse {
	return &task.MainTaskResponse{
		State: state,
		// OnSuccess and OnError will be set by specific handlers if needed
		// NextTask will be set by transition processing if needed
	}
}

// processSuccessTransition handles success transition logic
func (h *BaseResponseHandler) processSuccessTransition(_ context.Context, _ *ResponseInput) error {
	// Transition processing will be implemented by specific task handlers
	// that understand how to create and configure the next task
	return fmt.Errorf("transition processing not implemented - override in specific task handler")
}

// processErrorTransition handles error transition logic
func (h *BaseResponseHandler) processErrorTransition(_ context.Context, _ *ResponseInput) error {
	// Transition processing will be implemented by specific task handlers
	// that understand how to create and configure error handling tasks
	return fmt.Errorf("transition processing not implemented - override in specific task handler")
}
