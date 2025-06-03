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
//	@Param			state_id	path		string										true	"Workflow Execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Success		200					{object}	router.Response{data=workflow.State}	"Workflow execution retrieved successfully"
//	@Failure		400					{object}	router.Response{error=router.ErrorInfo}		"Invalid execution ID"
//	@Failure		404					{object}	router.Response{error=router.ErrorInfo}		"Execution not found"
//	@Failure		500					{object}	router.Response{error=router.ErrorInfo}		"Internal server error"
//	@Router			/executions/workflows/{state_id} [get]
func getExecution(c *gin.Context) {
	stateID := router.GetWorkflowStateID(c)
	if stateID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Worker.WorkflowRepo()
	useCase := uc.NewGetExecution(repo, stateID)
	exec, err := useCase.Execute(c.Request.Context())
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
//	@Tags			workflows
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	router.Response{data=object{executions=[]workflow.State}}	"Workflow executions retrieved successfully"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}								"Internal server error"
//	@Router			/executions/workflows [get]
func listAllExecutions(c *gin.Context) {
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Worker.WorkflowRepo()
	useCase := uc.NewListAllExecutions(repo)
	executions, err := useCase.Execute(c.Request.Context())
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
//	@Tags			workflows
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string																true	"Workflow ID"	example("data-processing")
//	@Success		200			{object}	router.Response{data=object{executions=[]workflow.State}}	"Workflow executions retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}								"Invalid workflow ID"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}								"Internal server error"
//	@Router			/workflows/{workflow_id}/executions [get]
func listExecutionsByID(c *gin.Context) {
	wfID := router.GetWorkflowID(c)
	if wfID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Worker.WorkflowRepo()
	useCase := uc.NewListExecutionsByID(repo, wfID)
	execs, err := useCase.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list executions by ID",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "workflow executions retrieved", gin.H{
		"executions": execs,
	})
}

// pauseExecution pauses a workflow execution
//
//	@Summary		Pause workflow execution
//	@Description	Pause a specific workflow execution by its execution ID
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			state_id	path		string	true	"Workflow Execution ID"	example("workflowID_execID")
//	@Success		200			{object}	router.Response{data=string}	"Workflow execution paused successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid execution ID"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/executions/workflows/{state_id}/pause [post]
func pauseExecution(c *gin.Context) {
	stateID := router.GetWorkflowStateID(c)
	if stateID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	useCase := uc.NewPauseExecution(appState.Worker, stateID)
	err := useCase.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to pause execution",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "workflow execution paused", nil)
}

// resumeExecution resumes a workflow execution
//
//	@Summary		Resume workflow execution
//	@Description	Resume a specific workflow execution by its execution ID
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			state_id	path		string	true	"Workflow Execution ID"	example("workflowID_execID")
//	@Success		200			{object}	router.Response{data=string}	"Workflow execution resumed successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid execution ID"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/executions/workflows/{state_id}/resume [post]
func resumeExecution(c *gin.Context) {
	stateID := router.GetWorkflowStateID(c)
	if stateID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	useCase := uc.NewResumeExecution(appState.Worker, stateID)
	err := useCase.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to resume execution",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "workflow execution resumed", nil)
}

// cancelExecution cancels a workflow execution
//
//	@Summary		Cancel workflow execution
//	@Description	Cancel a specific workflow execution by its execution ID
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			state_id	path		string	true	"Workflow Execution ID"	example("workflowID_execID")
//	@Success		200			{object}	router.Response{data=string}	"Workflow execution canceled successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid execution ID"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/executions/workflows/{state_id}/cancel [post]
func cancelExecution(c *gin.Context) {
	stateID := router.GetWorkflowStateID(c)
	if stateID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	useCase := uc.NewCancelExecution(appState.Worker, stateID)
	err := useCase.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to cancel execution",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "workflow execution canceled", nil)
}
