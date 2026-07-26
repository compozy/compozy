package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func forceReleasedTaskRun(previous taskpkg.Run) taskpkg.Run {
	next := previous
	next.Status = taskpkg.TaskRunStatusQueued
	next.ClaimedBy = nil
	next.ClaimedAt = time.Time{}
	next.SessionID = ""
	next.ClaimTokenHash = ""
	next.LeaseUntil = time.Time{}
	next.HeartbeatAt = time.Time{}
	next.StartedAt = time.Time{}
	next.EndedAt = time.Time{}
	next.Error = ""
	next.FailureKind = ""
	next.Result = nil
	return next
}

func forceFailedTaskRun(previous taskpkg.Run, reason string, now time.Time) taskpkg.Run {
	next := previous
	next.Status = taskpkg.TaskRunStatusFailed
	next.Error = strings.TrimSpace(reason)
	next.FailureKind = taskpkg.FailureKindOperatorForced
	next.Result = nil
	next.ClaimTokenHash = ""
	next.LeaseUntil = time.Time{}
	next.HeartbeatAt = time.Time{}
	next.EndedAt = now
	return next
}

func forceFailTaskRunStatusAllowed(status taskpkg.RunStatus) bool {
	switch status.Normalize() {
	case taskpkg.TaskRunStatusQueued, taskpkg.TaskRunStatusClaimed:
		return true
	default:
		return false
	}
}

func updateTaskRunRecordWithSnapshotCAS(
	ctx context.Context,
	exec taskSQLExecutor,
	previous taskpkg.Run,
	next taskpkg.Run,
) error {
	params, err := forceTaskRunSnapshotParams(previous, next)
	if err != nil {
		return err
	}
	rowsAffected, err := sqlcgen.New(exec).ForceUpdateTaskRunSnapshot(
		ctx,
		params,
	)
	if err != nil {
		return fmt.Errorf("store: force update task run %q: %w", next.ID, err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return forceRunCASMiss(ctx, exec, next.ID)
}

func forceRunCASMiss(ctx context.Context, exec taskSQLExecutor, runID string) error {
	if _, err := sqlcgen.New(exec).GetTaskRunTaskID(ctx, runID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.ErrTaskRunNotFound
		}
		return fmt.Errorf("store: inspect force update CAS miss for task run %q: %w", runID, err)
	}
	return fmt.Errorf(
		"%w: task run %q changed before force operation applied",
		taskpkg.ErrInvalidStatusTransition,
		runID,
	)
}

func requireRetryDepthWithExecutor(ctx context.Context, exec taskSQLExecutor, source taskpkg.Run) error {
	byID, err := taskRunsByIDForTaskWithExecutor(ctx, exec, source.TaskID)
	if err != nil {
		return err
	}
	depth := 0
	for current := source; strings.TrimSpace(current.PreviousRunID) != ""; {
		depth++
		if depth >= taskpkg.MaxRetryRunChainDepth {
			return taskpkg.ErrRetryChainTooDeep
		}
		previous, ok := byID[strings.TrimSpace(current.PreviousRunID)]
		if !ok {
			break
		}
		current = previous
	}
	return nil
}

func taskRunsByIDForTaskWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
) (runs map[string]taskpkg.Run, err error) {
	runIDs, err := sqlcgen.New(exec).ListTaskRunIDsForTask(ctx, nullableTaskString(taskID))
	if err != nil {
		return nil, fmt.Errorf("store: list retry lineage runs for task %q: %w", taskID, err)
	}
	byID := make(map[string]taskpkg.Run, len(runIDs))
	for _, runID := range runIDs {
		row, err := sqlcgen.New(exec).GetTaskRun(ctx, runID)
		if err != nil {
			return nil, fmt.Errorf("store: get retry lineage run %q: %w", runID, err)
		}
		run, err := taskRunFromGenerated(&row)
		if err != nil {
			return nil, err
		}
		byID[run.ID] = run
	}
	return byID, nil
}

func requireNoRetryChildWithExecutor(ctx context.Context, exec taskSQLExecutor, sourceRunID string) error {
	existing, err := sqlcgen.New(exec).GetRetryChildRunID(ctx, nullableTaskString(sourceRunID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("store: lookup retry child for task run %q: %w", sourceRunID, err)
	}
	return fmt.Errorf(
		"%w: task run %q already has retry run %q",
		taskpkg.ErrInvalidStatusTransition,
		sourceRunID,
		existing,
	)
}

func forceRunCASTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return store.FormatTimestamp(value.UTC())
}

func normalizedForceRunTime(value time.Time, fallback func() time.Time) time.Time {
	if value.IsZero() {
		return fallback().UTC()
	}
	return value.UTC()
}
