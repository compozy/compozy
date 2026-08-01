package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func commitQueuedPromptAdmissionDispatch(
	ctx context.Context,
	exec globalSQLExecutor,
	entry *store.SessionInputQueueEntry,
	nowRaw string,
) error {
	if entry.PromptAdmissionID == "" {
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
	if entry.PromptAdmissionID == "" {
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
	if entry.PromptAdmissionID == "" {
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
	return nil
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
