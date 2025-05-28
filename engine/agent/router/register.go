package agentrouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
	// Agent definition routes
	agentsGroup := apiBase.Group("/agents")
	{
		// GET /api/v0/agents
		// List all agents
		agentsGroup.GET("", listAgents)

		// GET /api/v0/agents/:agent_id
		// Get agent definition
		agentsGroup.GET("/:agent_id", getAgentByID)

		// GET /api/v0/agents/:agent_id/executions
		// List executions for an agent
		agentsGroup.GET("/:agent_id/executions", listExecutionsByAgentID)
	}

	// Global execution routes
	executionsGroup := apiBase.Group("/executions")
	{
		// Agent execution routes
		agentExecGroup := executionsGroup.Group("/agents")
		{
			_ = agentExecGroup // TODO: implement agent execution routes
			// TODO: implement agent execution routes
			// GET /api/v0/executions/agents
			// List all agent executions

			// GET /api/v0/executions/agents/:agent_exec_id
			// Get agent execution details

			// GET /api/v0/executions/agents/:agent_exec_id/logs
			// Get logs for an agent execution
		}
	}
}
