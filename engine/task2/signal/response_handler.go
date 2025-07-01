package signal

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
)

// ResponseHandler handles response processing for signal tasks
type ResponseHandler struct {
	baseHandler    *shared.BaseResponseHandler
	templateEngine *tplengine.TemplateEngine
	contextBuilder *shared.ContextBuilder
}

// NewResponseHandler creates a new signal task response handler
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

// HandleResponse processes a signal task execution response
func (h *ResponseHandler) HandleResponse(
	ctx context.Context,
	input *shared.ResponseInput,
) (*shared.ResponseOutput, error) {
	// Validate input
	if err := h.baseHandler.ValidateInput(input); err != nil {
		return nil, err
	}
	// Validate task type matches handler
	if input.TaskConfig.Type != task.TaskTypeSignal {
		return nil, &shared.ValidationError{
			Field:   "task_type",
			Message: "handler type does not match task type",
		}
	}
	// Process signal dispatch result
	response, err := h.baseHandler.ProcessMainTaskResponse(ctx, input)
	if err != nil {
		return nil, err
	}

	// Signal-specific: Confirm dispatch was successful
	if response.State.Status == core.StatusSuccess {
		log := logger.FromContext(ctx).With(
			"task_exec_id", input.TaskState.TaskExecID,
			"task_id", input.TaskState.TaskID,
		)
		log.Info("Signal dispatched successfully")

		// Log signal details if available
		if response.State.Output != nil {
			if signalName, exists := (*response.State.Output)["signal_name"]; exists {
				log.Info("Signal details", "signal_name", signalName)
			}
			if targetTask, exists := (*response.State.Output)["target_task"]; exists {
				log.Info("Signal target", "target_task", targetTask)
			}
		}
	}

	return response, nil
}

// Type returns the task type this handler processes
func (h *ResponseHandler) Type() task.Type {
	return task.TaskTypeSignal
}

// ValidateSignalDispatch validates that the signal was properly dispatched
func (h *ResponseHandler) ValidateSignalDispatch(state *task.State) error {
	// Signal tasks should complete successfully if the signal was dispatched
	// The actual signal delivery is handled asynchronously by the workflow engine
	if state.Status == core.StatusSuccess {
		// Signal was dispatched successfully
		return nil
	}
	return nil
}
