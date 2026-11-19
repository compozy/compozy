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

func commitQueuedPromptAdmissionDispatch(
	ctx context.Context,
	exec globalSQLExecutor,
	entry *store.SessionInputQueueEntry,
	nowRaw string,
) error {
	if entry.PromptAdmissionID == "" || entry.SteerDelivery != "" {
		// A resolved steer receipt records acceptance. Its fallback queue entry
		// owns subsequent delivery state without reopening that receipt.
		return nil
	}
	affected, err := sqlcgen.New(exec).CommitQueuedSessionPromptAdmissionDispatch(
		ctx,
		sqlcgen.CommitQueuedSessionPromptAdmissionDispatchParams{
			DispatchCommittedState: store.SessionPromptAdmissionDispatchCommitted,
			DispatchCommittedAt:    sql.NullString{String: nowRaw, Valid: true},
			UpdatedAt:              nowRaw,
			ID:                     entry.PromptAdmissionID,
			SessionID:              entry.SessionID,
			CompletedState:         store.SessionPromptAdmissionCompleted,
		},
	)
	if err != nil {
		return fmt.Errorf("store: commit queued prompt admission dispatch: %w", err)
	}
	if affected != 1 {
		return queuedPromptAdmissionCASFailure(ctx, exec, entry)
	}
	return nil
}

func completeQueuedPromptAdmissionDispatch(
	ctx context.Context,
	exec globalSQLExecutor,
	entry *store.SessionInputQueueEntry,
	nowRaw string,
) error {
	if entry.PromptAdmissionID == "" || entry.SteerDelivery != "" {
		return nil
	}
	affected, err := sqlcgen.New(exec).CompleteQueuedSessionPromptAdmissionDispatch(
		ctx,
		sqlcgen.CompleteQueuedSessionPromptAdmissionDispatchParams{
			CompletedState:         store.SessionPromptAdmissionCompleted,
			CompletedAt:            sql.NullString{String: nowRaw, Valid: true},
			UpdatedAt:              nowRaw,
			ID:                     entry.PromptAdmissionID,
			SessionID:              entry.SessionID,
			DispatchCommittedState: store.SessionPromptAdmissionDispatchCommitted,
		},
	)
	if err != nil {
		return fmt.Errorf("store: complete queued prompt admission dispatch: %w", err)
	}
	if affected != 1 {
		return queuedPromptAdmissionCASFailure(ctx, exec, entry)
	}
	return nil
}

func markQueuedPromptAdmissionIndeterminate(
	ctx context.Context,
	exec globalSQLExecutor,
	entry *store.SessionInputQueueEntry,
	summary string,
	nowRaw string,
) error {
	if entry.PromptAdmissionID == "" || entry.SteerDelivery != "" {
		return nil
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = "dispatch outcome is unknown"
	}
	affected, err := sqlcgen.New(exec).MarkQueuedSessionPromptAdmissionIndeterminate(
		ctx,
		sqlcgen.MarkQueuedSessionPromptAdmissionIndeterminateParams{
			IndeterminateState:     store.SessionPromptAdmissionIndeterminate,
			IndeterminateReason:    summary,
			UpdatedAt:              nowRaw,
			ID:                     entry.PromptAdmissionID,
			SessionID:              entry.SessionID,
			DispatchCommittedState: store.SessionPromptAdmissionDispatchCommitted,
		},
	)
	if err != nil {
		return fmt.Errorf("store: mark queued prompt admission indeterminate: %w", err)
	}
	if affected != 1 {
		return queuedPromptAdmissionCASFailure(ctx, exec, entry)
	}
	return sqlcgen.New(exec).FailPromptReports(ctx, entry.PromptAdmissionID)
}

func queuedPromptAdmissionCASFailure(
	ctx context.Context,
	exec globalSQLExecutor,
	entry *store.SessionInputQueueEntry,
) error {
	state, err := sqlcgen.New(exec).GetQueuedSessionPromptAdmissionState(
		ctx,
		sqlcgen.GetQueuedSessionPromptAdmissionStateParams{
			ID: entry.PromptAdmissionID, SessionID: entry.SessionID,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return store.ErrSessionPromptAdmissionInProgress
	}
	if err != nil {
		return fmt.Errorf("store: read queued prompt admission after compare-and-swap failure: %w", err)
	}
	switch state {
	case store.SessionPromptAdmissionDispatchCommitted, store.SessionPromptAdmissionIndeterminate:
		return store.ErrSessionPromptDispatchIndeterminate
	default:
		return store.ErrSessionPromptAdmissionInProgress
	}
}

// CommitSessionPromptDispatch crosses the at-most-once boundary before any non-idempotent effect.
func (g *SessionRepo) CommitSessionPromptDispatch(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	idempotencyKey string,
	now time.Time,
) error {
	if err := g.checkReady(ctx, "commit session prompt dispatch"); err != nil {
		return err
	}
	workspaceID, sessionID, idempotencyKey, now, err := normalizePromptAdmissionScope(
		workspaceID,
		sessionID,
		idempotencyKey,
		now,
	)
	if err != nil {
		return err
	}
	var budgetBlocked bool
	var pauseTrip *store.InstancePauseTrip
	err = g.withImmediateTransaction(ctx, "commit session prompt dispatch", func(exec globalSQLExecutor) error {
		budgetBlocked, pauseTrip = false, nil
		queries := sqlcgen.New(exec)
		admission, lookupErr := queries.GetSessionPromptAdmissionByIdempotencyKey(ctx,
			sqlcgen.GetSessionPromptAdmissionByIdempotencyKeyParams{
				WorkspaceID: workspaceID, SessionID: sessionID, IdempotencyKey: idempotencyKey,
			})
		if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
			return lookupErr
		}
		// Human-origin input is deliberately outside the autonomous budget. An
		// already-committed/replayed turn must never be admitted a second time.
		if lookupErr == nil && admission.State == store.SessionPromptAdmissionReserved &&
			admission.Operation == store.SessionPromptOperationPrompt && !store.PromptControlOnly(ctx) &&
			(admission.HumanIngress == 0 || admission.AppIngress != 0) {
			allowed, trip, admissionErr := admitDailyBudgetTurnTx(ctx, queries, sessionID, now)
			if admissionErr != nil {
				return admissionErr
			}
			if !allowed {
				budgetBlocked, pauseTrip = true, trip
				return nil
			}
		}
		affected, commitErr := queries.CommitSessionPromptDispatch(
			ctx,
			sqlcgen.CommitSessionPromptDispatchParams{
				DispatchCommittedState: store.SessionPromptAdmissionDispatchCommitted,
				DispatchCommittedAt:    nullableSessionTime(now), UpdatedAt: store.FormatTimestamp(now),
				WorkspaceID: workspaceID, SessionID: sessionID, IdempotencyKey: idempotencyKey,
				ReservedState: store.SessionPromptAdmissionReserved,
			},
		)
		if commitErr != nil {
			return fmt.Errorf("store: commit session prompt dispatch: %w", commitErr)
		}
		if affected != 1 {
			return promptAdmissionCASFailure(ctx, exec, workspaceID, sessionID, idempotencyKey)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if budgetBlocked {
		return &store.DailyBudgetTurnBlockedError{PausedInstance: pauseTrip}
	}
	return nil
}
