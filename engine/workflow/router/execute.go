package wfrouter

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/gin-gonic/gin"
)

// ExecuteWorkflowRequest represents the request body for workflow execution
// This is only used for Swagger documentation - the actual handler uses core.Input directly
type ExecuteWorkflowRequest struct {
	Input  core.Input `json:"input"`
	TaskID string     `json:"task_id"`
}

// ExecuteWorkflowResponse represents the response for workflow execution
type ExecuteWorkflowResponse struct {
	ExecURL    string `json:"exec_url"    example:"https://api.compozy.dev/api/v0/executions/workflows/2Z4PVTL6K27XVT4A3NPKMDD5BG"`
	ExecID     string `json:"exec_id"     example:"2Z4PVTL6K27XVT4A3NPKMDD5BG"`
	WorkflowID string `json:"workflow_id" example:"data-processing"`
}

// handleExecute triggers a workflow execution
//
//	@Summary		Execute workflow
//	@Description	Trigger the execution of a workflow with provided input data
//	@Tags			workflows
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string											true	"Workflow ID"			example("data-processing")
//	@Param			input		body		object	true	"Workflow input data"	SchemaExample({"data": "example", "config": {"timeout": 300}})
//	@Param			X-Correlation-ID	header		string		false	"Optional correlation ID for request tracing"
//	@Success		202			{object}	router.Response{data=ExecuteWorkflowResponse}	"Workflow triggered successfully"
//	@Header			202			{string}	Location	"Execution status URL"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}			"Invalid input or workflow ID"
//	@Failure		404			{object}	router.Response{error=router.ErrorInfo}			"Workflow not found"
//	@Failure		503			{object}	router.Response{error=router.ErrorInfo}			"Worker unavailable"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}			"Internal server error"
//	@Router			/workflows/{workflow_id}/executions [post]
func handleExecute(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	state := router.GetAppStateWithWorker(c)
	if state == nil {
		return
	}
	body := router.GetRequestBody[ExecuteWorkflowRequest](c)
	if body == nil {
		return
	}
	worker := state.Worker
	workflowStateID, err := worker.TriggerWorkflow(
		c.Request.Context(),
		workflowID,
		&body.Input,
		body.TaskID,
	)
	if err != nil {
		reason := fmt.Sprintf("failed to trigger workflow: %s", workflowID)
		reqErr := router.WorkflowExecutionError(workflowID, reason, err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	execID := workflowStateID.WorkflowExecID.String()
	execURL := fmt.Sprintf("%s/executions/workflows/%s", routes.Base(), execID)
	c.Header("Location", execURL)
	router.RespondAccepted(c, "workflow triggered successfully", gin.H{
		"exec_url":    execURL,
		"exec_id":     execID,
		"workflow_id": workflowStateID.WorkflowID,
	})
}
