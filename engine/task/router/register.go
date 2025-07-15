package tkrouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
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
