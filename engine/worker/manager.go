package worker

import (
	"context"
	"os"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	wf "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
	"github.com/compozy/compozy/pkg/logger"
)

// -----------------------------------------------------------------------------
// Workflow Orchestrator
// -----------------------------------------------------------------------------

type Manager struct {
	*ContextBuilder
	*WorkflowExecutor
	*TaskExecutor
}

func NewManager(contextBuilder *ContextBuilder) *Manager {
	workflowExecutor := NewWorkflowExecutor(contextBuilder)
	taskExecutor := NewTaskExecutor(contextBuilder)
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
		logger := workflow.GetLogger(ctx)
		if temporal.IsCanceledError(err) || err == workflow.ErrCanceled {
			logger.Info("Workflow canceled")
			return err
		}

		// For non-cancellation errors, update status to failed in a disconnected context
		// to ensure the status update happens even if workflow is being terminated
		logger.Info("Updating workflow status to Failed due to error", "error", err)
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
			logger.Error("Failed to update workflow status to Failed", "error", updateErr)
		} else {
			logger.Info("Successfully updated workflow status to Failed")
		}
		return err
	}
}

// CancelCleanup - Cleanup function for canceled workflows
func (m *Manager) CancelCleanup(ctx workflow.Context) {
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
		WorkflowID:     m.WorkflowID,
		WorkflowExecID: m.WorkflowExecID,
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

// -----------------------------------------------------------------------------
// MCP Registration
// -----------------------------------------------------------------------------

// initMCPRegistrar initializes MCP registration with proxy if any MCPs use proxy mode
func initMCPRegistrar(ctx workflow.Context, workflowConfig *wf.Config) {
	logger := workflow.GetLogger(ctx)

	if workflowConfig == nil || len(workflowConfig.MCPs) == 0 {
		return
	}

	// Check if any MCPs use proxy mode
	var proxyMCPs []mcp.Config
	for _, mcpConfig := range workflowConfig.MCPs {
		mcpConfig.SetDefaults() // Ensure defaults are applied
		if mcpConfig.UseProxy {
			proxyMCPs = append(proxyMCPs, mcpConfig)
		}
	}

	if len(proxyMCPs) == 0 {
		return // No proxy MCPs found
	}

	logger.Info("Initializing MCP registrar for proxy mode", "proxy_mcps_count", len(proxyMCPs))

	// Get proxy configuration from environment or first proxy MCP
	proxyURL := os.Getenv("MCP_PROXY_URL")
	adminToken := os.Getenv("MCP_PROXY_ADMIN_TOKEN")

	if proxyURL == "" && len(proxyMCPs) > 0 {
		proxyURL = proxyMCPs[0].ProxyURL
	}

	if proxyURL == "" {
		logger.Error("No proxy URL configured for MCP proxy mode")
		return // Don't fail, just skip proxy registration
	}

	// Create registrar service with a reasonable timeout
	registrar := mcp.NewWithTimeout(proxyURL, adminToken, 30*time.Second)

	// Ensure all proxy MCPs are registered
	// Note: This runs in background context to avoid cancellation during workflow execution
	bgCtx := context.Background()
	if err := registrar.EnsureMultiple(bgCtx, proxyMCPs); err != nil {
		logger.Error("Failed to register some MCPs with proxy", "error", err)
		// Don't return error, just log it
	} else {
		logger.Info("Successfully registered MCPs with proxy", "count", len(proxyMCPs))
	}
}

// -----------------------------------------------------------------------------
// Manager Factory
// -----------------------------------------------------------------------------

func InitManager(ctx workflow.Context, input WorkflowInput) (*Manager, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting workflow", "workflow_id", input.WorkflowID, "exec_id", input.WorkflowExecID)
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

	// Initialize MCP registrar if proxy mode is enabled
	initMCPRegistrar(ctx, data.WorkflowConfig)

	contextBuilder := NewContextBuilder(
		data.Workflows,
		data.ProjectConfig,
		data.WorkflowConfig,
		&input,
	)
	return NewManager(contextBuilder), nil
}
