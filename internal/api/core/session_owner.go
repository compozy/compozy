package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

// SessionOwnerReader resolves the catalog scope that owns a session without repairing it.
type SessionOwnerReader interface {
	SessionOwner(ctx context.Context, sessionID string) (store.SessionOwner, error)
}

// GetSessionOwner returns the minimal workspace ownership projection for one session.
// It never writes the session: an owner lookup (a permalink, or the CLI resolving a
// continue/fork source) must not repair an inactive session's metadata.
func (h *BaseHandlers) GetSessionOwner(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("%s: session_id path is required", h.transportName()))
		return
	}
	reader, ok := h.Sessions.(SessionOwnerReader)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session owner reader is required"))
		return
	}

	owner, err := reader.SessionOwner(c.Request.Context(), sessionID)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}

	if owner.WorkspaceID == "" {
		c.JSON(http.StatusOK, contract.SessionOwner{SessionID: owner.SessionID, WorkspaceName: "Global"})
		return
	}
	workspace, err := h.Workspaces.Get(c.Request.Context(), owner.WorkspaceID)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, fmt.Errorf("resolve session owner workspace: %w", err))
		return
	}

	c.JSON(http.StatusOK, contract.SessionOwner{
		SessionID:     strings.TrimSpace(owner.SessionID),
		WorkspaceID:   strings.TrimSpace(workspace.ID),
		WorkspaceName: strings.TrimSpace(workspace.Name),
	})
}
