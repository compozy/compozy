package executors

import (
	"fmt"
	"maps"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
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
	log := workflow.GetLogger(ctx)
	// First, create the wait task state via activity
	response, err := e.initializeWaitTask(ctx, taskConfig)
	if err != nil {
		return nil, err
	}
	// Parse timeout duration
	timeout, err := core.ParseHumanDuration(taskConfig.Timeout)
	if err != nil {
		log.Error("Invalid timeout format", "timeout", taskConfig.Timeout, "error", err)
		return nil, fmt.Errorf("invalid timeout: %w", err)
	}
	// Validate timeout is positive
	if timeout <= 0 {
		return nil, fmt.Errorf("wait task requires a positive timeout (got %s)", taskConfig.Timeout)
	}
	// Create the wait state for tracking signals
	waitState := &WaitState{
		ConditionMet: false,
	}
	// Set up signal channel
	signalChan := workflow.GetSignalChannel(ctx, taskConfig.WaitFor)
	// Set up timeout with cancellable context
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	timer := workflow.NewTimer(timerCtx, timeout)
	log.Info("Wait task started",
		"task_id", taskConfig.ID,
		"signal_name", taskConfig.WaitFor,
		"timeout", timeout,
		"has_processor", taskConfig.Processor != nil)
	// Update workflow status to PAUSED when wait task starts
	updateInput := &wfacts.UpdateStateInput{
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		Status:         core.StatusPaused,
	}
	err = workflow.ExecuteActivity(ctx, wfacts.UpdateStateLabel, updateInput).Get(ctx, nil)
	if err != nil {
		log.Error("Failed to update workflow status to paused", "error", err)
		// Continue execution despite status update failure
	}
	// Main event loop
	e.waitForCondition(ctx, taskConfig, waitState, signalChan, timer, cancelTimer)
	// Update response based on outcome
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
	log := workflow.GetLogger(ctx)
	for !waitState.ConditionMet {
		// Create a new selector for each iteration
		selector := workflow.NewSelector(ctx)
		// Handle incoming signals
		selector.AddReceive(signalChan, func(c workflow.ReceiveChannel, _ bool) {
			var signal task.SignalEnvelope
			c.Receive(ctx, &signal)
			log.Info("Signal received",
				"signal_id", signal.Metadata.SignalID,
				"workflow_id", signal.Metadata.WorkflowID)
			// Process signal (evaluate condition)
			// Note: Signal deduplication is handled by Temporal server when signals are sent with unique IDs
			shouldContinue, processorOutput, err := e.processSignal(ctx, taskConfig, &signal)
			if err != nil {
				waitState.Error = err
				waitState.ConditionMet = true // Exit the loop to finalize
				return
			}
			if shouldContinue {
				waitState.ConditionMet = true
				waitState.MatchingSignal = &signal
				waitState.ProcessorOutput = processorOutput
				log.Info("Condition met, continuing workflow",
					"signal_id", signal.Metadata.SignalID,
					"task_id", taskConfig.ID)
				// Cancel the timer to prevent non-deterministic behavior
				cancelTimer()
				// Consume the cancellation event to maintain determinism
				if err := timer.Get(ctx, nil); err != nil {
					log.Warn("Timer cancellation error", "error", err)
				}
				waitState.TimerCancelled = true
			} else {
				log.Info("Condition not met, continuing to wait",
					"signal_id", signal.Metadata.SignalID,
					"task_id", taskConfig.ID)
			}
		})
		// Handle timeout - only add if timer wasn't canceled
		if !waitState.TimerCancelled {
			selector.AddFuture(timer, func(_ workflow.Future) {
				log.Info("Wait task timed out", "task_id", taskConfig.ID)
				waitState.TimedOut = true
				waitState.ConditionMet = true // Exit the loop
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
		// Handle fatal error from processor or evaluation
		response.State.Status = core.StatusFailed
		response.State.Error = core.NewError(
			waitState.Error,
			"WAIT_TASK_ERROR",
			nil,
		)
		// Route to on_error
		if taskConfig.OnError != nil && taskConfig.OnError.Next != nil {
			response.NextTask = e.loadNextTaskConfig(ctx, taskConfig.ID, *taskConfig.OnError.Next)
		}
	case waitState.TimedOut:
		// Handle timeout scenario
		response.State.Status = core.StatusFailed
		response.State.Error = core.NewError(
			fmt.Errorf("wait task timed out after %s", timeout),
			"WAIT_TIMEOUT",
			map[string]any{
				"signal_name": taskConfig.WaitFor,
				"timeout":     taskConfig.Timeout,
			},
		)
		// Route to on_timeout or on_error
		if taskConfig.OnTimeout != "" {
			response.NextTask = e.loadNextTaskConfig(ctx, taskConfig.ID, taskConfig.OnTimeout)
		} else if taskConfig.OnError != nil && taskConfig.OnError.Next != nil {
			response.NextTask = e.loadNextTaskConfig(ctx, taskConfig.ID, *taskConfig.OnError.Next)
		}
	case waitState.MatchingSignal != nil:
		// Success - condition was met
		response.State.Status = core.StatusSuccess
		response.State.Output = &core.Output{
			"wait_status":      "completed",
			"signal":           waitState.MatchingSignal,
			"processor_output": waitState.ProcessorOutput,
		}
		// Update workflow status back to RUNNING only on success
		updateInput := &wfacts.UpdateStateInput{
			WorkflowID:     e.WorkflowID,
			WorkflowExecID: e.WorkflowExecID,
			Status:         core.StatusRunning,
		}
		err := workflow.ExecuteActivity(ctx, wfacts.UpdateStateLabel, updateInput).Get(ctx, nil)
		if err != nil {
			workflow.GetLogger(ctx).Error("Failed to update workflow status to running", "error", err)
			// Continue execution despite status update failure
		}
		// Route to on_success
		if taskConfig.OnSuccess != nil && taskConfig.OnSuccess.Next != nil {
			response.NextTask = e.loadNextTaskConfig(ctx, taskConfig.ID, *taskConfig.OnSuccess.Next)
		}
	}
	return response, nil
}

func (e *TaskWaitExecutor) processSignal(
	ctx workflow.Context,
	config *task.Config,
	signal *task.SignalEnvelope,
) (bool, *task.ProcessorOutput, error) {
	log := workflow.GetLogger(ctx)
	// Use the task context which includes proper activity options from config
	// This respects timeout settings from project/workflow/task config
	ctx = e.BuildTaskContext(ctx, config.ID)
	// If there's a processor, execute it first
	// NOTE: Processor execution is currently synchronous and blocks signal reception.
	// Processors should be designed to be short-lived to avoid blocking the wait loop.
	// Future improvement: Execute processors asynchronously or in child workflows
	// to maintain responsiveness to new signals and timer events.
	var processorOutput *task.ProcessorOutput
	if config.Processor != nil {
		log.Info("Executing wait task processor", "processor_id", config.Processor.ID)
		// Normalize processor templates with signal context via activity (proper deterministic approach)
		var processorConfig *task.Config
		normInput := &tkacts.NormalizeWaitProcessorInput{
			WorkflowID:      e.WorkflowID,
			WorkflowExecID:  e.WorkflowExecID,
			ProcessorConfig: config.Processor,
			Signal:          signal,
		}
		err := workflow.ExecuteActivity(ctx, tkacts.NormalizeWaitProcessorLabel, normInput).Get(ctx, &processorConfig)
		if err != nil {
			log.Error("Failed to normalize processor templates with signal context", "error", err)
			return false, nil, fmt.Errorf("failed to normalize processor: %w", err)
		}
		// Inject signal data into the processor's input for runtime access
		processorInput := core.Input{}
		if processorConfig.With != nil {
			// Copy existing normalized input to avoid mutation
			maps.Copy(processorInput, *processorConfig.With)
		}
		processorInput["signal"] = signal
		processorConfig.With = &processorInput
		// Execute the processor as a nested task using HandleExecution
		// This supports all task types (basic, router, parallel, etc.)
		processorResponse, err := e.ExecutionHandler(ctx, processorConfig, 1)
		if err != nil {
			log.Error("Processor execution failed", "error", err)
			// Return the error to fail the task immediately
			return false, nil, fmt.Errorf("processor failed: %w", err)
		}
		// Extract processor output from the response
		if mainTaskResponse, ok := processorResponse.(*task.MainTaskResponse); ok && mainTaskResponse.State != nil {
			processorOutput = &task.ProcessorOutput{
				Output: mainTaskResponse.State.Output,
				Error:  mainTaskResponse.State.Error,
			}
		}
	}
	// Evaluate condition using CEL via activity
	evalInput := &tkacts.EvaluateConditionInput{
		Expression:      config.Condition,
		Signal:          signal,
		ProcessorOutput: processorOutput,
	}
	var conditionMet bool
	err := workflow.ExecuteActivity(ctx, tkacts.EvaluateConditionLabel, evalInput).Get(ctx, &conditionMet)
	if err != nil {
		log.Error("Condition evaluation failed", "error", err, "expression", config.Condition)
		// Return the error to fail the task immediately
		return false, processorOutput, fmt.Errorf("condition evaluation failed: %w", err)
	}
	return conditionMet, processorOutput, nil
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
