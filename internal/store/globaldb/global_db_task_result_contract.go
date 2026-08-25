package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

var _ taskpkg.ResultContractRepairStore = (*TaskRepo)(nil)

// AdmitResultContractRepair verifies the current completion fence and records
// exactly one repair event in the same write transaction.
func (g *TaskRepo) AdmitResultContractRepair(
	ctx context.Context,
	admission taskpkg.ResultContractRepairAdmission,
) (bool, error) {
	if err := g.checkReady(ctx, "admit task result contract repair"); err != nil {
		return false, err
	}
	event, err := g.normalizeTaskEventForCreate(admission.Event)
	if err != nil {
		return false, err
	}
	if event.EventType != "task.run_rejected" {
		return false, fmt.Errorf(
			"%w: result contract repair requires task.run_rejected event",
			taskpkg.ErrValidation,
		)
	}
	if admission.Now.IsZero() {
		return false, fmt.Errorf("%w: result contract repair time is required", taskpkg.ErrValidation)
	}

	inserted := false
	err = g.withTaskImmediateTransaction(ctx, "admit task result contract repair", func(exec taskSQLExecutor) error {
		run, loadErr := g.getTaskRunWithExecutor(ctx, exec, event.RunID)
		if loadErr != nil {
			return loadErr
		}
		if run.TaskID != event.TaskID {
			return fmt.Errorf(
				"%w: task run %q does not belong to task %q",
				taskpkg.ErrValidation,
				run.ID,
				event.TaskID,
			)
		}
		actor := taskpkg.ActorContext{Actor: event.Actor, Origin: event.Origin}
		if err := requireResultContractRepairAuthority(run, admission.ClaimToken, admission.Now, actor); err != nil {
			return err
		}

		existing, lookupErr := sqlcgen.New(exec).GetTaskEventRecord(ctx, event.ID)
		switch {
		case lookupErr == nil:
			if existing.TaskID != event.TaskID || existing.RunID.String != event.RunID ||
				existing.EventType != event.EventType {
				return fmt.Errorf("%w: result contract repair event id conflict", taskpkg.ErrConflict)
			}
			return nil
		case !errors.Is(lookupErr, sql.ErrNoRows):
			return fmt.Errorf("store: lookup result contract repair event: %w", lookupErr)
		}

		_, appendErr := appendTaskEventRecordWithExecutor(ctx, exec, EventRecordInsert{
			ID: event.ID, TaskID: event.TaskID, RunID: event.RunID, EventType: event.EventType,
			Actor: event.Actor, Origin: event.Origin, Payload: event.Payload, Timestamp: event.Timestamp,
		})
		if appendErr != nil {
			return appendErr
		}
		inserted = true
		return nil
	})
	return inserted, err
}

func requireResultContractRepairAuthority(
	run taskpkg.Run,
	claimToken string,
	now time.Time,
	actor taskpkg.ActorContext,
) error {
	if strings.TrimSpace(claimToken) != "" {
		if err := requireCurrentRunLease(run, claimToken, now); err != nil {
			return err
		}
		return taskpkg.RequireLeaseSettlementActor(run, actor)
	}
	if strings.TrimSpace(run.ClaimTokenHash) != "" {
		return fmt.Errorf("%w: task run %q requires token-fenced completion", taskpkg.ErrInvalidClaimToken, run.ID)
	}
	if run.Status.Normalize() != taskpkg.TaskRunStatusRunning {
		return fmt.Errorf(
			"%w: task run %q cannot repair a result from %q",
			taskpkg.ErrInvalidStatusTransition,
			run.ID,
			run.Status.Normalize(),
		)
	}
	return nil
}
