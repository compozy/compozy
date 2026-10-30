package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

// ArchiveSession removes one session from default catalog listings. A live
// session is refused unless the request opts into stopping it first.
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
	if existing.Lineage != nil && existing.Lineage.SpawnRole == "subagent" {
		subagentError(
			c,
			http.StatusConflict,
			"subagent_archive_follows_parent",
			"Subagent sessions are archived with their parent session.",
		)
		return
	}
	policy := contract.SessionProcessPolicy(c.Query("processes"))
	if archived && !h.prepareProcessPolicy(c, policy) {
		return
	}
	var info *session.Info
	if archived {
		var stop bool
		stop, err = parseBoolQuery(c, "stop")
		if err != nil {
			h.respondError(c, http.StatusBadRequest, err)
			return
		}
		archive := archiver.Archive
		if stop {
			archive = archiver.ArchiveWithStop
		}
		info, err = archive(c.Request.Context(), scope.SessionWorkspaceID(), sessionID)
	} else {
		info, err = archiver.Unarchive(c.Request.Context(), scope.SessionWorkspaceID(), sessionID)
	}
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	if archived && !h.finishProcessPolicy(c, existing, policy) {
		return
	}
	payload, err := h.sessionPayloadWithOptionalHealth(c.Request.Context(), info, false)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SessionResponse{Session: payload})
}
