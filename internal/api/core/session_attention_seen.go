package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

// SessionAttentionSeenManager marks the current Needs you occurrence of a session as seen.
type SessionAttentionSeenManager interface {
	MarkAttentionSeen(ctx context.Context, sessionID string) (*session.Info, error)
}

// MarkSessionSeen clears a session's Needs you for the operator until its next occurrence.
func (h *BaseHandlers) MarkSessionSeen(c *gin.Context) {
	manager, ok := h.Sessions.(SessionAttentionSeenManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session attention manager is required"))
		return
	}
	_, sessionID, existing, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	profileScope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if !h.requireSessionInProfile(c, existing, profileScope) {
		return
	}
	info, err := manager.MarkAttentionSeen(context.WithoutCancel(c.Request.Context()), sessionID)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	payload, err := h.sessionPayloadWithOptionalHealth(c.Request.Context(), info, false)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SessionResponse{Session: payload})
}

func statusForAttentionSeenError(err error) (int, bool) {
	switch {
	case errors.Is(err, session.ErrAttentionRequiresAction), errors.Is(err, session.ErrAttentionChanged):
		return http.StatusConflict, true
	case errors.Is(err, session.ErrAttentionSeenUnavailable):
		return http.StatusServiceUnavailable, true
	default:
		return 0, false
	}
}

func attentionSeenErrorCode(err error) string {
	switch {
	case errors.Is(err, session.ErrAttentionRequiresAction):
		return "attention_requires_action"
	case errors.Is(err, session.ErrAttentionChanged):
		return "attention_changed"
	default:
		return ""
	}
}
