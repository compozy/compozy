package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

const sessionInputGenerationColumn = "input_generation"

// EnqueueSessionInput appends one queued operator input entry under the configured cap.
func (g *SessionRepo) EnqueueSessionInput(
	ctx context.Context,
	req store.SessionInputQueueInsert,
) (entry store.SessionInputQueueEntry, position int, err error) {
	if err := g.checkReady(ctx, "enqueue session input"); err != nil {
		return store.SessionInputQueueEntry{}, 0, err
	}
	normalized := req.Normalize()
	if normalized.Mode == "" {
		normalized.Mode = store.SessionInputQueueModeQueue
	}
	if err := normalized.Validate(); err != nil {
		return store.SessionInputQueueEntry{}, 0, err
	}

	err = g.withImmediateTransaction(ctx, "enqueue session input", func(exec globalSQLExecutor) error {
		count, countErr := countPendingSessionInputs(ctx, exec, normalized.SessionID)
		if countErr != nil {
			return countErr
		}
		if count >= normalized.QueueCap {
			return fmt.Errorf(
				"%w: session %s cap %d",
				store.ErrSessionInputQueueFull,
				normalized.SessionID,
				normalized.QueueCap,
			)
		}
		inserted, insertErr := insertSessionInputQueueEntry(ctx, exec, normalized)
		if insertErr != nil {
			return insertErr
		}
		entry = inserted
		position = count + 1
		return nil
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, 0, err
	}
	return entry, position, nil
}

// StageSessionSteer replaces any active staged steer entry for the session.
func (g *SessionRepo) StageSessionSteer(
	ctx context.Context,
	req store.SessionInputQueueInsert,
) (entry store.SessionInputQueueEntry, err error) {
	if err := g.checkReady(ctx, "stage session steer"); err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	normalized := req.Normalize()
	normalized.Mode = store.SessionInputQueueModeSteer
	if err := normalized.Validate(); err != nil {
		return store.SessionInputQueueEntry{}, err
	}

	err = g.withImmediateTransaction(ctx, "stage session steer", func(exec globalSQLExecutor) error {
		nowRaw := store.FormatTimestamp(normalized.Now)
		if cancelErr := sqlcgen.New(exec).CancelPriorSessionSteers(ctx, sqlcgen.CancelPriorSessionSteersParams{
			CanceledStatus: store.SessionInputQueueStatusCanceled, CanceledAt: nullableSessionTime(normalized.Now),
			UpdatedAt: nowRaw, SessionID: normalized.SessionID, SteerMode: store.SessionInputQueueModeSteer,
			QueuedStatus:      store.SessionInputQueueStatusQueued,
			DispatchingStatus: store.SessionInputQueueStatusDispatching,
		}); cancelErr != nil {
			return fmt.Errorf("store: cancel prior session steer input: %w", cancelErr)
		}
		inserted, insertErr := insertSessionInputQueueEntry(ctx, exec, normalized)
		if insertErr != nil {
			return insertErr
		}
		entry = inserted
		return nil
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	return entry, nil
}

// ConsumeSessionSteer atomically marks the staged steer entry as sent and returns it once.
func (g *SessionRepo) ConsumeSessionSteer(
	ctx context.Context,
	sessionID string,
	now time.Time,
) (entry store.SessionInputQueueEntry, ok bool, err error) {
	if err := g.checkReady(ctx, "consume session steer"); err != nil {
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

	err = g.withImmediateTransaction(ctx, "consume session steer", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		row, scanErr := queries.GetQueuedSessionSteer(ctx, sqlcgen.GetQueuedSessionSteerParams{
			SessionID: target, SteerMode: store.SessionInputQueueModeSteer,
			QueuedStatus: store.SessionInputQueueStatusQueued,
		})
		if errors.Is(scanErr, sql.ErrNoRows) {
			return nil
		}
		if scanErr != nil {
			return scanErr
		}
		staged, scanErr := sessionInputQueueFromGenerated(&row)
		if scanErr != nil {
			return scanErr
		}
		nowRaw := store.FormatTimestamp(now)
		if _, updateErr := queries.ConsumeQueuedSessionSteer(ctx, sqlcgen.ConsumeQueuedSessionSteerParams{
			SentStatus: store.SessionInputQueueStatusSent, Now: nullableSessionTime(now), UpdatedAt: nowRaw,
			ID: staged.ID, SessionID: target, SteerMode: store.SessionInputQueueModeSteer,
			QueuedStatus: store.SessionInputQueueStatusQueued,
		}); updateErr != nil {
			return fmt.Errorf("store: consume session steer: %w", updateErr)
		}
		refreshed, getErr := getSessionInputQueueEntry(ctx, exec, target, staged.ID)
		if getErr != nil {
			return getErr
		}
		entry = refreshed
		ok = true
		return nil
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	return entry, ok, nil
}

// ClaimNextSessionInput atomically leases the next eligible input for dispatch.
func (g *SessionRepo) ClaimNextSessionInput(
	ctx context.Context,
	sessionID string,
	now time.Time,
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
		queries := sqlcgen.New(exec)
		row, scanErr := queries.GetNextDispatchableSessionInput(
			ctx,
			sqlcgen.GetNextDispatchableSessionInputParams{
				SessionID: target, QueuedStatus: store.SessionInputQueueStatusQueued,
			},
		)
		if errors.Is(scanErr, sql.ErrNoRows) {
			return nil
		}
		if scanErr != nil {
			return scanErr
		}
		claimed, scanErr := sessionInputQueueFromGenerated(&row)
		if scanErr != nil {
			return scanErr
		}
		nowRaw := store.FormatTimestamp(now)
		affected, updateErr := queries.ClaimSessionInput(ctx, sqlcgen.ClaimSessionInputParams{
			DispatchingStatus: store.SessionInputQueueStatusDispatching,
			Now:               nullableSessionTime(now), UpdatedAt: nowRaw, ID: claimed.ID, SessionID: target,
			QueuedStatus: store.SessionInputQueueStatusQueued,
		})
		if updateErr != nil {
			return fmt.Errorf("store: mark session input dispatching: %w", updateErr)
		}
		if affected != 1 {
			return goalControlStaleError("mark human session input dispatching lost compare-and-swap")
		}
		refreshed, getErr := getSessionInputQueueEntry(ctx, exec, target, claimed.ID)
		if getErr != nil {
			return getErr
		}
		entry = refreshed
		ok = true
		return nil
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	return entry, ok, nil
}

// PeekNextSessionInput returns the oldest eligible human or managed row without claiming it.
func (g *SessionRepo) PeekNextSessionInput(
	ctx context.Context,
	sessionID string,
) (store.SessionInputQueueEntry, bool, error) {
	if err := g.checkReady(ctx, "peek next session input"); err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return store.SessionInputQueueEntry{}, false, errors.New("store: session id is required")
	}
	row, err := g.queries.PeekNextSessionInput(ctx, sqlcgen.PeekNextSessionInputParams{
		SessionID: target, QueuedStatus: store.SessionInputQueueStatusQueued,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return store.SessionInputQueueEntry{}, false, nil
	}
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	entry, err := sessionInputQueueFromGenerated(&row)
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	return entry, true, nil
}

// GetSessionInputQueueEntry returns one exact queue entry for durable await/recovery.
func (g *SessionRepo) GetSessionInputQueueEntry(
	ctx context.Context,
	sessionID string,
	entryID string,
) (store.SessionInputQueueEntry, error) {
	if err := g.checkReady(ctx, "get session input queue entry"); err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	return getSessionInputQueueEntry(ctx, g.db, strings.TrimSpace(sessionID), strings.TrimSpace(entryID))
}

// AdvanceSessionInputGeneration increments the session generation used to fence stale queue entries.
func (g *SessionRepo) AdvanceSessionInputGeneration(
	ctx context.Context,
	sessionID string,
	now time.Time,
) (int64, error) {
	if err := g.checkReady(ctx, "advance session input generation"); err != nil {
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
	var generation int64
	err := g.withImmediateTransaction(ctx, "advance session input generation", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		affected, updateErr := queries.AdvanceSessionInputGeneration(ctx, sqlcgen.AdvanceSessionInputGenerationParams{
			UpdatedAt: nowRaw, ID: target,
		})
		if updateErr != nil {
			return fmt.Errorf("store: advance session input generation: %w", updateErr)
		}
		if affected == 0 {
			return fmt.Errorf("%w: %s", store.ErrSessionInputQueueEntryNotFound, target)
		}
		var scanErr error
		generation, scanErr = queries.GetSessionInputGeneration(ctx, target)
		if scanErr != nil {
			return fmt.Errorf("store: read session input generation: %w", scanErr)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return generation, nil
}

// CurrentSessionInputGeneration returns the persisted busy-input generation for a session.
func (g *SessionRepo) CurrentSessionInputGeneration(ctx context.Context, sessionID string) (int64, error) {
	if err := g.checkReady(ctx, "read session input generation"); err != nil {
		return 0, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return 0, errors.New("store: session id is required")
	}
	generation, err := g.queries.GetSessionInputGeneration(ctx, target)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("%w: %s", store.ErrSessionNotFound, target)
		}
		return 0, fmt.Errorf("store: read session input generation: %w", err)
	}
	return generation, nil
}

// SessionInputQueueSummary returns the current generation and active pending counts for one session.
func (g *SessionRepo) SessionInputQueueSummary(
	ctx context.Context,
	sessionID string,
) (summary store.SessionInputQueueSummary, err error) {
	if err := g.checkReady(ctx, "read session input queue summary"); err != nil {
		return store.SessionInputQueueSummary{}, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return store.SessionInputQueueSummary{}, errors.New("store: session id is required")
	}
	summary.SessionID = target
	summary.Generation, err = g.queries.GetSessionInputGeneration(ctx, target)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.SessionInputQueueSummary{}, fmt.Errorf("%w: %s", store.ErrSessionNotFound, target)
		}
		return store.SessionInputQueueSummary{}, fmt.Errorf("store: read session input generation: %w", err)
	}
	rows, err := g.queries.ListSessionInputQueueSummary(ctx, sqlcgen.ListSessionInputQueueSummaryParams{
		SessionID: target, SessionGeneration: summary.Generation,
		QueuedStatus:      store.SessionInputQueueStatusQueued,
		DispatchingStatus: store.SessionInputQueueStatusDispatching,
	})
	if err != nil {
		return store.SessionInputQueueSummary{}, fmt.Errorf("store: query session input queue summary: %w", err)
	}
	for _, row := range rows {
		count := int(row.Count)
		summary.PendingActive += count
		switch strings.TrimSpace(row.Mode) {
		case store.SessionInputQueueModeQueue:
			summary.PendingQueued += count
		case store.SessionInputQueueModeSteer:
			summary.PendingSteer += count
		}
		if strings.TrimSpace(row.Status) == store.SessionInputQueueStatusDispatching {
			summary.PendingLeased += count
		}
	}
	return summary, nil
}

func insertSessionInputQueueEntry(
	ctx context.Context,
	exec globalSQLExecutor,
	req store.SessionInputQueueInsert,
) (store.SessionInputQueueEntry, error) {
	normalized := req.Normalize()
	nowRaw := store.FormatTimestamp(normalized.Now)
	var runGeneration sql.NullInt64
	if normalized.RunGeneration != nil {
		runGeneration = sql.NullInt64{Int64: *normalized.RunGeneration, Valid: true}
	}
	if err := sqlcgen.New(exec).InsertSessionInputQueueEntry(ctx, sqlcgen.InsertSessionInputQueueEntryParams{
		ID: normalized.ID, SessionID: normalized.SessionID, Status: store.SessionInputQueueStatusQueued,
		Mode: normalized.Mode, Text: normalized.Text, SessionGeneration: normalized.SessionGeneration,
		TaskRunID: normalized.TaskRunID, RunGeneration: runGeneration, EnqueuedAt: nowRaw, UpdatedAt: nowRaw,
	}); err != nil {
		return store.SessionInputQueueEntry{}, fmt.Errorf("store: insert session input queue entry: %w", err)
	}
	return getSessionInputQueueEntry(ctx, exec, normalized.SessionID, normalized.ID)
}

func countPendingSessionInputs(ctx context.Context, exec globalSQLExecutor, sessionID string) (int, error) {
	count, err := sqlcgen.New(exec).CountPendingSessionInputs(ctx, sqlcgen.CountPendingSessionInputsParams{
		SessionID: sessionID, QueuedStatus: store.SessionInputQueueStatusQueued,
		DispatchingStatus: store.SessionInputQueueStatusDispatching,
	})
	if err != nil {
		return 0, fmt.Errorf("store: count pending session inputs: %w", err)
	}
	return int(count), nil
}
