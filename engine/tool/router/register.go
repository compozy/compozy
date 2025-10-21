package toolrouter

import (
	"github.com/gin-gonic/gin"
)

func Register(apiBase *gin.RouterGroup) {
	toolsGroup := apiBase.Group("/tools")
	{
		toolsGroup.POST("/export", exportTools)
		toolsGroup.POST("/import", importTools)
		toolsGroup.GET("", listToolsTop)
		toolsGroup.GET("/:tool_id", getToolTop)
		toolsGroup.PUT("/:tool_id", upsertToolTop)
		toolsGroup.DELETE("/:tool_id", deleteToolTop)
	}
	workflowsGroup := apiBase.Group("/workflows/:workflow_id")
	{
		toolsGroup := workflowsGroup.Group("/tools")
		{
			toolsGroup.GET("", listTools)

			toolsGroup.GET("/:tool_id", getToolByID)
		}
	}
}
