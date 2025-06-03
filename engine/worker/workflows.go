package worker

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	wf "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
	"github.com/compozy/compozy/pkg/logger"
)

type WorkflowInput struct {
	WorkflowID     string
	WorkflowExecID core.ID
	Input          *core.Input
}

// -----------------------------------------------------------------------------
// Workflow Definition
// -----------------------------------------------------------------------------

func CompozyWorkflow(ctx workflow.Context, input WorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting workflow", "workflow_id", input.WorkflowID, "exec_id", input.WorkflowExecID)
	ctx = initialContext(ctx)

	// Execute main trigger activity
	logger.Info("Executing main trigger activity...")
	var wfState *wf.State
	triggerLabel := wfacts.TriggerLabel
	triggerInput := &wfacts.TriggerInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		Input:          input.Input,
	}
	err := workflow.ExecuteActivity(ctx, triggerLabel, triggerInput).Get(ctx, &wfState)
	if err != nil {
		logger.Error("Failed to execute trigger activity", "error", err)
		return err
	}

	// Setup signals for PAUSE/RESUME/CANCEL
	ctx, cancel := workflow.WithCancel(ctx)
	errorHandler := buildErrorHandler(ctx, &input)
	pauseGate, err := RegisterSignals(ctx, cancel, &input)
	if err != nil {
		errorHandler(err)
		return err
	}

	if err := sleepWithPause(ctx, 5*time.Minute, pauseGate); err != nil {
		errorHandler(err)
		return err
	}

	logger.Info("Workflow completed",
		"workflow_id", input.WorkflowID,
		"exec_id", input.WorkflowExecID,
	)
	return nil
}

// -----------------------------------------------------------------------------
// Context
// -----------------------------------------------------------------------------

func initialContext(ctx workflow.Context) workflow.Context {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	})
	return ctx
}

// -----------------------------------------------------------------------------
// Error Handler
// -----------------------------------------------------------------------------

func buildErrorHandler(ctx workflow.Context, input *WorkflowInput) func(err error) error {
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
		logger.Info("Updating workflow status to Failed due to error")
		label := wfacts.UpdateStateLabel
		stateID := wf.NewStateID(input.WorkflowID, input.WorkflowExecID)
		statusInput := &wfacts.UpdateStateInput{
			StateID: stateID,
			Status:  core.StatusFailed,
			Error:   core.NewError(err, "workflow_execution_error", nil),
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

func sleepWithPause(ctx workflow.Context, dur time.Duration, g *PauseGate) error {
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
