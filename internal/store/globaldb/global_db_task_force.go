package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	taskpkg "github.com/compozy/compozy/internal/task"
)

type retryTaskRunArgs struct {
	sourceRunID string
	sourceFence taskpkg.RunMutationFence
	newRunID    string
	origin      taskpkg.Origin
	metadata    json.RawMessage
	queuedAt    time.Time
	reason      string
}

func (g *TaskRunRepo) forceReleaseTaskRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	release taskpkg.ForceReleaseRunMutation,
) (taskpkg.ForceRunMutationResult, error) {
	fence := release.Fence()
	runID, err := requireTaskValue(fence.RunID(), "task run id")
	if err != nil {
		return taskpkg.ForceRunMutationResult{}, err
	}
	previous, err := g.tasks.getTaskRunWithExecutor(ctx, exec, runID)
	if err != nil {
		return taskpkg.ForceRunMutationResult{}, err
	}
	if !fence.Matches(previous) {
		return taskpkg.ForceRunMutationResult{}, fmt.Errorf(
			"%w: task run %q changed before force release",
			taskpkg.ErrInvalidStatusTransition,
			previous.ID,
		)
	}
	if previous.Status.Normalize() != taskpkg.TaskRunStatusClaimed {
		return taskpkg.ForceRunMutationResult{}, fmt.Errorf(
			"%w: task run %q is %s; only claimed runs can be force released",
			taskpkg.ErrInvalidStatusTransition,
			previous.ID,
			previous.Status.Normalize(),
		)
	}
	next, err := forceReleaseClaimedTaskRunWithExecutor(ctx, exec, previous, fence)
	if err != nil {
		return taskpkg.ForceRunMutationResult{}, err
	}
	if err := updateTaskCurrentRunProjectionForRunUpdate(ctx, exec, previous, next); err != nil {
		return taskpkg.ForceRunMutationResult{}, err
	}
	return taskpkg.ForceRunMutationResult{Previous: previous, Run: next}, nil
}

func (g *TaskRunRepo) forceFailTaskRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	failure taskpkg.ForceFailRunMutation,
) (taskpkg.ForceRunMutationResult, error) {
	fence := failure.Fence()
	runID, err := requireTaskValue(fence.RunID(), "task run id")
	if err != nil {
		return taskpkg.ForceRunMutationResult{}, err
	}
	reason := taskpkg.RedactClaimTokens(strings.TrimSpace(failure.Reason()))
	if reason == "" {
		return taskpkg.ForceRunMutationResult{}, fmt.Errorf("%w: force fail reason is required", taskpkg.ErrValidation)
	}
	if err := taskpkg.ValidateReasonSize(reason, "force_fail.reason"); err != nil {
		return taskpkg.ForceRunMutationResult{}, err
	}
	now := normalizedForceRunTime(failure.Now(), g.now)

	previous, err := g.tasks.getTaskRunWithExecutor(ctx, exec, runID)
	if err != nil {
		return taskpkg.ForceRunMutationResult{}, err
	}
	if !fence.Matches(previous) {
		return taskpkg.ForceRunMutationResult{}, fmt.Errorf(
			"%w: task run %q changed before force failure",
			taskpkg.ErrInvalidStatusTransition,
			previous.ID,
		)
	}
	if !forceFailTaskRunStatusAllowed(previous.Status) {
		return taskpkg.ForceRunMutationResult{}, fmt.Errorf(
			"%w: task run %q is %s; only queued or claimed runs can be force failed",
			taskpkg.ErrInvalidStatusTransition,
			previous.ID,
			previous.Status.Normalize(),
		)
	}
	next, err := forceFailEligibleTaskRunWithExecutor(ctx, exec, previous, fence, reason, now)
	if err != nil {
		return taskpkg.ForceRunMutationResult{}, err
	}
	if err := updateTaskCurrentRunProjectionForRunUpdate(ctx, exec, previous, next); err != nil {
		return taskpkg.ForceRunMutationResult{}, err
	}
	return taskpkg.ForceRunMutationResult{Previous: previous, Run: next}, nil
}

func normalizeRetryTaskRunArgs(
	retry taskpkg.RetryRunMutation,
	now func() time.Time,
) (retryTaskRunArgs, error) {
	fence := retry.Fence()
	sourceRunID, err := requireTaskValue(fence.RunID(), "source task run id")
	if err != nil {
		return retryTaskRunArgs{}, err
	}
	newRunID, err := requireTaskValue(retry.NewRunID(), "new task run id")
	if err != nil {
		return retryTaskRunArgs{}, err
	}
	retryOrigin := retry.Origin()
	origin := taskpkg.Origin{Kind: retryOrigin.Kind.Normalize(), Ref: strings.TrimSpace(retryOrigin.Ref)}
	if err := origin.Validate("retry_run.origin"); err != nil {
		return retryTaskRunArgs{}, err
	}
	metadata := normalizeTaskJSON(retry.Metadata())
	if err := taskpkg.ValidateMetadataSize(metadata, "retry_run.metadata"); err != nil {
		return retryTaskRunArgs{}, err
	}
	return retryTaskRunArgs{
		sourceRunID: sourceRunID,
		sourceFence: fence,
		newRunID:    newRunID,
		origin:      origin,
		metadata:    metadata,
		queuedAt:    normalizedForceRunTime(retry.QueuedAt(), now),
	}, nil
}

func (g *TaskRunRepo) retryTaskRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	args retryTaskRunArgs,
) (taskpkg.RetryRunResult, error) {
	source, err := g.retryTaskRunSource(ctx, exec, args.sourceFence)
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
	fence taskpkg.RunMutationFence,
) (taskpkg.Run, error) {
	source, err := g.tasks.getTaskRunWithExecutor(ctx, exec, fence.RunID())
	if err != nil {
		return taskpkg.Run{}, err
	}
	if !fence.Matches(source) {
		return taskpkg.Run{}, fmt.Errorf(
			"%w: task run %q changed before retry",
			taskpkg.ErrInvalidStatusTransition,
			source.ID,
		)
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
	normalizedRun.Metadata = taskpkg.RedactClaimTokenJSON(normalizedRun.Metadata)
	if err := taskpkg.ValidateMetadataSize(normalizedRun.Metadata, "task_run.metadata"); err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	if err := insertQueuedTaskRun(ctx, exec, normalizedRun); err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	return taskpkg.RetryRunResult{PreviousRun: source, Run: normalizedRun}, nil
}

func normalizeRecoverTaskRunArgs(
	mutation taskpkg.RecoverRunMutation,
	now func() time.Time,
) (retryTaskRunArgs, error) {
	fence := mutation.Fence()
	sourceRunID, err := requireTaskValue(fence.RunID(), "source task run id")
	if err != nil {
		return retryTaskRunArgs{}, err
	}
	newRunID, err := requireTaskValue(mutation.NewRunID(), "new task run id")
	if err != nil {
		return retryTaskRunArgs{}, err
	}
	mutationOrigin := mutation.Origin()
	origin := taskpkg.Origin{Kind: mutationOrigin.Kind.Normalize(), Ref: strings.TrimSpace(mutationOrigin.Ref)}
	if err := origin.Validate("recover_run.origin"); err != nil {
		return retryTaskRunArgs{}, err
	}
	metadata := normalizeTaskJSON(mutation.Metadata())
	if err := taskpkg.ValidateMetadataSize(metadata, "recover_run.metadata"); err != nil {
		return retryTaskRunArgs{}, err
	}
	reason := taskpkg.RedactClaimTokens(strings.TrimSpace(mutation.Reason()))
	if err := taskpkg.ValidateReasonSize(reason, "recover_run.reason"); err != nil {
		return retryTaskRunArgs{}, err
	}
	return retryTaskRunArgs{
		sourceRunID: sourceRunID,
		sourceFence: fence,
		newRunID:    newRunID,
		origin:      origin,
		metadata:    metadata,
		queuedAt:    normalizedForceRunTime(mutation.QueuedAt(), now),
		reason:      reason,
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
	if !args.sourceFence.Matches(source) {
		return taskpkg.RetryRunResult{}, fmt.Errorf(
			"%w: task run %q changed before recovery",
			taskpkg.ErrInvalidStatusTransition,
			source.ID,
		)
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
	failed, err := failNeedsAttentionTaskRunForRecoveryWithExecutor(
		ctx,
		exec,
		source,
		args.sourceFence,
		args.reason,
		args.queuedAt,
	)
	if err != nil {
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
