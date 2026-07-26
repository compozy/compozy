package core

import (
	"errors"
	"net/http"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	"github.com/gin-gonic/gin"
)

const sessionCatalogChangedEvent = "session_catalog_changed"

// StreamSessionCatalog emits workspace-identified wake signals for catalog reconciliation.
func (h *BaseHandlers) StreamSessionCatalog(c *gin.Context) {
	subscriber, ok := h.Sessions.(SessionCatalogEventSubscriber)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session catalog stream is required"))
		return
	}
	events, cancel, err := subscriber.SubscribeSessionCatalogEvents(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	defer cancel()

	writer, err := PrepareSSE(c)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	if err := WriteSSEComment(writer, "session catalog stream ready"); err != nil {
		h.logSSEWriteFailure(sessionCatalogChangedEvent, err)
		return
	}

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case event, open := <-events:
			if !open {
				return
			}
			if err := WriteSSE(writer, SSEMessage{
				Name: sessionCatalogChangedEvent,
				Data: sessionCatalogEventPayload(event),
			}); err != nil {
				h.logSSEWriteFailure(sessionCatalogChangedEvent, err)
				return
			}
		}
	}
}

func sessionCatalogEventPayload(event session.CatalogEvent) contract.SessionCatalogEventPayload {
	return contract.SessionCatalogEventPayload{
		Kind:        string(event.Kind),
		WorkspaceID: event.WorkspaceID,
		SessionID:   event.SessionID,
	}
}
