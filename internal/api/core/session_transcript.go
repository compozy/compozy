package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/transcript"
	"github.com/gin-gonic/gin"
)

type inheritedTranscriptReader interface {
	InheritedTranscript(context.Context, *session.Info, transcript.PageQuery) (transcript.Page, int64, error)
}

// SessionInheritedTranscript returns the source range of an authorized derived child.
func (h *BaseHandlers) SessionInheritedTranscript(c *gin.Context) {
	query, err := parseSessionTranscriptQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	childID, child, ok := h.routeSessionRead(c)
	if !ok {
		return
	}
	reader, ok := h.Sessions.(inheritedTranscriptReader)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("inherited transcript reader is unavailable"))
		return
	}
	page, epoch, err := reader.InheritedTranscript(c.Request.Context(), child, query)
	if errors.Is(err, session.ErrInheritedTranscriptUnavailable) {
		c.JSON(http.StatusNotFound, contract.ErrorPayload{Error: err.Error(), Code: "inherited_transcript_unavailable"})
		return
	}
	if err != nil {
		h.logSessionReadError("inherited transcript", childID, err)
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	respondSessionTranscript(c, page, epoch, query.Limit)
}

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

	respondSessionTranscript(c, page, transcriptEpoch(info), query.Limit)
}

func respondSessionTranscript(c *gin.Context, page transcript.Page, epoch int64, limit int) {
	c.JSON(http.StatusOK, contract.SessionTranscriptResponse{
		Entries:            transcript.CloneEntries(page.Entries),
		Epoch:              epoch,
		Generation:         page.Generation,
		MaxSequence:        page.MaxSequence,
		HasOlder:           page.HasOlder,
		NextBeforeSequence: page.NextBeforeSequence,
		Limit:              limit,
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
