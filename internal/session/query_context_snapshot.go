package session

import (
	"context"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/session/contextusage"
	"github.com/compozy/compozy/internal/store"
)

type ContextEvents struct {
	WorkTime    contextusage.WorkTime
	Usage       []UsageEventEnvelope
	Deliveries  []DeliveryEventEnvelope
	Compactions []CompactionEnvelope
	Settled     SettledTurn
	ClearedBy   *contextusage.ClearedBy
}

// ContextEvents derives context history and its boundaries from one ledger snapshot.
func (m *Manager) ContextEvents(ctx context.Context, id string) (ContextEvents, error) {
	rows, err := m.Events(ctx, id, store.EventQuery{Types: []string{
		acp.EventTypeUsage, acp.EventTypeDone, acp.EventTypePromptDelivery,
		acp.EventTypeError, events.SessionTurnQuiesced,
		events.SessionCompactionFired, acp.EventTypeCompaction,
	}})
	if err != nil {
		return ContextEvents{}, err
	}
	return contextEventsFromRows(rows)
}

func contextEventsFromRows(rows []store.SessionEvent) (ContextEvents, error) {
	var result ContextEvents
	usage, err := appendUsageEventsFromRows(make([]UsageEventEnvelope, 0), rows)
	if err != nil {
		return result, err
	}
	deliveries, err := appendDeliveryEventsFromRows(make([]DeliveryEventEnvelope, 0), rows)
	if err != nil {
		return result, err
	}
	hasCompaction := false
	for _, row := range rows {
		if row.Type == acp.EventTypeDone {
			result.Settled = SettledTurn{TurnID: row.TurnID, Sequence: row.Sequence}
		}
		hasCompaction = hasCompaction || row.Type == acp.EventTypeCompaction
	}
	if hasCompaction {
		result.Compactions, err = compactionMarkersFromRows(rows)
		if err != nil {
			return ContextEvents{}, err
		}
		result.ClearedBy, err = compactionClearBoundaryFromRows(rows)
		if err != nil {
			return ContextEvents{}, err
		}
	}
	result.Usage, result.Deliveries = usage, deliveries
	result.WorkTime = sessionWorkTime(rows, deliveries)
	return result, nil
}
