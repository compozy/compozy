package parallel

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// ResponseHandler handles response processing for parallel tasks
type ResponseHandler struct {
	baseHandler    *shared.BaseResponseHandler
	templateEngine *tplengine.TemplateEngine
	contextBuilder *shared.ContextBuilder
}

// NewResponseHandler creates a new parallel task response handler
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

// HandleResponse processes a parallel task execution response
func (h *ResponseHandler) HandleResponse(
	ctx context.Context,
	input *shared.ResponseInput,
) (*shared.ResponseOutput, error) {
	if err := h.baseHandler.ValidateInput(input); err != nil {
		return nil, err
	}
	if input.TaskConfig.Type != task.TaskTypeParallel {
		return nil, &shared.ValidationError{
			Field:   "task_type",
			Message: "handler type does not match task type",
		}
	}
	response, err := h.baseHandler.ProcessMainTaskResponse(ctx, input)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// Type returns the task type this handler processes
func (h *ResponseHandler) Type() task.Type {
	return task.TaskTypeParallel
}

// ApplyDeferredOutputTransformation applies output transformation after children complete
// This method is called by the orchestrator after child tasks have finished per strategy
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
		return fmt.Errorf("parallel deferred transformation failed: %w", err)
	}
	return nil
}

// HandleSubtaskResponse processes parallel subtask responses
// This handles child completion based on parallel strategy
func (h *ResponseHandler) HandleSubtaskResponse(
	_ context.Context,
	_ *task.State,
	childState *task.State,
	childConfig *task.Config,
	_ task.ParallelStrategy,
) (*task.SubtaskResponse, error) {
	return &task.SubtaskResponse{
		TaskID: childConfig.ID,
		Output: childState.Output,
		Error:  childState.Error,
		Status: childState.Status,
		State:  childState,
	}, nil
}

// ExtractParallelStrategy extracts the parallel strategy from parent state
// Deprecated: Use TaskConfigRepository.ExtractParallelStrategy instead
func (h *ResponseHandler) ExtractParallelStrategy(_ *task.State) task.ParallelStrategy {
	panic("ExtractParallelStrategy is deprecated. Use TaskConfigRepository.ExtractParallelStrategy instead")
}
