package agentrouter

import (
	"github.com/gin-gonic/gin"
)

func Register(apiBase *gin.RouterGroup) {
	agentsGroup := apiBase.Group("/agents")
	{
		// POST /agents/export
		// Export agents to YAML
		agentsGroup.POST("/export", exportAgents)
		// POST /agents/import
		// Import agents from YAML
		agentsGroup.POST("/import", importAgents)
		agentsGroup.POST("/:agent_id/executions", executeAgentAsync)
		agentsGroup.POST("/:agent_id/executions/sync", executeAgentSync)
		agentsGroup.GET("", listAgentsTop)
		agentsGroup.GET("/:agent_id", getAgentTop)
		agentsGroup.PUT("/:agent_id", upsertAgentTop)
		agentsGroup.DELETE("/:agent_id", deleteAgentTop)
	}
	execGroup := apiBase.Group("/executions")
	{
		agentExecGroup := execGroup.Group("/agents")
		agentExecGroup.GET("/:agent_exec_id", getAgentExecutionStatus)
	}
	// Agent definition routes under workflows
	workflowsGroup := apiBase.Group("/workflows/:workflow_id")
	{
		agentsGroup := workflowsGroup.Group("/agents")
		{
			// GET /workflows/:workflow_id/agents
			// List all agents for a workflow
			agentsGroup.GET("", listAgents)

			// GET /workflows/:workflow_id/agents/:agent_id
			// Get agent definition
			agentsGroup.GET("/:agent_id", getAgentByID)
		}
	}
}
