package wfrouter

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/workflow/events"
	"github.com/gin-gonic/gin"
)

// ExecuteWorkflowRequest represents the request body for workflow execution
// This is only used for Swagger documentation - the actual handler uses core.Input directly
type ExecuteWorkflowRequest struct {
	Input map[string]interface{} `json:"input" swaggerignore:"true"`
}

// ExecuteWorkflowResponse represents the response for workflow execution
type ExecuteWorkflowResponse struct {
	WorkflowID     string `json:"workflow_id" example:"data-processing"`
	WorkflowExecID string `json:"workflow_exec_id" example:"2Z4PVTL6K27XVT4A3NPKMDD5BG"`
	ExecURL        string `json:"exec_url" example:"localhost:8080/api/workflows/executions/2Z4PVTL6K27XVT4A3NPKMDD5BG"`
}

// handleExecute triggers a workflow execution
//
//	@Summary		Execute workflow
//	@Description	Trigger the execution of a workflow with provided input data
//	@Tags			workflows
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string											true	"Workflow ID"			example("data-processing")
//	@Param			input		body		object											true	"Workflow input data"	SchemaExample({"data": "example", "config": {"timeout": 300}})
//	@Success		202			{object}	router.Response{data=ExecuteWorkflowResponse}	"Workflow triggered successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}			"Invalid input or workflow ID"
//	@Failure		404			{object}	router.Response{error=router.ErrorInfo}			"Workflow not found"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}			"Internal server error"
//	@Router			/workflows/{workflow_id}/executions [post]
func handleExecute(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	state := router.GetAppState(c)
	if state == nil {
		return
	}
	input := router.GetRequestBody[core.Input](c)
	if input == nil {
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
