package core

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

// RenameSession changes one user session's durable display name.
func (h *BaseHandlers) RenameSession(c *gin.Context) {
	manager, ok := h.Sessions.(SessionRenameManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session rename manager is required"))
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
	var req contract.RenameSessionRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode session rename request: %w", h.transportName(), err),
		)
		return
	}
	info, err = manager.Rename(
		c.Request.Context(),
		scope.SessionWorkspaceID(),
		sessionID,
		req.Name,
	)
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
