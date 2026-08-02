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

func (s *Service) emitTransition(ctx context.Context, entity store.DeadEntity, marked bool) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if s.events == nil {
		return nil
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
		s.logger.Warn("deadentity: marshal transition event failed", "type", eventType, "error", err)
		return nil
	}
	if err := s.events.WriteEventSummary(ctx, store.EventSummary{
		WorkspaceID: entity.WorkspaceID,
		Type:        eventType,
		Outcome:     string(events.OutcomeFor(eventType)),
		Content:     content,
		Summary:     summary,
		Timestamp:   s.now().UTC(),
	}); err != nil {
		if ctxErr := contextError(ctx); ctxErr != nil {
			return ctxErr
		}
		s.logger.Warn(
			"deadentity: write transition event failed open",
			"type", eventType,
			"workspace_id", entity.WorkspaceID,
			"kind", entity.Kind,
			"entity_id", entity.EntityID,
			"error", err,
		)
	}
	return contextError(ctx)
}
