package router

import (
	"fmt"
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

func GetServerAddress(c *gin.Context) string {
	return c.Request.Host
}

func GetURLParam(c *gin.Context, key string) string {
	param := c.Param(key)
	if param == "" {
		reqErr := NewRequestError(
			http.StatusBadRequest,
			fmt.Sprintf("%s is required", key),
			nil,
		)
		RespondWithError(c, reqErr.StatusCode, reqErr)
		return ""
	}
	return param
}

func GetWorkflowID(c *gin.Context) string {
	workflowID := c.Param("workflow_id")
	if workflowID == "" {
		reqErr := NewRequestError(
			http.StatusBadRequest,
			"workflow_id is required",
			nil,
		)
		RespondWithError(c, reqErr.StatusCode, reqErr)
		return ""
	}
	return workflowID
}

func GetAppState(c *gin.Context) *appstate.State {
	appState, err := appstate.GetState(c.Request.Context())
	if err != nil {
		reqErr := NewRequestError(
			http.StatusInternalServerError,
			"failed to get application state",
			err,
		)
		logger.Error("Failed to get app state", "error", err)
		RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil
	}
	return appState
}

func GetRequestBody[T any](c *gin.Context) *T {
	var input T
	if err := c.ShouldBindJSON(&input); err != nil {
		reqErr := NewRequestError(
			http.StatusBadRequest,
			"invalid input",
			err,
		)
		RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil
	}

	return &input
}
