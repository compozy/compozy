package basic

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// ResponseHandler handles response processing for basic tasks
type ResponseHandler struct {
	baseHandler    *shared.BaseResponseHandler
	templateEngine *tplengine.TemplateEngine
	contextBuilder *shared.ContextBuilder
}

// NewResponseHandler creates a new basic task response handler
func NewResponseHandler(
	templateEngine *tplengine.TemplateEngine,
	contextBuilder *shared.ContextBuilder,
	baseHandler *shared.BaseResponseHandler,
) (*ResponseHandler, error) {
	if baseHandler == nil {
		return nil, fmt.Errorf("failed to create basic response handler: baseHandler is required but was nil")
	}
	if templateEngine == nil {
		return nil, fmt.Errorf("failed to create basic response handler: templateEngine is required but was nil")
	}
	if contextBuilder == nil {
		return nil, fmt.Errorf("failed to create basic response handler: contextBuilder is required but was nil")
	}
	return &ResponseHandler{
		baseHandler:    baseHandler,
		templateEngine: templateEngine,
		contextBuilder: contextBuilder,
	}, nil
}

// HandleResponse processes a basic task execution response
func (h *ResponseHandler) HandleResponse(
	ctx context.Context,
	input *shared.ResponseInput,
) (*shared.ResponseOutput, error) {
	// Validate input
	if err := h.baseHandler.ValidateInput(input); err != nil {
		return nil, err
	}
	// Validate task type matches handler
	if input.TaskConfig.Type != task.TaskTypeBasic {
		return nil, &shared.ValidationError{
			Field: "task_type",
			Message: fmt.Sprintf(
				"basic response handler received incorrect task type: expected '%s', got '%s'",
				task.TaskTypeBasic,
				input.TaskConfig.Type,
			),
		}
	}
	// Basic tasks use standard main task processing without any special handling
	// Simply delegate to base handler for all common logic
	return h.baseHandler.ProcessMainTaskResponse(ctx, input)
}

// Type returns the task type this handler processes
func (h *ResponseHandler) Type() task.Type {
	return task.TaskTypeBasic
}
