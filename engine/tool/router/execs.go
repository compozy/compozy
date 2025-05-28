package toolrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/tool/uc"
	"github.com/gin-gonic/gin"
)

// getToolExecution retrieves a tool execution by ID
//
//	@Summary		Get tool execution by ID
//	@Description	Retrieve a specific tool execution by its execution ID
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			tool_exec_id	path		string									true	"Tool Execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Success		200				{object}	router.Response{data=tool.Execution}	"Tool execution retrieved successfully"
//	@Failure		400				{object}	router.Response{error=router.ErrorInfo}	"Invalid execution ID"
//	@Failure		404				{object}	router.Response{error=router.ErrorInfo}	"Execution not found"
//	@Failure		500				{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/executions/tools/{tool_exec_id} [get]
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

// listAllToolExecutions retrieves all tool executions
//
//	@Summary		List all tool executions
//	@Description	Retrieve a list of all tool executions across all workflows
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	router.Response{data=object{executions=[]tool.Execution}}	"Tool executions retrieved successfully"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/executions/tools [get]
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

// listExecutionsByToolID retrieves executions for a specific tool
//
//	@Summary		List executions by tool ID
//	@Description	Retrieve all executions for a specific tool
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			tool_id	path		string														true	"Tool ID"	example("format-code")
//	@Success		200		{object}	router.Response{data=object{executions=[]tool.Execution}}	"Tool executions retrieved successfully"
//	@Failure		400		{object}	router.Response{error=router.ErrorInfo}						"Invalid tool ID"
//	@Failure		500		{object}	router.Response{error=router.ErrorInfo}						"Internal server error"
//	@Router			/tools/{tool_id}/executions [get]
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
