package wfrouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
	// Workflow definition routes
	workflowsGroup := apiBase.Group("/workflows")
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

			// TODO: implement logs route
			// GET /api/v0/executions/workflows/:exec_id/logs
			// Get logs for a workflow execution
		}
	}
}
