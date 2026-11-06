package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/compozy/compozy/internal/store"
)

// Observe only committed rows. Claim eligibility remains entirely in the queue transaction.
func (g *SessionRepo) observeHeldInstanceInput(ctx context.Context, entry *store.SessionInputQueueEntry) {
	if entry.OriginClass == store.SessionInputOriginClassHuman {
		return
	}
	hold, err := g.readQueries.GetSessionInputHold(ctx, entry.SessionID)
	if err == nil {
		slog.InfoContext(ctx, "input.held", "instance_id", hold.ID, "reason", hold.PauseReason,
			"session_id", entry.SessionID, "input_id", entry.ID)
	}
}

func observeReleasedInstanceInput(ctx context.Context, entry *store.SessionInputQueueEntry) {
	if entry.OriginClass != store.SessionInputOriginClassHuman {
		slog.InfoContext(ctx, "input.released", "session_id", entry.SessionID, "input_id", entry.ID)
	}
}

func (g *SessionRepo) SessionInputHold(ctx context.Context, sessionID string) (*store.SessionInputHold, error) {
	if err := g.checkReady(ctx, "read session input hold"); err != nil {
		return nil, err
	}
	row, err := g.readQueries.GetHeldSessionInput(ctx, sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	since, err := store.ParseTimestamp(row.Since)
	if err != nil {
		return nil, err
	}
	return &store.SessionInputHold{InstanceID: row.ID, Reason: row.PauseReason, Since: since}, nil
}
