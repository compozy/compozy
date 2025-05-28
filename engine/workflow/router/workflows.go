package wfrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	wfuc "github.com/compozy/compozy/engine/workflow/uc"
	"github.com/gin-gonic/gin"
)

func getWorkflowByID(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	uc := wfuc.NewGetWorkflow(appState.Workflows, workflowID)
	workflow, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusNotFound,
			"workflow not found",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "workflow retrieved", workflow)
}

func listWorkflows(c *gin.Context) {
	appState := router.GetAppState(c)
	if appState == nil {
		return
	}
	uc := wfuc.NewListWorkflows(appState.Workflows)
	workflows, err := uc.Execute(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list workflows",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	router.RespondOK(c, "workflows retrieved", gin.H{
		"workflows": workflows,
	})
}
