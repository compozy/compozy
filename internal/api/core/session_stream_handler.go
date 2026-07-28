package core

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// StreamSession streams session events over SSE.
func (h *BaseHandlers) StreamSession(c *gin.Context) {
	if err := rejectTranscriptBackwardCursor(c); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	streamOptions, err := parseSessionStreamOptions(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	_, sessionID, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}

	query, err := h.parseSessionStreamEventQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	query, err = normalizeSessionStreamQuery(query, streamOptions)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	subscription, err := h.subscribeSessionEventStream(
		c.Request.Context(),
		sessionID,
		query.AfterSequence,
		streamOptions.frameMode,
	)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	initial, err := h.sessionStreamInitialEvents(c.Request.Context(), sessionID, query, streamOptions)
	if err != nil {
		subscription.cancelIfActive()
		h.respondError(c, StatusForSessionError(err), err)
		return
	}

	writer, err := PrepareSSE(c)
	if err != nil {
		subscription.cancelIfActive()
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	closeTelemetry := h.beginSessionStreamTelemetry(
		c.Request.Context(),
		sessionID,
		info,
		query,
		len(initial),
		streamOptions,
		subscription.active(),
	)
	defer closeTelemetry()

	h.streamSessionWithMode(c, writer, sessionID, info, query, initial, streamOptions, subscription)
}
