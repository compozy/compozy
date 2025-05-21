package handlers

import (
	"net/http"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/app"
	"github.com/gin-gonic/gin"
)

// Workflow Handlers
// func HandleListWorkflows(c *gin.Context)          { PlaceholderHandler(c) }
// func HandleGetWorkflowDefinition(c *gin.Context)  { PlaceholderHandler(c) }
// func HandleListWorkflowExecutions(c *gin.Context) { PlaceholderHandler(c) } // This is for a specific workflow_id
func HandleExecuteWorkflow(c *gin.Context) {
	// Get the workflow ID from the request
	workflowID := c.Param("workflow_id")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow_id is required"})
		return
	}

	// Get app state from context
	appState, err := app.GetState(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Ensure orchestrator is available
	if appState.Orchestrator == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "orchestrator not initialized"})
		return
	}

	// Parse input from request body
	var input common.Input
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input: " + err.Error()})
		return
	}

	// Trigger the workflow using the orchestrator
	err = appState.Orchestrator.TriggerWorkflow(
		c.Request.Context(),
		common.CompID(workflowID),
		appState.ProjectConfig,
		&input,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to trigger workflow: " + err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "workflow triggered successfully"})
}

// Workflow Execution Handlers (Global)
// func HandleListAllWorkflowExecutions(c *gin.Context)  { PlaceholderHandler(c) } // This is for all workflows
// func HandleGetWorkflowExecution(c *gin.Context)       { PlaceholderHandler(c) }
// func HandleGetWorkflowExecutionStatus(c *gin.Context) { PlaceholderHandler(c) }
// func HandleResumeWorkflowExecution(c *gin.Context)    { PlaceholderHandler(c) }
// func HandleCancelWorkflowExecution(c *gin.Context)    { PlaceholderHandler(c) }
