package tkrouter

import (
	"github.com/gin-gonic/gin"
)

func Register(apiBase *gin.RouterGroup) {
	tasksGroup := apiBase.Group("/tasks")
	{
		// POST /tasks/export
		// Export tasks to YAML
		tasksGroup.POST("/export", exportTasks)
		// POST /tasks/import
		// Import tasks from YAML
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
	}
	// Task definition routes under workflows
	workflowsGroup := apiBase.Group("/workflows/:workflow_id")
	{
		tasksGroup := workflowsGroup.Group("/tasks")
		{
			// GET /workflows/:workflow_id/tasks
			// List tasks for a workflow
			tasksGroup.GET("", listTasks)

			// GET /workflows/:workflow_id/tasks/:task_id
			// Get task definition
			tasksGroup.GET("/:task_id", getTaskByID)
		}
	}
}
