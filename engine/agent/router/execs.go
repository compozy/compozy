package agentrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/agent/uc"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/gin-gonic/gin"
)

// getAgentExecution retrieves an agent execution by ID
//
//	@Summary		Get agent execution by ID
//	@Description	Retrieve a specific agent execution by its execution ID
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			agent_exec_id	path		string									true	"Agent Execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Success		200				{object}	router.Response{data=agent.Execution}	"Agent execution retrieved successfully"
//	@Failure		400				{object}	router.Response{error=router.ErrorInfo}	"Invalid execution ID"
//	@Failure		404				{object}	router.Response{error=router.ErrorInfo}	"Execution not found"
//	@Failure		500				{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/executions/agents/{agent_exec_id} [get]
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

// listAllAgentExecutions retrieves all agent executions
//
//	@Summary		List all agent executions
//	@Description	Retrieve a list of all agent executions across all workflows
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	router.Response{data=object{executions=[]agent.Execution}}	"Agent executions retrieved successfully"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/executions/agents [get]
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

// listExecutionsByAgentID retrieves executions for a specific agent
//
//	@Summary		List executions by agent ID
//	@Description	Retrieve all executions for a specific agent
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			agent_id	path		string														true	"Agent ID"	example("code-assistant")
//	@Success		200			{object}	router.Response{data=object{executions=[]agent.Execution}}	"Agent executions retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}						"Invalid agent ID"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/agents/{agent_id}/executions [get]
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
