package collection

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// ResponseHandler handles response processing for collection tasks
type ResponseHandler struct {
	baseHandler    *shared.BaseResponseHandler
	templateEngine *tplengine.TemplateEngine
	contextBuilder *shared.ContextBuilder
}

// NewResponseHandler creates a new collection task response handler
func NewResponseHandler(
	templateEngine *tplengine.TemplateEngine,
	contextBuilder *shared.ContextBuilder,
	baseHandler *shared.BaseResponseHandler,
) *ResponseHandler {
	return &ResponseHandler{
		baseHandler:    baseHandler,
		templateEngine: templateEngine,
		contextBuilder: contextBuilder,
	}
}

// HandleResponse processes a collection task execution response
func (h *ResponseHandler) HandleResponse(
	ctx context.Context,
	input *shared.ResponseInput,
) (*shared.ResponseOutput, error) {
	// Validate input
	if err := h.baseHandler.ValidateInput(input); err != nil {
		return nil, err
	}
	// Validate task type matches handler
	if input.TaskConfig.Type != task.TaskTypeCollection {
		return nil, &shared.ValidationError{
			Field:   "task_type",
			Message: "handler type does not match task type",
		}
	}
	// Apply collection context variables before processing
	h.applyCollectionContext(ctx, input)
	// Capture after validation to avoid nil deref
	originalExecID := input.TaskState.TaskExecID
	// Delegate to base handler for common logic
	response, err := h.baseHandler.ProcessMainTaskResponse(ctx, input)
	if err != nil {
		// Ensure the original ID is still restored even on error
		input.TaskState.TaskExecID = originalExecID
		return nil, err
	}
	// Restore the original TaskExecID on both the input state and the response state
	input.TaskState.TaskExecID = originalExecID
	if response != nil && response.State != nil {
		response.State.TaskExecID = originalExecID
	}
	// Collection tasks use deferred output transformation
	// The transformation happens after all child tasks complete
	// This is handled by the orchestrator calling ApplyDeferredOutputTransformation
	// We don't need to do it here as it's done after children processing
	return response, nil
}

// Type returns the task type this handler processes
func (h *ResponseHandler) Type() task.Type {
	return task.TaskTypeCollection
}

// applyCollectionContext applies collection-specific context variables to the normalization context
func (h *ResponseHandler) applyCollectionContext(ctx context.Context, input *shared.ResponseInput) {
	taskInput := input.TaskConfig.With
	if taskInput == nil {
		return
	}
	// Build normalization context for variable access
	normCtx := h.contextBuilder.BuildContext(ctx, input.WorkflowState, input.WorkflowConfig, input.TaskConfig)
	// Apply standard item variable
	if item, exists := (*taskInput)[shared.FieldCollectionItem]; exists {
		normCtx.Variables["item"] = item

		// Apply custom item variable name if specified
		if itemVar, exists := (*taskInput)[shared.FieldCollectionItemVar]; exists {
			if varName, ok := itemVar.(string); ok && varName != "" {
				normCtx.Variables[varName] = item
			}
		}
	}
	// Apply standard index variable
	if index, exists := (*taskInput)[shared.FieldCollectionIndex]; exists {
		normCtx.Variables["index"] = index

		// Apply custom index variable name if specified
		if indexVar, exists := (*taskInput)[shared.FieldCollectionIndexVar]; exists {
			if varName, ok := indexVar.(string); ok && varName != "" {
				normCtx.Variables[varName] = index
			}
		}
	}
}

// ApplyDeferredOutputTransformation applies output transformation after children complete
// This method is called by the orchestrator after all child tasks have finished
func (h *ResponseHandler) ApplyDeferredOutputTransformation(
	ctx context.Context,
	input *shared.ResponseInput,
) error {
	// Validate input
	if err := h.baseHandler.ValidateInput(input); err != nil {
		return err
	}
	// Ensure we should defer transformation for this task type
	if !h.baseHandler.ShouldDeferOutputTransformation(input.TaskConfig) {
		return nil
	}
	// Apply the deferred transformation using the base handler
	if err := h.baseHandler.ApplyDeferredOutputTransformation(ctx, input); err != nil {
		return fmt.Errorf("collection deferred transformation failed: %w", err)
	}
	return nil
}

// HandleSubtaskResponse processes collection subtask responses
// This extracts logic from TaskResponder.HandleSubtask specific to collections
func (h *ResponseHandler) HandleSubtaskResponse(
	_ context.Context,
	_ *task.State,
	childState *task.State,
	childConfig *task.Config,
) (*task.SubtaskResponse, error) {
	// For collection tasks, we aggregate child outputs into parent output
	// This logic will be implemented when we extract subtask handling

	// Basic response for now
	return &task.SubtaskResponse{
		TaskID: childConfig.ID,
		Output: childState.Output,
		Error:  childState.Error,
		Status: childState.Status,
		State:  childState,
	}, nil
}

// ValidateCollectionOutput validates the aggregated collection output
func (h *ResponseHandler) ValidateCollectionOutput(output *core.Output) error {
	if output == nil {
		return nil
	}
	// No validation is currently needed for collection outputs because:
	// 1. Collection outputs are dynamically structured based on the collection configuration
	// 2. The output structure is determined by the user-defined transformation in the workflow
	// 3. Validation would overly constrain the flexibility of collection transformations
	// The orchestrator and output transformer handle the actual aggregation and structuring
	return nil
}
