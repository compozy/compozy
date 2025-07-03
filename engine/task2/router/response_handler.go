package router

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
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
	// Validate input
	if err := h.baseHandler.ValidateInput(input); err != nil {
		return nil, err
	}
	// Validate task type matches handler
	if input.TaskConfig.Type != task.TaskTypeRouter {
		return nil, &shared.ValidationError{
			Field:   "task_type",
			Message: "handler type does not match task type",
		}
	}
	// Process routing decision result
	response, err := h.baseHandler.ProcessMainTaskResponse(ctx, input)
	if err != nil {
		return nil, err
	}

	// Router-specific: Validate routing decision was made
	if response.State.Status == core.StatusSuccess {
		if response.State.Output == nil {
			return nil, fmt.Errorf("router task %s must produce routing decision output", input.TaskConfig.ID)
		}

		// Log the routing decision
		log := logger.FromContext(ctx).With(
			"task_exec_id", response.State.TaskExecID,
			"task_id", response.State.TaskID,
		)
		log.Info("Router task completed, routing decision made")
	}

	// Router-specific: Extract the selected route from output and set NextTaskOverride
	// Router tasks produce output with the selected route
	if response.State.Status == core.StatusSuccess && response.State.Output != nil {
		if err := h.setNextTaskFromRoute(input, response); err != nil {
			// Log error but don't fail - let workflow engine handle routing
			log := logger.FromContext(ctx).With("task_id", input.TaskConfig.ID)
			log.Warn("Failed to set next task from route", "error", err)
		}
	}

	return response, nil
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

	// The actual routing logic validation would depend on the specific
	// router implementation and its expected output format
	// This is a placeholder for router-specific validation

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

	// Extract route_taken from output
	routeTaken, exists := (*response.State.Output)[shared.FieldRouteTaken]
	if !exists {
		return nil // No explicit route taken
	}

	// Convert route to string
	routeStr, ok := routeTaken.(string)
	if !ok {
		return fmt.Errorf("route_taken must be a string, got %T", routeTaken)
	}

	// Find the task config for the route
	for i := range input.WorkflowConfig.Tasks {
		if input.WorkflowConfig.Tasks[i].ID == routeStr {
			// Set the NextTaskOverride in the input for the base handler to process
			input.NextTaskOverride = &input.WorkflowConfig.Tasks[i]
			return nil
		}
	}

	return fmt.Errorf("route '%s' not found in workflow tasks", routeStr)
}
