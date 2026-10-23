package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

const sessionReparentCodePrefix = "session_reparent_"

// SessionReparentManager owns adopting and detaching user session provenance parents.
type SessionReparentManager interface {
	AdoptSession(
		ctx context.Context, workspaceID string, sessionID string, parentSessionID string,
	) (*session.Info, error)
	DetachSession(ctx context.Context, workspaceID string, sessionID string) (*session.Info, error)
}

// SessionReparentErrorCode returns the stable error code of a lineage-rule refusal.
func SessionReparentErrorCode(err error) string {
	refusal, ok := errors.AsType[*session.ReparentError](err)
	if !ok || refusal == nil {
		return ""
	}
	return sessionReparentCodePrefix + string(refusal.Reason)
}

// SetSessionParent adopts a parentless user session as a child of another session.
func (h *BaseHandlers) SetSessionParent(c *gin.Context) {
	h.reparentSession(c, true)
}

// ClearSessionParent makes an adopted child session top-level again.
func (h *BaseHandlers) ClearSessionParent(c *gin.Context) {
	h.reparentSession(c, false)
}

func (h *BaseHandlers) reparentSession(c *gin.Context, adopt bool) {
	manager, ok := h.Sessions.(SessionReparentManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session reparent manager is required"))
		return
	}
	scope, sessionID, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	profileScope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if !h.requireSessionInProfile(c, info, profileScope) {
		return
	}
	if adopt {
		var req contract.SetSessionParentRequest
		if err := decodeStrictJSONBody(c, &req); err != nil {
			h.respondError(
				c,
				http.StatusBadRequest,
				fmt.Errorf("%s: decode session parent request: %w", h.transportName(), err),
			)
			return
		}
		info, err = manager.AdoptSession(c.Request.Context(), scope.SessionWorkspaceID(), sessionID, req.ParentSessionID)
	} else {
		info, err = manager.DetachSession(c.Request.Context(), scope.SessionWorkspaceID(), sessionID)
	}
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
