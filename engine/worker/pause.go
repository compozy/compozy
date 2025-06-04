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
// Pause Gate
// -----------------------------------------------------------------------------

// PauseGate blocks workflow progress whenever a PAUSE signal is received.
type PauseGate struct {
	paused bool
	pause  workflow.ReceiveChannel
	resume workflow.ReceiveChannel
	await  func() error
	input  *WorkflowInput
	logger log.Logger
}

// NewPauseGate installs signal listeners + query handler and returns a gate.
func NewPauseGate(ctx workflow.Context, input *WorkflowInput) (*PauseGate, error) {
	logger := workflow.GetLogger(ctx)
	g := &PauseGate{
		pause:  workflow.GetSignalChannel(ctx, SignalPause),
		resume: workflow.GetSignalChannel(ctx, SignalResume),
		input:  input,
		logger: logger,
	}
	g.await = func() error {
		// Check if context is already canceled
		if ctx.Err() == workflow.ErrCanceled {
			return workflow.ErrCanceled
		}
		return workflow.Await(ctx, func() bool {
			// Don't block if context is canceled
			if ctx.Err() == workflow.ErrCanceled {
				return true
			}
			return !g.paused
		})
	}

	// Setup activity options for status updates
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	// Handle pause/resume signals and update workflow status
	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			// Check if context is canceled before creating selector
			if ctx.Err() == workflow.ErrCanceled {
				return
			}

			sel := workflow.NewSelector(ctx)
			sel.AddReceive(g.pause, func(workflow.ReceiveChannel, bool) {
				if ctx.Err() != workflow.ErrCanceled {
					g.handlePause(activityCtx)
				}
			})
			sel.AddReceive(g.resume, func(workflow.ReceiveChannel, bool) {
				if ctx.Err() != workflow.ErrCanceled {
					g.handleResume(activityCtx)
				}
			})

			// Use a non-blocking select if context is canceled
			if ctx.Err() == workflow.ErrCanceled {
				return
			}

			sel.Select(ctx)

			// Check again after select
			if ctx.Err() == workflow.ErrCanceled {
				return
			}
		}
	})

	// expose live state for operators
	if err := workflow.SetQueryHandler(ctx, "state", func() (string, error) {
		if g.paused {
			return "paused", nil
		}
		return "running", nil
	}); err != nil {
		return nil, err
	}
	return g, nil
}

func (g *PauseGate) handlePause(ctx workflow.Context) {
	g.paused = true
	g.logger.Info("Workflow pause signal received. Updating status to Paused.")

	// Don't try to update status if context is already canceled
	if ctx.Err() == workflow.ErrCanceled {
		g.logger.Info("Skipping pause status update due to cancellation")
		return
	}

	statusInput := &wfacts.UpdateStateInput{
		WorkflowID:     g.input.WorkflowID,
		WorkflowExecID: g.input.WorkflowExecID,
		Status:         core.StatusPaused,
	}

	// Use a detached context to avoid blocking issues
	future := workflow.ExecuteActivity(ctx, wfacts.UpdateStateLabel, statusInput)

	// Use a selector to make this non-blocking if context gets canceled
	selector := workflow.NewSelector(ctx)
	var updateError error

	selector.AddFuture(future, func(f workflow.Future) {
		updateError = f.Get(ctx, nil)
	})

	// Don't block if context is canceled
	if ctx.Err() != workflow.ErrCanceled {
		selector.Select(ctx)
	}

	if updateError != nil {
		if temporal.IsCanceledError(updateError) {
			g.logger.Info("Pause status update canceled")
		} else {
			g.logger.Error("Failed to update workflow status to Paused", "error", updateError)
		}
	} else if ctx.Err() != workflow.ErrCanceled {
		g.logger.Info("Workflow status updated to Paused successfully.")
	}
}

func (g *PauseGate) handleResume(ctx workflow.Context) {
	g.paused = false
	g.logger.Info("Workflow resume signal received. Updating status to Running.")

	// Don't try to update status if context is already canceled
	if ctx.Err() == workflow.ErrCanceled {
		g.logger.Info("Skipping resume status update due to cancellation")
		return
	}

	statusInput := &wfacts.UpdateStateInput{
		WorkflowID:     g.input.WorkflowID,
		WorkflowExecID: g.input.WorkflowExecID,
		Status:         core.StatusRunning,
	}

	// Use a detached context to avoid blocking issues
	future := workflow.ExecuteActivity(ctx, wfacts.UpdateStateLabel, statusInput)

	// Use a selector to make this non-blocking if context gets canceled
	selector := workflow.NewSelector(ctx)
	var updateError error

	selector.AddFuture(future, func(f workflow.Future) {
		updateError = f.Get(ctx, nil)
	})

	// Don't block if context is canceled
	if ctx.Err() != workflow.ErrCanceled {
		selector.Select(ctx)
	}

	if updateError != nil {
		if temporal.IsCanceledError(updateError) {
			g.logger.Info("Resume status update canceled")
		} else {
			g.logger.Error("Failed to update workflow status to Running", "error", updateError)
		}
	} else if ctx.Err() != workflow.ErrCanceled {
		g.logger.Info("Workflow status updated to Running successfully.")
	}
}

func (g *PauseGate) Await() error {
	return g.await()
}

// IsPaused returns whether the workflow is currently paused
func (g *PauseGate) IsPaused() bool {
	return g.paused
}
