package globaldb

import (
	"context"
	"encoding/json"
	"fmt"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/worktree"
)

func (g *GlobalDB) PublishWorktreeEvent(ctx context.Context, event worktree.LifecycleEvent) error {
	if g == nil || g.ObserveRepo == nil {
		return fmt.Errorf("store: worktree event writer is required")
	}
	outcome := string(eventspkg.OutcomeFor(event.Name))
	content := append(json.RawMessage(nil), event.Payload...)
	return g.ObserveRepo.WriteEventSummary(ctx, store.EventSummary{
		WorkspaceID: event.WorkspaceID,
		Type:        event.Name,
		Outcome:     outcome,
		Content:     content,
		EventCorrelation: store.EventCorrelation{
			WorktreeID: event.WorktreeID,
			RunID:      event.RunID,
		},
	})
}

var _ worktree.EventSink = (*GlobalDB)(nil)
