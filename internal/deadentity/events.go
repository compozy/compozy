package deadentity

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
)

type transitionContent struct {
	Kind     store.DeadEntityKind `json:"kind"`
	EntityID string               `json:"entity_id"`
	Reason   string               `json:"reason,omitempty"`
	MarkedAt string               `json:"marked_at,omitempty"`
}

func (s *Service) emitTransition(ctx context.Context, entity store.DeadEntity, marked bool) {
	if s.events == nil {
		return
	}
	eventType := events.DeadEntityCleared
	summary := fmt.Sprintf("%s %s recovered", entity.Kind, entity.EntityID)
	if marked {
		eventType = events.DeadEntityMarked
		summary = fmt.Sprintf("%s %s marked dead", entity.Kind, entity.EntityID)
	}
	content, err := json.Marshal(transitionContent{
		Kind:     entity.Kind,
		EntityID: entity.EntityID,
		Reason:   entity.Reason,
		MarkedAt: store.FormatTimestamp(entity.MarkedAt),
	})
	if err != nil {
		s.logger.Warn(
			"deadentity: marshal transition event failed",
			"type", eventType,
			"profile_id", entity.ProfileID,
			"workspace_id", entity.WorkspaceID,
			"kind", entity.Kind,
			"entity_id", entity.EntityID,
			"error", err,
		)
		return
	}
	// The durable transition already committed, so caller cancellation must not drop its event.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.transitionEventTimeout)
	defer cancel()
	event := store.EventSummary{
		ProfileID:   entity.ProfileID,
		WorkspaceID: entity.WorkspaceID,
		Type:        eventType,
		Outcome:     string(events.OutcomeFor(eventType)),
		Summary:     summary,
		Timestamp:   s.now().UTC(),
	}
	event.SetContent(content)
	if err := s.events.WriteEventSummary(writeCtx, event); err != nil {
		s.logger.Warn(
			"deadentity: write transition event failed open",
			"type", eventType,
			"profile_id", entity.ProfileID,
			"workspace_id", entity.WorkspaceID,
			"kind", entity.Kind,
			"entity_id", entity.EntityID,
			"error", err,
		)
	}
}
