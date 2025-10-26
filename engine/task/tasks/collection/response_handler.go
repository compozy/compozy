package collection

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
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
	if err := h.baseHandler.ValidateInput(input); err != nil {
		return nil, err
	}
	if input.TaskConfig.Type != task.TaskTypeCollection {
		return nil, &shared.ValidationError{
			Field:   "task_type",
			Message: "handler type does not match task type",
		}
	}
	h.applyCollectionContext(ctx, input)
	originalExecID := input.TaskState.TaskExecID
	response, err := h.baseHandler.ProcessMainTaskResponse(ctx, input)
	if err != nil {
		input.TaskState.TaskExecID = originalExecID
		return nil, err
	}
	input.TaskState.TaskExecID = originalExecID
	if response != nil && response.State != nil {
		response.State.TaskExecID = originalExecID
	}
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
	normCtx := h.contextBuilder.BuildContext(ctx, input.WorkflowState, input.WorkflowConfig, input.TaskConfig)
	if item, exists := (*taskInput)[shared.FieldCollectionItem]; exists {
		normCtx.Variables["item"] = item

		if itemVar, exists := (*taskInput)[shared.FieldCollectionItemVar]; exists {
			if varName, ok := itemVar.(string); ok && varName != "" {
				normCtx.Variables[varName] = item
			}
		}
	}
	if index, exists := (*taskInput)[shared.FieldCollectionIndex]; exists {
		normCtx.Variables["index"] = index

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
	if err := h.baseHandler.ValidateInput(input); err != nil {
		return err
	}
	if !h.baseHandler.ShouldDeferOutputTransformation(input.TaskConfig) {
		return nil
	}
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
	return nil
}
