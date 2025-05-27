package wfrouter

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/workflow/events"
	"github.com/gin-gonic/gin"
)

func handleExecute(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	state := router.GetAppState(c)
	input := router.GetRequestBody[core.Input](c)
	if input == nil {
		// Error already handled by GetRequestBody
		return
	}

	inputMap, err := input.ToStruct()
	if err != nil {
		reason := fmt.Sprintf("failed to convert trigger to struct: %s", workflowID)
		reqErr := router.WorkflowExecutionError(workflowID, reason, err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Send workflow trigger
	evt := events.NewCmdTrigger(state.NatsClient, inputMap, workflowID)
	if err := evt.Publish(c.Request.Context()); err != nil {
		reason := fmt.Sprintf("failed to publish workflow trigger: %s", workflowID)
		reqErr := router.WorkflowExecutionError(workflowID, reason, err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	execURL := fmt.Sprintf("%s/api/workflows/executions/%s", router.GetServerAddress(c), evt.Response.WorkflowExecID)
	router.RespondAccepted(c, "workflow triggered successfully", gin.H{
		"workflow_id":      evt.Response.WorkflowID,
		"workflow_exec_id": evt.Response.WorkflowExecID,
		"exec_url":         execURL,
	})
}
