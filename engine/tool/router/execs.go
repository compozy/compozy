package toolrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/tool/uc"
	"github.com/gin-gonic/gin"
)

func getToolExecution(c *gin.Context) {
	toolExecID := router.GetToolExecID(c)
	if toolExecID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().ToolRepoFactory()
	uc := uc.NewGetExecution(repo, toolExecID)
	execution, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to get tool execution",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "tool execution retrieved", execution)
}

func listAllToolExecutions(c *gin.Context) {
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().ToolRepoFactory()
	uc := uc.NewListAllExecutions(repo)
	executions, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list all tool executions",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "all tool executions retrieved", gin.H{
		"executions": executions,
	})
}

func listExecutionsByToolID(c *gin.Context) {
	toolID := router.GetToolID(c)
	if toolID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().ToolRepoFactory()
	uc := uc.NewListExecutionsByToolID(repo, toolID)
	executions, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list tool executions",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "tool executions retrieved", gin.H{
		"executions": executions,
	})
}
