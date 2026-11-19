package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
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
		q := sqlcgen.New(tx)
		owner, err := q.GetTokenUsageReceiptSessionOwner(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("store: resolve daily budget turn owner: %w", err)
		}
		if owner.InstanceID == "" {
			allowed = true
			return nil
		}
		row, err := q.GetAgentInstance(ctx, owner.InstanceID)
		if err != nil {
			return fmt.Errorf("store: resolve daily budget instance: %w", err)
		}
		if row.ProfileID != owner.ProfileID || row.ArchivedAt.Valid {
			return agentinstance.ErrScopeMismatch
		}
		// A budget pause is operator-owned even when its caps are later removed.
		if row.State == "paused" && row.PauseReason == "daily_budget" {
			return nil
		}
		limitsRow, err := q.GetAgentInstanceDailyBudgetLimits(ctx, owner.InstanceID)
		if errors.Is(err, sql.ErrNoRows) {
			allowed = true
			return nil
		}
		if err != nil {
			return fmt.Errorf("store: read daily budget admission limits: %w", err)
		}
		limits, err := decodeInstanceDailyBudgetLimits(limitsRow)
		if err != nil {
			return err
		}
		if limits.Tokens == nil && limits.USD == nil {
			allowed = true
			return nil
		}
		if row.State != "active" {
			return nil
		}
		now := r.now()
		usage, err := readDailyBudgetUsage(ctx, q, owner.ProfileID, owner.InstanceID, store.LocalDay(now))
		if err != nil {
			return err
		}
		assessment, err := agentinstance.EvaluateDailyBudget(limits, usage, false)
		if err != nil {
			return err
		}
		if !assessment.Exhausted {
			allowed = true
			return nil
		}
		stamp := store.FormatTimestamp(now)
		changed, err := q.PauseAgentInstance(ctx, sqlcgen.PauseAgentInstanceParams{
			ID: owner.InstanceID, PauseReason: "daily_budget", UpdatedAt: stamp,
		})
		if err != nil {
			return err
		}
		if changed != 1 {
			return errors.New("store: daily budget pause lost active instance ownership")
		}
		if err := q.CancelInstanceAutoPicks(ctx, sqlcgen.CancelInstanceAutoPicksParams{
			CanceledAt: sql.NullString{String: stamp, Valid: true},
			UpdatedAt:  stamp, InstanceID: owner.InstanceID,
		}); err != nil {
			return err
		}
		if err := q.CancelInstanceWake(ctx, owner.InstanceID); err != nil {
			return err
		}
		instance, err := q.GetAgentInstance(ctx, owner.InstanceID)
		if err != nil {
			return err
		}
		paused, err := instanceFromRow(instance)
		if err != nil {
			return err
		}
		trip = &store.InstancePauseTrip{
			InstanceID: paused.ID, ProfileID: paused.ProfileID,
			WorkspaceID: paused.WorkspaceID, AgentName: paused.AgentName,
			MainSessionID: paused.MainSessionID, Reason: paused.PauseReason,
			CreatedAt: paused.CreatedAt, PausedAt: paused.UpdatedAt,
		}
		return nil
	})
	if err != nil {
		return false, nil, err
	}
	return allowed, trip, nil
}
