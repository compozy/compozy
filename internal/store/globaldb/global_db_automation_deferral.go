package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	automation "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// SetScheduledDeferral changes only the fire that still owns this schedule cursor.
func (g *AutomationRepo) SetScheduledDeferral(
	ctx context.Context,
	claim automation.SchedulerClaim,
	retryAt *time.Time,
) (automation.SchedulerState, error) {
	if err := g.checkReady(ctx, "defer automation scheduled fire"); err != nil {
		return automation.SchedulerState{}, err
	}
	err := g.withImmediateTransaction(ctx, "defer automation scheduled fire", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		affected, err := queries.SetAutomationScheduledDeferral(ctx, sqlcgen.SetAutomationScheduledDeferralParams{
			RetryAt: nullableAutomationTime(retryAt), UpdatedAt: store.FormatTimestamp(g.now().UTC()),
			JobID: claim.JobID, FireID: claim.FireID, ScheduleHash: claim.ScheduleHash,
		})
		if err != nil {
			return err
		}
		if affected == 0 {
			return automation.ErrScheduledFireAlreadyClaimed
		}
		if retryAt == nil {
			return nil
		}
		affected, err = queries.RestoreUnstartedAutomationReservation(
			ctx,
			sqlcgen.RestoreUnstartedAutomationReservationParams{
				RunID:  claim.RunID,
				JobID:  nullableAutomationString(claim.JobID),
				FireID: nullableAutomationString(claim.FireID),
			},
		)
		if err != nil {
			return err
		}
		if affected != 1 {
			return automation.ErrRunReservationConflict
		}
		return nil
	})
	if err != nil {
		return automation.SchedulerState{}, err
	}
	return g.GetSchedulerState(ctx, claim.JobID)
}

func resumeDeferredScheduledRun(
	ctx context.Context,
	tx *sql.Tx,
	state automation.SchedulerState,
	claim automation.SchedulerClaim,
) (automation.SchedulerClaimResult, error) {
	row := tx.QueryRowContext(ctx, `SELECT
  id, `+automationRunProfileIDSQL+`, job_id, trigger_id, session_id, task_id, task_run_id, fire_id,
  status, attempt, scheduled_at, started_at, ended_at, error,
  delivery_error, delivery_error_at, loop_run_id, network_participation, metadata_json
  FROM automation_runs WHERE id = ?`, claim.RunID)
	run, err := scanAutomationRun(row)
	if err != nil {
		return automation.SchedulerClaimResult{}, err
	}
	if run.Status != automation.RunScheduled {
		state.DeferredUntil = nil
		if err := upsertSchedulerStateTx(ctx, tx, state); err != nil {
			return automation.SchedulerClaimResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return automation.SchedulerClaimResult{}, err
	}
	return automation.SchedulerClaimResult{State: state, Run: run, Skipped: run.Status != automation.RunScheduled}, nil
}

func (g *AutomationRepo) saveSchedulerState(
	ctx context.Context,
	state automation.SchedulerState,
) (automation.SchedulerState, error) {
	err := g.withImmediateTransaction(ctx, "save automation schedule", func(exec globalSQLExecutor) error {
		existing, err := getSchedulerStateTx(ctx, exec, state.JobID)
		if err != nil && !errors.Is(err, automation.ErrSchedulerStateNotFound) {
			return err
		}
		if existing.ScheduleHash != state.ScheduleHash {
			if err := cancelSupersededAutomationReservation(ctx, exec, state.JobID, g.now()); err != nil {
				return err
			}
		}
		return upsertSchedulerStateTx(ctx, exec, state)
	})
	return state, err
}

func cancelSupersededAutomationReservation(ctx context.Context, tx sqlcgen.DBTX, jobID string, now time.Time) error {
	return sqlcgen.New(tx).
		CancelSupersededAutomationReservation(ctx, sqlcgen.CancelSupersededAutomationReservationParams{
			JobID: nullableAutomationString(jobID), EndedAt: nullableAutomationString(store.FormatTimestamp(now.UTC())),
		})
}

func (g *AutomationRepo) deleteSchedulerState(ctx context.Context, jobID string) error {
	return g.withImmediateTransaction(ctx, "delete automation schedule", func(exec globalSQLExecutor) error {
		if err := cancelSupersededAutomationReservation(ctx, exec, jobID, g.now()); err != nil {
			return err
		}
		return sqlcgen.New(exec).DeleteAutomationSchedulerState(ctx, jobID)
	})
}
