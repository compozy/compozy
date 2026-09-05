package globaldb

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// EnqueueInterruptSessionInput atomically fences stale pending input and persists its replacement.
func (g *SessionRepo) EnqueueInterruptSessionInput(
	ctx context.Context,
	req store.SessionInputQueueInsert,
) (entry store.SessionInputQueueEntry, canceled int, err error) {
	if err := g.checkReady(ctx, "enqueue interrupt session input"); err != nil {
		return store.SessionInputQueueEntry{}, 0, err
	}
	req.Mode = store.SessionInputQueueModeInterrupt
	req.Delivery = store.SessionInputDeliveryInterruptThenPrompt
	req = req.Normalize()
	if err := req.Validate(); err != nil {
		return store.SessionInputQueueEntry{}, 0, err
	}
	err = g.withImmediateTransaction(ctx, "enqueue interrupt session input", func(exec globalSQLExecutor) error {
		prepared, canceledCount, prepareErr := prepareInterruptSessionInput(ctx, exec, req)
		if prepareErr != nil {
			return prepareErr
		}
		inserted, insertErr := insertSessionInputQueueEntry(ctx, exec, prepared)
		if insertErr != nil {
			return insertErr
		}
		entry = inserted
		canceled = canceledCount
		return nil
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, 0, err
	}
	return entry, canceled, nil
}

func prepareInterruptSessionInput(
	ctx context.Context,
	exec globalSQLExecutor,
	req store.SessionInputQueueInsert,
) (store.SessionInputQueueInsert, int, error) {
	queries := sqlcgen.New(exec)
	nowRaw := store.FormatTimestamp(req.Now)
	affected, err := queries.AdvanceSessionInputGeneration(ctx, sqlcgen.AdvanceSessionInputGenerationParams{
		UpdatedAt: nowRaw,
		ID:        req.SessionID,
	})
	if err != nil {
		return store.SessionInputQueueInsert{}, 0, fmt.Errorf("store: advance interrupt generation: %w", err)
	}
	if affected == 0 {
		return store.SessionInputQueueInsert{}, 0, fmt.Errorf("%w: %s", store.ErrSessionNotFound, req.SessionID)
	}
	generation, err := queries.GetSessionInputGeneration(ctx, req.SessionID)
	if err != nil {
		return store.SessionInputQueueInsert{}, 0, fmt.Errorf("store: read interrupt generation: %w", err)
	}
	canceled, err := queries.CancelPendingSessionInputs(ctx, sqlcgen.CancelPendingSessionInputsParams{
		CanceledStatus:    store.SessionInputQueueStatusCanceled,
		Now:               nullableSessionTime(req.Now),
		UpdatedAt:         nowRaw,
		SessionID:         req.SessionID,
		SessionGeneration: generation,
		QueuedStatus:      store.SessionInputQueueStatusQueued,
		DispatchingStatus: store.SessionInputQueueStatusDispatching,
	})
	if err != nil {
		return store.SessionInputQueueInsert{}, 0, fmt.Errorf("store: fence interrupt queue: %w", err)
	}
	req.SessionGeneration = generation
	return req, int(canceled), nil
}

// ListPendingSessionInputs returns current-generation operator inputs in dispatch order.
func (g *SessionRepo) ListPendingSessionInputs(
	ctx context.Context,
	sessionID string,
) ([]store.SessionInputQueueEntry, error) {
	if err := g.checkReady(ctx, "list pending session inputs"); err != nil {
		return nil, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return nil, errors.New("store: session id is required")
	}
	rows, err := g.queries.ListPendingSessionInputs(ctx, sqlcgen.ListPendingSessionInputsParams{
		SessionID:         target,
		QueuedStatus:      store.SessionInputQueueStatusQueued,
		DispatchingStatus: store.SessionInputQueueStatusDispatching,
	})
	if err != nil {
		return nil, fmt.Errorf("store: list pending session inputs: %w", err)
	}
	entries := make([]store.SessionInputQueueEntry, 0, len(rows))
	for index := range rows {
		entry, mapErr := sessionInputQueueFromGenerated(&rows[index])
		if mapErr != nil {
			return nil, mapErr
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// ReplaceSessionInput atomically cancels one queued entry and inserts its replacement.
func (g *SessionRepo) ReplaceSessionInput(
	ctx context.Context,
	sessionID string,
	entryID string,
	replacement store.SessionInputQueueInsert,
) (store.SessionInputQueueEntry, bool, error) {
	replacement.Mode = store.SessionInputQueueModeQueue
	replacement.Delivery = store.SessionInputDeliveryAfterTurn
	return g.replacePendingSessionInput(ctx, "replace session input", sessionID, entryID, replacement, false)
}

// PromoteSessionInputToSteer atomically replaces one queued entry with priority steering input.
func (g *SessionRepo) PromoteSessionInputToSteer(
	ctx context.Context,
	sessionID string,
	entryID string,
	replacement store.SessionInputQueueInsert,
) (store.SessionInputQueueEntry, bool, error) {
	replacement.Mode = store.SessionInputQueueModeSteer
	replacement.Delivery = store.SessionInputDeliveryInterruptThenPrompt
	return g.replacePendingSessionInput(ctx, "promote session input to steer", sessionID, entryID, replacement, true)
}

func (g *SessionRepo) replacePendingSessionInput(
	ctx context.Context,
	action string,
	sessionID string,
	entryID string,
	replacement store.SessionInputQueueInsert,
	cancelPriorSteers bool,
) (entry store.SessionInputQueueEntry, created bool, err error) {
	if err := g.checkReady(ctx, action); err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	target := strings.TrimSpace(sessionID)
	entryID = strings.TrimSpace(entryID)
	replacement = replacement.Normalize()
	replacement.SessionID = target
	if target == "" || entryID == "" {
		return store.SessionInputQueueEntry{}, false, errors.New("store: session id and queue entry id are required")
	}
	if replacement.ID == entryID {
		return store.SessionInputQueueEntry{}, false, errors.New("store: replacement queue entry id must be new")
	}
	if err := replacement.Validate(); err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}

	err = g.withImmediateTransaction(ctx, action, func(exec globalSQLExecutor) error {
		replayed, replayErr := getSessionInputQueueEntry(ctx, exec, target, replacement.ID)
		if replayErr == nil {
			if !sameSessionInputMutation(&replayed, replacement) {
				return fmt.Errorf("%w: %s", store.ErrSessionInputMutationConflict, replacement.ID)
			}
			entry = replayed
			return nil
		}
		if !errors.Is(replayErr, store.ErrSessionInputQueueEntryNotFound) {
			return replayErr
		}
		existing, getErr := getSessionInputQueueEntry(ctx, exec, target, entryID)
		if getErr != nil {
			return getErr
		}
		if existing.OwnerKind != "" {
			return fmt.Errorf("%w: %s", store.ErrSessionInputQueueEntryNotFound, entryID)
		}
		if existing.Status != store.SessionInputQueueStatusQueued {
			return fmt.Errorf("%w: %s", store.ErrSessionInputQueueEntryNotQueued, entryID)
		}
		replacement.SessionGeneration = existing.SessionGeneration
		var superseded []string
		if cancelPriorSteers {
			var cancelErr error
			superseded, cancelErr = cancelPriorPendingSessionSteers(ctx, exec, target, replacement.Now)
			if cancelErr != nil {
				return cancelErr
			}
		}
		queries := sqlcgen.New(exec)
		nowRaw := store.FormatTimestamp(replacement.Now)
		if cancelErr := queries.CancelSessionInput(ctx, sqlcgen.CancelSessionInputParams{
			CanceledStatus: store.SessionInputQueueStatusCanceled,
			Now:            nullableSessionTime(replacement.Now),
			UpdatedAt:      nowRaw,
			ID:             entryID,
			SessionID:      target,
		}); cancelErr != nil {
			return fmt.Errorf("store: cancel replaced session input: %w", cancelErr)
		}
		inserted, insertErr := insertSessionInputQueueEntry(ctx, exec, replacement)
		if insertErr != nil {
			return insertErr
		}
		inserted.SupersededIDs = superseded
		entry = inserted
		created = true
		return nil
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	return entry, created, nil
}

func sameSessionInputMutation(
	entry *store.SessionInputQueueEntry,
	replacement store.SessionInputQueueInsert,
) bool {
	return entry.SessionID == replacement.SessionID &&
		entry.MessageID == replacement.MessageID &&
		entry.IdempotencyKey == replacement.IdempotencyKey &&
		entry.TargetTurnID == replacement.TargetTurnID &&
		entry.Mode == replacement.Mode &&
		entry.Delivery == replacement.Delivery &&
		entry.Text == replacement.Text &&
		slices.Equal(entry.SkillInvocations, replacement.SkillInvocations) &&
		slices.Equal(entry.Attachments, replacement.Attachments)
}

func cancelPriorPendingSessionSteers(
	ctx context.Context,
	exec globalSQLExecutor,
	sessionID string,
	now time.Time,
) ([]string, error) {
	nowRaw := store.FormatTimestamp(now)
	ids, err := sqlcgen.New(exec).CancelPriorSessionSteers(ctx, sqlcgen.CancelPriorSessionSteersParams{
		CanceledStatus: store.SessionInputQueueStatusCanceled,
		CanceledAt:     nullableSessionTime(now),
		UpdatedAt:      nowRaw,
		SessionID:      sessionID,
		SteerMode:      store.SessionInputQueueModeSteer,
		QueuedStatus:   store.SessionInputQueueStatusQueued,
	})
	if err != nil {
		return nil, fmt.Errorf("store: cancel prior session steer input: %w", err)
	}
	return ids, nil
}
