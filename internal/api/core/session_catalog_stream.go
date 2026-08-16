package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

const sessionCatalogChangedEvent = "session_catalog_changed"

// StreamSessionCatalog emits workspace-identified wake signals for catalog reconciliation.
func (h *BaseHandlers) StreamSessionCatalog(c *gin.Context) {
	if !h.requireOperatorSurface(c, "session catalog stream") {
		return
	}
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
			name, data := sessionCatalogSSEMessage(event)
			if err := WriteSSE(writer, SSEMessage{
				Name: name,
				Data: data,
			}); err != nil {
				h.logSSEWriteFailure(name, err)
				return
			}
		}
	}
}

func sessionCatalogSSEMessage(event session.CatalogEvent) (string, any) {
	if event.Name == session.CatalogEventNameAttention && event.Attention != nil {
		return string(session.CatalogEventNameAttention), contract.SessionAttentionEventPayload{
			SessionID: event.Attention.SessionID, WorkspaceID: event.Attention.WorkspaceID,
			From: event.Attention.From, To: event.Attention.To,
			Class: event.Attention.Class, At: event.Attention.At,
		}
	}
	return sessionCatalogChangedEvent, sessionCatalogEventPayload(event)
}

func sessionCatalogEventPayload(event session.CatalogEvent) contract.SessionCatalogEventPayload {
	return contract.SessionCatalogEventPayload{
		Kind:        string(event.Kind),
		WorkspaceID: event.WorkspaceID,
		SessionID:   event.SessionID,
	}
}
