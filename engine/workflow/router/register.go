package wfrouter

import (
	"github.com/gin-gonic/gin"
)

// Register registers HTTP API routes under the provided apiBase RouterGroup.
// It adds endpoints for events, workflow definitions, and workflow executions.
// Authentication is handled globally at the server level.
func Register(apiBase *gin.RouterGroup) {
	// Event routes (v0)
	apiBase.POST("/events", handleEvent)

	// Workflow definition routes
	workflowsGroup := apiBase.Group("/workflows")

	{
		// POST /workflows/export
		// Export workflows to YAML
		workflowsGroup.POST("/export", exportWorkflows)
		// POST /workflows/import
		// Import workflows from YAML
		workflowsGroup.POST("/import", importWorkflows)

		// GET /workflows
		// List all workflows
		workflowsGroup.GET("", listWorkflows)

		// GET /workflows/:workflow_id
		// Get workflow definition
		workflowsGroup.GET("/:workflow_id", getWorkflowByID)

		// PUT /workflows/:workflow_id
		// Create or update workflow definition
		workflowsGroup.PUT("/:workflow_id", upsertWorkflow)

		// DELETE /workflows/:workflow_id
		// Delete workflow definition
		workflowsGroup.DELETE("/:workflow_id", deleteWorkflow)

		// GET /workflows/:workflow_id/executions
		// List all executions for a workflow
		workflowsGroup.GET("/:workflow_id/executions", listExecutionsByID)

		// POST /workflows/:workflow_id/executions
		// Start a new workflow execution
		workflowsGroup.POST("/:workflow_id/executions", handleExecute)

		// POST /workflows/:workflow_id/executions/sync
		// Execute workflow synchronously
		workflowsGroup.POST("/:workflow_id/executions/sync", executeWorkflowSync)
	}

	// Global execution routes
	executionsGroup := apiBase.Group("/executions")
	{
		// Workflow execution routes
		workflowExecGroup := executionsGroup.Group("/workflows")
		{
			// GET /executions/workflows
			// List all workflow executions
			workflowExecGroup.GET("", listAllExecutions)

			// GET /executions/workflows/:exec_id
			// Get workflow execution details
			workflowExecGroup.GET("/:exec_id", getExecution)

			// POST /executions/workflows/:exec_id/pause
			// Pause workflow execution
			workflowExecGroup.POST("/:exec_id/pause", pauseExecution)

			// POST /executions/workflows/:exec_id/resume
			// Resume workflow execution
			workflowExecGroup.POST("/:exec_id/resume", resumeExecution)

			// POST /executions/workflows/:exec_id/cancel
			// Cancel workflow execution
			workflowExecGroup.POST("/:exec_id/cancel", cancelExecution)

			// POST /executions/workflows/:exec_id/signals
			// Send signal to workflow execution
			workflowExecGroup.POST("/:exec_id/signals", sendSignalToExecution)
		}
	}
}
