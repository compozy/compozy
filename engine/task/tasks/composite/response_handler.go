package composite

import (
	"context"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// ResponseHandler handles response processing for composite tasks
type ResponseHandler struct {
	baseHandler    *shared.BaseResponseHandler
	templateEngine *tplengine.TemplateEngine
	contextBuilder *shared.ContextBuilder
}

// NewResponseHandler creates a new composite task response handler
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

// HandleResponse processes a composite task execution response
func (h *ResponseHandler) HandleResponse(
	ctx context.Context,
	input *shared.ResponseInput,
) (*shared.ResponseOutput, error) {
	if err := h.baseHandler.ValidateInput(input); err != nil {
		return nil, err
	}
	if input.TaskConfig.Type != task.TaskTypeComposite {
		return nil, &shared.ValidationError{
			Field:   "task_type",
			Message: "handler type does not match task type",
		}
	}
	return h.baseHandler.ProcessMainTaskResponse(ctx, input)
}

// Type returns the task type this handler processes
func (h *ResponseHandler) Type() task.Type {
	return task.TaskTypeComposite
}

// HandleSubtaskResponse processes composite subtask responses
// For composite tasks, each child executes sequentially
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
