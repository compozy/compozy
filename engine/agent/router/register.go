package agentrouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
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
