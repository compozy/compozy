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
	cancelChan := workflow.GetSignalChannel(ctx, SignalCancel)

	// Create PauseGate first (it handles pause/resume signals)
	pauseGate, err := NewPauseGate(ctx, input)
	if err != nil {
		return nil, err
	}

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	// Handle only CANCEL signal here (pause/resume handled by PauseGate)
	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			selector := workflow.NewSelector(ctx)
			selector.AddReceive(cancelChan, func(c workflow.ReceiveChannel, _ bool) {
				handleCancelSignal(ctx, c, logger, cancelFunc, input)
			})
			selector.Select(ctx)
			if ctx.Err() == workflow.ErrCanceled {
				logger.Info("Signal handling goroutine exiting due to cancellation")
				return
			}
		}
	})

	return pauseGate, nil
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
	} else {
		logger.Info("UpdateStateActivity to Canceled completed.")
	}

	cancelFunc()
	logger.Info("Workflow context canceled.")
}
