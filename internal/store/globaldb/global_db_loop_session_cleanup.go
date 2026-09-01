package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ looppkg.SessionCleanupStore = (*GoalRepo)(nil)

// ClaimLoopSessionCleanup lists the oldest unacknowledged run-owned session cleanup effects.
func (g *GoalRepo) ClaimLoopSessionCleanup(
	ctx context.Context,
	limit int,
) (obligations []looppkg.SessionCleanupObligation, err error) {
	if err := g.checkReady(ctx, "claim Loop session cleanup"); err != nil {
		return nil, err
	}
	if limit < 0 || limit > 200 {
		return nil, fmt.Errorf("%w: Loop cleanup claim limit must be between 0 and 200", looppkg.ErrValidation)
	}
	if limit == 0 {
		limit = 50
	}
	rows, err := sqlcgen.New(g.db).ClaimLoopSessionCleanup(ctx, sqlcgen.ClaimLoopSessionCleanupParams{
		UnsettledFailureCode: goalNullableString(goalBindingFailureStopCreationUnsettled), ClaimLimit: int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("store: claim Loop session cleanup: %w", err)
	}
	obligations = make([]looppkg.SessionCleanupObligation, 0, len(rows))
	for _, row := range rows {
		obligation := loopSessionCleanupFromGenerated(row)
		if err := obligation.Validate(); err != nil {
			return nil, fmt.Errorf("store: validate claimed Loop session cleanup: %w", err)
		}
		obligations = append(obligations, obligation)
	}
	return obligations, nil
}

// ReconcileLoopSessionCleanup releases Stop/create races left in-flight by a previous daemon process.
func (g *GoalRepo) ReconcileLoopSessionCleanup(ctx context.Context) error {
	if err := g.checkReady(ctx, "reconcile Loop session cleanup"); err != nil {
		return err
	}
	err := sqlcgen.New(g.db).ReconcileLoopSessionCleanup(ctx, sqlcgen.ReconcileLoopSessionCleanupParams{
		SettledFailureCode:   goalNullableString(goalBindingFailureControlRevokedInFlight),
		UnsettledFailureCode: goalNullableString(goalBindingFailureStopCreationUnsettled),
	})
	if err != nil {
		return fmt.Errorf("store: reconcile stopped Goal binding creation: %w", err)
	}
	return nil
}

// AcknowledgeLoopSessionCleanup records the first successful idempotent session Stop.
func (g *GoalRepo) AcknowledgeLoopSessionCleanup(
	ctx context.Context,
	cleanupID string,
	completedAt time.Time,
) error {
	if err := g.checkReady(ctx, "acknowledge Loop session cleanup"); err != nil {
		return err
	}
	cleanupID = strings.TrimSpace(cleanupID)
	if cleanupID == "" || completedAt.IsZero() {
		return fmt.Errorf("%w: Loop cleanup acknowledgement is incomplete", looppkg.ErrValidation)
	}
	completedAt = completedAt.UTC()
	return g.withImmediateTransaction(ctx, "acknowledge Loop session cleanup", func(exec globalSQLExecutor) error {
		obligation, err := loadLoopSessionCleanup(ctx, exec, cleanupID)
		if err != nil {
			return err
		}
		if obligation.CompletedAt != nil {
			return nil
		}
		if completedAt.Before(obligation.CreatedAt) {
			return fmt.Errorf("%w: Loop cleanup completion precedes creation", looppkg.ErrValidation)
		}
		affected, err := sqlcgen.New(exec).
			AcknowledgeLoopSessionCleanup(ctx, sqlcgen.AcknowledgeLoopSessionCleanupParams{
				CompletedAt: store.FormatTimestamp(completedAt), CleanupID: cleanupID,
			})
		if err != nil {
			return fmt.Errorf("store: acknowledge Loop session cleanup %q: %w", cleanupID, err)
		}
		return requireGoalAffectedCount(affected, "acknowledge Loop session cleanup")
	})
}

func enqueueGoalSessionCleanupWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	binding goal.SessionBinding,
	cause looppkg.SessionCleanupCause,
	createdAt time.Time,
) error {
	if binding.Ownership != goal.BindingOwnershipRunOwned {
		return nil
	}
	return enqueueLoopSessionCleanupWithExecutor(ctx, exec, looppkg.SessionCleanupObligation{
		CleanupID: loopSessionCleanupID(
			binding.Key.LoopRunID,
			looppkg.SessionCleanupSourceGoalBinding,
			binding.Key.Handle,
			binding.BindingEpoch,
		),
		WorkspaceID: binding.Key.WorkspaceID,
		LoopRunID:   binding.Key.LoopRunID,
		SourceKind:  looppkg.SessionCleanupSourceGoalBinding,
		SourceID:    binding.Key.Handle,
		SourceEpoch: binding.BindingEpoch,
		SessionID:   binding.SessionID,
		Cause:       cause,
		CreatedAt:   createdAt.UTC(),
	})
}

func enqueueTaskRunSessionCleanupWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID looppkg.WorkspaceID,
	loopRunID looppkg.RunID,
	taskRunID string,
	sessionID string,
	cause looppkg.SessionCleanupCause,
	createdAt time.Time,
) error {
	return enqueueLoopSessionCleanupWithExecutor(ctx, exec, looppkg.SessionCleanupObligation{
		CleanupID:   loopSessionCleanupID(loopRunID, looppkg.SessionCleanupSourceTaskRun, taskRunID, 0),
		WorkspaceID: workspaceID,
		LoopRunID:   loopRunID,
		SourceKind:  looppkg.SessionCleanupSourceTaskRun,
		SourceID:    strings.TrimSpace(taskRunID),
		SessionID:   strings.TrimSpace(sessionID),
		Cause:       cause,
		CreatedAt:   createdAt.UTC(),
	})
}

func enqueueLoopSessionCleanupWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	obligation looppkg.SessionCleanupObligation,
) error {
	obligation.SourceID = strings.TrimSpace(obligation.SourceID)
	if err := obligation.Validate(); err != nil {
		return err
	}
	if err := sqlcgen.New(exec).EnqueueLoopSessionCleanup(ctx, sqlcgen.EnqueueLoopSessionCleanupParams{
		CleanupID: obligation.CleanupID, WorkspaceID: string(obligation.WorkspaceID),
		LoopRunID: string(obligation.LoopRunID), SourceKind: string(obligation.SourceKind),
		SourceID: obligation.SourceID, SourceEpoch: obligation.SourceEpoch,
		SessionID: obligation.SessionID, Cause: string(obligation.Cause),
		CreatedAt: store.FormatTimestamp(obligation.CreatedAt),
	}); err != nil {
		return fmt.Errorf("store: enqueue Loop session cleanup %q: %w", obligation.CleanupID, err)
	}
	persisted, err := loadLoopSessionCleanup(ctx, exec, obligation.CleanupID)
	if err != nil {
		return err
	}
	if !loopSessionCleanupIdentityMatches(persisted, obligation) {
		return fmt.Errorf(
			"%w: Loop session cleanup %q payload changed",
			looppkg.ErrTransitionConflict,
			obligation.CleanupID,
		)
	}
	return nil
}

func loopSessionCleanupID(
	loopRunID looppkg.RunID,
	sourceKind looppkg.SessionCleanupSourceKind,
	sourceID string,
	sourceEpoch int64,
) string {
	return fmt.Sprintf(
		"loop-session-cleanup:%s:%s:%s:%d",
		loopRunID,
		sourceKind,
		strings.TrimSpace(sourceID),
		sourceEpoch,
	)
}

func loadLoopSessionCleanup(
	ctx context.Context,
	exec globalSQLExecutor,
	cleanupID string,
) (looppkg.SessionCleanupObligation, error) {
	row, err := sqlcgen.New(exec).GetLoopSessionCleanup(ctx, strings.TrimSpace(cleanupID))
	if errors.Is(err, sql.ErrNoRows) {
		return looppkg.SessionCleanupObligation{}, fmt.Errorf(
			"%w: Loop session cleanup %q",
			looppkg.ErrTransitionConflict,
			cleanupID,
		)
	}
	if err != nil {
		return looppkg.SessionCleanupObligation{}, fmt.Errorf("store: load Loop session cleanup %q: %w", cleanupID, err)
	}
	obligation := loopSessionCleanupFromGenerated(row)
	if err := obligation.Validate(); err != nil {
		return looppkg.SessionCleanupObligation{}, err
	}
	return obligation, nil
}

func loopSessionCleanupIdentityMatches(
	persisted looppkg.SessionCleanupObligation,
	want looppkg.SessionCleanupObligation,
) bool {
	return persisted.CleanupID == want.CleanupID && persisted.WorkspaceID == want.WorkspaceID &&
		persisted.LoopRunID == want.LoopRunID && persisted.SourceKind == want.SourceKind &&
		persisted.SourceID == want.SourceID && persisted.SourceEpoch == want.SourceEpoch &&
		persisted.SessionID == want.SessionID
}
