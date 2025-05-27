package wfrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/workflow/uc"
	"github.com/gin-gonic/gin"
)

func getExecution(c *gin.Context) {
	workflowExecID := router.GetWorkflowExecID(c)
	appState := router.GetAppState(c)
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

func listAllExecutions(c *gin.Context) {
	appState := router.GetAppState(c)
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

func listExecutionsByID(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	appState := router.GetAppState(c)
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
