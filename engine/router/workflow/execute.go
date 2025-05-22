package wfroute

import (
	"fmt"
	"net/http"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/router"
	"github.com/compozy/compozy/pkg/app"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

func HandleExecute(c *gin.Context) {
	// Get the workflow ID from the request
	workflowID := c.Param("workflow_id")
	if workflowID == "" {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			"workflow_id is required",
			nil,
		)
		c.JSON(reqErr.StatusCode, reqErr.ToErrorResponse())
		return
	}

	// Get app state from context
	appState, err := app.GetState(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to get application state",
			err,
		)
		logger.Error("Failed to get app state", "error", err)
		c.JSON(reqErr.StatusCode, reqErr.ToErrorResponse())
		return
	}

	// Ensure orchestrator is available
	if appState.Orchestrator == nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"orchestrator not initialized",
			router.ErrInternal,
		)
		c.JSON(reqErr.StatusCode, reqErr.ToErrorResponse())
		return
	}

	// Parse input from request body
	var input common.Input
	if err := c.ShouldBindJSON(&input); err != nil {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			"invalid input",
			err,
		)
		c.JSON(reqErr.StatusCode, reqErr.ToErrorResponse())
		return
	}

	// Trigger the workflow using the orchestrator
	res, err := appState.Orchestrator.SendTriggerWorkflow(
		common.ID(workflowID),
		appState.ProjectConfig,
		&input,
	)
	if err != nil {
		reqErr := router.WorkflowExecutionError(
			workflowID,
			fmt.Sprintf("failed to execute workflow: %s", workflowID),
			err,
		)
		logger.Error("Workflow execution failed",
			"workflow_id", workflowID,
			"error", err,
		)
		c.JSON(reqErr.StatusCode, reqErr.ToErrorResponse())
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":  "workflow triggered successfully",
		"response": res,
	})
}
