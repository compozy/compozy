package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

// ArchiveSession removes one stopped session from default catalog listings.
func (h *BaseHandlers) ArchiveSession(c *gin.Context) {
	h.setSessionArchived(c, true)
}

// UnarchiveSession restores one session to default catalog listings.
func (h *BaseHandlers) UnarchiveSession(c *gin.Context) {
	h.setSessionArchived(c, false)
}

func (h *BaseHandlers) setSessionArchived(c *gin.Context, archived bool) {
	archiver, ok := h.Sessions.(SessionArchiveManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session archive manager is required"))
		return
	}
	scope, sessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	var (
		info *session.Info
		err  error
	)
	if archived {
		info, err = archiver.Archive(c.Request.Context(), scope.SessionWorkspaceID(), sessionID)
	} else {
		info, err = archiver.Unarchive(c.Request.Context(), scope.SessionWorkspaceID(), sessionID)
	}
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SessionResponse{Session: SessionPayloadFromInfo(info)})
}
