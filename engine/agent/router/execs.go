package agentrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/agent/uc"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/gin-gonic/gin"
)

func getAgentExecution(c *gin.Context) {
	agentExecID := router.GetAgentExecID(c)
	if agentExecID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().AgentRepoFactory()
	uc := uc.NewGetExecution(repo, agentExecID)
	execution, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to get agent execution",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "agent execution retrieved", execution)
}

func listAllAgentExecutions(c *gin.Context) {
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().AgentRepoFactory()
	uc := uc.NewListAllExecutions(repo)
	executions, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list all agent executions",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "all agent executions retrieved", gin.H{
		"executions": executions,
	})
}

func listExecutionsByAgentID(c *gin.Context) {
	agentID := router.GetAgentID(c)
	if agentID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	repo := appState.Orchestrator.Config().AgentRepoFactory()
	uc := uc.NewListExecutionsByAgentID(repo, agentID)
	executions, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list agent executions",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "agent executions retrieved", gin.H{
		"executions": executions,
	})
}
