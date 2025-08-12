package wfrouter

import (
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
)

// Register registers HTTP API routes under the provided apiBase RouterGroup.
// It adds endpoints for events, workflow definitions, and workflow executions,
// and conditionally attaches authentication middleware from authManager when
// server authentication is enabled in the runtime configuration.
func Register(apiBase *gin.RouterGroup, authManager *authmw.Manager) {
	cfg := config.Get()
	// Event routes (v1)
	apiBase.POST("/events", handleEvent)

	// Workflow definition routes
	workflowsGroup := apiBase.Group("/workflows")

	// Apply authentication middleware based on configuration
	if cfg.Server.Auth.Enabled {
		workflowsGroup.Use(authManager.Middleware())
		workflowsGroup.Use(authmw.WorkflowAuthMiddleware(authManager))
	}

	{
		// GET /api/v0/workflows
		// List all workflows
		workflowsGroup.GET("", listWorkflows)

		// GET /api/v0/workflows/:workflow_id
		// Get workflow definition
		workflowsGroup.GET("/:workflow_id", getWorkflowByID)

		// GET /api/v0/workflows/:workflow_id/executions
		// List all executions for a workflow
		workflowsGroup.GET("/:workflow_id/executions", listExecutionsByID)

		// POST /api/v0/workflows/:workflow_id/executions
		// Start a new workflow execution
		workflowsGroup.POST("/:workflow_id/executions", handleExecute)
	}

	// Global execution routes
	executionsGroup := apiBase.Group("/executions")

	// Apply authentication middleware to executions based on configuration
	if cfg.Server.Auth.Enabled {
		executionsGroup.Use(authManager.Middleware())
		executionsGroup.Use(authManager.RequireAuth())
	}

	{
		// Workflow execution routes
		workflowExecGroup := executionsGroup.Group("/workflows")
		{
			// GET /api/v0/executions/workflows
			// List all workflow executions
			workflowExecGroup.GET("", listAllExecutions)

			// GET /api/v0/executions/workflows/:exec_id
			// Get workflow execution details
			workflowExecGroup.GET("/:exec_id", getExecution)

			// POST /api/v0/executions/workflows/:exec_id/pause
			// Pause workflow execution
			workflowExecGroup.POST("/:exec_id/pause", pauseExecution)

			// POST /api/v0/executions/workflows/:exec_id/resume
			// Resume workflow execution
			workflowExecGroup.POST("/:exec_id/resume", resumeExecution)

			// POST /api/v0/executions/workflows/:exec_id/cancel
			// Cancel workflow execution
			workflowExecGroup.POST("/:exec_id/cancel", cancelExecution)

			// POST /api/v0/executions/workflows/:exec_id/signals
			// Send signal to workflow execution
			workflowExecGroup.POST("/:exec_id/signals", sendSignalToExecution)
		}
	}
}
