package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/tools"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) GetSessionMCP(c *gin.Context) {
	c.Request = c.Request.WithContext(tools.WithCachedMCPProjection(c.Request.Context()))
	_, id, _, ok := h.routeScopedSessionInWorkspace(c)
	if !ok {
		return
	}
	reader, ok := h.Sessions.(session.MCPReader)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("session MCP read model unavailable"))
		return
	}
	result, err := reader.ListSessionMCP(c.Request.Context(), id)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *BaseHandlers) withSessionMCPSummary(ctx context.Context, payload *contract.SessionPayload) error {
	reader, ok := h.Sessions.(session.MCPReader)
	if !ok {
		return nil
	}
	result, err := reader.ListSessionMCP(tools.WithCachedMCPProjection(ctx), payload.ID)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		// The summary decorates the session; a read-model failure must not hide the session itself.
		if h.Logger != nil {
			h.Logger.Warn("api: session MCP summary unavailable", "session_id", payload.ID, "error", err)
		}
		return nil
	}
	if len(result.CompozyServers)+len(result.RuntimeServers.Servers) > 0 {
		payload.MCPSummary = &result.MCPSummary
	}
	return nil
}
