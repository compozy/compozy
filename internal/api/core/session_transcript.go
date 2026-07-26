package core

import (
	"net/http"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/transcript"
	"github.com/gin-gonic/gin"
)

// SessionTranscript returns one bounded materialized transcript page.
func (h *BaseHandlers) SessionTranscript(c *gin.Context) {
	query, err := parseSessionTranscriptQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	_, sessionID, info, ok := h.routeSessionInWorkspace(c)
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
