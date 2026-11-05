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

// ResolveSessionSteer atomically records reserved guidance's delivery and completes its admission receipt.
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
		if delivery == store.SteerDeliveryInterruptFallback || delivery == store.SteerDeliveryPendingBoundary ||
			delivery == store.SteerDeliveryAfterTurn {
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
			SteerDelivery: string(delivery), TargetTurnID: entry.TargetTurnID, Delivery: entry.Delivery,
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
	switch delivery {
	case store.SteerDeliveryInjected, store.SteerDeliveryInterruptFallback,
		store.SteerDeliveryPendingBoundary, store.SteerDeliveryAfterTurn:
	default:
		return entry, false, errors.New("store: invalid pending steer settlement")
	}
	if now.IsZero() {
		now = g.now()
	}
	fallbackTurnID := ""
	if delivery != store.SteerDeliveryInjected {
		fallbackTurnID, err = store.NewID("turn")
		if err != nil {
			return entry, false, fmt.Errorf("store: allocate fallback steer turn: %w", err)
		}
	}
	err = g.withImmediateTransaction(ctx, "settle pending session steer", func(exec globalSQLExecutor) error {
		affected, updateErr := sqlcgen.New(exec).SettlePendingSessionSteer(ctx, sqlcgen.SettlePendingSessionSteerParams{
			SessionID: sessionID, ID: entryID, Now: store.FormatTimestamp(now),
			FallbackTurnID: fallbackTurnID,
			SteerDelivery:  sql.NullString{String: string(delivery), Valid: true},
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
	_, err := executeWriteQueryResult(
		ctx,
		g.repoBase,
		"globaldb.CancelPendingSessionSteer",
		func(ctx context.Context, queries *sqlcgen.Queries) (int64, error) {
			return queries.CancelPendingSessionSteer(ctx, sqlcgen.CancelPendingSessionSteerParams{
				SessionID: sessionID, ID: entryID, Now: nullableSessionTime(now),
			})
		},
	)
	if err != nil {
		return fmt.Errorf("store: cancel pending steer: %w", err)
	}
	return nil
}

// supersedeBoundarySteersForInterrupt keeps replacement admission and retirement in one transaction.
func supersedeBoundarySteersForInterrupt(
	ctx context.Context,
	exec globalSQLExecutor,
	req store.SessionInputQueueInsert,
) ([]string, error) {
	if !store.IsHumanSessionInputOriginClass(req.OriginClass) {
		return nil, nil
	}
	queries := sqlcgen.New(exec)
	ids, err := queries.SupersedeBoundarySteersForInterrupt(ctx, sqlcgen.SupersedeBoundarySteersForInterruptParams{
		SessionID:         req.SessionID,
		TargetTurnID:      req.TargetTurnID,
		ReplacementID:     sql.NullString{String: req.ID, Valid: true},
		SessionGeneration: req.SessionGeneration,
		Now:               nullableSessionTime(req.Now),
	})
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		entry, err := getSessionInputQueueEntry(ctx, exec, req.SessionID, id)
		if err != nil {
			return nil, err
		}
		if entry.PromptAdmissionID == "" {
			continue
		}
		if _, err := queries.CompleteSupersededSteerAdmission(ctx, sqlcgen.CompleteSupersededSteerAdmissionParams{
			ID: entry.PromptAdmissionID, SessionID: req.SessionID,
			TargetTurnID: entry.TargetTurnID, Now: nullableSessionTime(req.Now),
		}); err != nil {
			return nil, fmt.Errorf("store: complete superseded steer admission: %w", err)
		}
	}
	return ids, nil
}

// SupersedeSessionBoundarySteers retires undelivered human guidance before an empty interrupt.
func (g *SessionRepo) SupersedeSessionBoundarySteers(
	ctx context.Context,
	sessionID, targetTurnID, replacementID string,
	now time.Time,
) ([]string, error) {
	if err := g.checkReady(ctx, "supersede boundary steers"); err != nil {
		return nil, err
	}
	var ids []string
	err := g.withImmediateTransaction(ctx, "supersede boundary steers", func(exec globalSQLExecutor) error {
		req, err := prepareInterruptSessionInput(
			ctx,
			exec,
			store.SessionInputQueueInsert{
				ID:           replacementID,
				SessionID:    sessionID,
				TargetTurnID: targetTurnID,
				OriginClass:  store.SessionInputOriginClassHuman,
				Now:          now,
			},
		)
		if err != nil {
			return err
		}
		ids, err = supersedeBoundarySteersForInterrupt(ctx, exec, req)
		return err
	})
	return ids, err
}

// ListSupersededSessionSteers pages durable supersessions for transcript repair on resume.
func (g *SessionRepo) ListSupersededSessionSteers(
	ctx context.Context,
	sessionID, afterID string,
) ([]store.SessionInputQueueEntry, error) {
	if err := g.checkReady(ctx, "list superseded steers"); err != nil {
		return nil, err
	}
	rows, err := g.readQueries.ListSupersededSessionSteers(
		ctx,
		sqlcgen.ListSupersededSessionSteersParams{SessionID: sessionID, AfterID: afterID},
	)
	if err != nil {
		return nil, err
	}
	entries := make([]store.SessionInputQueueEntry, 0, len(rows))
	for i := range rows {
		entry, err := sessionInputQueueFromGenerated(&rows[i])
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
