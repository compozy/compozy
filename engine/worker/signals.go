package worker

import (
	"time"

	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

// -----------------------------------------------------------------------------
// Signal Constants
// -----------------------------------------------------------------------------

const (
	SignalPause  = "WORKFLOW:PAUSE"
	SignalResume = "WORKFLOW:RESUME"
	SignalCancel = "WORKFLOW:CANCEL"
)

// -----------------------------------------------------------------------------
// Signal Handlers
// -----------------------------------------------------------------------------

func RegisterSignals(
	ctx workflow.Context,
	cancelFunc workflow.CancelFunc,
	input *WorkflowInput,
) (*PauseGate, error) {
	logger := workflow.GetLogger(ctx)
	pauseChan := workflow.GetSignalChannel(ctx, SignalPause)
	resumeChan := workflow.GetSignalChannel(ctx, SignalResume)
	cancelChan := workflow.GetSignalChannel(ctx, SignalCancel)
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})
	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			selector := workflow.NewSelector(ctx)
			selector.AddReceive(pauseChan, func(c workflow.ReceiveChannel, _ bool) {
				handlePauseSignal(ctx, c, logger, input)
			})
			selector.AddReceive(resumeChan, func(c workflow.ReceiveChannel, _ bool) {
				handleResumeSignal(ctx, c, logger, input)
			})
			selector.AddReceive(cancelChan, func(c workflow.ReceiveChannel, _ bool) {
				handleCancelSignal(ctx, c, logger, cancelFunc, input)
			})
			selector.Select(ctx)
			if ctx.Err() != nil {
				logger.Info("Signal handling goroutine exiting", "reason", ctx.Err())
				if ctx.Err() != workflow.ErrCanceled {
					logger.Info("Goroutine sees manual cancel; main context might not be canceled yet.")
				}
				return
			}
		}
	})
	return NewPauseGate(ctx)
}

func handlePauseSignal(
	ctx workflow.Context,
	channel workflow.ReceiveChannel,
	logger log.Logger,
	input *WorkflowInput,
) {
	var signal any
	channel.Receive(ctx, &signal)
	logger.Info("Workflow pause signal received. Initiating status update.")
	statusInput := &wfacts.UpdateStateInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		Status:         core.StatusPaused,
	}
	label := wfacts.UpdateStateLabel
	future := workflow.ExecuteActivity(ctx, label, statusInput)
	_ = future
	logger.Info("UpdateStateActivity to Paused initiated.")
}

func handleResumeSignal(
	ctx workflow.Context,
	channel workflow.ReceiveChannel,
	logger log.Logger,
	input *WorkflowInput,
) {
	var signal any
	channel.Receive(ctx, &signal)
	logger.Info("Workflow resume signal received. Initiating status update.")
	statusInput := &wfacts.UpdateStateInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		Status:         core.StatusRunning,
	}
	label := wfacts.UpdateStateLabel
	future := workflow.ExecuteActivity(ctx, label, statusInput)
	_ = future
	logger.Info("UpdateStateActivity to Running initiated.")
}

func handleCancelSignal(
	ctx workflow.Context,
	channel workflow.ReceiveChannel,
	logger log.Logger,
	cancelFunc workflow.CancelFunc,
	input *WorkflowInput,
) {
	var signal any
	channel.Receive(ctx, &signal)
	logger.Info("Workflow cancel signal received. Initiating status update and cancellation.")
	label := wfacts.UpdateStateLabel
	statusInput := &wfacts.UpdateStateInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		Status:         core.StatusCanceled,
	}
	future := workflow.ExecuteActivity(ctx, label, statusInput)
	if err := future.Get(ctx, nil); err != nil {
		logger.Error("Failed to update workflow status to Canceled", "error", err)
		// Optionally handle failure (e.g., retry or log for manual intervention)
	} else {
		logger.Info("UpdateStateActivity to Canceled completed.")
	}
	cancelFunc()
	logger.Info("Workflow context canceled.")
}
