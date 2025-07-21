package tkrouter

import (
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
)

func Register(apiBase *gin.RouterGroup, authManager *authmw.Manager) {
	cfg := config.Get()
	// Task definition routes under workflows
	workflowsGroup := apiBase.Group("/workflows/:workflow_id")

	// Apply authentication middleware based on configuration
	if cfg.Server.Auth.Enabled {
		workflowsGroup.Use(authManager.Middleware())
		workflowsGroup.Use(authmw.WorkflowAuthMiddleware(authManager))
	}

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
