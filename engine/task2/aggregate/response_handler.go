package aggregate

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
)

// ResponseHandler handles response processing for aggregate tasks
type ResponseHandler struct {
	baseHandler    *shared.BaseResponseHandler
	templateEngine *tplengine.TemplateEngine
	contextBuilder *shared.ContextBuilder
}

// NewResponseHandler creates a new aggregate task response handler
func NewResponseHandler(
	templateEngine *tplengine.TemplateEngine,
	contextBuilder *shared.ContextBuilder,
	baseHandler *shared.BaseResponseHandler,
) *ResponseHandler {
	if baseHandler == nil {
		panic("baseHandler cannot be nil")
	}
	return &ResponseHandler{
		baseHandler:    baseHandler,
		templateEngine: templateEngine,
		contextBuilder: contextBuilder,
	}
}

// HandleResponse processes an aggregate task execution response
func (h *ResponseHandler) HandleResponse(
	ctx context.Context,
	input *shared.ResponseInput,
) (*shared.ResponseOutput, error) {
	// Validate input
	if err := h.baseHandler.ValidateInput(input); err != nil {
		return nil, err
	}
	// Validate task type matches handler
	if input.TaskConfig.Type != task.TaskTypeAggregate {
		return nil, &shared.ValidationError{
			Field:   "task_type",
			Message: "handler type does not match task type",
		}
	}
	// Process aggregation completion
	response, err := h.baseHandler.ProcessMainTaskResponse(ctx, input)
	if err != nil {
		return nil, err
	}

	// Aggregate-specific: Validate aggregation result
	if response.State.Status == core.StatusSuccess && response.State.Output != nil {
		// Aggregation completed - validate result structure
		if err := h.validateAggregationResult(response.State.Output); err != nil {
			return nil, fmt.Errorf("invalid aggregation result: %w", err)
		}

		log := logger.FromContext(ctx).With(
			"task_exec_id", input.TaskState.TaskExecID,
			"task_id", input.TaskState.TaskID,
		)
		log.Info("Aggregate task completed successfully")
	}

	return response, nil
}

// Type returns the task type this handler processes
func (h *ResponseHandler) Type() task.Type {
	return task.TaskTypeAggregate
}

// validateAggregationResult validates the structure of aggregation output
func (h *ResponseHandler) validateAggregationResult(output *core.Output) error {
	if output == nil {
		return fmt.Errorf("aggregate output cannot be nil for validation")
	}

	// Check if the output contains expected aggregation fields
	outputMap := map[string]any(*output)

	// Validate that aggregated data exists
	if _, exists := outputMap[shared.FieldAggregated]; !exists {
		// Not all aggregate tasks may have "aggregated" field, check for other patterns
		// This validation can be customized based on specific aggregate task requirements

		// Check if it has any data at all
		if len(outputMap) == 0 {
			return fmt.Errorf("aggregate output is empty: no data to aggregate")
		}
	}

	return nil
}

// HandleAggregateCompletion handles the completion of an aggregate task
func (h *ResponseHandler) HandleAggregateCompletion(
	ctx context.Context,
	state *task.State,
	aggregatedData map[string]any,
) error {
	// Update the state with aggregated data
	if state.Output == nil {
		output := core.Output(aggregatedData)
		state.Output = &output
	} else {
		// Merge aggregated data into existing output
		merged := core.CopyMaps(*state.Output, aggregatedData)
		mergedOutput := core.Output(merged)
		state.Output = &mergedOutput
	}

	log := logger.FromContext(ctx).With(
		"task_exec_id", state.TaskExecID,
		"task_id", state.TaskID,
	)
	log.Info("Aggregation completed", "data_keys", len(aggregatedData))

	return nil
}
