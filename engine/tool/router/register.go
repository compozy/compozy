package toolrouter

import (
	"github.com/gin-gonic/gin"
)

func Register(apiBase *gin.RouterGroup) {
	toolsGroup := apiBase.Group("/tools")
	{
		// POST /api/v0/tools/export
		// Export tools to YAML
		toolsGroup.POST("/export", exportTools)

		// POST /api/v0/tools/import
		// Import tools from YAML
		toolsGroup.POST("/import", importTools)

		toolsGroup.GET("", listToolsTop)
		toolsGroup.GET("/:tool_id", getToolTop)
		toolsGroup.PUT("/:tool_id", upsertToolTop)
		toolsGroup.DELETE("/:tool_id", deleteToolTop)
	}
	// Tool definition routes under workflows
	workflowsGroup := apiBase.Group("/workflows/:workflow_id")
	{
		toolsGroup := workflowsGroup.Group("/tools")
		{
			// GET /api/v0/workflows/:workflow_id/tools
			// List all tools for a workflow
			toolsGroup.GET("", listTools)

			// GET /api/v0/workflows/:workflow_id/tools/:tool_id
			// Get tool definition
			toolsGroup.GET("/:tool_id", getToolByID)
		}
	}
}
