package tkrouter

import (
	"github.com/gin-gonic/gin"
)

func Register(apiBase *gin.RouterGroup) {
	tasksGroup := apiBase.Group("/tasks")
	{
		// POST /api/v0/tasks/export
		// Export tasks to YAML
		tasksGroup.POST("/export", exportTasks)
		// POST /api/v0/tasks/import
		// Import tasks from YAML
		tasksGroup.POST("/import", importTasks)
		tasksGroup.GET("", listTasksTop)
		tasksGroup.GET("/:task_id", getTaskTop)
		tasksGroup.PUT("/:task_id", upsertTaskTop)
		tasksGroup.DELETE("/:task_id", deleteTaskTop)
	}
	// Task definition routes under workflows
	workflowsGroup := apiBase.Group("/workflows/:workflow_id")
	{
		tasksGroup := workflowsGroup.Group("/tasks")
		{
			// GET /api/v0/workflows/:workflow_id/tasks
			// List tasks for a workflow
			tasksGroup.GET("", listTasks)

			// GET /api/v0/workflows/:workflow_id/tasks/:task_id
			// Get task definition
			tasksGroup.GET("/:task_id", getTaskByID)
		}
	}
}
