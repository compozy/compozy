package agentrouter

import (
	"github.com/gin-gonic/gin"
)

func Register(apiBase *gin.RouterGroup) {
	agentsGroup := apiBase.Group("/agents")
	{
		agentsGroup.POST("/export", exportAgents)
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
		agentExecGroup.GET("/:agent_exec_id/stream", streamAgentExecution)
	}
	workflowsGroup := apiBase.Group("/workflows/:workflow_id")
	{
		agentsGroup := workflowsGroup.Group("/agents")
		{
			agentsGroup.GET("", listAgents)

			agentsGroup.GET("/:agent_id", getAgentByID)
		}
	}
}
