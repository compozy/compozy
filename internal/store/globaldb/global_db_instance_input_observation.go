package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// Observe only committed rows. Claim eligibility remains entirely in the queue transaction.
func (g *SessionRepo) observeHeldInstanceInput(ctx context.Context, entry *store.SessionInputQueueEntry) {
	hold, err := g.readQueries.GetSessionInputHold(ctx, entry.SessionID)
	if err == nil && queueEntryHeldByPause(entry, hold.PauseReason) {
		slog.InfoContext(ctx, "input.held", "instance_id", hold.ID, "reason", hold.PauseReason,
			"session_id", entry.SessionID, "input_id", entry.ID)
	}
}

func queueEntryHeldByPause(entry *store.SessionInputQueueEntry, reason string) bool {
	switch reason {
	case "daily_budget", "daily_budget_recovery":
		return !entry.HumanIngress || entry.AppIngress
	default:
		return !store.IsHumanSessionInputOriginClass(entry.OriginClass)
	}
}

func sessionInputHeldInTransaction(
	ctx context.Context, queries *sqlcgen.Queries, entry *store.SessionInputQueueEntry,
) (bool, error) {
	if store.IsHumanSessionInputOriginClass(entry.OriginClass) && entry.HumanIngress && !entry.AppIngress {
		return false, nil
	}
	hold, err := queries.GetSessionInputHold(ctx, entry.SessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil && queueEntryHeldByPause(entry, hold.PauseReason), err
}

func observeReleasedInstanceInput(ctx context.Context, entry *store.SessionInputQueueEntry) {
	if entry.OriginClass != store.SessionInputOriginClassHuman {
		slog.InfoContext(ctx, "input.released", "session_id", entry.SessionID, "input_id", entry.ID)
	}
}

// SessionInputHold reads the oldest queued input blocked by an instance pause.
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
