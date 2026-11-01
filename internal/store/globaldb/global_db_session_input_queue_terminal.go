package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// MarkSessionInputSent records successful dispatch for one queue entry.
func (g *SessionRepo) MarkSessionInputSent(
	ctx context.Context,
	sessionID string,
	entryID string,
	now time.Time,
) error {
	return g.MarkSessionInputSentInTurn(ctx, sessionID, entryID, "", now)
}

// MarkSessionInputSentInTurn records successful dispatch and the turn that consumed the entry; a
// batched dispatch stamps every entry with the one turn that carried them. An empty turn id keeps
// the stored one.
func (g *SessionRepo) MarkSessionInputSentInTurn(
	ctx context.Context,
	sessionID string,
	entryID string,
	turnID string,
	now time.Time,
) error {
	return g.updateSessionInputTerminal(
		ctx,
		"mark session input sent",
		sessionID,
		entryID,
		store.SessionInputQueueStatusSent,
		strings.TrimSpace(turnID),
		now,
	)
}

// ReleaseSessionInput returns a leased entry to the queued state after a dispatch race.
func (g *SessionRepo) ReleaseSessionInput(ctx context.Context, sessionID string, entryID string, now time.Time) error {
	if err := g.checkReady(ctx, "release session input"); err != nil {
		return err
	}
	target := strings.TrimSpace(sessionID)
	entryID = strings.TrimSpace(entryID)
	if target == "" || entryID == "" {
		return errors.New("store: session id and queue entry id are required")
	}
	if now.IsZero() {
		now = g.now()
	}
	nowRaw := store.FormatTimestamp(now.UTC())
	return g.withImmediateTransaction(ctx, "release session input", func(exec globalSQLExecutor) error {
		entry, getErr := getSessionInputQueueEntry(ctx, exec, target, entryID)
		if getErr != nil {
			return getErr
		}
		affected, releaseErr := sqlcgen.New(exec).ReleaseSessionInput(ctx, sqlcgen.ReleaseSessionInputParams{
			QueuedStatus: store.SessionInputQueueStatusQueued, UpdatedAt: nowRaw,
			ID: entryID, SessionID: target, DispatchingStatus: store.SessionInputQueueStatusDispatching,
		})
		if releaseErr != nil {
			return fmt.Errorf("store: release session input: %w", releaseErr)
		}
		if affected != 1 {
			return fmt.Errorf("%w: %s", store.ErrSessionInputQueueEntryNotFound, entryID)
		}
		return completeQueuedPromptAdmissionDispatch(ctx, exec, &entry, nowRaw)
	})
}

// MarkSessionInputFailed records a dispatch failure for one queue entry.
func (g *SessionRepo) MarkSessionInputFailed(
	ctx context.Context,
	sessionID string,
	entryID string,
	summary string,
	now time.Time,
) error {
	return g.updateSessionInputTerminal(
		ctx,
		"mark session input failed",
		sessionID,
		entryID,
		store.SessionInputQueueStatusFailed,
		summary,
		now,
	)
}

// updateSessionInputTerminal settles one dispatching entry. detail is the failure summary of a
// failed entry and the consuming turn id of a sent one.

func (g *SessionRepo) updateSessionInputTerminal(
	ctx context.Context,
	action string,
	sessionID string,
	entryID string,
	status string,
	detail string,
	now time.Time,
) error {
	if err := g.checkReady(ctx, action); err != nil {
		return err
	}
	target := strings.TrimSpace(sessionID)
	entryID = strings.TrimSpace(entryID)
	if target == "" || entryID == "" {
		return errors.New("store: session id and queue entry id are required")
	}
	if now.IsZero() {
		now = g.now()
	}
	nowRaw := store.FormatTimestamp(now.UTC())
	return g.withImmediateTransaction(ctx, action, func(exec globalSQLExecutor) error {
		entry, getErr := getSessionInputQueueEntry(ctx, exec, target, entryID)
		if getErr != nil {
			return getErr
		}
		if status == store.SessionInputQueueStatusSent && entry.Status == store.SessionInputQueueStatusSent {
			// A retried sent receipt after the turn started is already settled.
			return nil
		}
		var affected int64
		var updateErr error
		if status == store.SessionInputQueueStatusFailed {
			affected, updateErr = sqlcgen.New(exec).MarkSessionInputFailed(ctx, sqlcgen.MarkSessionInputFailedParams{
				FailedStatus: status, Now: nullableSessionTime(now), FailureSummary: strings.TrimSpace(detail),
				UpdatedAt: nowRaw, ID: entryID, SessionID: target,
				DispatchingStatus: store.SessionInputQueueStatusDispatching,
			})
		} else {
			affected, updateErr = sqlcgen.New(exec).MarkSessionInputSentInTurn(
				ctx,
				sqlcgen.MarkSessionInputSentInTurnParams{
					Now: nullableSessionTime(now), TurnID: detail, ID: entryID, SessionID: target,
				},
			)
		}
		if updateErr != nil {
			return fmt.Errorf("store: %s: %w", action, updateErr)
		}
		if affected != 1 {
			return fmt.Errorf("%w: %s", store.ErrSessionInputQueueEntryNotFound, entryID)
		}
		if status == store.SessionInputQueueStatusFailed {
			return markQueuedPromptAdmissionIndeterminate(ctx, exec, &entry, strings.TrimSpace(detail), nowRaw)
		}
		return completeQueuedPromptAdmissionDispatch(ctx, exec, &entry, nowRaw)
	})
}

// CancelSessionInput cancels one pending queue entry.
func (g *SessionRepo) CancelSessionInput(
	ctx context.Context,
	sessionID string,
	entryID string,
	now time.Time,
) (store.SessionInputQueueEntry, error) {
	if err := g.checkReady(ctx, "cancel session input"); err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	target := strings.TrimSpace(sessionID)
	entryID = strings.TrimSpace(entryID)
	if target == "" || entryID == "" {
		return store.SessionInputQueueEntry{}, errors.New("store: session id and queue entry id are required")
	}
	if now.IsZero() {
		now = g.now()
	}
	var entry store.SessionInputQueueEntry
	err := g.withImmediateTransaction(ctx, "cancel session input", func(exec globalSQLExecutor) error {
		existing, getErr := getSessionInputQueueEntry(ctx, exec, target, entryID)
		if getErr != nil {
			return getErr
		}
		if existing.OwnerKind == goalPromptOwnerKind {
			return fmt.Errorf("%w: %s", store.ErrSessionInputQueueEntryNotFound, entryID)
		}
		if existing.Status == store.SessionInputQueueStatusSent ||
			existing.Status == store.SessionInputQueueStatusFailed ||
			existing.Status == store.SessionInputQueueStatusDispatching {
			return &store.SessionInputNotQueuedError{EntryID: entryID, Status: existing.Status, Text: existing.Text}
		}
		if existing.Status == store.SessionInputQueueStatusCanceled {
			entry = existing
			return nil
		}
		nowRaw := store.FormatTimestamp(now.UTC())
		if updateErr := sqlcgen.New(exec).CancelSessionInput(ctx, sqlcgen.CancelSessionInputParams{
			CanceledStatus: store.SessionInputQueueStatusCanceled, Now: nullableSessionTime(now),
			UpdatedAt: nowRaw, ID: entryID, SessionID: target,
		}); updateErr != nil {
			return fmt.Errorf("store: cancel session input: %w", updateErr)
		}
		updated, getUpdatedErr := getSessionInputQueueEntry(ctx, exec, target, entryID)
		if getUpdatedErr != nil {
			return getUpdatedErr
		}
		entry = updated
		return sqlcgen.New(exec).ReleasePromptReports(ctx, existing.PromptAdmissionID)
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	return entry, nil
}

// CancelPendingSessionInputs cancels stale entries older than the supplied generation.
func (g *SessionRepo) CancelPendingSessionInputs(
	ctx context.Context,
	sessionID string,
	generation int64,
	now time.Time,
) (int, error) {
	if err := g.checkReady(ctx, "cancel pending session inputs"); err != nil {
		return 0, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return 0, errors.New("store: session id is required")
	}
	if now.IsZero() {
		now = g.now()
	}
	nowRaw := store.FormatTimestamp(now.UTC())
	var rows int64
	err := g.withImmediateTransaction(ctx, "cancel pending session inputs", func(exec globalSQLExecutor) error {
		var cancelErr error
		rows, cancelErr = sqlcgen.New(exec).CancelPendingSessionInputs(ctx, sqlcgen.CancelPendingSessionInputsParams{
			CanceledStatus: store.SessionInputQueueStatusCanceled, Now: nullableSessionTime(now),
			UpdatedAt: nowRaw, SessionID: target, SessionGeneration: generation,
			QueuedStatus:      store.SessionInputQueueStatusQueued,
			DispatchingStatus: store.SessionInputQueueStatusDispatching,
		})
		if cancelErr != nil {
			return fmt.Errorf("store: cancel pending session inputs: %w", cancelErr)
		}
		return sqlcgen.New(exec).ReleaseCanceledSessionReports(ctx, target)
	})
	if err != nil {
		return 0, err
	}
	return int(rows), nil
}

// FailStaleDispatchingSessionInputs settles non-Goal leases left by a previous
// daemon process. Their dispatch outcome is unknown, so they fail (marking any
// admission receipt indeterminate) instead of requeueing and risking a
// duplicate turn; either way they stop holding queue capacity. Steers staged
// but never reserved are recovered first so they cannot stay undispatchable.
func (g *SessionRepo) FailStaleDispatchingSessionInputs(ctx context.Context, now time.Time) (int, error) {
	if err := g.checkReady(ctx, "fail stale dispatching session inputs"); err != nil {
		return 0, err
	}
	rows, err := g.queries.ListStaleDispatchingSessionInputs(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: list stale dispatching session inputs: %w", err)
	}
	if err := g.queries.RecoverUncertainPromptReports(ctx); err != nil {
		return 0, err
	}
	if err := g.queries.RecoverReservedPromptReports(ctx); err != nil {
		return 0, err
	}
	failed := 0
	var errs []error
	if _, err := g.queries.RecoverStagedSessionSteers(ctx, store.FormatTimestamp(now.UTC())); err != nil {
		errs = append(errs, fmt.Errorf("store: recover staged session steers: %w", err))
	}
	for _, row := range rows {
		if err := g.MarkSessionInputFailed(
			ctx, row.SessionID, row.ID, store.SessionInputStaleDispatchSummary, now,
		); err != nil {
			errs = append(errs, fmt.Errorf("store: fail stale dispatching input %s: %w", row.ID, err))
			continue
		}
		failed++
	}
	return failed, errors.Join(errs...)
}
