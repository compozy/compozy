package worker

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	tkacts "github.com/compozy/compozy/engine/task/activities"
	wf "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

type WorkflowInput = wfacts.TriggerInput

// -----------------------------------------------------------------------------
// Workflow Definition
// -----------------------------------------------------------------------------

func CompozyWorkflow(ctx workflow.Context, input WorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting workflow", "workflow_id", input.WorkflowID, "exec_id", input.WorkflowExecID)
	ctx = initialContext(ctx)

	// Execute main trigger activity
	logger.Info("Executing main trigger activity...")
	wState, err := triggerWorkflow(ctx, &input)
	if err != nil {
		logger.Error("Failed to execute trigger activity", "error", err)
		return err
	}

	// Setup signals for PAUSE/RESUME/CANCEL
	ctx, cancel := workflow.WithCancel(ctx)
	errorHandler := BuildErrorHandler(ctx, &input)
	pauseGate, err := RegisterSignals(ctx, cancel, &input)
	if err != nil {
		return errorHandler(err)
	}

	_, err = dispatchFirstTask(ctx, pauseGate, wState, &input)
	if err != nil {
		return errorHandler(err)
	}

	err = completeWorkflow(ctx, pauseGate, wState)
	if err != nil {
		return errorHandler(err)
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
// Activities
// -----------------------------------------------------------------------------

func triggerWorkflow(
	ctx workflow.Context,
	input *wfacts.TriggerInput,
) (*wf.State, error) {
	var state *wf.State
	actLabel := wfacts.TriggerLabel
	actInput := &wfacts.TriggerInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		Input:          input.Input,
		InitialTaskID:  input.InitialTaskID,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func dispatchFirstTask(
	ctx workflow.Context,
	pauseGate *PauseGate,
	wState *wf.State,
	input *wfacts.TriggerInput,
) (*task.State, error) {
	if err := pauseGate.Await(); err != nil {
		return nil, err
	}
	var state *task.State
	actLabel := tkacts.DispatchLabel
	actInput := &tkacts.DispatchInput{
		WorkflowID:     wState.WorkflowID,
		WorkflowExecID: wState.WorkflowExecID,
		TaskID:         input.InitialTaskID,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, &state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func completeWorkflow(
	ctx workflow.Context,
	pauseGate *PauseGate,
	wState *wf.State,
) error {
	if err := pauseGate.Await(); err != nil {
		return err
	}
	actLabel := wfacts.UpdateStateLabel
	actInput := &wfacts.UpdateStateInput{
		WorkflowID:     wState.WorkflowID,
		WorkflowExecID: wState.WorkflowExecID,
		Status:         core.StatusSuccess,
	}
	err := workflow.ExecuteActivity(ctx, actLabel, actInput).Get(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}
