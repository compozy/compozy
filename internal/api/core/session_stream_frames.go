package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

func (h *BaseHandlers) writeGoalSnapshotChangedEvents(
	ctx context.Context,
	writer FlushWriter,
	sessionID string,
	afterSequence int64,
	events []store.SessionEvent,
) error {
	if events == nil {
		latest, err := h.Sessions.LatestSessionEventByType(ctx, sessionID, session.EventTypeGoalSnapshotChanged)
		if err != nil {
			return fmt.Errorf("query latest Goal snapshot event: %w", err)
		}
		if latest != nil {
			events = []store.SessionEvent{*latest}
		}
	}
	for _, event := range events {
		if event.Sequence <= afterSequence || event.Type != session.EventTypeGoalSnapshotChanged {
			continue
		}
		content := []byte(event.Content)
		if !json.Valid(content) {
			return fmt.Errorf("decode Goal snapshot event %d: invalid JSON", event.Sequence)
		}
		if err := WriteSSE(writer, SSEMessage{
			ID:   strconv.FormatInt(event.Sequence, 10),
			Name: contract.SessionStreamEventGoalSnapshotChanged,
			Data: json.RawMessage(append([]byte(nil), content...)),
		}); err != nil {
			return fmt.Errorf("write Goal snapshot event %d: %w", event.Sequence, err)
		}
	}
	return nil
}
