package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	automation "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ReserveRun atomically checks a rolling fire-limit window and reserves one run.
func (g *AutomationRepo) ReserveRun(
	ctx context.Context,
	reservation automation.RunReservation,
) (automation.RunReservationResult, error) {
	if err := g.checkReady(ctx, "reserve automation run"); err != nil {
		return automation.RunReservationResult{}, err
	}

	normalized, params, err := g.prepareAutomationRunInsert(reservation.Run)
	if err != nil {
		return automation.RunReservationResult{}, err
	}
	reservation.Run = normalized
	reservation.ExistingRunID = strings.TrimSpace(reservation.ExistingRunID)
	if err := validateAutomationRunReservation(reservation); err != nil {
		return automation.RunReservationResult{}, err
	}

	result := automation.RunReservationResult{Run: normalized}
	err = g.withImmediateTransaction(ctx, "reserve automation run", func(exec globalSQLExecutor) error {
		result = automation.RunReservationResult{Run: normalized}
		count, retryAt, err := evaluateAutomationRunReservation(ctx, exec, reservation)
		if err != nil {
			return err
		}
		result.Count = count
		result.RetryAt = retryAt
		result.Reserved = false
		if count >= int64(reservation.Limit) {
			return nil
		}

		if reservation.ExistingRunID != "" {
			activated, err := activateExistingAutomationRunReservation(ctx, exec, reservation)
			if err != nil {
				return err
			}
			result.Run = activated
		} else {
			if err := sqlcgen.New(exec).InsertAutomationRun(ctx, params); err != nil {
				return fmt.Errorf(
					"store: reserve automation run %q: %w",
					normalized.ID,
					mapAutomationRunConstraintError(err),
				)
			}
		}
		result.Reserved = true
		return nil
	})
	if err != nil {
		return automation.RunReservationResult{}, err
	}

	return result, nil
}

func validateAutomationRunReservation(reservation automation.RunReservation) error {
	if err := validateAutomationRunReservationShape(reservation); err != nil {
		return err
	}
	return validateAutomationRunReservationStatuses(reservation)
}

func validateAutomationRunReservationShape(reservation automation.RunReservation) error {
	switch {
	case reservation.ExistingRunID != "" && reservation.ExistingRunID != reservation.Run.ID:
		return errors.New("store: existing automation run id must match reservation run id")
	case reservation.ExistingRunID != "" && reservation.ExpectedStatus == "":
		return errors.New("store: existing automation run expected status is required")
	case reservation.ExistingRunID != "" && reservation.ExpectedAttempt <= 0:
		return errors.New("store: existing automation run expected attempt must be positive")
	case reservation.ExistingRunID == "" && reservation.ExpectedStatus != "":
		return errors.New("store: new automation run must not define an expected status")
	case reservation.ExistingRunID == "" && reservation.ExpectedAttempt != 0:
		return errors.New("store: new automation run must not define an expected attempt")
	case reservation.Since.IsZero():
		return errors.New("store: automation run reservation since is required")
	case reservation.Until.IsZero():
		return errors.New("store: automation run reservation until is required")
	case reservation.Until.Before(reservation.Since):
		return errors.New("store: automation run reservation until must not be before since")
	case reservation.Window <= 0:
		return errors.New("store: automation run reservation window must be positive")
	case !reservation.Since.Add(reservation.Window).Equal(reservation.Until):
		return errors.New("store: automation run reservation bounds must match window")
	case reservation.Limit <= 0:
		return errors.New("store: automation run reservation limit must be positive")
	case len(reservation.CountedStatuses) == 0:
		return errors.New("store: automation run reservation counted statuses are required")
	}
	return nil
}

func validateAutomationRunReservationStatuses(reservation automation.RunReservation) error {
	if reservation.ExpectedStatus != "" {
		if err := reservation.ExpectedStatus.Validate("run_reservation.expected_status"); err != nil {
			return err
		}
	}

	seen := make(map[automation.RunStatus]struct{}, len(reservation.CountedStatuses))
	for _, status := range reservation.CountedStatuses {
		if err := status.Validate("run_reservation.counted_statuses"); err != nil {
			return err
		}
		if _, exists := seen[status]; exists {
			return fmt.Errorf("store: duplicate automation run reservation status %q", status)
		}
		seen[status] = struct{}{}
	}
	return nil
}

func activateExistingAutomationRunReservation(
	ctx context.Context,
	exec globalSQLExecutor,
	reservation automation.RunReservation,
) (automation.Run, error) {
	// dynamic-sql: job and trigger reservations use mutually exclusive identity predicates.
	query := `UPDATE automation_runs
		SET status = ?, attempt = ?, session_id = NULL, task_id = NULL,
			task_run_id = NULL, loop_run_id = NULL, ended_at = NULL, error = NULL,
			delivery_error = NULL, delivery_error_at = NULL
		WHERE id = ? AND status = ? AND attempt = ?`
	args := []any{
		string(automation.RunRunning),
		reservation.Run.Attempt,
		reservation.ExistingRunID,
		string(reservation.ExpectedStatus),
		reservation.ExpectedAttempt,
	}
	if reservation.Run.JobID != "" {
		query += " AND job_id = ? AND trigger_id IS NULL"
		args = append(args, reservation.Run.JobID)
	} else {
		query += " AND trigger_id = ? AND job_id IS NULL"
		args = append(args, reservation.Run.TriggerID)
	}

	result, err := exec.ExecContext(ctx, query, args...)
	if err != nil {
		return automation.Run{}, fmt.Errorf("store: activate existing automation run reservation: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return automation.Run{}, fmt.Errorf("store: count activated automation run reservation rows: %w", err)
	}
	if affected != 1 {
		return automation.Run{}, fmt.Errorf(
			"store: automation run %q: %w",
			reservation.ExistingRunID,
			automation.ErrRunReservationConflict,
		)
	}

	activated := reservation.Run
	activated.Status = automation.RunRunning
	activated.SessionID = ""
	activated.TaskID = ""
	activated.TaskRunID = ""
	activated.LoopRunID = ""
	activated.EndedAt = nil
	activated.Error = ""
	activated.DeliveryError = ""
	activated.DeliveryErrorAt = nil
	return activated, nil
}

func evaluateAutomationRunReservation(
	ctx context.Context,
	exec globalSQLExecutor,
	reservation automation.RunReservation,
) (int64, time.Time, error) {
	where, args := buildAutomationRunClauses(automation.RunQuery{
		JobID:     reservation.Run.JobID,
		TriggerID: reservation.Run.TriggerID,
		ExcludeID: reservation.ExistingRunID,
		Since:     reservation.Since,
		Until:     reservation.Until,
	})
	where = append(where, sqlInClause("status", len(reservation.CountedStatuses)))
	for _, status := range reservation.CountedStatuses {
		args = append(args, string(status))
	}

	// dynamic-sql: the explicit counted status set determines the placeholder count.
	query := store.AppendWhere("SELECT COUNT(*), MIN(started_at) FROM automation_runs", where)
	var (
		count       int64
		earliestRaw sql.NullString
	)
	if err := exec.QueryRowContext(ctx, query, args...).Scan(&count, &earliestRaw); err != nil {
		return 0, time.Time{}, fmt.Errorf("store: evaluate automation run reservation: %w", err)
	}
	if count < int64(reservation.Limit) {
		return count, time.Time{}, nil
	}
	if !earliestRaw.Valid {
		return count, reservation.Until, nil
	}

	earliest, err := store.ParseTimestamp(earliestRaw.String)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("store: parse automation run reservation retry time: %w", err)
	}
	retryAt := earliest.Add(reservation.Window)
	if retryAt.Before(reservation.Until) {
		retryAt = reservation.Until
	}
	return count, retryAt, nil
}
