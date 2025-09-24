package toolrouter

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/tool/uc"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/gin-gonic/gin"
)

// getToolByID retrieves a tool by ID
//
//	@Summary		Get tool by ID
//	@Description	Retrieve a specific tool configuration by its ID
//	@Tags			workflows
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string									true	"Workflow ID"
//	@Param			tool_id	path		string									true	"Tool ID"	example("format-code")
//	@Success		200		{object}	router.Response{data=tool.Config}		"Tool retrieved successfully"
//	@Failure		400		{object}	router.Response{error=router.ErrorInfo}	"Invalid tool ID"
//	@Failure		404		{object}	router.Response{error=router.ErrorInfo}	"Tool not found"
//	@Failure		500		{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/workflows/{workflow_id}/tools/{tool_id} [get]
func getToolByID(c *gin.Context) {
	toolID := router.GetToolID(c)
	if toolID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	wfCfg, err := workflow.FindConfig(appState.GetWorkflows(), workflowID)
	if err != nil {
		reqErr := router.NewRequestError(http.StatusNotFound, "workflow not found", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	usecase := uc.NewGetTool([]*workflow.Config{wfCfg}, toolID)
	tool, err := usecase.Execute(c.Request.Context())
	if err != nil {
		if errors.Is(err, uc.ErrToolNotFound) {
			reqErr := router.NewRequestError(http.StatusNotFound, "tool not found", err)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return
		}
		reqErr := router.NewRequestError(http.StatusInternalServerError, "failed to retrieve tool", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "tool retrieved", tool)
}

// listTools retrieves all tools
//
//	@Summary		List all tools
//	@Description	Retrieve a list of all available tool configurations
//	@Tags			tools
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string									true	"Workflow ID"
//	@Success		200	{object}	router.Response{data=object{tools=[]tool.Config}}	"Tools retrieved successfully"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}				"Internal server error"
//	@Router			/workflows/{workflow_id}/tools [get]
func listTools(c *gin.Context) {
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	wfCfg, err := workflow.FindConfig(appState.GetWorkflows(), workflowID)
	if err != nil {
		reqErr := router.NewRequestError(http.StatusNotFound, "workflow not found", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	usecase := uc.NewListTools([]*workflow.Config{wfCfg})
	tools, err := usecase.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list tools",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "tools retrieved", gin.H{
		"tools": tools,
	})
}
