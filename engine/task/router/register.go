package tkrouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
	workflowsGroup := apiBase.Group("/workflows")
	{
		workflowsGroup.GET("/:workflow_id/tasks/executions", listWorkflowTaskExecutions)
		workflowsGroup.GET("/:workflow_id/tasks/:task_id/executions", listTaskExecutions)
		workflowsGroup.GET("/:workflow_id/tasks/executions/:task_exec_id", getTaskExecution)
	}
}
