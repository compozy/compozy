package observe

import (
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/events"
)

func observedEventContent(event acp.AgentEvent) []byte {
	switch strings.TrimSpace(event.Type) {
	case events.TranscriptMarkerCreated,
		events.TranscriptMarkerRedacted,
		events.SessionStreamSnapshotServed,
		events.SessionStreamSubscribed,
		events.SessionStreamOverflowFallback:
		return acp.CloneRawMessage(event.Raw)
	default:
		return nil
	}
}
