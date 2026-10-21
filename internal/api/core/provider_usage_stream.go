package core

import (
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

// StreamProviderUsage emits cache invalidations, never credentials or account data.
func (h *BaseHandlers) StreamProviderUsage(c *gin.Context) {
	profile, ok := h.providerUsageProfile(c)
	if !ok || !h.requireProviderUsage(c) {
		return
	}
	updates, cancel := h.ProviderUsage.Subscribe(profile)
	defer cancel()
	envelope, err := h.ProviderUsage.Envelope(c.Request.Context(), profile)
	if err != nil {
		h.respondProviderUsageError(c, err)
		return
	}
	writer, closeStream, ok := h.prepareEventStream(c)
	if !ok {
		return
	}
	defer closeStream()
	bootID := h.ProviderUsage.BootID()
	if err := WriteSSE(writer, SSEMessage{Name: "ready", Data: contract.ProviderUsageStreamEvent{Revision: envelope.Revision, BootID: bootID}}); err != nil {
		return
	}
	ticker := time.NewTicker(eventStreamPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case <-ticker.C:
			if err := WriteSSEComment(writer, "heartbeat"); err != nil {
				return
			}
		case revision, open := <-updates:
			if !open {
				return
			}
			if revision.BootID == bootID && revision.Revision <= envelope.Revision {
				continue
			}
			envelope.Revision, bootID = revision.Revision, revision.BootID
			if err := WriteSSE(writer, SSEMessage{Name: "provider_usage_changed", Data: contract.ProviderUsageStreamEvent{Revision: revision.Revision, BootID: revision.BootID}}); err != nil {
				return
			}
		}
	}
}
