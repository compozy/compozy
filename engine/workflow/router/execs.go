package wfrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/workflow/uc"
	"github.com/gin-gonic/gin"
)

// getExecution retrieves a workflow execution by ID
//
//	@Summary		Get workflow execution by ID
//	@Description	Retrieve a specific workflow execution by its execution ID
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			workflow_exec_id	path		string										true	"Workflow Execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Success		200					{object}	router.Response{data=core.MainExecutionMap}	"Workflow execution retrieved successfully"
//	@Failure		400					{object}	router.Response{error=router.ErrorInfo}		"Invalid execution ID"
//	@Failure		404					{object}	router.Response{error=router.ErrorInfo}		"Execution not found"
//	@Failure		500					{object}	router.Response{error=router.ErrorInfo}		"Internal server error"
//	@Router			/executions/workflows/{workflow_exec_id} [get]
func getExecution(c *gin.Context) {
	workflowExecID := router.GetWorkflowExecID(c)
	if workflowExecID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().WorkflowRepoFactory()
	uc := uc.NewGetExecution(repo, workflowExecID)
	exec, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to get execution",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "workflow execution retrieved", exec)
}

// listAllExecutions retrieves all workflow executions
//
//	@Summary		List all workflow executions
//	@Description	Retrieve a list of all workflow executions across all workflows
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	router.Response{data=object{executions=[]core.MainExecutionMap}}	"Workflow executions retrieved successfully"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}								"Internal server error"
//	@Router			/executions/workflows [get]
func listAllExecutions(c *gin.Context) {
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().WorkflowRepoFactory()
	uc := uc.NewListAllExecutions(repo)
	executions, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list executions",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "workflow executions retrieved", gin.H{
		"executions": executions,
	})
}

// listExecutionsByID retrieves executions for a specific workflow
//
//	@Summary		List executions by workflow ID
//	@Description	Retrieve all executions for a specific workflow
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string																true	"Workflow ID"	example("data-processing")
//	@Success		200			{object}	router.Response{data=object{executions=[]core.MainExecutionMap}}	"Workflow executions retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}								"Invalid workflow ID"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}								"Internal server error"
//	@Router			/workflows/{workflow_id}/executions [get]
func listExecutionsByID(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().WorkflowRepoFactory()
	uc := uc.NewListExecutionsByID(repo, workflowID)
	executions, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list executions",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "workflow executions retrieved", gin.H{
		"executions": executions,
	})
}
