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

func BuildErrHandler(ctx workflow.Context, input WorkflowInput) func(err error) error {
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

		// For non-cancellation errors, update status to failed in a disconnected context
		// to ensure the status update happens even if workflow is being terminated
		logger.Info("Updating workflow status to Failed due to error", "error", err)
		cleanupCtx, _ := workflow.NewDisconnectedContext(ctx)
		cleanupCtx = workflow.WithActivityOptions(cleanupCtx, workflow.ActivityOptions{
			StartToCloseTimeout: 30 * time.Second,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 3,
			},
		})

		label := wfacts.UpdateStateLabel
		statusInput := &wfacts.UpdateStateInput{
			WorkflowID:     input.WorkflowID,
			WorkflowExecID: input.WorkflowExecID,
			Status:         core.StatusFailed,
			Error:          core.NewError(err, "workflow_execution_error", nil),
		}
		future := workflow.ExecuteActivity(cleanupCtx, label, statusInput)
		if updateErr := future.Get(cleanupCtx, nil); updateErr != nil {
			logger.Error("Failed to update workflow status to Failed", "error", updateErr)
		} else {
			logger.Info("Successfully updated workflow status to Failed")
		}
		return err
	}
}

// -----------------------------------------------------------------------------
// Sleep
// -----------------------------------------------------------------------------

func SleepWithPause(ctx workflow.Context, dur time.Duration) error {
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
		sel.Select(ctx)
		// Check again after select
		if ctx.Err() == workflow.ErrCanceled {
			logger.Info("Sleep interrupted by cancellation")
			return workflow.ErrCanceled
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// handleAct - Curry function for cancellation and pause checks
// -----------------------------------------------------------------------------

func handleAct[T any](ctx workflow.Context, errHandler func(err error) error, fn func() (T, error)) func() (T, error) {
	return func() (T, error) {
		var zero T
		if ctx.Err() == workflow.ErrCanceled {
			return zero, errHandler(workflow.ErrCanceled)
		}
		result, err := fn()
		if err != nil {
			if err == workflow.ErrCanceled || temporal.IsCanceledError(err) {
				return zero, err
			}
			return zero, errHandler(err)
		}
		return result, nil
	}
}

func cancelCleanup(ctx workflow.Context, input *WorkflowInput) {
	if ctx.Err() != workflow.ErrCanceled {
		return
	}
	logger.Info("Workflow canceled, performing cleanup...")
	cleanupCtx, _ := workflow.NewDisconnectedContext(ctx)
	cleanupCtx = workflow.WithActivityOptions(cleanupCtx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	})

	// Update workflow status to canceled
	statusInput := &wfacts.UpdateStateInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		Status:         core.StatusCanceled,
	}
	if err := workflow.ExecuteActivity(
		cleanupCtx,
		wfacts.UpdateStateLabel,
		statusInput,
	).Get(cleanupCtx, nil); err != nil {
		logger.Error("Failed to update workflow status to Canceled during cleanup", "error", err)
	} else {
		logger.Info("Successfully updated workflow status to Canceled")
	}
}
