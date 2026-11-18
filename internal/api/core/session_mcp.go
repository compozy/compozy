package core

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/tools"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) GetSessionMCP(c *gin.Context) {
	c.Request = c.Request.WithContext(h.toolReadContext(c.Request.Context()))
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
	if _, app := AppTokenFromContext(c.Request.Context()); app {
		result.CompozyServers = slices.Clone(result.CompozyServers)
		for i := range result.CompozyServers {
			row := &result.CompozyServers[i]
			row.Command, row.URL, row.Source.Path = "", "", ""
			row.Args = nil
		}
	}
	c.JSON(http.StatusOK, result)
}

func (h *BaseHandlers) withSessionMCPSummary(ctx context.Context, payload *contract.SessionPayload) error {
	reader, ok := h.Sessions.(session.MCPReader)
	if !ok {
		return nil
	}
	var summary *session.MCPSummary
	var err error
	if cached, ok := h.Sessions.(interface {
		SessionMCPSummary(context.Context, string) (*session.MCPSummary, error)
	}); ok {
		summary, err = cached.SessionMCPSummary(ctx, payload.ID)
	} else {
		var result session.MCP
		result, err = reader.ListSessionMCP(tools.WithCachedMCPProjection(ctx), payload.ID)
		if len(result.CompozyServers)+len(result.RuntimeServers.Servers) > 0 {
			summary = &result.MCPSummary
		}
	}
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
	payload.MCPSummary = summary
	return nil
}

func (h *BaseHandlers) toolReadContext(ctx context.Context) context.Context {
	ctx = tools.EnsureProjectionListings(tools.WithCachedMCPProjection(ctx))
	if reader, ok := h.Tools.(interface {
		ReadContext(context.Context) context.Context
	}); ok {
		return reader.ReadContext(ctx)
	}
	return ctx
}
