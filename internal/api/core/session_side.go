package core

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

type SessionSideManager interface {
	OpenSide(context.Context, session.OpenSideOpts) (session.DeriveResult, error)
	ListSides(context.Context, string, string, bool) ([]*session.Info, error)
}

func (h *BaseHandlers) OpenSessionSide(c *gin.Context) {
	var req contract.OpenSessionSideRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondDeriveError(c, http.StatusBadRequest, invalidDeriveRequest(err))
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		h.respondDeriveError(c, http.StatusBadRequest, session.ErrSideMessageRequired)
		return
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		h.respondDeriveError(c, http.StatusBadRequest, invalidDeriveRequest(fmt.Errorf("idempotency_key is required")))
		return
	}
	scope, sessionID, _, ok := h.deriveRoute(c)
	if !ok {
		return
	}
	manager, ok := h.Sessions.(SessionSideManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errDeriveUnavailable)
		return
	}
	result, err := manager.OpenSide(c.Request.Context(), session.OpenSideOpts{
		SourceSessionID: sessionID, WorkspaceID: scope.SessionWorkspaceID(), ProfileID: scope.Profile.ProfileID,
		Message: req.Message, Name: req.Name, IdempotencyKey: req.IdempotencyKey,
	})
	h.respondDerive(c, result, err)
}

func (h *BaseHandlers) ListSessionSides(c *gin.Context) {
	scope, sessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	archived := false
	if value, exists := c.GetQuery("archived"); exists {
		var err error
		archived, err = strconv.ParseBool(value)
		if err != nil {
			h.respondError(c, http.StatusBadRequest, err)
			return
		}
	}
	manager, ok := h.Sessions.(SessionSideManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errDeriveUnavailable)
		return
	}
	sides, err := manager.ListSides(c.Request.Context(), scope.SessionWorkspaceID(), sessionID, archived)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusOK, SessionSidesPayload(sides))
}

func SessionSidesPayload(sides []*session.Info) contract.SessionSidesResponse {
	result := contract.SessionSidesResponse{Sides: []contract.SessionSideItemPayload{}}
	for _, side := range sides {
		result.Sides = append(result.Sides, contract.SessionSideItemPayload{
			ID: side.ID, Name: side.Name, State: string(side.State), Prompting: side.Prompting,
			PendingPermissionCount: side.PendingPermissionCount, PendingClarifyCount: side.PendingClarifyCount,
			ArchivedAt: side.ArchivedAt, CreatedAt: side.CreatedAt, UpdatedAt: side.UpdatedAt,
		})
	}
	return result
}
