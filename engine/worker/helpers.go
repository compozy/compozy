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
		if temporal.IsCanceledError(err) {
			logger.Info("Workflow canceled during sleep")
			return nil
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
		if err := future.Get(ctx, nil); err != nil {
			logger.Error("Failed to update workflow status to Failed", "error", err)
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
	timerDone := false
	timer := workflow.NewTimer(ctx, dur)

	for !timerDone {
		sel := workflow.NewSelector(ctx)
		sel.AddFuture(timer, func(workflow.Future) { timerDone = true })
		sel.AddReceive(g.pause, func(workflow.ReceiveChannel, bool) { g.paused = true })
		sel.AddReceive(g.resume, func(workflow.ReceiveChannel, bool) { g.paused = false })
		sel.Select(ctx)

		// NEW: abort immediately on cancellation
		if ctx.Err() != workflow.ErrCanceled {
			logger.Info("Sleep interrupted by cancellation")
			return nil
		}

		if g.paused {
			if err := g.Await(); err != nil { // propagates cancel too
				return err
			}
		}
	}
	return nil
}
