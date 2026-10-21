package core

import (
	"log/slog"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) loopListBreadth(c *gin.Context) (WorkspaceBreadth, bool) {
	if c.Param("workspace_id") == "" && !h.requireOperatorSurface(c, "aggregate loops") {
		return WorkspaceBreadth{}, false
	}
	breadth, err := parseWorkspaceBreadth(c, c.Param("workspace_id"), breadthEncoding{Selector: "workspace_id"})
	if err != nil {
		code := "workspace_breadth_invalid"
		if typed, ok := err.(*workspaceBreadthError); ok {
			code = typed.code
		}
		c.JSON(http.StatusBadRequest, contract.ErrorPayload{Code: code, Error: err.Error()})
		return WorkspaceBreadth{}, false
	}
	if c.Param("workspace_id") == "" && !breadth.AllWorkspaces {
		c.JSON(http.StatusBadRequest, contract.ErrorPayload{Code: "workspace_breadth_invalid", Error: "aggregate loop routes require all_workspaces=true; use the workspace path for one project"})
		return WorkspaceBreadth{}, false
	}
	return breadth, true
}

func logLoopAggregate(c *gin.Context, count int) {
	slog.DebugContext(c.Request.Context(), "loop aggregate read", "route", c.FullPath(), "all_workspaces", true, "count", count)
}
