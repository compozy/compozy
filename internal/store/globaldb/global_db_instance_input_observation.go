package globaldb

import (
	"context"
	"log/slog"

	"github.com/compozy/compozy/internal/store"
)

// Observe only committed rows. Claim eligibility remains entirely in the queue transaction.
func (g *SessionRepo) observeHeldInstanceInput(ctx context.Context, entry *store.SessionInputQueueEntry) {
	if entry.OriginClass == store.SessionInputOriginClassHuman {
		return
	}
	caller, err := g.queries.GetInstanceCaller(ctx, entry.SessionID)
	if err != nil {
		return
	}
	id, ok := caller.OriginatingInstanceID.(string)
	if !ok || id == "" {
		return
	}
	instance, err := g.queries.GetAgentInstance(ctx, id)
	if err == nil && instance.State == goalStatusPaused && !instance.ArchivedAt.Valid {
		slog.InfoContext(ctx, "input.held", "instance_id", id, "session_id", entry.SessionID, "input_id", entry.ID)
	}
}

func observeReleasedInstanceInput(ctx context.Context, entry *store.SessionInputQueueEntry) {
	if entry.OriginClass != store.SessionInputOriginClassHuman {
		slog.InfoContext(ctx, "input.released", "session_id", entry.SessionID, "input_id", entry.ID)
	}
}
