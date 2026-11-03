package core

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

// GetSessionByID returns one session snapshot without requiring the caller to know its workspace.
func (h *BaseHandlers) GetSessionByID(c *gin.Context) {
	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		sessionID = strings.TrimSpace(c.Param("id"))
	}
	if sessionID == "" {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("%s: session_id path is required", h.transportName()))
		return
	}
	info, err := h.Sessions.Status(c.Request.Context(), sessionID)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	if !h.requireSessionInProfile(c, info, readScope) {
		return
	}
	includeHealth, err := parseBoolQuery(c, "include_health")
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	payload, err := h.sessionPayloadWithOptionalHealth(c.Request.Context(), info, includeHealth)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	h.withSessionWorkingDir(c.Request.Context(), &payload)

	c.JSON(http.StatusOK, contract.SessionResponse{Session: payload})
}

// withSessionWorkingDir decorates a single-session detail read with the daemon-resolved
// working directory. A resolution failure omits the field and never fails the read.
func (h *BaseHandlers) withSessionWorkingDir(ctx context.Context, payload *contract.SessionPayload) {
	if h.Sessions == nil || payload == nil || payload.ID == "" {
		return
	}
	dir, err := h.Sessions.WorkingDir(ctx, payload.ID)
	if err != nil {
		h.Logger.Debug("api: session working directory unavailable", "session_id", payload.ID, "error", err)
		return
	}
	payload.WorkingDir = dir
}
