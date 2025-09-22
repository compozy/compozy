package agentrouter

import (
	"github.com/gin-gonic/gin"
)

func Register(apiBase *gin.RouterGroup) {
	agentsGroup := apiBase.Group("/agents")
	{
		// POST /api/v0/agents/export
		// Export agents to YAML
		agentsGroup.POST("/export", exportAgents)

		// POST /api/v0/agents/import
		// Import agents from YAML
		agentsGroup.POST("/import", importAgents)

		agentsGroup.GET("", listAgentsTop)
		agentsGroup.GET("/:agent_id", getAgentTop)
		agentsGroup.PUT("/:agent_id", upsertAgentTop)
		agentsGroup.DELETE("/:agent_id", deleteAgentTop)
	}
	// Agent definition routes under workflows
	workflowsGroup := apiBase.Group("/workflows/:workflow_id")
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
