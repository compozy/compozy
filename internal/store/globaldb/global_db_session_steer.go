package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *SessionRepo) ReserveSessionSteer(
	ctx context.Context, sessionID, entryID string, now time.Time,
) (entry store.SessionInputQueueEntry, reserved bool, err error) {
	if err := g.checkReady(ctx, "reserve session steer"); err != nil {
		return entry, false, err
	}
	if now.IsZero() {
		now = g.now()
	}
	err = g.withImmediateTransaction(ctx, "reserve session steer", func(exec globalSQLExecutor) error {
		var err error
		entry, err = getSessionInputQueueEntry(ctx, exec, sessionID, entryID)
		if err != nil {
			return err
		}
		if entry.Mode != store.SessionInputQueueModeSteer {
			return errors.New("store: only steer inputs can be reserved for live delivery")
		}
		nowRaw := store.FormatTimestamp(now)
		affected, err := sqlcgen.New(exec).ReserveSessionSteer(ctx, sqlcgen.ReserveSessionSteerParams{
			SessionID: sessionID, ID: entryID, Now: nullableSessionTime(now),
		})
		if err != nil || affected == 0 {
			return err
		}
		if err := commitQueuedPromptAdmissionDispatch(ctx, exec, &entry, nowRaw); err != nil {
			return err
		}
		entry, err = getSessionInputQueueEntry(ctx, exec, sessionID, entryID)
		reserved = err == nil
		return err
	})
	return entry, reserved && err == nil, err
}

func (g *SessionRepo) ResolveSessionSteer(
	ctx context.Context, sessionID, entryID string, delivery store.SteerDeliveryMode, now time.Time,
) (entry store.SessionInputQueueEntry, err error) {
	if err := g.checkReady(ctx, "resolve session steer"); err != nil {
		return entry, err
	}
	if err := delivery.Validate(); err != nil {
		return entry, err
	}
	if delivery == "" {
		return entry, errors.New("store: resolved steer delivery is required")
	}
	if now.IsZero() {
		now = g.now()
	}
	err = g.withImmediateTransaction(ctx, "resolve session steer", func(exec globalSQLExecutor) error {
		nowRaw := store.FormatTimestamp(now)
		status := store.SessionInputQueueStatusSent
		sentAt := sql.NullString{String: nowRaw, Valid: true}
		if delivery == store.SteerDeliveryInterruptFallback {
			status = store.SessionInputQueueStatusQueued
			sentAt = sql.NullString{}
		}
		queries := sqlcgen.New(exec)
		affected, err := queries.ResolveSessionSteer(ctx, sqlcgen.ResolveSessionSteerParams{
			SessionID: sessionID, ID: entryID, Status: status, Now: nowRaw,
			SteerDelivery: sql.NullString{String: string(delivery), Valid: true}, SentAt: sentAt,
		})
		if err != nil {
			return fmt.Errorf("store: resolve session steer: %w", err)
		}
		if affected != 1 {
			return store.ErrSessionInputQueueEntryNotQueued
		}
		entry, err = getSessionInputQueueEntry(ctx, exec, sessionID, entryID)
		if err != nil || entry.PromptAdmissionID == "" {
			return err
		}
		affected, err = queries.ResolveSessionSteerAdmission(ctx, sqlcgen.ResolveSessionSteerAdmissionParams{
			ID: entry.PromptAdmissionID, SessionID: sessionID, Now: nullableSessionTime(now),
			SteerDelivery: string(delivery), TargetTurnID: entry.TargetTurnID,
		})
		if err != nil {
			return fmt.Errorf("store: resolve session steer receipt: %w", err)
		}
		if affected != 1 {
			return store.ErrSessionPromptAdmissionInProgress
		}
		return nil
	})
	return entry, err
}

// SettlePendingSessionSteer records delivery completion without rewriting the
// immutable admission receipt. Only the first result in the current generation
// may release the original entry for fallback dispatch.
func (g *SessionRepo) SettlePendingSessionSteer(
	ctx context.Context, sessionID, entryID string, delivery store.SteerDeliveryMode, now time.Time,
) (entry store.SessionInputQueueEntry, settled bool, err error) {
	if err := g.checkReady(ctx, "settle pending session steer"); err != nil {
		return entry, false, err
	}
	if delivery != store.SteerDeliveryInjected && delivery != store.SteerDeliveryInterruptFallback {
		return entry, false, errors.New("store: pending steer must settle as injected or interrupt fallback")
	}
	if now.IsZero() {
		now = g.now()
	}
	err = g.withImmediateTransaction(ctx, "settle pending session steer", func(exec globalSQLExecutor) error {
		affected, updateErr := sqlcgen.New(exec).SettlePendingSessionSteer(ctx, sqlcgen.SettlePendingSessionSteerParams{
			SessionID: sessionID, ID: entryID, Now: store.FormatTimestamp(now),
			SteerDelivery: sql.NullString{String: string(delivery), Valid: true},
		})
		if updateErr != nil {
			return fmt.Errorf("store: settle pending steer: %w", updateErr)
		}
		entry, updateErr = getSessionInputQueueEntry(ctx, exec, sessionID, entryID)
		settled = affected == 1 && updateErr == nil
		return updateErr
	})
	return entry, settled && err == nil, err
}

// CancelPendingSessionSteer retires guidance canceled with its owning session,
// leaving the original admission receipt available for idempotent replay.
func (g *SessionRepo) CancelPendingSessionSteer(ctx context.Context, sessionID, entryID string, now time.Time) error {
	if err := g.checkReady(ctx, "cancel pending session steer"); err != nil {
		return err
	}
	if now.IsZero() {
		now = g.now()
	}
	_, err := sqlcgen.New(g.db).CancelPendingSessionSteer(ctx, sqlcgen.CancelPendingSessionSteerParams{
		SessionID: sessionID, ID: entryID, Now: nullableSessionTime(now),
	})
	if err != nil {
		return fmt.Errorf("store: cancel pending steer: %w", err)
	}
	return nil
}
