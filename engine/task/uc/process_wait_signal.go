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
	taskState, taskConfig, err := uc.loadWaitTaskResources(ctx, input.TaskExecID)
	if err != nil {
		return nil, err
	}
	if err := uc.validateWaitTaskState(taskState, taskConfig); err != nil {
		return nil, err
	}
	if mismatch := uc.handleSignalMismatch(ctx, taskConfig, input); mismatch != nil {
		return mismatch, nil
	}
	evalContext := uc.buildEvaluationContext(input, taskState, taskConfig)
	conditionMet, err := uc.evaluateWaitCondition(ctx, taskConfig, taskState, input.SignalName, evalContext)
	if err != nil {
		return nil, err
	}
	output := &ProcessWaitSignalOutput{ConditionMet: conditionMet}
	uc.populateProcessorOutput(ctx, taskConfig, taskState, output)
	logger.FromContext(ctx).Debug("Signal processing complete",
		"signal", input.SignalName,
		"conditionMet", conditionMet)
	return output, nil
}

// loadWaitTaskResources retrieves the task state and configuration for evaluation.
func (uc *ProcessWaitSignal) loadWaitTaskResources(
	ctx context.Context,
	taskExecID core.ID,
) (*task.State, *task.Config, error) {
	taskState, err := uc.taskRepo.GetState(ctx, taskExecID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get task state: %w", err)
	}
	taskConfig, err := uc.configStore.Get(ctx, taskState.TaskExecID.String())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load task config: %w", err)
	}
	return taskState, taskConfig, nil
}

// validateWaitTaskState ensures the task is a wait task in a waiting status.
func (uc *ProcessWaitSignal) validateWaitTaskState(taskState *task.State, taskConfig *task.Config) error {
	if taskConfig.Type != task.TaskTypeWait {
		return fmt.Errorf("task %s is not a wait task", taskState.TaskID)
	}
	if taskState.Status != core.StatusWaiting {
		return fmt.Errorf("task %s is not in waiting state (current: %s)", taskState.TaskID, taskState.Status)
	}
	return nil
}

// handleSignalMismatch returns a default output when the signal does not match.
func (uc *ProcessWaitSignal) handleSignalMismatch(
	ctx context.Context,
	taskConfig *task.Config,
	input *ProcessWaitSignalInput,
) *ProcessWaitSignalOutput {
	if taskConfig.WaitFor == input.SignalName {
		return nil
	}
	logger.FromContext(ctx).Debug("Signal does not match expected signal",
		"expected", taskConfig.WaitFor,
		"received", input.SignalName)
	return &ProcessWaitSignalOutput{ConditionMet: false}
}

// evaluateWaitCondition executes the wait condition and records failures.
func (uc *ProcessWaitSignal) evaluateWaitCondition(
	ctx context.Context,
	taskConfig *task.Config,
	taskState *task.State,
	signalName string,
	evalContext map[string]any,
) (bool, error) {
	log := logger.FromContext(ctx)
	log.Debug("Evaluating condition",
		"condition", taskConfig.Condition,
		"signal_name", signalName,
		"task_id", taskState.TaskID)
	conditionMet, err := uc.evaluator.Evaluate(ctx, taskConfig.Condition, evalContext)
	if err != nil {
		uc.handleConditionEvaluationFailure(ctx, taskConfig, taskState, err)
		return false, fmt.Errorf("condition evaluation failed")
	}
	return conditionMet, nil
}

// handleConditionEvaluationFailure updates task state when condition evaluation errors.
func (uc *ProcessWaitSignal) handleConditionEvaluationFailure(
	ctx context.Context,
	taskConfig *task.Config,
	taskState *task.State,
	err error,
) {
	log := logger.FromContext(ctx)
	log.Error("Failed to evaluate condition",
		"condition", taskConfig.Condition,
		"error", err)
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
}

// populateProcessorOutput copies processor output data into the response when present.
func (uc *ProcessWaitSignal) populateProcessorOutput(
	ctx context.Context,
	taskConfig *task.Config,
	taskState *task.State,
	output *ProcessWaitSignalOutput,
) {
	if taskConfig.Processor == nil || taskState.Output == nil {
		return
	}
	processorOutput, ok := (*taskState.Output)["processor_output"]
	if !ok {
		return
	}
	switch v := processorOutput.(type) {
	case *core.Output:
		output.ProcessorOutput = v
	case core.Output:
		output.ProcessorOutput = &v
	default:
		logger.FromContext(ctx).Warn("Processor output has unexpected type",
			"expectedType", "*core.Output",
			"actualType", fmt.Sprintf("%T", processorOutput))
	}
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
	if taskConfig.Processor != nil && taskState.Output != nil {
		if processorData, ok := (*taskState.Output)["processor_output"]; ok {
			evalContext["processor"] = map[string]any{
				"output": processorData,
			}
		}
	}
	return evalContext
}
