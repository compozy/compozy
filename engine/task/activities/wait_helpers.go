package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

const (
	EvaluateConditionLabel = "EvaluateCondition"
)

// EvaluateConditionInput represents input for CEL condition evaluation
type EvaluateConditionInput struct {
	Expression      string                `json:"expression"`
	Signal          *task.SignalEnvelope  `json:"signal"`
	ProcessorOutput *task.ProcessorOutput `json:"processor_output,omitempty"`
}

// EvaluateCondition handles CEL expression evaluation
type EvaluateCondition struct {
	evaluator *task.CELEvaluator
}

// NewEvaluateCondition creates a new condition evaluator activity
func NewEvaluateCondition(evaluator *task.CELEvaluator) *EvaluateCondition {
	return &EvaluateCondition{
		evaluator: evaluator,
	}
}

func (a *EvaluateCondition) Run(ctx context.Context, input *EvaluateConditionInput) (bool, error) {
	signalMap, err := core.AsMapDefault(input.Signal)
	if err != nil {
		return false, fmt.Errorf("failed to convert signal to map: %w", err)
	}
	data := map[string]any{
		"signal": signalMap,
	}
	if input.ProcessorOutput != nil {
		processorMap, err := core.AsMapDefault(input.ProcessorOutput)
		if err != nil {
			return false, fmt.Errorf("failed to convert processor output to map: %w", err)
		}
		data["processor"] = processorMap
	}
	result, err := a.evaluator.Evaluate(ctx, input.Expression, data)
	if err != nil {
		return false, fmt.Errorf("condition evaluation failed: %w", err)
	}
	return result, nil
}
