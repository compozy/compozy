package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

type retryTaskRunArgs struct {
	sourceRunID string
	newRunID    string
	origin      taskpkg.Origin
	metadata    json.RawMessage
	queuedAt    time.Time
	reason      string
}

// ForceReleaseTaskRun requeues one claimed run with snapshot fencing.
func (g *TaskRunRepo) ForceReleaseTaskRun(
	ctx context.Context,
	release taskpkg.ForceReleaseRunMutation,
) (taskpkg.ForceRunMutationResult, error) {
	if err := g.checkReady(ctx, "force release task run"); err != nil {
		return taskpkg.ForceRunMutationResult{}, err
	}
	runID, err := requireTaskValue(release.RunID, "task run id")
	if err != nil {
		return taskpkg.ForceRunMutationResult{}, err
	}

	var result taskpkg.ForceRunMutationResult
	if err := g.tasks.withTaskImmediateTransaction(ctx, "force release task run", func(exec taskSQLExecutor) error {
		previous, err := g.tasks.getTaskRunWithExecutor(ctx, exec, runID)
		if err != nil {
			return err
		}
		if previous.Status.Normalize() != taskpkg.TaskRunStatusClaimed {
			return fmt.Errorf(
				"%w: task run %q is %s; only claimed runs can be force released",
				taskpkg.ErrInvalidStatusTransition,
				previous.ID,
				previous.Status.Normalize(),
			)
		}
		next := forceReleasedTaskRun(previous)
		if err := updateTaskRunRecordWithSnapshotCAS(ctx, exec, previous, next); err != nil {
			return err
		}
		if err := updateTaskCurrentRunProjectionForRunUpdate(ctx, exec, previous, next); err != nil {
			return err
		}
		result = taskpkg.ForceRunMutationResult{Previous: previous, Run: next}
		return nil
	}); err != nil {
		return taskpkg.ForceRunMutationResult{}, err
	}
	return result, nil
}

// ForceFailTaskRun marks one queued or claimed run as operator-forced failed with snapshot fencing.
func (g *TaskRunRepo) ForceFailTaskRun(
	ctx context.Context,
	failure taskpkg.ForceFailRunMutation,
) (taskpkg.ForceRunMutationResult, error) {
	if err := g.checkReady(ctx, "force fail task run"); err != nil {
		return taskpkg.ForceRunMutationResult{}, err
	}
	runID, err := requireTaskValue(failure.RunID, "task run id")
	if err != nil {
		return taskpkg.ForceRunMutationResult{}, err
	}
	reason := strings.TrimSpace(failure.Reason)
	if reason == "" {
		return taskpkg.ForceRunMutationResult{}, fmt.Errorf("%w: force fail reason is required", taskpkg.ErrValidation)
	}
	now := normalizedForceRunTime(failure.Now, g.now)

	var result taskpkg.ForceRunMutationResult
	if err := g.tasks.withTaskImmediateTransaction(ctx, "force fail task run", func(exec taskSQLExecutor) error {
		previous, err := g.tasks.getTaskRunWithExecutor(ctx, exec, runID)
		if err != nil {
			return err
		}
		if !forceFailTaskRunStatusAllowed(previous.Status) {
			return fmt.Errorf(
				"%w: task run %q is %s; only queued or claimed runs can be force failed",
				taskpkg.ErrInvalidStatusTransition,
				previous.ID,
				previous.Status.Normalize(),
			)
		}
		next := forceFailedTaskRun(previous, reason, now)
		if err := updateTaskRunRecordWithSnapshotCAS(ctx, exec, previous, next); err != nil {
			return err
		}
		if err := updateTaskCurrentRunProjectionForRunUpdate(ctx, exec, previous, next); err != nil {
			return err
		}
		result = taskpkg.ForceRunMutationResult{Previous: previous, Run: next}
		return nil
	}); err != nil {
		return taskpkg.ForceRunMutationResult{}, err
	}
	return result, nil
}

// RetryTaskRun creates one queued retry run linked to a failed source run.
func (g *TaskRunRepo) RetryTaskRun(
	ctx context.Context,
	retry taskpkg.RetryRunMutation,
) (taskpkg.RetryRunResult, error) {
	if err := g.checkReady(ctx, "retry task run"); err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	args, err := normalizeRetryTaskRunArgs(retry, g.now)
	if err != nil {
		return taskpkg.RetryRunResult{}, err
	}

	var result taskpkg.RetryRunResult
	if err := g.tasks.withTaskImmediateTransaction(ctx, "retry task run", func(exec taskSQLExecutor) error {
		created, err := g.retryTaskRunWithExecutor(ctx, exec, args)
		if err == nil {
			result = created
		}
		return err
	}); err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	return result, nil
}

func normalizeRetryTaskRunArgs(
	retry taskpkg.RetryRunMutation,
	now func() time.Time,
) (retryTaskRunArgs, error) {
	sourceRunID, err := requireTaskValue(retry.SourceRunID, "source task run id")
	if err != nil {
		return retryTaskRunArgs{}, err
	}
	newRunID, err := requireTaskValue(retry.NewRunID, "new task run id")
	if err != nil {
		return retryTaskRunArgs{}, err
	}
	origin := taskpkg.Origin{Kind: retry.Origin.Kind.Normalize(), Ref: strings.TrimSpace(retry.Origin.Ref)}
	if err := origin.Validate("retry_run.origin"); err != nil {
		return retryTaskRunArgs{}, err
	}
	metadata := normalizeTaskJSON(retry.Metadata)
	if err := taskpkg.ValidateMetadataSize(metadata, "retry_run.metadata"); err != nil {
		return retryTaskRunArgs{}, err
	}
	return retryTaskRunArgs{
		sourceRunID: sourceRunID,
		newRunID:    newRunID,
		origin:      origin,
		metadata:    metadata,
		queuedAt:    normalizedForceRunTime(retry.QueuedAt, now),
	}, nil
}

func (g *TaskRunRepo) retryTaskRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	args retryTaskRunArgs,
) (taskpkg.RetryRunResult, error) {
	source, err := g.retryTaskRunSource(ctx, exec, args.sourceRunID)
	if err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	taskRecord, err := g.retryTaskRunTask(ctx, exec, source.TaskID)
	if err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	nextAttempt, err := nextTaskRunAttemptWithExecutor(ctx, exec, taskRecord)
	if err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	return g.insertRetryTaskRun(ctx, exec, args, source, taskRecord, nextAttempt)
}

func (g *TaskRunRepo) retryTaskRunSource(
	ctx context.Context,
	exec taskSQLExecutor,
	sourceRunID string,
) (taskpkg.Run, error) {
	source, err := g.tasks.getTaskRunWithExecutor(ctx, exec, sourceRunID)
	if err != nil {
		return taskpkg.Run{}, err
	}
	if source.Status.Normalize() != taskpkg.TaskRunStatusFailed {
		return taskpkg.Run{}, fmt.Errorf(
			"%w: task run %q is %s; only failed runs can be retried",
			taskpkg.ErrInvalidStatusTransition,
			source.ID,
			source.Status.Normalize(),
		)
	}
	if err := requireRetryDepthWithExecutor(ctx, exec, source); err != nil {
		return taskpkg.Run{}, err
	}
	if err := requireNoRetryChildWithExecutor(ctx, exec, source.ID); err != nil {
		return taskpkg.Run{}, err
	}
	return source, nil
}

func (g *TaskRunRepo) retryTaskRunTask(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
) (taskpkg.Task, error) {
	taskRecord, err := g.tasks.getTaskWithExecutor(ctx, exec, taskID)
	if err != nil {
		return taskpkg.Task{}, err
	}
	if err := validateTaskForQueuedRunReservation(taskRecord); err != nil {
		return taskpkg.Task{}, err
	}
	openRunID, err := g.tasks.findOpenRunIDForQueuedRunReservation(ctx, exec, taskRecord.ID, "")
	if err != nil {
		return taskpkg.Task{}, err
	}
	if openRunID != "" {
		return taskpkg.Task{}, fmt.Errorf(
			"%w: task %q has open run %q; finish or cancel it before retrying another run",
			taskpkg.ErrInvalidStatusTransition,
			taskRecord.ID,
			openRunID,
		)
	}
	return taskRecord, nil
}

func (g *TaskRunRepo) insertRetryTaskRun(
	ctx context.Context,
	exec taskSQLExecutor,
	args retryTaskRunArgs,
	source taskpkg.Run,
	taskRecord taskpkg.Task,
	nextAttempt int,
) (taskpkg.RetryRunResult, error) {
	runAttempt, err := taskRunAttemptFromInt(nextAttempt)
	if err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	run := taskpkg.Run{
		ID:            args.newRunID,
		TaskID:        taskRecord.ID,
		WorkspaceID:   source.WorkspaceID,
		Status:        taskpkg.TaskRunStatusQueued,
		Attempt:       runAttempt,
		PreviousRunID: source.ID,
		Origin:        args.origin,
		Metadata:      args.metadata,
		QueuedAt:      args.queuedAt,
	}
	run.SetNetworkState(source.NetworkSpecSnapshot(), "", "", "")
	normalizedRun, err := g.tasks.normalizeTaskRunForCreate(run)
	if err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	if err := insertQueuedTaskRun(ctx, exec, normalizedRun); err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	return taskpkg.RetryRunResult{PreviousRun: source, Run: normalizedRun}, nil
}

// RecoverTaskRun terminalizes a needs_attention run as failed and queues one fresh child in
// the same transaction, so the source leaves the open-run set before the requeue reservation.
func (g *TaskRunRepo) RecoverTaskRun(
	ctx context.Context,
	mutation taskpkg.RecoverRunMutation,
) (taskpkg.RetryRunResult, error) {
	if err := g.checkReady(ctx, "recover task run"); err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	args, err := normalizeRecoverTaskRunArgs(mutation, g.now)
	if err != nil {
		return taskpkg.RetryRunResult{}, err
	}

	var result taskpkg.RetryRunResult
	if err := g.tasks.withTaskImmediateTransaction(ctx, "recover task run", func(exec taskSQLExecutor) error {
		created, err := g.recoverTaskRunWithExecutor(ctx, exec, args)
		if err == nil {
			result = created
		}
		return err
	}); err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	return result, nil
}

func normalizeRecoverTaskRunArgs(
	mutation taskpkg.RecoverRunMutation,
	now func() time.Time,
) (retryTaskRunArgs, error) {
	sourceRunID, err := requireTaskValue(mutation.SourceRunID, "source task run id")
	if err != nil {
		return retryTaskRunArgs{}, err
	}
	newRunID, err := requireTaskValue(mutation.NewRunID, "new task run id")
	if err != nil {
		return retryTaskRunArgs{}, err
	}
	origin := taskpkg.Origin{Kind: mutation.Origin.Kind.Normalize(), Ref: strings.TrimSpace(mutation.Origin.Ref)}
	if err := origin.Validate("recover_run.origin"); err != nil {
		return retryTaskRunArgs{}, err
	}
	metadata := normalizeTaskJSON(mutation.Metadata)
	if err := taskpkg.ValidateMetadataSize(metadata, "recover_run.metadata"); err != nil {
		return retryTaskRunArgs{}, err
	}
	return retryTaskRunArgs{
		sourceRunID: sourceRunID,
		newRunID:    newRunID,
		origin:      origin,
		metadata:    metadata,
		queuedAt:    normalizedForceRunTime(mutation.QueuedAt, now),
		reason:      strings.TrimSpace(mutation.Reason),
	}, nil
}

func (g *TaskRunRepo) recoverTaskRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	args retryTaskRunArgs,
) (taskpkg.RetryRunResult, error) {
	source, err := g.tasks.getTaskRunWithExecutor(ctx, exec, args.sourceRunID)
	if err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	if source.Status.Normalize() != taskpkg.TaskRunStatusNeedsAttention {
		return taskpkg.RetryRunResult{}, fmt.Errorf(
			"%w: task run %q is %s; only needs_attention runs can be recovered",
			taskpkg.ErrInvalidStatusTransition,
			source.ID,
			source.Status.Normalize(),
		)
	}
	if err := requireRetryDepthWithExecutor(ctx, exec, source); err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	if err := requireNoRetryChildWithExecutor(ctx, exec, source.ID); err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	failed := forceFailedTaskRun(source, args.reason, args.queuedAt)
	if err := updateTaskRunRecordWithSnapshotCAS(ctx, exec, source, failed); err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	taskRecord, err := g.retryTaskRunTask(ctx, exec, failed.TaskID)
	if err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	nextAttempt, err := nextTaskRunAttemptWithExecutor(ctx, exec, taskRecord)
	if err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	return g.insertRetryTaskRun(ctx, exec, args, failed, taskRecord, nextAttempt)
}

// MarkTaskRunNeedsAttention transitions one nonterminal run to needs_attention via a status CAS.
func (g *TaskRunRepo) MarkTaskRunNeedsAttention(
	ctx context.Context,
	runID string,
	diagnostic string,
) (taskpkg.Run, error) {
	if err := g.checkReady(ctx, "mark task run needs attention"); err != nil {
		return taskpkg.Run{}, err
	}
	id, err := requireTaskValue(runID, "task run id")
	if err != nil {
		return taskpkg.Run{}, err
	}
	var run taskpkg.Run
	if err := g.tasks.withTaskImmediateTransaction(
		ctx,
		"mark task run needs attention",
		func(exec taskSQLExecutor) error {
			affected, err := sqlcgen.New(exec).MarkTaskRunNeedsAttention(
				ctx,
				sqlcgen.MarkTaskRunNeedsAttentionParams{
					NeedsAttentionStatus: taskpkg.TaskRunStatusNeedsAttention.String(),
					Error:                nullableTaskString(strings.TrimSpace(diagnostic)),
					ID:                   id,
					QueuedStatus:         taskpkg.TaskRunStatusQueued.String(),
					ClaimedStatus:        taskpkg.TaskRunStatusClaimed.String(),
					StartingStatus:       taskpkg.TaskRunStatusStarting.String(),
					RunningStatus:        taskpkg.TaskRunStatusRunning.String(),
				},
			)
			if err != nil {
				return fmt.Errorf("store: mark task run needs attention: %w", err)
			}
			if affected == 0 {
				return fmt.Errorf("%w: task run %q is not nonterminal", taskpkg.ErrInvalidStatusTransition, id)
			}
			updated, err := g.tasks.getTaskRunWithExecutor(ctx, exec, id)
			if err != nil {
				return err
			}
			run = updated
			return nil
		},
	); err != nil {
		return taskpkg.Run{}, err
	}
	return run, nil
}
