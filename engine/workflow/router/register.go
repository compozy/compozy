package wfrouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
	workflowsGroup := apiBase.Group("/workflows")
	{
		workflowsGroup.POST("/:workflow_id/execute", handleExecute)
		workflowsGroup.GET("/:workflow_id/executions", listExecutionsByWorkflowID)
		workflowsGroup.GET("/executions", listExecutions)
		workflowsGroup.GET("/executions/:exec_id", getExecutionByExecID)
	}
}
