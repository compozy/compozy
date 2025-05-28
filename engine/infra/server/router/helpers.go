package router

import (
	"fmt"
	"net/http"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

func GetServerAddress(c *gin.Context) string {
	return c.Request.Host
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
	return GetURLParam(c, "workflow_id")
}

func GetWorkflowExecID(c *gin.Context) core.ID {
	return core.ID(GetURLParam(c, "workflow_exec_id"))
}

func GetTaskID(c *gin.Context) string {
	return GetURLParam(c, "task_id")
}

func GetTaskExecID(c *gin.Context) core.ID {
	return core.ID(GetURLParam(c, "task_exec_id"))
}

func GetAgentID(c *gin.Context) string {
	return GetURLParam(c, "agent_id")
}

func GetAgentExecID(c *gin.Context) core.ID {
	return core.ID(GetURLParam(c, "agent_exec_id"))
}

func GetToolID(c *gin.Context) string {
	return GetURLParam(c, "tool_id")
}

func GetToolExecID(c *gin.Context) core.ID {
	return core.ID(GetURLParam(c, "tool_exec_id"))
}
