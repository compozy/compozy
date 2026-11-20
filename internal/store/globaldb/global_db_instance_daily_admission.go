package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

const (
	dailyBudgetPauseReason         = "daily_budget"
	dailyBudgetRecoveryPauseReason = "daily_budget_recovery"
)

// AdmitDailyBudgetTurn serializes a *covered, fresh* turn's budget decision
// with receipt writes and operator resume. The caller must exclude human-origin
// and recovery work; this is not a generic session-pause admission gate.
// A newly exhausted instance is paused durably; the caller must deliver the
// returned trip to existing pause effects. A positive result authorizes only
// the immediately dispatched turn, not a durable future queue reservation.
// Paused budget instances never auto-resume at the next daily boundary.
func (r *InstanceRepo) AdmitDailyBudgetTurn(
	ctx context.Context,
	sessionID string,
) (allowed bool, trip *store.InstancePauseTrip, err error) {
	if err := r.checkReady(ctx, "admit daily budget turn"); err != nil {
		return false, nil, err
	}
	if sessionID == "" || strings.TrimSpace(sessionID) != sessionID {
		return false, nil, errors.New("agentinstance: canonical session id required for budget admission")
	}
	err = store.ExecuteWriteOperation(ctx, r.db, "admit daily budget turn", func(
		ctx context.Context, tx *store.WriteTx,
	) error {
		allowed, trip = false, nil // retries must discard results from a rolled-back attempt
		allowed, trip, err = admitDailyBudgetTurnTx(ctx, sqlcgen.New(tx), sessionID, r.now())
		return err
	})
	if err != nil {
		return false, nil, err
	}
	return allowed, trip, nil
}

// admitDailyBudgetTurnTx is the shared transactional admission fence. Queue
// claims and durable prompt commits call it BEFORE mutating their dispatch
// state, so a denied turn never consumes a lease or idempotency receipt.
func admitDailyBudgetTurnTx(
	ctx context.Context, q *sqlcgen.Queries, sessionID string, now time.Time,
) (allowed bool, trip *store.InstancePauseTrip, err error) {
	requiresEnforcement, err := q.HasDailyBudgetEnforcementForSession(ctx, sessionID)
	if err != nil {
		return false, nil, fmt.Errorf("store: check daily budget enforcement: %w", err)
	}
	if requiresEnforcement == 0 {
		return true, nil, nil
	}
	owner, err := q.GetTokenUsageReceiptSessionOwner(ctx, sessionID)
	if err != nil {
		return false, nil, fmt.Errorf("store: resolve daily budget turn owner: %w", err)
	}
	if owner.OwnershipUnresolved != 0 {
		if err := checkUnresolvedSessionBudgetPolicy(ctx, q, sessionID); err != nil {
			return false, nil, err
		}
	}
	if owner.InstanceID == "" {
		return true, nil, nil
	}
	return admitOwnedDailyBudgetTurn(ctx, q, owner, now)
}

func admitOwnedDailyBudgetTurn(
	ctx context.Context, q *sqlcgen.Queries, owner sqlcgen.GetTokenUsageReceiptSessionOwnerRow, now time.Time,
) (bool, *store.InstancePauseTrip, error) {
	row, err := q.GetAgentInstance(ctx, owner.InstanceID)
	if err != nil {
		return false, nil, fmt.Errorf("store: resolve daily budget instance: %w", err)
	}
	if row.ProfileID != owner.ProfileID {
		return false, nil, agentinstance.ErrScopeMismatch
	}
	if row.ArchivedAt.Valid {
		// Legacy human-priority inputs remain governed by the existing
		// archived-session selector, not an inactive instance budget.
		return true, nil, nil
	}

	// A budget pause is operator-owned even when its caps are later removed.
	if row.State == goalStatusPaused &&
		(row.PauseReason == dailyBudgetPauseReason || row.PauseReason == dailyBudgetRecoveryPauseReason) {
		return false, nil, nil
	}
	latched, err := q.CountLatchedAgentInstanceBudgetPauses(ctx, owner.InstanceID)
	if err != nil {
		return false, nil, fmt.Errorf("store: read daily budget latch for instance %q: %w", row.ID, err)
	}
	if latched > 0 {
		return false, nil, nil
	}
	limitsRow, err := q.GetAgentInstanceDailyBudgetLimits(ctx, owner.InstanceID)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil, nil
	}
	if err != nil {
		return false, nil, fmt.Errorf("store: read daily budget admission limits: %w", err)
	}
	limits, err := decodeInstanceDailyBudgetLimits(limitsRow)
	if err != nil {
		return false, nil, err
	}
	if limits.Tokens == nil && limits.USD == nil {
		return true, nil, nil
	}
	return admitConfiguredDailyBudgetTurn(ctx, q, row, limits, now)
}

func admitConfiguredDailyBudgetTurn(
	ctx context.Context,
	q *sqlcgen.Queries,
	row sqlcgen.AgentInstance,
	limits agentinstance.DailyBudgetLimits,
	now time.Time,
) (bool, *store.InstancePauseTrip, error) {
	generation, err := dailyBudgetRecoveryGeneration(ctx, q, row.ID)
	if err != nil {
		return false, nil, err
	}
	if generation != 0 {
		if row.State == "active" {
			return pauseDailyBudgetTurn(ctx, q, row.ID, dailyBudgetRecoveryPauseReason, now)
		}
		if row.State == goalStatusPaused {
			return false, nil, q.LatchAgentInstanceDailyBudgetPause(ctx, row.ID)
		}
		return false, nil, nil
	}
	uncertain, err := countUncertainBudgetDay(ctx, q, row.ID, now)
	if err != nil {
		return false, nil, err
	}
	if uncertain {
		return holdDailyBudgetTurn(ctx, q, row, now)
	}

	usage, err := readDailyBudgetUsage(ctx, q, row.ProfileID, row.ID, store.LocalDay(now))
	if err != nil {
		return false, nil, err
	}
	assessment, err := agentinstance.EvaluateDailyBudget(limits, usage, false)
	if err != nil {
		return false, nil, err
	}
	if !assessment.Exhausted {
		return true, nil, nil
	}
	return holdDailyBudgetTurn(ctx, q, row, now)
}

func holdDailyBudgetTurn(
	ctx context.Context, q *sqlcgen.Queries, row sqlcgen.AgentInstance, now time.Time,
) (bool, *store.InstancePauseTrip, error) {
	// Definition-paused descendants have a narrow, pre-existing dispatch
	// exemption. When their budget is exhausted, the budget pause must also
	// latch durably; otherwise that exemption dispatches again after midnight
	// without the required operator action.
	if row.State != "active" && row.State != goalStatusPaused {
		return false, nil, nil
	}
	if row.State == goalStatusPaused {
		// The original pause owns its reason and effects. Require operator
		// resume even after the spend window rolls over.
		return false, nil, q.LatchAgentInstanceDailyBudgetPause(ctx, row.ID)
	}
	return pauseDailyBudgetTurn(ctx, q, row.ID, dailyBudgetPauseReason, now)
}

func pauseDailyBudgetTurn(
	ctx context.Context, q *sqlcgen.Queries, instanceID, reason string, now time.Time,
) (bool, *store.InstancePauseTrip, error) {
	stamp := store.FormatTimestamp(now)
	changed, err := q.PauseAgentInstance(ctx, sqlcgen.PauseAgentInstanceParams{
		ID: instanceID, PauseReason: reason, UpdatedAt: stamp,
	})
	if err != nil {
		return false, nil, fmt.Errorf("store: persist daily budget pause for instance %q: %w", instanceID, err)
	}
	if changed != 1 {
		return false, nil, errors.New("store: daily budget pause lost instance ownership")
	}
	if err := q.CancelInstanceWake(ctx, instanceID); err != nil {
		return false, nil, fmt.Errorf("store: cancel wake for budget-paused instance %q: %w", instanceID, err)
	}
	instance, err := q.GetAgentInstance(ctx, instanceID)
	if err != nil {
		return false, nil, fmt.Errorf("store: load budget-paused instance %q: %w", instanceID, err)
	}
	paused, err := instanceFromRow(instance)
	if err != nil {
		return false, nil, err
	}
	trip := &store.InstancePauseTrip{
		InstanceID: paused.ID, ProfileID: paused.ProfileID,
		WorkspaceID: paused.WorkspaceID, AgentName: paused.AgentName,
		MainSessionID: paused.MainSessionID, Reason: paused.PauseReason,
		CreatedAt: paused.CreatedAt, PausedAt: paused.UpdatedAt,
	}
	return false, trip, nil
}
