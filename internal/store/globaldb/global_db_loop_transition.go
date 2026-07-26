package globaldb

import (
	"context"
	"fmt"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func compareAndSwapLoopRunStatusWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	from looppkg.Status,
	to looppkg.Status,
	cause looppkg.TransitionCause,
	at time.Time,
) error {
	current, err := getLoopRunByIDWithExecutor(ctx, exec, runID)
	if err != nil {
		return err
	}
	clearControlState := loopStatusClearsControlState(to)
	affected, err := sqlcgen.New(exec).CompareAndSwapLoopRunStatus(ctx, sqlcgen.CompareAndSwapLoopRunStatusParams{
		Status: string(to), ClearControlState: int64(boolToInt(clearControlState)),
		ID: string(runID), ExpectedStatus: string(from),
	})
	if err != nil {
		return fmt.Errorf("store: transition loop run %q: %w", runID, err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: run_id=%s from=%s to=%s", looppkg.ErrTransitionConflict, runID, from, to)
	}
	if err := appendLoopRunStatusEvent(ctx, exec, runID, current.WorkspaceID, from, to, cause, at); err != nil {
		return err
	}
	if to.Terminal() {
		if err := sweepOrphanedLoopOutputBlobsWithExecutor(ctx, exec); err != nil {
			return err
		}
	}
	return nil
}

func loopStatusClearsControlState(status looppkg.Status) bool {
	switch status {
	case looppkg.StatusRunning,
		looppkg.StatusPaused,
		looppkg.StatusDone,
		looppkg.StatusNoOp,
		looppkg.StatusBlocked,
		looppkg.StatusFailed,
		looppkg.StatusExhausted,
		looppkg.StatusStalled:
		return true
	default:
		return false
	}
}
