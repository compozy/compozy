package toolrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/tool/uc"
	"github.com/gin-gonic/gin"
)

func getToolByID(c *gin.Context) {
	toolID := router.GetToolID(c)
	if toolID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	uc := uc.NewGetTool(appState.Workflows, toolID)
	tool, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusNotFound,
			"tool not found",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "tool retrieved", tool)
}

func listTools(c *gin.Context) {
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	uc := uc.NewListTools(appState.Workflows)
	tools, err := uc.Execute(c.Request.Context())
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
