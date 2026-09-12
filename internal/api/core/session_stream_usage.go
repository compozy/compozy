package core

import (
	"cmp"
	"context"
	"fmt"
	"slices"

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
) error {
	if events == nil {
		events = make([]store.SessionEvent, 0)
		for _, kind := range []string{acp.EventTypeUsage, acp.EventTypeDone, acp.EventTypePromptDelivery} {
			rows, err := h.Sessions.Events(ctx, sessionID, store.EventQuery{Type: kind, AfterSequence: afterSequence})
			if err != nil {
				return fmt.Errorf("query session usage changes: %w", err)
			}
			events = append(events, rows...)
		}
		slices.SortFunc(events, func(a, b store.SessionEvent) int { return cmp.Compare(a.Sequence, b.Sequence) })
	}
	for _, event := range events {
		if event.Sequence <= afterSequence {
			continue
		}
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
			return fmt.Errorf("write session usage change %d: %w", event.Sequence, err)
		}
	}
	return nil
}
