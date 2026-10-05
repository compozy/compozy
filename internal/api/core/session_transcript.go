package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/transcript"
	"github.com/gin-gonic/gin"
)

// SessionTranscript returns one bounded materialized transcript page.
func (h *BaseHandlers) SessionTranscript(c *gin.Context) {
	query, err := parseSessionTranscriptQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	sessionID, info, ok := h.routeSessionRead(c)
	if !ok {
		return
	}
	page, err := h.Sessions.TranscriptPage(c.Request.Context(), sessionID, query)
	if err != nil {
		h.logSessionReadError("transcript", sessionID, err)
		h.respondError(c, StatusForSessionError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.SessionTranscriptResponse{
		Entries:            transcript.CloneEntries(page.Entries),
		Epoch:              transcriptEpoch(info),
		Generation:         page.Generation,
		MaxSequence:        page.MaxSequence,
		HasOlder:           page.HasOlder,
		NextBeforeSequence: page.NextBeforeSequence,
		Limit:              query.Limit,
	})
}

// routeSessionRead keeps Global reads fenced by workspace-free ownership and profile.
func (h *BaseHandlers) routeSessionRead(c *gin.Context) (string, *session.Info, bool) {
	if strings.TrimSpace(c.Param("workspace_id")) != "" {
		_, id, info, ok := h.routeSessionInWorkspace(c)
		return id, info, ok
	}
	id := strings.TrimSpace(c.Param("session_id"))
	if id == "" {
		h.respondError(c, http.StatusBadRequest, errors.New("session_id path is required"))
		return "", nil, false
	}
	info, err := h.requireSessionInWorkspace(c.Request.Context(), "", id)
	if err != nil {
		h.respondError(c, statusForWorkspaceScopedResourceError(err), err)
		return "", nil, false
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return "", nil, false
	}
	if !h.requireSessionInProfile(c, info, scope) {
		return "", nil, false
	}
	return id, info, true
}
