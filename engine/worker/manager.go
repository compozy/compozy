package worker

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/worker/executors"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
)

// -----------------------------------------------------------------------------
// Workflow Orchestrator
// -----------------------------------------------------------------------------

type Manager struct {
	*ContextBuilder
	*executors.WorkflowExecutor
	*executors.TaskExecutor
}

func NewManager(contextBuilder *ContextBuilder) *Manager {
	// Convert to executors.ContextBuilder
	executorContextBuilder := executors.NewContextBuilder(
		contextBuilder.Workflows,
		contextBuilder.ProjectConfig,
		contextBuilder.WorkflowConfig,
		contextBuilder.WorkflowInput,
	)

	workflowExecutor := executors.NewWorkflowExecutor(executorContextBuilder)
	taskExecutor := executors.NewTaskExecutor(executorContextBuilder)

	return &Manager{
		ContextBuilder:   contextBuilder,
		WorkflowExecutor: workflowExecutor,
		TaskExecutor:     taskExecutor,
	}
}

func (m *Manager) BuildErrHandler(ctx workflow.Context) func(err error) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})
	return func(err error) error {
		log := workflow.GetLogger(ctx)
		if temporal.IsCanceledError(err) || err == workflow.ErrCanceled {
			log.Info("Workflow canceled")
			return err
		}

		// For non-cancellation errors, update status to failed in a disconnected context
		// to ensure the status update happens even if workflow is being terminated
		log.Info("Updating workflow status to Failed due to error", "error", err)
		cleanupCtx, _ := workflow.NewDisconnectedContext(ctx)
		label := wfacts.UpdateStateLabel
		statusInput := &wfacts.UpdateStateInput{
			WorkflowID:     m.WorkflowID,
			WorkflowExecID: m.WorkflowExecID,
			Status:         core.StatusFailed,
			Error:          core.NewError(err, "workflow_execution_error", nil),
		}

		if updateErr := workflow.ExecuteActivity(
			cleanupCtx,
			label,
			statusInput,
		).Get(cleanupCtx, nil); updateErr != nil {
			log.Error("Failed to update workflow status to Failed", "error", updateErr)
		} else {
			log.Debug("Successfully updated workflow status to Failed")
		}
		return err
	}
}

// CancelCleanup - Cleanup function for canceled workflows
func (m *Manager) CancelCleanup(ctx workflow.Context) {
	if ctx.Err() != workflow.ErrCanceled {
		return
	}
	log := workflow.GetLogger(ctx)
	log.Info("Workflow canceled, performing cleanup...")
	cleanupCtx, _ := workflow.NewDisconnectedContext(ctx)
	cleanupCtx = workflow.WithActivityOptions(cleanupCtx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	})

	// Update workflow status to canceled
	statusInput := &wfacts.UpdateStateInput{
		WorkflowID:     m.WorkflowID,
		WorkflowExecID: m.WorkflowExecID,
		Status:         core.StatusCanceled,
	}
	if err := workflow.ExecuteActivity(
		cleanupCtx,
		wfacts.UpdateStateLabel,
		statusInput,
	).Get(cleanupCtx, nil); err != nil {
		log.Error("Failed to update workflow status to Canceled during cleanup", "error", err)
	} else {
		log.Debug("Successfully updated workflow status to Canceled")
	}
}

// -----------------------------------------------------------------------------
// Manager Factory
// -----------------------------------------------------------------------------

func InitManager(ctx workflow.Context, input WorkflowInput) (*Manager, error) {
	log := workflow.GetLogger(ctx)
	log.Info("Starting workflow", "workflow_id", input.WorkflowID, "exec_id", input.WorkflowExecID)
	ctx = workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	})
	actLabel := wfacts.GetDataLabel
	actInput := &wfacts.GetDataInput{WorkflowID: input.WorkflowID}
	var data *wfacts.GetData
	err := workflow.ExecuteLocalActivity(ctx, actLabel, actInput).Get(ctx, &data)
	if err != nil {
		return nil, err
	}

	// MCP servers are initialized at server startup, not during workflow execution
	contextBuilder := NewContextBuilder(
		data.Workflows,
		data.ProjectConfig,
		data.WorkflowConfig,
		&input,
	)
	return NewManager(contextBuilder), nil
}
