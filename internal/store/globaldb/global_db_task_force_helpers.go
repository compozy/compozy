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
	next.Error = taskpkg.RedactClaimTokens(strings.TrimSpace(reason))
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

func forceReleaseClaimedTaskRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	previous taskpkg.Run,
	fence taskpkg.RunMutationFence,
) (taskpkg.Run, error) {
	next := forceReleasedTaskRun(previous)
	rowsAffected, err := sqlcgen.New(exec).ForceReleaseClaimedTaskRun(
		ctx,
		forceReleaseClaimedTaskRunParams(fence),
	)
	if err != nil {
		return taskpkg.Run{}, fmt.Errorf("store: force release task run %q: %w", previous.ID, err)
	}
	if rowsAffected == 0 {
		return taskpkg.Run{}, forceRunCASMiss(ctx, exec, previous.ID)
	}
	return next, nil
}

func forceFailEligibleTaskRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	previous taskpkg.Run,
	fence taskpkg.RunMutationFence,
	reason string,
	now time.Time,
) (taskpkg.Run, error) {
	next := forceFailedTaskRun(previous, reason, now)
	rowsAffected, err := sqlcgen.New(exec).ForceFailEligibleTaskRun(
		ctx,
		forceFailEligibleTaskRunParams(fence, next),
	)
	if err != nil {
		return taskpkg.Run{}, fmt.Errorf("store: force fail task run %q: %w", previous.ID, err)
	}
	if rowsAffected == 0 {
		return taskpkg.Run{}, forceRunCASMiss(ctx, exec, previous.ID)
	}
	return next, nil
}

func failNeedsAttentionTaskRunForRecoveryWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	previous taskpkg.Run,
	fence taskpkg.RunMutationFence,
	reason string,
	now time.Time,
) (taskpkg.Run, error) {
	next := forceFailedTaskRun(previous, reason, now)
	rowsAffected, err := sqlcgen.New(exec).FailNeedsAttentionTaskRunForRecovery(
		ctx,
		failNeedsAttentionTaskRunForRecoveryParams(fence, next),
	)
	if err != nil {
		return taskpkg.Run{}, fmt.Errorf("store: fail recovered task run %q: %w", previous.ID, err)
	}
	if rowsAffected == 0 {
		return taskpkg.Run{}, forceRunCASMiss(ctx, exec, previous.ID)
	}
	return next, nil
}

func completeParentRollupTaskRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	previous taskpkg.Run,
	settledAt time.Time,
) (taskpkg.Run, error) {
	completed, err := completedParentRollupRun(previous, settledAt)
	if err != nil {
		return taskpkg.Run{}, err
	}
	rowsAffected, err := sqlcgen.New(exec).CompleteParentRollupTaskRun(
		ctx,
		completeParentRollupTaskRunParams(previous, completed),
	)
	if err != nil {
		return taskpkg.Run{}, fmt.Errorf("store: complete parent rollup task run %q: %w", previous.ID, err)
	}
	if rowsAffected == 0 {
		return taskpkg.Run{}, forceRunCASMiss(ctx, exec, previous.ID)
	}
	return completed, nil
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

func normalizedForceRunTime(value time.Time, fallback func() time.Time) time.Time {
	if value.IsZero() {
		return fallback().UTC()
	}
	return value.UTC()
}
