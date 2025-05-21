package server

import (
	"github.com/compozy/compozy/pkg/app"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/server/handlers"
	"github.com/gin-gonic/gin"
)

func registerSystemRoutes(router *gin.Engine, apiBase *gin.RouterGroup) {
	// apiBase.GET("", handlers.HandleGetAPIInfo)
	// apiBase.GET("/health", handlers.HandleGetHealth)
	// apiBase.GET("/metrics", handlers.HandleGetMetrics)
	// apiBase.GET("/version", handlers.HandleGetVersion)

	// // Standalone routes (not under /api)
	// router.GET("/openapi.json", handlers.HandleGetOpenAPISchema)
	// router.GET("/swagger.ui", handlers.HandleGetSwaggerUI)
}

func registerWorkflowRoutes(apiBase *gin.RouterGroup) {
	workflowsGroup := apiBase.Group("/workflows")
	{
		// workflowsGroup.GET("", handlers.HandleListWorkflows)
		// workflowsGroup.GET("/:workflow_id/definition", handlers.HandleGetWorkflowDefinition)
		// List executions for a specific workflow
		// workflowsGroup.GET("/:workflow_id/executions", handlers.HandleListWorkflowExecutions)
		workflowsGroup.POST("/:workflow_id/execute", handlers.HandleExecuteWorkflow)

		// Global Workflow Execution Routes (under /api/workflows/executions)
		// workflowExecutionsGroup := workflowsGroup.Group("/executions")
		// {
		// 	// List all executions across all workflows
		// 	workflowExecutionsGroup.GET("", handlers.HandleListAllWorkflowExecutions)
		// 	workflowExecutionsGroup.GET("/:workflow_exec_id", handlers.HandleGetWorkflowExecution)
		// 	workflowExecutionsGroup.GET("/:workflow_exec_id/status", handlers.HandleGetWorkflowExecutionStatus)
		// 	workflowExecutionsGroup.POST("/:workflow_exec_id/resume", handlers.HandleResumeWorkflowExecution)
		// 	workflowExecutionsGroup.POST("/:workflow_exec_id/cancel", handlers.HandleCancelWorkflowExecution)
		// }
	}
}

func registerTaskRoutes(apiBase *gin.RouterGroup) {
	// tasksGroup := apiBase.Group("/tasks")
	// {
	// 	tasksGroup.GET("", handlers.HandleListTasks)
	// 	tasksGroup.GET("/:task_id/definition", handlers.HandleGetTaskDefinition)
	// 	// List executions for a specific task
	// 	tasksGroup.GET("/:task_id/executions", handlers.HandleListTaskExecutions)
	// 	tasksGroup.POST("/:task_id/trigger", handlers.HandleTriggerTask)
	// 	// Global Task Execution Routes (under /api/tasks/executions)
	// 	taskExecutionsGroup := tasksGroup.Group("/executions")
	// 	{
	// 		// List all executions across all tasks
	// 		taskExecutionsGroup.GET("", handlers.HandleListAllTaskExecutions)
	// 		taskExecutionsGroup.GET("/:task_exec_id", handlers.HandleGetTaskExecution)
	// 		taskExecutionsGroup.GET("/:task_exec_id/status", handlers.HandleGetTaskExecutionStatus)
	// 		taskExecutionsGroup.POST("/:task_exec_id/resume", handlers.HandleResumeTaskExecution)
	// 	}
	// }
}

func registerAgentRoutes(apiBase *gin.RouterGroup) {
	// agentsGroup := apiBase.Group("/agents")
	// {
	// 	agentsGroup.GET("", handlers.HandleListAgents)
	// 	agentsGroup.GET("/:agent_id/definition", handlers.HandleGetAgentDefinition)
	// 	// List executions for a specific agent
	// 	agentsGroup.GET("/:agent_id/executions", handlers.HandleListAgentExecutions)

	// 	// Global Agent Execution Routes (under /api/agents/executions)
	// 	agentExecutionsGroup := agentsGroup.Group("/executions")
	// 	{
	// 		// List all executions across all agents
	// 		agentExecutionsGroup.GET("", handlers.HandleListAllAgentExecutions)
	// 		agentExecutionsGroup.GET("/:agent_exec_id", handlers.HandleGetAgentExecution)
	// 		agentExecutionsGroup.GET("/:agent_exec_id/status", handlers.HandleGetAgentExecutionStatus)
	// 	}
	// }
}

func registerToolRoutes(apiBase *gin.RouterGroup) {
	// toolsGroup := apiBase.Group("/tools")
	// {
	// 	toolsGroup.GET("", handlers.HandleListTools)
	// 	toolsGroup.GET("/:tool_id/definition", handlers.HandleGetToolDefinition)
	// 	// List executions for a specific tool
	// 	toolsGroup.GET("/:tool_id/executions", handlers.HandleListToolExecutions)

	// 	// Global Tool Execution Routes (under /api/tools/executions)
	// 	toolExecutionsGroup := toolsGroup.Group("/executions")
	// 	{
	// 		// List all executions across all tools
	// 		toolExecutionsGroup.GET("", handlers.HandleListAllToolExecutions)
	// 		toolExecutionsGroup.GET("/:tool_exec_id", handlers.HandleGetToolExecution)
	// 		toolExecutionsGroup.GET("/:tool_exec_id/status", handlers.HandleGetToolExecutionStatus)
	// 	}
	// }
}

func registerLogRoutes(apiBase *gin.RouterGroup) {
	// logsGroup := apiBase.Group("/logs")
	// {
	// 	logsGroup.GET("", handlers.HandleListLogs)
	// 	logsGroup.GET("/:log_id", handlers.HandleGetLogByID)
	// 	logsGroup.GET("/workflows/:workflow_exec_id", handlers.HandleGetLogsForWorkflowExecution)
	// }
}

func RegisterRoutes(router *gin.Engine, state *app.State) error {
	apiBase := router.Group("/api")
	registerSystemRoutes(router, apiBase)
	registerWorkflowRoutes(apiBase)
	registerTaskRoutes(apiBase)
	registerAgentRoutes(apiBase)
	registerToolRoutes(apiBase)
	registerLogRoutes(apiBase)

	logger.Info("Completed route registration",
		"total_workflows_in_state_for_context", len(state.Workflows),
	)

	return nil
}
