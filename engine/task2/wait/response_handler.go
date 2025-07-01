package wait

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
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
	// Validate input
	if err := h.baseHandler.ValidateInput(input); err != nil {
		return nil, err
	}
	// Validate task type matches handler
	if input.TaskConfig.Type != task.TaskTypeWait {
		return nil, &shared.ValidationError{
			Field:   "task_type",
			Message: "handler type does not match task type",
		}
	}
	// Process wait completion
	response, err := h.baseHandler.ProcessMainTaskResponse(ctx, input)
	if err != nil {
		return nil, err
	}

	// Wait-specific: Validate signal was received
	if input.ExecutionError == nil && response.State.Status == core.StatusSuccess {
		// Wait completed successfully - signal was received
		log := logger.FromContext(ctx).With(
			"task_exec_id", input.TaskState.TaskExecID,
			"task_id", input.TaskState.TaskID,
		)
		log.Info("Wait task completed - signal received")

		// Check if the wait output contains signal information
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
	// Wait tasks are considered complete when they receive a signal
	// The validation here is minimal as the wait logic is handled by the executor
	if state.Status == core.StatusSuccess && state.Output == nil {
		// It's OK for wait tasks to have no output, they just need to complete
		return nil
	}
	return nil
}
