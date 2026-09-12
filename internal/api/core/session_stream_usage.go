package core

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
)

func (h *BaseHandlers) writeUsageChangedEvents(
	ctx context.Context,
	writer FlushWriter,
	sessionID string,
	afterSequence int64,
	events []store.SessionEvent,
) (int64, error) {
	if events == nil {
		var err error
		events, err = h.Sessions.Events(ctx, sessionID, store.EventQuery{AfterSequence: afterSequence})
		if err != nil {
			return afterSequence, fmt.Errorf("query session usage changes: %w", err)
		}
	}
	cursor := afterSequence
	for _, event := range events {
		if event.Sequence <= afterSequence {
			continue
		}
		cursor = max(cursor, event.Sequence)
		switch event.Type {
		case acp.EventTypeUsage, acp.EventTypeDone, acp.EventTypePromptDelivery:
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
