package toolrouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
	// Tool definition routes
	toolsGroup := apiBase.Group("/tools")
	{
		// GET /api/v0/tools
		// List all tools
		toolsGroup.GET("", listTools)

		// GET /api/v0/tools/:tool_id
		// Get tool definition
		toolsGroup.GET("/:tool_id", getToolByID)

		// GET /api/v0/tools/:tool_id/executions
		// List executions for a tool
		toolsGroup.GET("/:tool_id/executions", listExecutionsByToolID)
	}

	// Global execution routes
	executionsGroup := apiBase.Group("/executions")
	{
		// Tool execution routes
		toolExecGroup := executionsGroup.Group("/tools")
		{
			// GET /api/v0/executions/tools
			// List all tool executions
			toolExecGroup.GET("", listAllToolExecutions)

			// GET /api/v0/executions/tools/:tool_exec_id
			// Get tool execution details
			toolExecGroup.GET("/:tool_exec_id", getToolExecution)
		}
	}
}
