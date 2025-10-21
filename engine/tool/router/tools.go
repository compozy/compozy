package toolrouter

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/tool/uc"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
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
		router.RespondProblemWithCode(c, http.StatusNotFound, router.ErrNotFoundCode, "workflow not found")
		return
	}
	usecase := uc.NewGetTool([]*workflow.Config{wfCfg}, toolID)
	tool, err := usecase.Execute(c.Request.Context())
	if err != nil {
		if errors.Is(err, uc.ErrToolNotFound) {
			router.RespondProblemWithCode(c, http.StatusNotFound, router.ErrNotFoundCode, err.Error())
			return
		}
		logger.FromContext(c.Request.Context()).Error(
			"Failed to retrieve tool",
			"error", err,
			"workflow_id", workflowID,
			"tool_id", toolID,
		)
		router.RespondProblemWithCode(
			c,
			http.StatusInternalServerError,
			router.ErrInternalCode,
			"failed to retrieve tool",
		)
		return
	}
	router.RespondOK(c, "tool retrieved", tool)
}

// listTools retrieves all tools
//
//	@Summary		List all tools
//	@Description	Retrieve a list of all available tool configurations
//	@Tags			workflows
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
		router.RespondProblemWithCode(c, http.StatusNotFound, router.ErrNotFoundCode, "workflow not found")
		return
	}
	usecase := uc.NewListTools([]*workflow.Config{wfCfg})
	tools, err := usecase.Execute(c.Request.Context())
	if err != nil {
		logger.FromContext(c.Request.Context()).Error(
			"Failed to list tools",
			"error", err,
			"workflow_id", workflowID,
		)
		router.RespondProblemWithCode(c, http.StatusInternalServerError, router.ErrInternalCode, "failed to list tools")
		return
	}
	router.RespondOK(c, "tools retrieved", gin.H{
		"tools": tools,
	})
}
