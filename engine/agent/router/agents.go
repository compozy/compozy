package agentrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/agent/uc"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/gin-gonic/gin"
)

func getAgentByID(c *gin.Context) {
	agentID := router.GetAgentID(c)
	if agentID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	uc := uc.NewGetAgent(appState.Workflows, agentID)
	agent, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusNotFound,
			"agent not found",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "agent retrieved", agent)
}

func listAgents(c *gin.Context) {
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	uc := uc.NewListAgents(appState.Workflows)
	agents, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list agents",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "agents retrieved", gin.H{
		"agents": agents,
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
