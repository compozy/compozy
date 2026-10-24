package session

import (
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/session/contextusage"
	"github.com/compozy/compozy/internal/store"
)

func sessionWorkTime(rows []store.SessionEvent, deliveries []DeliveryEventEnvelope) contextusage.WorkTime {
	starts := make(map[string]time.Time)
	for _, delivery := range deliveries {
		at := delivery.Manifest.SentAt
		if at.IsZero() {
			at = delivery.At
		}
		if previous := starts[delivery.Manifest.TurnID]; previous.IsZero() || at.Before(previous) {
			starts[delivery.Manifest.TurnID] = at
		}
	}
	ends := make(map[string]store.SessionEvent)
	for _, row := range rows {
		if row.Type == acp.EventTypeDone || row.Type == acp.EventTypeError || row.Type == events.SessionTurnQuiesced {
			ends[row.TurnID] = row
		}
	}
	var result contextusage.WorkTime
	var latest int64
	for id, end := range ends {
		start := starts[id]
		if id == "" || start.IsZero() || end.Timestamp.Before(start) {
			continue
		}
		result.ActiveDurationMS += end.Timestamp.Sub(start).Milliseconds()
		result.TurnCount++
		if end.Sequence > latest {
			latest = end.Sequence
			result.LastCompletedTurnID = id
		}
	}
	return result
}
