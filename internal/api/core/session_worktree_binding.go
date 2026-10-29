package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

type SessionWorktreeBindingManager interface {
	ChangeWorktreeBinding(context.Context, string, string, *string) (*session.Info, error)
}

func (h *BaseHandlers) BindSessionWorktree(c *gin.Context) {
	var req contract.BindSessionWorktreeRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Worktree) == "" {
		h.respondError(c, http.StatusBadRequest, errors.New("worktree is required"))
		return
	}
	h.changeSessionWorktree(c, req.Worktree, req.ExpectedWorktreeID)
}

func (h *BaseHandlers) UnbindSessionWorktree(c *gin.Context) {
	var expected *string
	if values, ok := c.Request.URL.Query()["expected_worktree_id"]; ok {
		if len(values) != 1 {
			h.respondError(c, http.StatusBadRequest, errors.New("expected_worktree_id must occur once"))
			return
		}
		expected = &values[0]
	}
	h.changeSessionWorktree(c, "", expected)
}

func (h *BaseHandlers) changeSessionWorktree(c *gin.Context, ref string, expected *string) {
	_, id, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	manager, ok := h.Sessions.(SessionWorktreeBindingManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("session worktree binding is unavailable"))
		return
	}
	info, err := manager.ChangeWorktreeBinding(c.Request.Context(), id, ref, expected)
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
