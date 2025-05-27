package wfrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/workflow/uc"
	"github.com/gin-gonic/gin"
)

// Route: GET /api/workflows/executions
func listExecutions(c *gin.Context) {
	appState := router.GetAppState(c)
	repo := appState.Orchestrator.Config().WorkflowRepoFactory()
	uc := uc.NewListExecutionsUC(repo)
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

// Route: GET /api/workflows/executions/:id
func getExecution(c *gin.Context) {
	workflowExecID := core.ID(router.GetURLParam(c, "id"))
	appState := router.GetAppState(c)
	repo := appState.Orchestrator.Config().WorkflowRepoFactory()
	uc := uc.NewGetExecutionUC(repo, workflowExecID)
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
