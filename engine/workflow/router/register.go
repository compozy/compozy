package wfrouter

import (
	"github.com/gin-gonic/gin"
)

// Register registers HTTP API routes under the provided apiBase RouterGroup.
// It adds endpoints for events, workflow definitions, and workflow executions.
// Authentication is handled globally at the server level.
func Register(apiBase *gin.RouterGroup) {
	if apiBase == nil {
		return
	}
	apiBase.POST("/events", handleEvent)
	registerWorkflowDefinitionRoutes(apiBase.Group("/workflows"))
	registerWorkflowExecutionRoutes(apiBase.Group("/executions"))
}

func registerWorkflowDefinitionRoutes(group *gin.RouterGroup) {
	if group == nil {
		return
	}
	group.POST("/export", exportWorkflows)
	group.POST("/import", importWorkflows)
	group.GET("", listWorkflows)
	group.GET("/:workflow_id", getWorkflowByID)
	group.PUT("/:workflow_id", upsertWorkflow)
	group.DELETE("/:workflow_id", deleteWorkflow)
	group.GET("/:workflow_id/executions", listExecutionsByID)
	group.POST("/:workflow_id/executions", handleExecute)
	group.POST("/:workflow_id/executions/sync", executeWorkflowSync)
}

func registerWorkflowExecutionRoutes(group *gin.RouterGroup) {
	if group == nil {
		return
	}
	workflowExec := group.Group("/workflows")
	workflowExec.GET("", listAllExecutions)
	workflowExec.GET("/:exec_id", getExecution)
	workflowExec.POST("/:exec_id/pause", pauseExecution)
	workflowExec.POST("/:exec_id/resume", resumeExecution)
	workflowExec.POST("/:exec_id/cancel", cancelExecution)
	workflowExec.POST("/:exec_id/signals", sendSignalToExecution)
}
