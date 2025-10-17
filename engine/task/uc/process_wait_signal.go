package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/pkg/logger"
)

// ProcessWaitSignal handles signal processing for wait tasks
type ProcessWaitSignal struct {
	taskRepo    task.Repository
	configStore services.ConfigStore
	evaluator   ConditionEvaluator
}

// ConditionEvaluator evaluates conditional expressions for wait tasks.
type ConditionEvaluator interface {
	Evaluate(ctx context.Context, expression string, data map[string]any) (bool, error)
}

// NewProcessWaitSignal creates a new ProcessWaitSignal use case
func NewProcessWaitSignal(
	taskRepo task.Repository,
	configStore services.ConfigStore,
	evaluator ConditionEvaluator,
) *ProcessWaitSignal {
	return &ProcessWaitSignal{
		taskRepo:    taskRepo,
		configStore: configStore,
		evaluator:   evaluator,
	}
}

// ProcessWaitSignalInput contains the input for processing wait signals
type ProcessWaitSignalInput struct {
	TaskExecID core.ID        `json:"task_exec_id"`
	SignalName string         `json:"signal_name"`
	Payload    map[string]any `json:"payload"`
}

// ProcessWaitSignalOutput contains the result of signal processing
type ProcessWaitSignalOutput struct {
	ConditionMet    bool         `json:"condition_met"`
	ProcessorOutput *core.Output `json:"processor_output,omitempty"`
	Error           string       `json:"error,omitempty"`
}

// Execute processes a signal for a wait task
func (uc *ProcessWaitSignal) Execute(
	ctx context.Context,
	input *ProcessWaitSignalInput,
) (*ProcessWaitSignalOutput, error) {
	log := logger.FromContext(ctx)
	// Get task state
	taskState, err := uc.taskRepo.GetState(ctx, input.TaskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task state: %w", err)
	}
	// Get task config
	taskConfig, err := uc.configStore.Get(ctx, taskState.TaskExecID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to load task config: %w", err)
	}
	// Validate wait task
	if taskConfig.Type != task.TaskTypeWait {
		return nil, fmt.Errorf("task %s is not a wait task", taskState.TaskID)
	}
	// Validate task is in waiting state
	if taskState.Status != core.StatusWaiting {
		return nil, fmt.Errorf("task %s is not in waiting state (current: %s)", taskState.TaskID, taskState.Status)
	}
	// Check if signal matches expected signal
	if taskConfig.WaitFor != input.SignalName {
		log.Debug("Signal does not match expected signal",
			"expected", taskConfig.WaitFor,
			"received", input.SignalName)
		// Return false without error - mismatched signals are a normal flow
		return &ProcessWaitSignalOutput{
			ConditionMet: false,
		}, nil
	}
	// Build context for evaluation
	evalContext := uc.buildEvaluationContext(input, taskState, taskConfig)
	// Log evaluation without sensitive data
	log.Debug("Evaluating condition",
		"condition", taskConfig.Condition,
		"signal_name", input.SignalName,
		"task_id", taskState.TaskID)
	// Evaluate condition
	conditionMet, err := uc.evaluator.Evaluate(ctx, taskConfig.Condition, evalContext)
	if err != nil {
		log.Error("Failed to evaluate condition",
			"condition", taskConfig.Condition,
			"error", err)
		// Update task state to FAILED
		taskState.Status = core.StatusFailed
		taskState.Error = core.NewError(
			fmt.Errorf("condition evaluation failed: %v", err),
			"CONDITION_EVAL_ERROR",
			map[string]any{
				"condition": taskConfig.Condition,
				"error":     err.Error(),
			},
		)
		if updateErr := uc.taskRepo.UpsertState(ctx, taskState); updateErr != nil {
			log.Error("Failed to update task state to FAILED", "error", updateErr)
		}
		// Return error without original error details to prevent double wrapping
		return nil, fmt.Errorf("condition evaluation failed")
	}
	// Prepare output
	output := &ProcessWaitSignalOutput{
		ConditionMet: conditionMet,
	}
	// Include processor output if available
	if taskConfig.Processor != nil && taskState.Output != nil {
		if processorOutput, ok := (*taskState.Output)["processor_output"]; ok {
			// Attempt type assertion with proper error handling
			switch v := processorOutput.(type) {
			case *core.Output:
				output.ProcessorOutput = v
			case core.Output:
				// Handle non-pointer case
				output.ProcessorOutput = &v
			default:
				log.Warn("Processor output has unexpected type",
					"expectedType", "*core.Output",
					"actualType", fmt.Sprintf("%T", processorOutput))
			}
		}
	}
	log.Debug("Signal processing complete",
		"signal", input.SignalName,
		"conditionMet", conditionMet)
	return output, nil
}

// buildEvaluationContext builds the context for CEL evaluation
func (uc *ProcessWaitSignal) buildEvaluationContext(
	input *ProcessWaitSignalInput,
	taskState *task.State,
	taskConfig *task.Config,
) map[string]any {
	evalContext := map[string]any{
		"signal": map[string]any{
			"name":    input.SignalName,
			"payload": input.Payload,
		},
		"task": map[string]any{
			"id":     taskState.TaskID,
			"status": string(taskState.Status),
			"input":  taskState.Input,
			"output": taskState.Output,
		},
		"workflow": map[string]any{
			"id":      taskState.WorkflowID,
			"exec_id": taskState.WorkflowExecID.String(),
		},
	}
	// Add processor output if available
	if taskConfig.Processor != nil && taskState.Output != nil {
		if processorData, ok := (*taskState.Output)["processor_output"]; ok {
			evalContext["processor"] = map[string]any{
				"output": processorData,
			}
		}
	}
	return evalContext
}
