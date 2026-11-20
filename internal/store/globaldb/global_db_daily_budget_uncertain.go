package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *ObserveRepo) DailyBudgetSessionHasEnforcement(ctx context.Context, sessionID string) (bool, error) {
	if err := g.checkReady(ctx, "inspect session budget enforcement"); err != nil {
		return false, err
	}
	count, err := g.readQueries.HasDailyBudgetEnforcementForSession(ctx, sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return count != 0, err
}

func (g *ObserveRepo) CheckUnresolvedSessionBudgetPolicy(ctx context.Context, sessionID string) error {
	if err := g.checkReady(ctx, "inspect uncertain session budget policy"); err != nil {
		return err
	}
	return checkUnresolvedSessionBudgetPolicy(ctx, g.readQueries, sessionID)
}

func (g *ObserveRepo) ListUnresolvedSessionBudgetCandidates(
	ctx context.Context, sessionID string,
) ([]store.DailyBudgetPotentialOwner, error) {
	if err := g.checkReady(ctx, "list uncertain session budget candidates"); err != nil {
		return nil, err
	}
	rows, err := unresolvedSessionBudgetCandidates(ctx, g.readQueries, sessionID)
	if err != nil {
		return nil, err
	}
	result := make([]store.DailyBudgetPotentialOwner, 0, len(rows))
	for _, row := range rows {
		createdAt, err := store.ParseTimestamp(row.InstanceCreatedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, store.DailyBudgetPotentialOwner{InstanceID: row.InstanceID, CreatedAt: createdAt})
	}
	return result, nil
}

func unresolvedSessionBudgetCandidates(
	ctx context.Context, q *sqlcgen.Queries, sessionID string,
) ([]sqlcgen.ListUnresolvedSessionBudgetOwnersRow, error) {
	rows, err := q.ListUnresolvedSessionBudgetOwners(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("store: read plausible budget owners for session %q: %w", sessionID, err)
	}
	return rows, nil
}

func checkUnresolvedSessionBudgetPolicy(ctx context.Context, q *sqlcgen.Queries, sessionID string) error {
	rows, err := unresolvedSessionBudgetCandidates(ctx, q, sessionID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.EnforceBudget != 0 {
			return fmt.Errorf("%w: %w for session %q",
				store.ErrDailyBudgetRecoveryPending, agentinstance.ErrBudgetHistoryUncertain, sessionID)
		}
	}
	return nil
}

func markUnresolvedBudgetReceipt(
	ctx context.Context, q *sqlcgen.Queries, sessionID string, at time.Time,
) error {
	rows, err := unresolvedSessionBudgetCandidates(ctx, q, sessionID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		createdAt, err := store.ParseTimestamp(row.InstanceCreatedAt)
		if err != nil {
			return err
		}
		if at.Before(createdAt) {
			continue // earlier receipts must not affect an instance created later
		}
		if err := q.MarkUncertainDailyBudgetDay(ctx, sqlcgen.MarkUncertainDailyBudgetDayParams{
			InstanceID: row.InstanceID, Day: store.LocalDay(at), SourceSessionID: sessionID,
			RecordedAt: store.FormatTimestamp(at),
		}); err != nil {
			return err
		}
	}
	return nil
}

func countUncertainBudgetDay(ctx context.Context, q *sqlcgen.Queries, instanceID string, at time.Time) (bool, error) {
	count, err := q.CountUncertainDailyBudgetDays(ctx, sqlcgen.CountUncertainDailyBudgetDaysParams{
		InstanceID: instanceID, Day: store.LocalDay(at),
	})
	return count != 0, err
}
