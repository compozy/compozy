package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

// SessionPinManager pins sessions above the session list and unpins them.
type SessionPinManager interface {
	Pin(ctx context.Context, workspaceID string, sessionID string) (*session.Info, error)
	Unpin(ctx context.Context, workspaceID string, sessionID string) (*session.Info, error)
}

// PinSession pins a session above the session list; pinning it again changes nothing.
func (h *BaseHandlers) PinSession(c *gin.Context) {
	h.changeSessionPin(c, SessionPinManager.Pin)
}

// UnpinSession returns a pinned session to its place in the list; an unpinned session is unchanged.
func (h *BaseHandlers) UnpinSession(c *gin.Context) {
	h.changeSessionPin(c, SessionPinManager.Unpin)
}

func (h *BaseHandlers) changeSessionPin(
	c *gin.Context,
	change func(SessionPinManager, context.Context, string, string) (*session.Info, error),
) {
	manager, ok := h.Sessions.(SessionPinManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session pin manager is required"))
		return
	}
	scope, sessionID, _, ok := h.routeScopedSessionInWorkspace(c)
	if !ok {
		return
	}
	info, err := change(manager, c.Request.Context(), scope.SessionWorkspaceID(), sessionID)
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

func statusForSessionPinError(err error) (int, bool) {
	switch {
	case errors.Is(err, session.ErrSubagentPinFollowsParent), errors.Is(err, session.ErrSessionPinInstanceMain):
		return http.StatusConflict, true
	case errors.Is(err, session.ErrSessionPinUnavailable):
		return http.StatusServiceUnavailable, true
	default:
		return 0, false
	}
}

func sessionPinErrorCode(err error) string {
	switch {
	case errors.Is(err, session.ErrSubagentPinFollowsParent):
		return "subagent_pin_follows_parent"
	case errors.Is(err, session.ErrSessionPinInstanceMain):
		return "session_pin_instance_main"
	default:
		return ""
	}
}
