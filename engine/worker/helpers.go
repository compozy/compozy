package worker

import (
	"time"

	"github.com/compozy/compozy/engine/core"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
	"github.com/compozy/compozy/pkg/logger"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// -----------------------------------------------------------------------------
// Error Handler
// -----------------------------------------------------------------------------

func BuildErrorHandler(ctx workflow.Context, input *WorkflowInput) func(err error) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})
	return func(err error) error {
		if temporal.IsCanceledError(err) || err == workflow.ErrCanceled {
			logger.Info("Workflow canceled")
			return err
		}
		logger.Info("Updating workflow status to Failed due to error", "error", err)
		label := wfacts.UpdateStateLabel
		statusInput := &wfacts.UpdateStateInput{
			WorkflowID:     input.WorkflowID,
			WorkflowExecID: input.WorkflowExecID,
			Status:         core.StatusFailed,
			Error:          core.NewError(err, "workflow_execution_error", nil),
		}
		future := workflow.ExecuteActivity(ctx, label, statusInput)
		if updateErr := future.Get(ctx, nil); updateErr != nil {
			if temporal.IsCanceledError(updateErr) {
				logger.Info("Status update canceled")
			} else {
				logger.Error("Failed to update workflow status to Failed", "error", updateErr)
			}
		} else {
			logger.Info("Successfully updated workflow status to Failed")
		}
		return err
	}
}

// -----------------------------------------------------------------------------
// Sleep
// -----------------------------------------------------------------------------

func SleepWithPause(ctx workflow.Context, dur time.Duration, g *PauseGate) error {
	// Check if context is already canceled
	if ctx.Err() == workflow.ErrCanceled {
		logger.Info("Sleep skipped due to cancellation")
		return workflow.ErrCanceled
	}

	timerDone := false
	timer := workflow.NewTimer(ctx, dur)

	for !timerDone {
		// Check cancellation before each iteration
		if ctx.Err() == workflow.ErrCanceled {
			logger.Info("Sleep interrupted by cancellation")
			return workflow.ErrCanceled
		}

		sel := workflow.NewSelector(ctx)
		sel.AddFuture(timer, func(workflow.Future) { timerDone = true })
		sel.AddReceive(g.pause, func(workflow.ReceiveChannel, bool) {
			if ctx.Err() != workflow.ErrCanceled {
				g.paused = true
			}
		})
		sel.AddReceive(g.resume, func(workflow.ReceiveChannel, bool) {
			if ctx.Err() != workflow.ErrCanceled {
				g.paused = false
			}
		})
		sel.Select(ctx)

		// Check again after select
		if ctx.Err() == workflow.ErrCanceled {
			logger.Info("Sleep interrupted by cancellation")
			return workflow.ErrCanceled
		}

		if g.paused {
			if err := g.Await(); err != nil { // propagates cancel too
				return err
			}
		}
	}
	return nil
}

func checkCancellation(ctx workflow.Context, err error, msg string) error {
	logger := workflow.GetLogger(ctx)
	if err == workflow.ErrCanceled || temporal.IsCanceledError(err) {
		logger.Info(msg)
		return err
	}
	return nil
}
