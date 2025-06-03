package temporal

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

func RegisterSignalHandlers(ctx workflow.Context, wfInput WorkflowInput) {
	logger := workflow.GetLogger(ctx)
	pauseChan := workflow.GetSignalChannel(ctx, SignalPause)
	resumeChan := workflow.GetSignalChannel(ctx, SignalResume)
	cancelChan := workflow.GetSignalChannel(ctx, SignalCancel)
	statusUpdateAo := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	activityCtx := workflow.WithActivityOptions(ctx, statusUpdateAo)
	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			selector := workflow.NewSelector(ctx)
			selector.AddReceive(pauseChan, func(c workflow.ReceiveChannel, _ bool) {
				handlePauseSignal(ctx, c, wfInput, activityCtx, logger)
			})
			selector.AddReceive(resumeChan, func(c workflow.ReceiveChannel, _ bool) {
				handleResumeSignal(ctx, c, wfInput, activityCtx, logger)
			})
			selector.AddReceive(cancelChan, func(c workflow.ReceiveChannel, _ bool) {
				handleCancelSignal(ctx, c, wfInput, activityCtx, logger)
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
}

func handlePauseSignal(
	ctx workflow.Context,
	c workflow.ReceiveChannel,
	wfInput WorkflowInput,
	activityCtx workflow.Context,
	logger log.Logger,
) {
	var signal any
	c.Receive(ctx, &signal)
	logger.Info("Workflow pause signal received. Initiating status update.")
	statusInput := &wfacts.UpdateWorkflowStatusInput{
		WorkflowID:     wfInput.WorkflowID,
		WorkflowExecID: wfInput.WorkflowExecID,
		NewStatus:      core.StatusPaused,
	}
	future := workflow.ExecuteActivity(activityCtx, wfacts.UpdateWorkflowStatusLabel, statusInput)
	_ = future
	logger.Info("UpdateWorkflowStatusActivity to Paused initiated.")
}

func handleResumeSignal(
	ctx workflow.Context,
	c workflow.ReceiveChannel,
	wfInput WorkflowInput,
	activityCtx workflow.Context,
	logger log.Logger,
) {
	var signal any
	c.Receive(ctx, &signal)
	logger.Info("Workflow resume signal received. Initiating status update.")
	statusInput := &wfacts.UpdateWorkflowStatusInput{
		WorkflowID:     wfInput.WorkflowID,
		WorkflowExecID: wfInput.WorkflowExecID,
		NewStatus:      core.StatusRunning,
	}
	future := workflow.ExecuteActivity(activityCtx, wfacts.UpdateWorkflowStatusLabel, statusInput)
	_ = future
	logger.Info("UpdateWorkflowStatusActivity to Running initiated.")
}

func handleCancelSignal(
	ctx workflow.Context,
	c workflow.ReceiveChannel,
	wfInput WorkflowInput,
	activityCtx workflow.Context,
	logger log.Logger,
) {
	var signal any
	c.Receive(ctx, &signal)
	logger.Info("Workflow cancel signal received. Setting manual cancel flag and initiating status update.")
	statusInput := &wfacts.UpdateWorkflowStatusInput{
		WorkflowID:     wfInput.WorkflowID,
		WorkflowExecID: wfInput.WorkflowExecID,
		NewStatus:      core.StatusCanceled,
	}
	future := workflow.ExecuteActivity(activityCtx, wfacts.UpdateWorkflowStatusLabel, statusInput)
	_ = future
	logger.Info("UpdateWorkflowStatusActivity to Canceled initiated.")
}
