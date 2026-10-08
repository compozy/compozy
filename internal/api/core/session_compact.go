package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

// SessionCompactionManager owns advertised native compaction requests.
type SessionCompactionManager interface {
	RequestCompaction(context.Context, string) (session.CompactionRequestResult, <-chan acp.AgentEvent, error)
}

// CompactSession accepts one standalone maintenance turn (experimental).
func (h *BaseHandlers) CompactSession(c *gin.Context) {
	id, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	manager, ok := h.Sessions.(SessionCompactionManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session compaction unavailable"))
		return
	}
	source := "http"
	if strings.HasPrefix(c.Request.UserAgent(), "compozy-cli") {
		source = "cli"
	}
	result, _, err := manager.RequestCompaction(session.WithCompactionRequestedBy(c.Request.Context(), source), id)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusAccepted, SessionCompactPayload(result))
}

// SessionCompactPayload maps the acceptance receipt shared by transports and tools.
func SessionCompactPayload(result session.CompactionRequestResult) contract.SessionCompactResponse {
	return contract.SessionCompactResponse{
		SessionID: result.SessionID,
		PromptID:  result.PromptID,
		Command:   result.Command,
		Status:    "accepted",
	}
}
