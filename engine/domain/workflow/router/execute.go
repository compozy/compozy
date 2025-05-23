package wfrouter

import (
	"fmt"

	"github.com/compozy/compozy/server/router"
	"github.com/gin-gonic/gin"
)

// Route: POST /api/workflows/:workflow_id/execute
func handleExecute(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	state := router.GetAppState(c)
	ti := router.GetRequestBody(c)
	orch := state.Orchestrator

	// Send workflow trigger
	res, err := orch.SendWorkflowTrigger(ti, workflowID)
	if err != nil {
		reason := fmt.Sprintf("failed to execute workflow: %s", workflowID)
		reqErr := router.WorkflowExecutionError(workflowID, reason, err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	execURL := fmt.Sprintf("%s/api/workflows/executions/%s", router.GetServerAddress(c), res.StateID)
	router.RespondAccepted(c, "workflow triggered successfully", gin.H{
		"state_id":    res.StateID,
		"workflow_id": res.WorkflowID,
		"exec_url":    execURL,
	})
}
