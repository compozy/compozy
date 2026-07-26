package core

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/gin-gonic/gin"
)

// GetSessionByID returns one session snapshot without requiring the caller to know its workspace.
func (h *BaseHandlers) GetSessionByID(c *gin.Context) {
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

	c.JSON(http.StatusOK, contract.SessionResponse{Session: payload})
}
