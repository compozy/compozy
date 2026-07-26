package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

var _ goal.SessionCleanupStore = (*GoalRepo)(nil)

// ClaimGoalSessionCleanup lists the oldest unacknowledged run-owned session cleanup effects.
func (g *GoalRepo) ClaimGoalSessionCleanup(
	ctx context.Context,
	limit int,
) (obligations []goal.SessionCleanupObligation, err error) {
	if err := g.checkReady(ctx, "claim Goal session cleanup"); err != nil {
		return nil, err
	}
	if limit < 0 || limit > 200 {
		return nil, fmt.Errorf("%w: Goal cleanup claim limit must be between 0 and 200", looppkg.ErrValidation)
	}
	if limit == 0 {
		limit = 50
	}
	rows, err := sqlcgen.New(g.db).ClaimGoalSessionCleanup(ctx, sqlcgen.ClaimGoalSessionCleanupParams{
		UnsettledFailureCode: goalNullableString(goalBindingFailureStopCreationUnsettled), ClaimLimit: int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("store: claim Goal session cleanup: %w", err)
	}
	obligations = make([]goal.SessionCleanupObligation, 0, len(rows))
	for _, row := range rows {
		obligation := goalSessionCleanupFromGenerated(row)
		if err := obligation.Validate(); err != nil {
			return nil, fmt.Errorf("store: validate claimed Goal session cleanup: %w", err)
		}
		obligations = append(obligations, obligation)
	}
	return obligations, nil
}

// ReconcileGoalSessionCleanup releases Stop/create races left in-flight by a previous daemon process.
func (g *GoalRepo) ReconcileGoalSessionCleanup(ctx context.Context) error {
	if err := g.checkReady(ctx, "reconcile Goal session cleanup"); err != nil {
		return err
	}
	err := sqlcgen.New(g.db).ReconcileGoalSessionCleanup(ctx, sqlcgen.ReconcileGoalSessionCleanupParams{
		SettledFailureCode:   goalNullableString(goalBindingFailureStopCreationSettled),
		UnsettledFailureCode: goalNullableString(goalBindingFailureStopCreationUnsettled),
	})
	if err != nil {
		return fmt.Errorf("store: reconcile stopped Goal binding creation: %w", err)
	}
	return nil
}

// AcknowledgeGoalSessionCleanup records the first successful idempotent session Stop.
func (g *GoalRepo) AcknowledgeGoalSessionCleanup(
	ctx context.Context,
	cleanupID string,
	completedAt time.Time,
) error {
	if err := g.checkReady(ctx, "acknowledge Goal session cleanup"); err != nil {
		return err
	}
	cleanupID = strings.TrimSpace(cleanupID)
	if cleanupID == "" || completedAt.IsZero() {
		return fmt.Errorf("%w: Goal cleanup acknowledgement is incomplete", looppkg.ErrValidation)
	}
	completedAt = completedAt.UTC()
	return g.withImmediateTransaction(ctx, "acknowledge Goal session cleanup", func(exec globalSQLExecutor) error {
		obligation, err := loadGoalSessionCleanup(ctx, exec, cleanupID)
		if err != nil {
			return err
		}
		if obligation.CompletedAt != nil {
			return nil
		}
		if completedAt.Before(obligation.CreatedAt) {
			return fmt.Errorf("%w: Goal cleanup completion precedes creation", looppkg.ErrValidation)
		}
		affected, err := sqlcgen.New(exec).
			AcknowledgeGoalSessionCleanup(ctx, sqlcgen.AcknowledgeGoalSessionCleanupParams{
				CompletedAt: store.FormatTimestamp(completedAt), CleanupID: cleanupID,
			})
		if err != nil {
			return fmt.Errorf("store: acknowledge Goal session cleanup %q: %w", cleanupID, err)
		}
		return requireGoalAffectedCount(affected, "acknowledge Goal session cleanup")
	})
}

func enqueueGoalSessionCleanupWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	binding goal.SessionBinding,
	cause goal.SessionCleanupCause,
	createdAt time.Time,
) error {
	if binding.Ownership != goal.BindingOwnershipRunOwned {
		return nil
	}
	obligation := goal.SessionCleanupObligation{
		CleanupID:    goalSessionCleanupID(binding),
		WorkspaceID:  binding.Key.WorkspaceID,
		LoopRunID:    binding.Key.LoopRunID,
		Handle:       binding.Key.Handle,
		BindingEpoch: binding.BindingEpoch,
		SessionID:    binding.SessionID,
		Cause:        cause,
		CreatedAt:    createdAt.UTC(),
	}
	if err := obligation.Validate(); err != nil {
		return err
	}
	if err := sqlcgen.New(exec).EnqueueGoalSessionCleanup(ctx, sqlcgen.EnqueueGoalSessionCleanupParams{
		CleanupID: obligation.CleanupID, WorkspaceID: string(obligation.WorkspaceID),
		LoopRunID: string(obligation.LoopRunID), Handle: obligation.Handle, BindingEpoch: obligation.BindingEpoch,
		SessionID: obligation.SessionID, Cause: string(obligation.Cause),
		CreatedAt: store.FormatTimestamp(obligation.CreatedAt),
	}); err != nil {
		return fmt.Errorf("store: enqueue Goal session cleanup %q: %w", obligation.CleanupID, err)
	}
	persisted, err := loadGoalSessionCleanup(ctx, exec, obligation.CleanupID)
	if err != nil {
		return err
	}
	if !goalSessionCleanupMatches(persisted, obligation) {
		return fmt.Errorf(
			"%w: Goal session cleanup %q payload changed",
			looppkg.ErrTransitionConflict,
			obligation.CleanupID,
		)
	}
	return nil
}

func goalSessionCleanupID(binding goal.SessionBinding) string {
	return fmt.Sprintf(
		"goal-session-cleanup:%s:%s:%d",
		binding.Key.LoopRunID,
		binding.Key.Handle,
		binding.BindingEpoch,
	)
}

func loadGoalSessionCleanup(
	ctx context.Context,
	exec globalSQLExecutor,
	cleanupID string,
) (goal.SessionCleanupObligation, error) {
	row, err := sqlcgen.New(exec).GetGoalSessionCleanup(ctx, strings.TrimSpace(cleanupID))
	if errors.Is(err, sql.ErrNoRows) {
		return goal.SessionCleanupObligation{}, fmt.Errorf(
			"%w: Goal session cleanup %q",
			looppkg.ErrTransitionConflict,
			cleanupID,
		)
	}
	if err != nil {
		return goal.SessionCleanupObligation{}, fmt.Errorf("store: load Goal session cleanup %q: %w", cleanupID, err)
	}
	obligation := goalSessionCleanupFromGenerated(row)
	if err := obligation.Validate(); err != nil {
		return goal.SessionCleanupObligation{}, err
	}
	return obligation, nil
}

func goalSessionCleanupMatches(
	persisted goal.SessionCleanupObligation,
	want goal.SessionCleanupObligation,
) bool {
	return persisted.CleanupID == want.CleanupID && persisted.WorkspaceID == want.WorkspaceID &&
		persisted.LoopRunID == want.LoopRunID && persisted.Handle == want.Handle &&
		persisted.BindingEpoch == want.BindingEpoch && persisted.SessionID == want.SessionID &&
		persisted.Cause == want.Cause && persisted.CreatedAt.Equal(want.CreatedAt)
}
