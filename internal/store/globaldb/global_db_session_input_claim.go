package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ClaimNextSessionInput atomically leases the next eligible input for dispatch.
func (g *SessionRepo) ClaimNextSessionInput(
	ctx context.Context,
	sessionID string,
	now time.Time,
) (entry store.SessionInputQueueEntry, ok bool, err error) {
	return g.claimNextSessionInput(ctx, sessionID, "", now)
}

// ClaimSelectedSessionInput leases a row only if the queue head still matches the prior selection.
func (g *SessionRepo) ClaimSelectedSessionInput(
	ctx context.Context, sessionID, expectedID string, now time.Time,
) (store.SessionInputQueueEntry, bool, error) {
	return g.claimNextSessionInput(ctx, sessionID, expectedID, now)
}

func (g *SessionRepo) claimNextSessionInput(
	ctx context.Context, sessionID, expectedID string, now time.Time,
) (entry store.SessionInputQueueEntry, ok bool, err error) {
	if err := g.checkReady(ctx, "claim session input"); err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return store.SessionInputQueueEntry{}, false, errors.New("store: session id is required")
	}
	if now.IsZero() {
		now = g.now()
	}
	now = now.UTC()

	err = g.withImmediateTransaction(ctx, "claim session input", func(exec globalSQLExecutor) error {
		entry, ok = store.SessionInputQueueEntry{}, false // discard rolled-back retry results
		queries := sqlcgen.New(exec)
		row, scanErr := queries.GetNextDispatchableSessionInput(
			ctx,
			sqlcgen.GetNextDispatchableSessionInputParams{
				SessionID: target, QueuedStatus: store.SessionInputQueueStatusQueued,
			},
		)
		if errors.Is(scanErr, sql.ErrNoRows) {
			if expectedID != "" {
				return store.ErrSessionInputSelectionChanged
			}
			return freezeGoalInputBatch(ctx, queries, target)
		}
		if scanErr != nil {
			return scanErr
		}
		if expectedID != "" && row.ID != expectedID {
			return store.ErrSessionInputSelectionChanged
		}
		claimed, scanErr := sessionInputQueueFromGenerated(&row)
		if scanErr != nil {
			return scanErr
		}
		if !claimed.HumanIngress || claimed.AppIngress {
			allowed, trip, admissionErr := admitDailyBudgetTurnTx(ctx, queries, target, now)
			if admissionErr != nil {
				return admissionErr
			}
			if !allowed {
				// Commit the pause, but preserve the queue head, generation and
				// admission receipt untouched for an operator-authorized retry.
				entry.PausedInstance = trip
				return nil
			}
		}
		if err := freezeGoalInputBatch(ctx, queries, target); err != nil {
			return err
		}
		refreshed, claimErr := claimSessionInputInTransaction(
			ctx, exec, target, &claimed, now, store.FormatTimestamp(now),
		)
		if claimErr != nil {
			return claimErr
		}
		pauseTrip, countErr := advanceLineageUnattended(ctx, exec, &refreshed, g.unattendedTurnLimit.Load(), now)
		if countErr != nil {
			return countErr
		}
		refreshed.PausedInstance = pauseTrip
		entry = refreshed
		ok = true
		return nil
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	if ok {
		observeReleasedInstanceInput(ctx, &entry)
	}
	return entry, ok, nil
}

func freezeGoalInputBatch(ctx context.Context, queries *sqlcgen.Queries, sessionID string) error {
	if err := queries.FreezePreparedGoalInputBatch(ctx, sqlcgen.FreezePreparedGoalInputBatchParams{
		SessionID: sessionID, QueuedStatus: store.SessionInputQueueStatusQueued,
	}); err != nil {
		return fmt.Errorf("store: freeze prepared Goal input batch: %w", err)
	}
	return nil
}
