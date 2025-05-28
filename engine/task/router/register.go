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

			// GET /api/v0/workflows/:workflow_id/tasks/:task_id/executions
			// List executions for a task
			tasksGroup.GET("/:task_id/executions", listExecutionsByID)

			// TODO: implement task execution routes
			// POST /api/v0/workflows/:workflow_id/tasks/:task_id/executions
			// Start a task execution

			// GET /api/v0/workflows/:workflow_id/tasks/:task_id/executions/children
			// List children executions for a task
			tasksGroup.GET("/:task_id/executions/children", listChildrenExecutionsByID)

			// TODO: implement task execution sub-routes
			// GET /api/v0/workflows/:workflow_id/tasks/:task_id/executions/agents
			// List all agent executions for a task
			tasksGroup.GET("/:task_id/executions/agents", listAgentExecutionsByID)

			// GET /api/v0/workflows/:workflow_id/tasks/:task_id/executions/tools
			// List all tool executions for a task
			tasksGroup.GET("/:task_id/executions/tools", listToolExecutionsByID)
		}
	}

	// Global execution routes
	executionsGroup := apiBase.Group("/executions")
	{
		// Task execution routes
		taskExecGroup := executionsGroup.Group("/tasks")
		{
			// GET /api/v0/executions/tasks
			// List all task executions
			taskExecGroup.GET("", listAllExecutions)

			// GET /api/v0/executions/tasks/:task_exec_id
			// Get task execution details
			taskExecGroup.GET("/:task_exec_id", getTaskExecution)

			// TODO: implement remaining task execution routes
			// GET /api/v0/executions/tasks/:task_exec_id/logs
			// Get logs for a task execution

			// GET /api/v0/executions/tasks/:task_exec_id/executions
			// List all executions within a task execution
			taskExecGroup.GET("/:task_exec_id/executions", listChildrenExecutions)

			// GET /api/v0/executions/tasks/:task_exec_id/executions/agents
			// List agent executions within a task execution
			taskExecGroup.GET("/:task_exec_id/executions/agents", listAgentExecutions)

			// GET /api/v0/executions/tasks/:task_exec_id/executions/tools
			// List tool executions within a task execution
			taskExecGroup.GET("/:task_exec_id/executions/tools", listToolExecutions)
		}
	}
}
