package temporal

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

type WorkflowInput = wfacts.TriggerInput

// -----------------------------------------------------------------------------
// Workflow Definition
// -----------------------------------------------------------------------------

func CompozyWorkflow(ctx workflow.Context, input WorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting workflow", "workflow_id", input.WorkflowID, "exec_id", input.WorkflowExecID)
	RegisterSignalHandlers(ctx, input)

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	})

	logger.Info("Executing main trigger activity...")
	var executionResult *workflow.Execution
	err := workflow.ExecuteActivity(
		ctx,
		wfacts.TriggerLabel,
		input,
	).Get(ctx, &executionResult)
	if err != nil {
		logger.Error("Failed to execute trigger activity", "error", err)
		return err
	}

	logger.Info("Workflow logic implies completion.", "workflow_id", input.WorkflowID, "exec_id", input.WorkflowExecID)
	return nil
}
