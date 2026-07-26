package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

// MarkSessionInputSent records successful dispatch for one queue entry.
func (g *SessionRepo) MarkSessionInputSent(
	ctx context.Context,
	sessionID string,
	entryID string,
	now time.Time,
) error {
	return g.updateSessionInputTerminal(
		ctx,
		"mark session input sent",
		sessionID,
		entryID,
		store.SessionInputQueueStatusSent,
		"",
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
	affected, err := g.queries.ReleaseSessionInput(ctx, sqlcgen.ReleaseSessionInputParams{
		QueuedStatus: store.SessionInputQueueStatusQueued, UpdatedAt: nowRaw,
		ID: entryID, SessionID: target, DispatchingStatus: store.SessionInputQueueStatusDispatching,
	})
	if err != nil {
		return fmt.Errorf("store: release session input: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: %s", store.ErrSessionInputQueueEntryNotFound, entryID)
	}
	return nil
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

func (g *SessionRepo) updateSessionInputTerminal(
	ctx context.Context,
	action string,
	sessionID string,
	entryID string,
	status string,
	summary string,
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
	var affected int64
	var err error
	if status == store.SessionInputQueueStatusFailed {
		affected, err = g.queries.MarkSessionInputFailed(ctx, sqlcgen.MarkSessionInputFailedParams{
			FailedStatus: status, Now: nullableSessionTime(now), FailureSummary: strings.TrimSpace(summary),
			UpdatedAt: nowRaw, ID: entryID, SessionID: target,
			DispatchingStatus: store.SessionInputQueueStatusDispatching,
		})
	} else {
		affected, err = g.queries.MarkSessionInputSent(ctx, sqlcgen.MarkSessionInputSentParams{
			SentStatus: status, Now: nullableSessionTime(now), UpdatedAt: nowRaw,
			ID: entryID, SessionID: target, DispatchingStatus: store.SessionInputQueueStatusDispatching,
		})
	}
	if err != nil {
		return fmt.Errorf("store: %s: %w", action, err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: %s", store.ErrSessionInputQueueEntryNotFound, entryID)
	}
	return nil
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
		if existing.Status == store.SessionInputQueueStatusSent ||
			existing.Status == store.SessionInputQueueStatusFailed ||
			existing.Status == store.SessionInputQueueStatusCanceled {
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
		return nil
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
	rows, err := g.queries.CancelPendingSessionInputs(ctx, sqlcgen.CancelPendingSessionInputsParams{
		CanceledStatus: store.SessionInputQueueStatusCanceled, Now: nullableSessionTime(now),
		UpdatedAt: nowRaw, SessionID: target, SessionGeneration: generation,
		QueuedStatus:      store.SessionInputQueueStatusQueued,
		DispatchingStatus: store.SessionInputQueueStatusDispatching,
	})
	if err != nil {
		return 0, fmt.Errorf("store: cancel pending session inputs: %w", err)
	}
	return int(rows), nil
}
