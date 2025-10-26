package memory

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// ResponseHandler handles response processing for memory tasks
type ResponseHandler struct {
	baseHandler    *shared.BaseResponseHandler
	templateEngine *tplengine.TemplateEngine
	contextBuilder *shared.ContextBuilder
}

// NewResponseHandler creates a new memory task response handler
func NewResponseHandler(
	templateEngine *tplengine.TemplateEngine,
	contextBuilder *shared.ContextBuilder,
	baseHandler *shared.BaseResponseHandler,
) (*ResponseHandler, error) {
	if baseHandler == nil {
		return nil, fmt.Errorf("failed to create memory response handler: baseHandler is required but was nil")
	}
	if templateEngine == nil {
		return nil, fmt.Errorf("failed to create memory response handler: templateEngine is required but was nil")
	}
	if contextBuilder == nil {
		return nil, fmt.Errorf("failed to create memory response handler: contextBuilder is required but was nil")
	}
	return &ResponseHandler{
		baseHandler:    baseHandler,
		templateEngine: templateEngine,
		contextBuilder: contextBuilder,
	}, nil
}

// HandleResponse processes a memory task execution response
func (h *ResponseHandler) HandleResponse(
	ctx context.Context,
	input *shared.ResponseInput,
) (*shared.ResponseOutput, error) {
	if err := h.baseHandler.ValidateInput(input); err != nil {
		return nil, err
	}
	if input.TaskConfig.Type != task.TaskTypeMemory {
		return nil, &shared.ValidationError{
			Field: "task_type",
			Message: fmt.Sprintf(
				"memory response handler received incorrect task type: expected '%s', got '%s'",
				task.TaskTypeMemory,
				input.TaskConfig.Type,
			),
		}
	}
	return h.baseHandler.ProcessMainTaskResponse(ctx, input)
}

// Type returns the task type this handler processes
func (h *ResponseHandler) Type() task.Type {
	return task.TaskTypeMemory
}
