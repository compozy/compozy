package core

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

// GetSessionOwner returns the minimal workspace ownership projection for one session.
func (h *BaseHandlers) GetSessionOwner(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("%s: session_id path is required", h.transportName()))
		return
	}

	info, err := h.Sessions.Status(c.Request.Context(), sessionID)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}

	workspace, err := h.Workspaces.Get(c.Request.Context(), info.WorkspaceID)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, fmt.Errorf("resolve session owner workspace: %w", err))
		return
	}

	c.JSON(http.StatusOK, contract.SessionOwner{
		SessionID:     strings.TrimSpace(info.ID),
		WorkspaceID:   strings.TrimSpace(workspace.ID),
		WorkspaceName: strings.TrimSpace(workspace.Name),
	})
}
