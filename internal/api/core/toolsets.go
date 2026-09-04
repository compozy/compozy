package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/gin-gonic/gin"
)

// ListToolsets returns named toolsets with expansion diagnostics.
func (h *BaseHandlers) ListToolsets(c *gin.Context) {
	if h.Toolsets == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("toolset registry is not configured"))
		return
	}
	scope, err := h.resolveOperatorToolScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	views, err := h.Toolsets.ListToolsets(c.Request.Context(), scope)
	if err != nil {
		h.respondToolError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.ToolsetsResponse{Toolsets: ToolsetPayloadsFromViews(views)})
}

// GetToolset returns one named toolset with expansion diagnostics.
func (h *BaseHandlers) GetToolset(c *gin.Context) {
	if h.Toolsets == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("toolset registry is not configured"))
		return
	}
	id := toolspkg.ToolsetID(strings.TrimSpace(c.Param("id")))
	if err := id.Validate(); err != nil {
		h.respondToolError(c, err)
		return
	}
	scope, err := h.resolveOperatorToolScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	view, err := h.Toolsets.GetToolset(c.Request.Context(), scope, id)
	if err != nil {
		h.respondToolError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.ToolsetResponse{Toolset: ToolsetPayloadFromView(view)})
}
