package router

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
)

// ResponseHandler handles response processing for router tasks
type ResponseHandler struct {
	baseHandler    *shared.BaseResponseHandler
	templateEngine *tplengine.TemplateEngine
	contextBuilder *shared.ContextBuilder
}

// NewResponseHandler creates a new router task response handler
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

// HandleResponse processes a router task execution response
func (h *ResponseHandler) HandleResponse(
	ctx context.Context,
	input *shared.ResponseInput,
) (*shared.ResponseOutput, error) {
	if err := h.validateRequest(input); err != nil {
		return nil, err
	}
	response, err := h.baseHandler.ProcessMainTaskResponse(ctx, input)
	if err != nil {
		return nil, err
	}
	if response.State.Status == core.StatusSuccess {
		if err := h.processSuccessfulRoute(ctx, input, response); err != nil {
			return nil, err
		}
	}
	return response, nil
}

// validateRequest validates the input and task type
func (h *ResponseHandler) validateRequest(input *shared.ResponseInput) error {
	if err := h.baseHandler.ValidateInput(input); err != nil {
		return err
	}
	if input.TaskConfig.Type != task.TaskTypeRouter {
		return &shared.ValidationError{
			Field:   "task_type",
			Message: "handler type does not match task type",
		}
	}
	return nil
}

// processSuccessfulRoute handles router-specific logic for successful responses
func (h *ResponseHandler) processSuccessfulRoute(
	ctx context.Context,
	input *shared.ResponseInput,
	response *shared.ResponseOutput,
) error {
	if response.State.Output == nil {
		return fmt.Errorf("router task %s must produce routing decision output", input.TaskConfig.ID)
	}
	log := logger.FromContext(ctx).With(
		"task_exec_id", response.State.TaskExecID,
		"task_id", response.State.TaskID,
	)
	log.Info("Router task completed, routing decision made")
	if err := h.setNextTaskFromRoute(input, response); err != nil {
		log.Warn("Failed to set next task from route", "error", err, "task_id", input.TaskConfig.ID)
	}
	return nil
}

// Type returns the task type this handler processes
func (h *ResponseHandler) Type() task.Type {
	return task.TaskTypeRouter
}

// ValidateRoutingDecision validates that a routing decision was properly made
func (h *ResponseHandler) ValidateRoutingDecision(output *core.Output) error {
	if output == nil {
		return fmt.Errorf("router output cannot be nil for validation")
	}
	return nil
}

// setNextTaskFromRoute extracts the route from output and sets NextTaskOverride
func (h *ResponseHandler) setNextTaskFromRoute(
	input *shared.ResponseInput,
	response *shared.ResponseOutput,
) error {
	if response.State.Output == nil {
		return nil
	}
	routeTaken, exists := (*response.State.Output)[shared.FieldRouteTaken]
	if !exists {
		return nil // No explicit route taken
	}
	routeStr, ok := routeTaken.(string)
	if !ok {
		return fmt.Errorf("route_taken must be a string, got %T", routeTaken)
	}
	return h.applyRouteInheritance(input, routeStr)
}

// applyRouteInheritance finds the target task and applies config inheritance
func (h *ResponseHandler) applyRouteInheritance(
	input *shared.ResponseInput,
	routeStr string,
) error {
	for i := range input.WorkflowConfig.Tasks {
		if input.WorkflowConfig.Tasks[i].ID == routeStr {
			targetTaskConfig, err := input.WorkflowConfig.Tasks[i].Clone()
			if err != nil {
				return fmt.Errorf("failed to clone target task config for inheritance: %w", err)
			}
			if err := shared.InheritTaskConfig(targetTaskConfig, input.TaskConfig); err != nil {
				return fmt.Errorf("failed to inherit task config: %w", err)
			}
			input.NextTaskOverride = targetTaskConfig
			return nil
		}
	}
	return fmt.Errorf("route '%s' not found in workflow tasks", routeStr)
}
