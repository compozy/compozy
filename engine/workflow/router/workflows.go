package wfrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	wfuc "github.com/compozy/compozy/engine/workflow/uc"
	"github.com/gin-gonic/gin"
)

// getWorkflowByID retrieves a workflow by ID
//
//	@Summary		Get workflow by ID
//	@Description	Retrieve a specific workflow configuration by its ID
//	@Tags			workflows
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string									true	"Workflow ID"	example("data-processing")
//	@Success		200			{object}	router.Response{data=wfrouter.WorkflowResponse}	"Workflow retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid workflow ID"
//	@Failure		404			{object}	router.Response{error=router.ErrorInfo}	"Workflow not found"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/workflows/{workflow_id} [get]
func getWorkflowByID(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	uc := wfuc.NewGetWorkflow(appState.Workflows, workflowID)
	workflow, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusNotFound,
			"workflow not found",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	workflowResponse := ConvertWorkflowConfigToResponse(workflow)
	router.RespondOK(c, "workflow retrieved", workflowResponse)
}

// listWorkflows retrieves all workflows
//
//	@Summary		List all workflows
//	@Description	Retrieve a list of all available workflow configurations
//	@Tags			workflows
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	router.Response{data=object{workflows=[]wfrouter.WorkflowResponse}}	"Workflows retrieved successfully"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/workflows [get]
func listWorkflows(c *gin.Context) {
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	uc := wfuc.NewListWorkflows(appState.Workflows)
	workflows, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list workflows",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	workflowResponses := ConvertWorkflowConfigsToResponses(workflows)
	router.RespondOK(c, "workflows retrieved", gin.H{
		"workflows": workflowResponses,
	})
}
