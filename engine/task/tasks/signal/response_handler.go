package signal

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
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
	if err := h.baseHandler.ValidateInput(input); err != nil {
		return nil, err
	}
	if input.TaskConfig.Type != task.TaskTypeSignal {
		return nil, &shared.ValidationError{
			Field:   "task_type",
			Message: "handler type does not match task type",
		}
	}
	response, err := h.baseHandler.ProcessMainTaskResponse(ctx, input)
	if err != nil {
		return nil, err
	}
	if response.State.Status == core.StatusSuccess {
		log := logger.FromContext(ctx).With(
			"task_exec_id", input.TaskState.TaskExecID,
			"task_id", input.TaskState.TaskID,
		)
		log.Info("Signal dispatched successfully")

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
	if state == nil {
		return fmt.Errorf("signal task state cannot be nil")
	}
	if state.Status == core.StatusSuccess {
		return nil
	}
	if state.Status == core.StatusFailed {
		if state.Error != nil {
			return fmt.Errorf("signal dispatch failed: %s", state.Error.Message)
		}
		return fmt.Errorf("signal dispatch failed with unknown error")
	}
	return fmt.Errorf("signal dispatch not completed, current status: %s", state.Status)
}
