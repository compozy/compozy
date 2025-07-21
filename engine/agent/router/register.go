package agentrouter

import (
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
)

func Register(apiBase *gin.RouterGroup, authManager *authmw.Manager) {
	cfg := config.Get()
	// Agent definition routes under workflows
	workflowsGroup := apiBase.Group("/workflows/:workflow_id")

	// Apply authentication middleware based on configuration
	if cfg.Server.Auth.Enabled {
		workflowsGroup.Use(authManager.Middleware())
		workflowsGroup.Use(authmw.WorkflowAuthMiddleware(authManager))
	}

	{
		agentsGroup := workflowsGroup.Group("/agents")
		{
			// GET /api/v0/workflows/:workflow_id/agents
			// List all agents for a workflow
			agentsGroup.GET("", listAgents)

			// GET /api/v0/workflows/:workflow_id/agents/:agent_id
			// Get agent definition
			agentsGroup.GET("/:agent_id", getAgentByID)
		}
	}
}
