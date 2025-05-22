package wfrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/server/router"
	"github.com/gin-gonic/gin"
)

// Route: GET /api/workflows/executions
func handleGetExecutions(c *gin.Context) {
	router.RespondOK(c, "workflow executions retrieved", gin.H{
		// "executions": executions,
	})
}

// Route: GET /api/workflows/executions/:id
func handleGetExecution(c *gin.Context) {
	ID := router.GetURLParam(c, "id")
	stID, err := state.IDFromString(ID)
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			"invalid state id",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	appState := router.GetAppState(c)
	st, err := appState.Store.GetState(stID)
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to get state",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	router.RespondOK(c, "workflow execution retrieved", gin.H{
		"execution": st,
	})
}
