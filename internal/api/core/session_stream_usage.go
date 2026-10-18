package core

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
)

func (h *BaseHandlers) writeUsageChangedEvents(
	ctx context.Context,
	writer FlushWriter,
	sessionID string,
	afterSequence int64,
	events []store.SessionEvent,
) (int64, error) {
	var metadata []store.EventMetadata
	if events == nil {
		var err error
		metadata, err = h.Sessions.EventMetadata(ctx, sessionID, store.EventQuery{AfterSequence: afterSequence})
		if err != nil {
			return afterSequence, fmt.Errorf("query session usage changes: %w", err)
		}
	} else {
		metadata = make([]store.EventMetadata, 0, len(events))
		for _, event := range events {
			metadata = append(metadata, store.EventMetadata{
				Sequence: event.Sequence, TurnID: event.TurnID, Type: event.Type,
			})
		}
	}
	cursor := afterSequence
	for _, event := range metadata {
		if event.Sequence <= afterSequence {
			continue
		}
		cursor = max(cursor, event.Sequence)
		switch event.Type {
		case acp.EventTypeUsage, acp.EventTypeDone, acp.EventTypePromptDelivery,
			acp.EventTypeCompaction, eventspkg.SessionCompactionFired:
		default:
			continue
		}
		if err := WriteSSE(
			writer,
			SSEMessage{
				Name: contract.SessionStreamEventUsageChanged,
				Data: contract.SessionUsageChangedPayload{
					Sequence: event.Sequence,
					TurnID:   event.TurnID,
					Kind:     event.Type,
				},
			},
		); err != nil {
			return afterSequence, fmt.Errorf("write session usage change %d: %w", event.Sequence, err)
		}
	}
	return cursor, nil
}
