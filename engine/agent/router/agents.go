package agentrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/agent/uc"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/gin-gonic/gin"
)

// getAgentByID retrieves an agent by ID
//
//	@Summary		Get agent by ID
//	@Description	Retrieve a specific agent configuration by its ID
//	@Tags			agents
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string									true	"Workflow ID"
//	@Param			agent_id	path		string									true	"Agent ID"	example("code-assistant")
//	@Success		200			{object}	router.Response{data=agent.Config}		"Agent retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid agent ID"
//	@Failure		404			{object}	router.Response{error=router.ErrorInfo}	"Agent not found"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/workflows/{workflow_id}/agents/{agent_id} [get]
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

// listAgents retrieves all agents
//
//	@Summary		List all agents
//	@Description	Retrieve a list of all available agent configurations
//	@Tags			agents
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string									true	"Workflow ID"
//	@Success		200	{object}	router.Response{data=object{agents=[]agent.Config}}	"Agents retrieved successfully"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}				"Internal server error"
//	@Router			/workflows/{workflow_id}/agents [get]
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
