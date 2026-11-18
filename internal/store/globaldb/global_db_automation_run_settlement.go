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

// SettleRun finalizes only the unfinished execution observed by the caller.
func (g *AutomationRepo) SettleRun(ctx context.Context, expected automation.Run,
	status automation.RunStatus, reason string, endedAt time.Time,
) (automation.Run, error) {
	if err := g.checkReady(ctx, "settle automation run"); err != nil {
		return automation.Run{}, err
	}
	if status != automation.RunFailed && status != automation.RunCancelled {
		return automation.Run{}, errors.New("store: automation settlement must fail or cancel a run")
	}
	affected, err := executeWriteQueryResult(ctx, g.repoBase, "globaldb.SettleAutomationRun",
		func(ctx context.Context, queries *sqlcgen.Queries) (int64, error) {
			return queries.SettleAutomationRun(ctx, sqlcgen.SettleAutomationRunParams{
				ID:                expected.ID,
				Status:            string(status),
				Error:             nullableAutomationString(reason),
				EndedAt:           nullableAutomationString(store.FormatTimestamp(endedAt)),
				ExpectedStatus:    string(expected.Status),
				ExpectedSessionID: sql.NullString{String: expected.SessionID, Valid: true},
				ExpectedTaskID:    sql.NullString{String: expected.TaskID, Valid: true},
				ExpectedTaskRunID: sql.NullString{String: expected.TaskRunID, Valid: true},
				ExpectedLoopRunID: sql.NullString{String: expected.LoopRunID, Valid: true},
			})
		})
	if err != nil {
		return automation.Run{}, err
	}
	current, err := g.GetRun(ctx, expected.ID)
	if err != nil {
		return automation.Run{}, err
	}
	if affected == 0 && (current.Status == automation.RunScheduled || current.Status == automation.RunRunning) {
		return automation.Run{}, automation.ErrRunReservationConflict
	}
	return current, nil
}
