package wfroute

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
	workflowsGroup := apiBase.Group("/workflows")
	{
		workflowsGroup.POST("/:workflow_id/execute", HandleExecute)
	}
}
