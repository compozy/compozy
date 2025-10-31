package executors

import (
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
	"go.temporal.io/sdk/workflow"
)

type TaskWaitExecutor struct {
	*ContextBuilder
	// ExecutionHandler allows execution of processor tasks
	ExecutionHandler func(ctx workflow.Context, taskConfig *task.Config, depth ...int) (task.Response, error)
}

func NewTaskWaitExecutor(
	contextBuilder *ContextBuilder,
	handler func(ctx workflow.Context, taskConfig *task.Config, depth ...int) (task.Response, error),
) *TaskWaitExecutor {
	return &TaskWaitExecutor{
		ContextBuilder:   contextBuilder,
		ExecutionHandler: handler,
	}
}

func (e *TaskWaitExecutor) Execute(ctx workflow.Context, taskConfig *task.Config) (task.Response, error) {
	response, err := e.initializeWaitTask(ctx, taskConfig)
	if err != nil {
		return nil, err
	}
	timeout, err := e.extractTimeout(response)
	if err != nil {
		return nil, err
	}
	waitState := &WaitState{ConditionMet: false}
	signalChan := workflow.GetSignalChannel(ctx, taskConfig.WaitFor)
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	timer := workflow.NewTimer(timerCtx, timeout)
	e.logWaitTaskStart(ctx, taskConfig, timeout)
	err = e.updateWorkflowStatus(ctx, core.StatusPaused)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to update workflow status to paused", "error", err)
	}
	e.waitForCondition(ctx, taskConfig, waitState, signalChan, timer, cancelTimer)
	return e.finalizeResponse(ctx, taskConfig, response, waitState, timeout)
}

func (e *TaskWaitExecutor) initializeWaitTask(
	ctx workflow.Context,
	taskConfig *task.Config,
) (*task.MainTaskResponse, error) {
	var response *task.MainTaskResponse
	actLabel := tkacts.ExecuteWaitLabel
	actInput := tkacts.ExecuteWaitInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		TaskConfig:     taskConfig,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (e *TaskWaitExecutor) waitForCondition(
	ctx workflow.Context,
	taskConfig *task.Config,
	waitState *WaitState,
	signalChan workflow.ReceiveChannel,
	timer workflow.Future,
	cancelTimer workflow.CancelFunc,
) {
	for !waitState.ConditionMet {
		selector := workflow.NewSelector(ctx)
		selector.AddReceive(signalChan, func(c workflow.ReceiveChannel, _ bool) {
			e.handleSignalReceived(ctx, c, taskConfig, waitState, timer, cancelTimer)
		})
		if !waitState.TimerCancelled {
			selector.AddFuture(timer, func(_ workflow.Future) {
				e.handleTimeout(ctx, taskConfig, waitState)
			})
		}
		selector.Select(ctx)
	}
}

func (e *TaskWaitExecutor) loadNextTaskConfig(ctx workflow.Context, currentTaskID, nextTaskID string) *task.Config {
	var nextTaskConfig *task.Config
	actInput := &tkacts.LoadTaskConfigInput{
		WorkflowConfig: e.WorkflowConfig,
		TaskID:         nextTaskID,
	}
	taskCtx := e.BuildTaskContext(ctx, currentTaskID)
	err := workflow.ExecuteActivity(taskCtx, tkacts.LoadTaskConfigLabel, actInput).
		Get(taskCtx, &nextTaskConfig)
	if err != nil {
		workflow.GetLogger(ctx).
			Error("Failed to load next task config", "next_task_id", nextTaskID, "error", err)
		return nil
	}
	return nextTaskConfig
}

func (e *TaskWaitExecutor) finalizeResponse(
	ctx workflow.Context,
	taskConfig *task.Config,
	response *task.MainTaskResponse,
	waitState *WaitState,
	timeout time.Duration,
) (task.Response, error) {
	switch {
	case waitState.Error != nil:
		e.handleErrorResponse(response, waitState, taskConfig, ctx)
	case waitState.TimedOut:
		e.handleTimeoutResponse(response, waitState, taskConfig, timeout, ctx)
	case waitState.MatchingSignal != nil:
		e.handleSuccessResponse(ctx, response, waitState, taskConfig)
	}
	return response, nil
}

func (e *TaskWaitExecutor) processSignal(
	ctx workflow.Context,
	config *task.Config,
	signal *task.SignalEnvelope,
) (bool, *task.ProcessorOutput, error) {
	ctx = e.BuildTaskContext(ctx, config.ID)
	processorOutput, err := e.executeProcessor(ctx, config, signal)
	if err != nil {
		return false, nil, err
	}
	return e.evaluateCondition(ctx, config, signal, processorOutput)
}

// WaitState tracks the state of the wait task within the workflow
type WaitState struct {
	ConditionMet    bool
	TimedOut        bool
	TimerCancelled  bool
	MatchingSignal  *task.SignalEnvelope
	ProcessorOutput *task.ProcessorOutput
	Error           error
}

// extractTimeout retrieves the parsed timeout duration from the activity response output.
// It reads timeout_seconds from the response (set by the activity after template evaluation)
// and converts it to time.Duration. Handles both int64 and float64 types for JSON compatibility.
// Returns an error if the response is invalid, timeout is missing, or has an invalid value.
func (e *TaskWaitExecutor) extractTimeout(response *task.MainTaskResponse) (time.Duration, error) {
	if response == nil || response.State == nil || response.State.Output == nil {
		return 0, fmt.Errorf("response state or output is nil")
	}
	output := *response.State.Output
	timeoutSeconds, ok := output["timeout_seconds"]
	if !ok {
		return 0, fmt.Errorf("timeout_seconds not found in response output")
	}
	timeoutSecondsInt64, ok := timeoutSeconds.(int64)
	if !ok {
		timeoutSecondsFloat64, ok := timeoutSeconds.(float64)
		if !ok {
			return 0, fmt.Errorf("timeout_seconds has unexpected type: %T", timeoutSeconds)
		}
		timeoutSecondsInt64 = int64(timeoutSecondsFloat64)
	}
	if timeoutSecondsInt64 <= 0 {
		return 0, fmt.Errorf("wait task requires a positive timeout (got %d seconds)", timeoutSecondsInt64)
	}
	return time.Duration(timeoutSecondsInt64) * time.Second, nil
}

func (e *TaskWaitExecutor) logWaitTaskStart(ctx workflow.Context, taskConfig *task.Config, timeout time.Duration) {
	workflow.GetLogger(ctx).Info("Wait task started",
		"task_id", taskConfig.ID,
		"signal_name", taskConfig.WaitFor,
		"timeout", timeout,
		"has_processor", taskConfig.Processor != nil)
}

func (e *TaskWaitExecutor) updateWorkflowStatus(ctx workflow.Context, status core.StatusType) error {
	updateInput := &wfacts.UpdateStateInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		Status:         status,
	}
	return workflow.ExecuteActivity(ctx, wfacts.UpdateStateLabel, updateInput).Get(ctx, nil)
}

func (e *TaskWaitExecutor) handleSignalReceived(
	ctx workflow.Context,
	c workflow.ReceiveChannel,
	taskConfig *task.Config,
	waitState *WaitState,
	timer workflow.Future,
	cancelTimer workflow.CancelFunc,
) {
	var signal task.SignalEnvelope
	c.Receive(ctx, &signal)
	log := workflow.GetLogger(ctx)
	log.Info("Signal received", "signal_id", signal.Metadata.SignalID, "workflow_id", signal.Metadata.WorkflowID)
	shouldContinue, processorOutput, err := e.processSignal(ctx, taskConfig, &signal)
	if err != nil {
		waitState.Error = err
		waitState.ConditionMet = true
		return
	}
	if shouldContinue {
		waitState.ConditionMet = true
		waitState.MatchingSignal = &signal
		waitState.ProcessorOutput = processorOutput
		log.Info("Condition met, continuing workflow", "signal_id", signal.Metadata.SignalID, "task_id", taskConfig.ID)
		cancelTimer()
		if err := timer.Get(ctx, nil); err != nil {
			log.Warn("Timer cancellation error", "error", err)
		}
		waitState.TimerCancelled = true
	} else {
		log.Info("Condition not met, continuing to wait", "signal_id", signal.Metadata.SignalID, "task_id", taskConfig.ID)
	}
}

func (e *TaskWaitExecutor) handleTimeout(ctx workflow.Context, taskConfig *task.Config, waitState *WaitState) {
	workflow.GetLogger(ctx).Info("Wait task timed out", "task_id", taskConfig.ID)
	waitState.TimedOut = true
	waitState.ConditionMet = true
}

func (e *TaskWaitExecutor) handleErrorResponse(
	response *task.MainTaskResponse,
	waitState *WaitState,
	taskConfig *task.Config,
	ctx workflow.Context,
) {
	response.State.Status = core.StatusFailed
	response.State.Error = core.NewError(waitState.Error, "WAIT_TASK_ERROR", nil)
	if taskConfig.OnError != nil && taskConfig.OnError.Next != nil {
		response.NextTask = e.loadNextTaskConfig(ctx, taskConfig.ID, *taskConfig.OnError.Next)
	}
}

func (e *TaskWaitExecutor) handleTimeoutResponse(
	response *task.MainTaskResponse,
	_ *WaitState,
	taskConfig *task.Config,
	timeout time.Duration,
	ctx workflow.Context,
) {
	response.State.Status = core.StatusFailed
	response.State.Error = core.NewError(
		fmt.Errorf("wait task timed out after %s", timeout),
		"WAIT_TIMEOUT",
		map[string]any{
			"signal_name": taskConfig.WaitFor,
			"timeout":     taskConfig.Timeout,
			"timeout_ms":  timeout.Milliseconds(),
			"timeout_sec": int64(timeout.Seconds()),
		},
	)
	if taskConfig.OnTimeout != "" {
		response.NextTask = e.loadNextTaskConfig(ctx, taskConfig.ID, taskConfig.OnTimeout)
	} else if taskConfig.OnError != nil && taskConfig.OnError.Next != nil {
		response.NextTask = e.loadNextTaskConfig(ctx, taskConfig.ID, *taskConfig.OnError.Next)
	}
}

func (e *TaskWaitExecutor) handleSuccessResponse(
	ctx workflow.Context,
	response *task.MainTaskResponse,
	waitState *WaitState,
	taskConfig *task.Config,
) {
	response.State.Status = core.StatusSuccess
	response.State.Output = &core.Output{
		"wait_status":      "completed",
		"signal":           waitState.MatchingSignal,
		"processor_output": waitState.ProcessorOutput,
	}
	err := e.updateWorkflowStatus(ctx, core.StatusRunning)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to update workflow status to running", "error", err)
	}
	if taskConfig.OnSuccess != nil && taskConfig.OnSuccess.Next != nil {
		response.NextTask = e.loadNextTaskConfig(ctx, taskConfig.ID, *taskConfig.OnSuccess.Next)
	}
}

func (e *TaskWaitExecutor) executeProcessor(
	ctx workflow.Context,
	config *task.Config,
	signal *task.SignalEnvelope,
) (*task.ProcessorOutput, error) {
	if config.Processor == nil {
		return nil, nil
	}
	log := workflow.GetLogger(ctx)
	log.Info("Executing wait task processor", "processor_id", config.Processor.ID)
	processorConfig, err := e.normalizeProcessor(ctx, config, signal)
	if err != nil {
		return nil, err
	}
	return e.runProcessor(ctx, processorConfig, signal)
}

func (e *TaskWaitExecutor) normalizeProcessor(
	ctx workflow.Context,
	config *task.Config,
	signal *task.SignalEnvelope,
) (*task.Config, error) {
	normInput := &tkacts.NormalizeWaitProcessorInput{
		WorkflowID:       e.WorkflowID,
		WorkflowExecID:   e.WorkflowExecID,
		ProcessorConfig:  config.Processor,
		ParentTaskConfig: config,
		Signal:           signal,
	}
	var processorConfig *task.Config
	err := workflow.ExecuteActivity(ctx, tkacts.NormalizeWaitProcessorLabel, normInput).Get(ctx, &processorConfig)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to normalize processor templates with signal context", "error", err)
		return nil, fmt.Errorf("failed to normalize processor: %w", err)
	}
	return processorConfig, nil
}

func (e *TaskWaitExecutor) runProcessor(
	ctx workflow.Context,
	processorConfig *task.Config,
	signal *task.SignalEnvelope,
) (*task.ProcessorOutput, error) {
	processorInput := core.Input{"signal": signal}
	if processorConfig.With != nil {
		processorInput = core.CopyMaps(processorInput, *processorConfig.With)
	}
	processorInput["signal"] = signal
	processorConfig.With = &processorInput
	processorResponse, err := e.ExecutionHandler(ctx, processorConfig, 1)
	if err != nil {
		workflow.GetLogger(ctx).Error("Processor execution failed", "error", err)
		return nil, fmt.Errorf("processor failed: %w", err)
	}
	if mainTaskResponse, ok := processorResponse.(*task.MainTaskResponse); ok && mainTaskResponse.State != nil {
		return &task.ProcessorOutput{
			Output: mainTaskResponse.State.Output,
			Error:  mainTaskResponse.State.Error,
		}, nil
	}
	return nil, nil
}

func (e *TaskWaitExecutor) evaluateCondition(
	ctx workflow.Context,
	config *task.Config,
	signal *task.SignalEnvelope,
	processorOutput *task.ProcessorOutput,
) (bool, *task.ProcessorOutput, error) {
	evalInput := &tkacts.EvaluateConditionInput{
		Expression:      config.Condition,
		Signal:          signal,
		ProcessorOutput: processorOutput,
	}
	var conditionMet bool
	err := workflow.ExecuteActivity(ctx, tkacts.EvaluateConditionLabel, evalInput).Get(ctx, &conditionMet)
	if err != nil {
		workflow.GetLogger(ctx).Error("Condition evaluation failed", "error", err, "expression", config.Condition)
		return false, processorOutput, fmt.Errorf("condition evaluation failed: %w", err)
	}
	return conditionMet, processorOutput, nil
}
