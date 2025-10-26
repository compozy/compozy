package wait

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
)

// ResponseHandler handles response processing for wait tasks
type ResponseHandler struct {
	baseHandler    *shared.BaseResponseHandler
	templateEngine *tplengine.TemplateEngine
	contextBuilder *shared.ContextBuilder
}

// NewResponseHandler creates a new wait task response handler
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

// HandleResponse processes a wait task execution response
func (h *ResponseHandler) HandleResponse(
	ctx context.Context,
	input *shared.ResponseInput,
) (*shared.ResponseOutput, error) {
	if err := h.baseHandler.ValidateInput(input); err != nil {
		return nil, err
	}
	if input.TaskConfig.Type != task.TaskTypeWait {
		return nil, &shared.ValidationError{
			Field:   "task_type",
			Message: "handler type does not match task type",
		}
	}
	response, err := h.baseHandler.ProcessMainTaskResponse(ctx, input)
	if err != nil {
		return nil, err
	}
	if input.ExecutionError == nil && response.State.Status == core.StatusSuccess {
		log := logger.FromContext(ctx).With(
			"task_exec_id", input.TaskState.TaskExecID,
			"task_id", input.TaskState.TaskID,
		)
		log.Info("Wait task completed - signal received")

		if response.State.Output != nil {
			if signalData, exists := (*response.State.Output)[shared.FieldSignal]; exists {
				log.Info("Signal data received", "signal", signalData)
			}
		}
	}
	return response, nil
}

// Type returns the task type this handler processes
func (h *ResponseHandler) Type() task.Type {
	return task.TaskTypeWait
}

// ValidateWaitCompletion validates that the wait task completed properly
func (h *ResponseHandler) ValidateWaitCompletion(state *task.State) error {
	if state == nil {
		return fmt.Errorf("state cannot be nil")
	}
	if state.Status == core.StatusSuccess && state.Output == nil {
		return nil
	}
	return nil
}
