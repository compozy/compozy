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

		// GET /api/v0/workflows/:workflow_id/executions/children
		// List all children executions for a workflow
		workflowsGroup.GET("/:workflow_id/executions/children", listChildrenExecutionsByID)

		// GET /api/v0/workflows/:workflow_id/executions/tasks
		// List all children task executions for a workflow
		workflowsGroup.GET("/:workflow_id/executions/children/tasks", listTaskExecutionsByID)

		// GET /api/v0/workflows/:workflow_id/executions/agents
		// List all children agent executions for a workflow
		workflowsGroup.GET("/:workflow_id/executions/children/agents", listAgentExecutionsByID)

		// GET /api/v0/workflows/:workflow_id/executions/tools
		// List all children tool executions for a workflow
		workflowsGroup.GET("/:workflow_id/executions/children/tools", listToolExecutionsByID)
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

			// GET /api/v0/executions/workflows/:workflow_exec_id
			// Get workflow execution details
			workflowExecGroup.GET("/:workflow_exec_id", getExecution)

			// TODO: implement logs route
			// GET /api/v0/executions/workflows/:workflow_exec_id/logs
			// Get logs for a workflow execution

			// GET /api/v0/executions/workflows/:workflow_exec_id/executions
			// List all executions within a workflow execution
			workflowExecGroup.GET("/:workflow_exec_id/executions", listChildrenExecutions)

			// GET /api/v0/executions/workflows/:workflow_exec_id/executions/tasks
			// List task executions within a workflow execution
			workflowExecGroup.GET("/:workflow_exec_id/executions/tasks", listTaskExecutions)

			// GET /api/v0/executions/workflows/:workflow_exec_id/executions/agents
			// List agent executions within a workflow execution
			workflowExecGroup.GET("/:workflow_exec_id/executions/agents", listAgentExecutions)

			// GET /api/v0/executions/workflows/:workflow_exec_id/executions/tools
			// List tool executions within a workflow execution
			workflowExecGroup.GET("/:workflow_exec_id/executions/tools", listToolExecutions)
		}
	}
}
