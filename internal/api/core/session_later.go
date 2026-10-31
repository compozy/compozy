package core

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

// laterOperationTimeout bounds a Move to Later that keeps running after the
// client disconnects: stopping a session tree and its processes outlives requests.
const laterOperationTimeout = 3 * time.Minute

// SessionLaterManager parks sessions in Later and returns them to the normal flow.
type SessionLaterManager interface {
	MoveToLater(
		ctx context.Context,
		workspaceID string,
		sessionID string,
		req session.LaterRequest,
	) (*session.Info, error)
	ResumeFromLater(ctx context.Context, workspaceID string, sessionID string, withChildren bool) (*session.Info, error)
}

// MoveSessionToLater stops a session tree and parks it in Later.
func (h *BaseHandlers) MoveSessionToLater(c *gin.Context) {
	var req contract.MoveSessionToLaterRequest
	if err := decodeOptionalJSONBody(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	h.changeSessionLater(
		c,
		func(ctx context.Context, manager SessionLaterManager, ws, id string) (*session.Info, error) {
			return manager.MoveToLater(ctx, ws, id, session.LaterRequest{Reason: req.Reason, ReturnAt: req.ReturnAt})
		},
	)
}

// ResumeSessionFromLater returns a parked session to the normal flow without
// starting its runtime.
func (h *BaseHandlers) ResumeSessionFromLater(c *gin.Context) {
	var req contract.ResumeSessionFromLaterRequest
	if err := decodeOptionalJSONBody(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	h.changeSessionLater(
		c,
		func(ctx context.Context, manager SessionLaterManager, ws, id string) (*session.Info, error) {
			return manager.ResumeFromLater(ctx, ws, id, req.WithChildren)
		},
	)
}

func (h *BaseHandlers) changeSessionLater(
	c *gin.Context,
	change func(context.Context, SessionLaterManager, string, string) (*session.Info, error),
) {
	manager, ok := h.Sessions.(SessionLaterManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session later manager is required"))
		return
	}
	scope, sessionID, existing, ok := h.routeSessionInWorkspace(c)
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
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), laterOperationTimeout)
	defer cancel()
	info, err := change(ctx, manager, scope.SessionWorkspaceID(), sessionID)
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
