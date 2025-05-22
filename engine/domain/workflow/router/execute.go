package wfrouter

import (
	"fmt"

	"github.com/compozy/compozy/server/router"
	"github.com/gin-gonic/gin"
)

// Route: POST /api/workflows/:workflow_id/execute
func handleExecute(c *gin.Context) {
	wfID := router.GetWorkflowID(c)
	st := router.GetAppState(c)
	input := router.GetRequestBody(c)
	orch := st.Orchestrator

	// Send workflow trigger
	res, err := orch.SendWorkflowTrigger(wfID, input)
	if err != nil {
		reason := fmt.Sprintf("failed to execute workflow: %s", wfID)
		reqErr := router.WorkflowExecutionError(wfID, reason, err)
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
