package handlers

import (
	"github.com/gin-gonic/gin"
)

// Workflow Handlers
func HandleListWorkflows(c *gin.Context)          { PlaceholderHandler(c) }
func HandleGetWorkflowDefinition(c *gin.Context)  { PlaceholderHandler(c) }
func HandleListWorkflowExecutions(c *gin.Context) { PlaceholderHandler(c) } // This is for a specific workflow_id
func HandleExecuteWorkflow(c *gin.Context)        { PlaceholderHandler(c) }
func HandleExecuteWorkflowAsync(c *gin.Context)   { PlaceholderHandler(c) }

// Workflow Execution Handlers (Global)
func HandleListAllWorkflowExecutions(c *gin.Context)  { PlaceholderHandler(c) } // This is for all workflows
func HandleGetWorkflowExecution(c *gin.Context)       { PlaceholderHandler(c) }
func HandleGetWorkflowExecutionStatus(c *gin.Context) { PlaceholderHandler(c) }
func HandleResumeWorkflowExecution(c *gin.Context)    { PlaceholderHandler(c) }
func HandleCancelWorkflowExecution(c *gin.Context)    { PlaceholderHandler(c) }
