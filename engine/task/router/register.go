package tkrouter

import (
	"github.com/gin-gonic/gin"
)

func Register(apiBase *gin.RouterGroup) {
	tasksGroup := apiBase.Group("/tasks")
	{
		tasksGroup.POST("/export", exportTasks)
		tasksGroup.POST("/import", importTasks)
		tasksGroup.POST("/:task_id/executions", executeTaskAsync)
		tasksGroup.POST("/:task_id/executions/sync", executeTaskSync)
		tasksGroup.GET("", listTasksTop)
		tasksGroup.GET("/:task_id", getTaskTop)
		tasksGroup.PUT("/:task_id", upsertTaskTop)
		tasksGroup.DELETE("/:task_id", deleteTaskTop)
	}
	execGroup := apiBase.Group("/executions")
	{
		taskExecGroup := execGroup.Group("/tasks")
		taskExecGroup.GET("/:task_exec_id", getTaskExecutionStatus)
		taskExecGroup.GET("/:task_exec_id/stream", streamTaskExecution)
	}
	workflowsGroup := apiBase.Group("/workflows/:workflow_id")
	{
		tasksGroup := workflowsGroup.Group("/tasks")
		{
			tasksGroup.GET("", listTasks)

			tasksGroup.GET("/:task_id", getTaskByID)
		}
	}
}
