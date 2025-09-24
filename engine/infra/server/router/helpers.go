package router

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/resources/importer"
	"github.com/compozy/compozy/engine/worker"
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
		log := logger.FromContext(c.Request.Context())
		log.Error("Failed to get app state", "error", err)
		RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil
	}
	return appState
}

func GetAppStateWithWorker(c *gin.Context) *appstate.State {
	state := GetAppState(c)
	if state == nil {
		if !c.Writer.Written() {
			reqErr := NewRequestError(
				http.StatusServiceUnavailable,
				ErrMsgAppStateNotInitialized,
				nil,
			)
			RespondWithError(c, reqErr.StatusCode, reqErr)
		}
		return nil
	}
	if state.Worker == nil {
		reqErr := NewRequestError(
			http.StatusServiceUnavailable,
			ErrMsgWorkerNotRunning,
			nil,
		)
		RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil
	}
	return state
}

func GetWorker(c *gin.Context) *worker.Worker {
	state := GetAppStateWithWorker(c)
	if state == nil {
		return nil
	}
	return state.Worker
}

func GetResourceStore(c *gin.Context) (resources.ResourceStore, bool) {
	state := GetAppState(c)
	if state == nil {
		if !c.Writer.Written() {
			reqErr := NewRequestError(http.StatusInternalServerError, "application state not initialized", nil)
			RespondWithError(c, reqErr.StatusCode, reqErr)
		}
		return nil, false
	}
	v, ok := state.ResourceStore()
	if !ok || v == nil {
		reqErr := NewRequestError(http.StatusServiceUnavailable, "resource store not available", nil)
		RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	rs, ok := v.(resources.ResourceStore)
	if !ok || rs == nil {
		reqErr := NewRequestError(http.StatusInternalServerError, "invalid resource store instance", nil)
		RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	return rs, true
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
	return core.ID(GetURLParam(c, "exec_id"))
}

func GetWorkflowStateID(c *gin.Context) string {
	return GetURLParam(c, "state_id")
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

func ProjectRootPath(st *appstate.State) (string, bool) {
	if st == nil || st.CWD == nil {
		return "", false
	}
	path := st.CWD.PathStr()
	if path == "" {
		return "", false
	}
	return path, true
}

func ParseImportStrategyParam(value string) (importer.Strategy, error) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", string(importer.SeedOnly):
		return importer.SeedOnly, nil
	case string(importer.OverwriteConflicts):
		return importer.OverwriteConflicts, nil
	default:
		return "", fmt.Errorf("invalid strategy (allowed: %q|%q)", importer.SeedOnly, importer.OverwriteConflicts)
	}
}

func UpdatedBy(c *gin.Context) string {
	usr, ok := userctx.UserFromContext(c.Request.Context())
	if !ok || usr == nil {
		return ""
	}
	if usr.Email != "" {
		return usr.Email
	}
	if usr.ID != "" {
		return usr.ID.String()
	}
	return ""
}
