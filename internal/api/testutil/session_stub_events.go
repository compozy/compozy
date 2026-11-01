package testutil

import (
	"context"

	"github.com/compozy/compozy/internal/store"
)

func (s *StubSessionManager) EventMetadata(
	ctx context.Context,
	id string,
	query store.EventQuery,
) ([]store.EventMetadata, error) {
	if s.EventMetadataFn != nil {
		return s.EventMetadataFn(ctx, id, query)
	}
	events, err := s.Events(ctx, id, query)
	if err != nil {
		return nil, err
	}
	metadata := make([]store.EventMetadata, 0, len(events))
	for _, event := range events {
		metadata = append(metadata, store.EventMetadata{
			ID: event.ID, SessionID: event.SessionID, Sequence: event.Sequence, TurnID: event.TurnID,
			Type: event.Type, AgentName: event.AgentName, Timestamp: event.Timestamp,
		})
	}
	return metadata, nil
}
