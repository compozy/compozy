package wfrouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
	workflowsGroup := apiBase.Group("/workflows")
	{
		workflowsGroup.POST("/:workflow_id/execute", handleExecute)
		workflowsGroup.GET("/:workflow_id/executions", handleGetExecutions)
		workflowsGroup.GET("/executions/:id", handleGetExecution)
	}
}
