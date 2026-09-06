package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/transcript"
	"github.com/gin-gonic/gin"
)

// SessionNavigationReader is the session-scoped materialized navigation capability.
type SessionNavigationReader interface {
	TranscriptSearch(context.Context, string, transcript.SearchQuery) (transcript.SearchResult, error)
	TranscriptOutline(context.Context, string) (transcript.OutlineResult, error)
}

// SessionTranscriptSearch searches retained projected messages using literal text.
func (h *BaseHandlers) SessionTranscriptSearch(c *gin.Context) {
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	query, err := (transcript.SearchQuery{Query: c.Query("q"), Limit: limit}).Normalize()
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	_, id, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	reader, ok := h.Sessions.(SessionNavigationReader)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("session navigation is unavailable"))
		return
	}
	result, err := reader.TranscriptSearch(c.Request.Context(), id, query)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// SessionTranscriptOutline returns the full retained operator-message trail.
func (h *BaseHandlers) SessionTranscriptOutline(c *gin.Context) {
	_, id, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	reader, ok := h.Sessions.(SessionNavigationReader)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("session navigation is unavailable"))
		return
	}
	result, err := reader.TranscriptOutline(c.Request.Context(), id)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusOK, result)
}
