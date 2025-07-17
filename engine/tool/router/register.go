package toolrouter

import (
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
)

func Register(apiBase *gin.RouterGroup, authManager *authmw.Manager, cfg *config.Config) {
	// Tool definition routes under workflows
	workflowsGroup := apiBase.Group("/workflows/:workflow_id")

	// Apply authentication middleware based on configuration
	if cfg.Server.Auth.Enabled {
		workflowsGroup.Use(authManager.Middleware())
		workflowsGroup.Use(authmw.WorkflowAuthMiddleware(authManager, cfg))
	}

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
