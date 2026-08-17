package core

import (
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

// CancelSessionPrompt cancels one active turn through the canonical idempotent path.
func (h *BaseHandlers) CancelSessionPrompt(c *gin.Context) {
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	result, err := h.Sessions.CancelPrompt(c.Request.Context(), sessionID)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SessionPromptCancelResponse{
		SessionID: sessionID,
		Outcome:   string(result.Outcome),
		TurnID:    result.TurnID,
	})
}
