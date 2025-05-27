package toolrouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
	// Tool definition routes
	toolsGroup := apiBase.Group("/tools")
	{
		_ = toolsGroup // TODO: implement tool routes
		// TODO: implement tool definition routes
		// GET /api/v0/tools
		// List all tools

		// GET /api/v0/tools/:tool_id
		// Get tool definition

		// GET /api/v0/tools/:tool_id/executions
		// List executions for a tool
	}

	// Global execution routes
	executionsGroup := apiBase.Group("/executions")
	{
		// Tool execution routes
		toolExecGroup := executionsGroup.Group("/tools")
		{
			_ = toolExecGroup // TODO: implement tool execution routes
			// TODO: implement tool execution routes
			// GET /api/v0/executions/tools
			// List all tool executions

			// GET /api/v0/executions/tools/:tool_exec_id
			// Get tool execution details

			// GET /api/v0/executions/tools/:tool_exec_id/logs
			// Get logs for a tool execution
		}
	}
}
