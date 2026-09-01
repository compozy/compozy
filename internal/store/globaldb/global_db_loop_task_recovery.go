package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (g *TaskRunRepo) recoverLoopTaskRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	args retryTaskRunArgs,
	source taskpkg.Run,
) (taskpkg.RetryRunResult, error) {
	metadata, loopRun, err := loadBoundLoopTaskRunCell(ctx, exec, source)
	if err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	if loopRun.Status != looppkg.StatusRunning {
		return taskpkg.RetryRunResult{}, fmt.Errorf(
			"%w: Loop run %q is not running",
			looppkg.ErrTransitionConflict,
			loopRun.ID,
		)
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
	if err := updateTaskCurrentRunProjectionForRunUpdate(ctx, exec, source, failed); err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	taskRecord, err := g.retryTaskRunTask(ctx, exec, failed.TaskID)
	if err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	nextMetadata, err := loopTaskRecoveryMetadata(source.Metadata, args.metadata, metadata)
	if err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	worktree := taskpkg.RunWorktreeState{}
	if source.RunWorktreeState != nil {
		worktree = *source.RunWorktreeState
	}
	continuation, err := g.tasks.createQueuedRunWithExecutor(ctx, exec, taskRecord, queuedRunReservationInput{
		taskID:                failed.TaskID,
		runID:                 args.newRunID,
		runKind:               source.RunKind,
		loopRunID:             source.LoopRunID,
		origin:                args.origin,
		networkSpec:           source.NetworkSpecSnapshot(),
		designationGroupID:    source.DesignationGroupID,
		resolvedWorktreeMode:  worktree.ResolvedWorktreeMode,
		resolvedWorktreeRef:   worktree.ResolvedWorktreeRef,
		worktreeID:            worktree.WorktreeID,
		previousRunID:         source.ID,
		requiredCapabilities:  source.RequiredCapabilities,
		preferredCapabilities: source.PreferredCapabilities,
		metadata:              nextMetadata,
		queuedAt:              args.queuedAt,
	})
	if err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	if err := applyLoopTaskRecovery(ctx, exec, loopRun, source, continuation, metadata, args); err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	return taskpkg.RetryRunResult{PreviousRun: failed, Run: continuation}, nil
}

func loadBoundLoopTaskRunCell(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
) (loopNodeRunMetadata, looppkg.Run, error) {
	metadata, ok, err := loopNodeMetadataFromTaskRun(run.Metadata)
	if err != nil {
		return loopNodeRunMetadata{}, looppkg.Run{}, err
	}
	if !ok {
		return loopNodeRunMetadata{}, looppkg.Run{}, fmt.Errorf(
			"%w: loop worker %q has invalid lifecycle metadata",
			looppkg.ErrValidation,
			run.ID,
		)
	}
	loopRun, err := getLoopRunByIDWithExecutor(ctx, exec, looppkg.RunID(run.LoopRunID))
	if err != nil {
		return loopNodeRunMetadata{}, looppkg.Run{}, err
	}
	if strings.TrimSpace(run.WorkspaceID) != string(loopRun.WorkspaceID) {
		return loopNodeRunMetadata{}, looppkg.Run{}, fmt.Errorf(
			"%w: loop worker %q workspace does not match Loop run",
			looppkg.ErrTransitionConflict,
			run.ID,
		)
	}
	var status string
	err = exec.QueryRowContext(ctx, `SELECT status FROM loop_generation_outputs
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
		  AND task_run_id = ? AND epoch = ?`,
		run.LoopRunID,
		metadata.Generation,
		metadata.NodeID,
		metadata.ItemIndex,
		run.ID,
		metadata.Epoch,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return loopNodeRunMetadata{}, looppkg.Run{}, fmt.Errorf(
			"%w: loop worker %q is no longer bound to its node cell",
			looppkg.ErrTransitionConflict,
			run.ID,
		)
	}
	if err != nil {
		return loopNodeRunMetadata{}, looppkg.Run{}, fmt.Errorf("store: load Loop task run cell: %w", err)
	}
	switch status {
	case loopRunNodeOutputRunning, goalGenerationOutputEnqueued, loopGenerationOutputAwaitingGoal:
		return metadata, loopRun, nil
	default:
		return loopNodeRunMetadata{}, looppkg.Run{}, fmt.Errorf(
			"%w: loop worker %q cell is %s",
			looppkg.ErrTransitionConflict,
			run.ID,
			status,
		)
	}
}

func applyLoopTaskRecovery(
	ctx context.Context,
	exec taskSQLExecutor,
	loopRun looppkg.Run,
	source taskpkg.Run,
	continuation taskpkg.Run,
	metadata loopNodeRunMetadata,
	args retryTaskRunArgs,
) error {
	if err := advanceLoopTaskRecoveryCell(ctx, exec, source, continuation, metadata); err != nil {
		return err
	}
	attentionFlag, err := clearLoopTaskRecoveryAttention(ctx, exec, source, metadata, args.queuedAt)
	if err != nil {
		return err
	}
	if err := appendLoopTaskRecoveryAttempt(ctx, exec, source, metadata, args); err != nil {
		return err
	}
	if attentionFlag != "" {
		if err := appendLoopTaskRecoveryAttentionClearedEvent(
			ctx,
			exec,
			loopRun,
			continuation.ID,
			metadata,
			args.queuedAt,
		); err != nil {
			return err
		}
	}
	return appendLoopTaskRecoveryResumedEvent(ctx, exec, loopRun, continuation.ID, metadata, args)
}

func advanceLoopTaskRecoveryCell(
	ctx context.Context,
	exec taskSQLExecutor,
	source taskpkg.Run,
	continuation taskpkg.Run,
	metadata loopNodeRunMetadata,
) error {
	result, err := exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET
		status = 'enqueued', task_run_id = ?, attempt = ?, epoch = ?, next_attempt_at = NULL
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
		  AND task_run_id = ? AND epoch = ?`,
		continuation.ID,
		metadata.Attempt+1,
		metadata.Epoch+1,
		source.LoopRunID,
		metadata.Generation,
		metadata.NodeID,
		metadata.ItemIndex,
		source.ID,
		metadata.Epoch,
	)
	if err != nil {
		return fmt.Errorf("store: advance recovered Loop task cell: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: inspect recovered Loop task cell: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: recovered Loop task cell changed", looppkg.ErrTransitionConflict)
	}
	return nil
}

func clearLoopTaskRecoveryAttention(
	ctx context.Context,
	exec taskSQLExecutor,
	source taskpkg.Run,
	metadata loopNodeRunMetadata,
	at time.Time,
) (string, error) {
	var attentionFlag string
	err := exec.QueryRowContext(ctx, `SELECT attention_flag FROM loop_node_controls
		WHERE loop_run_id = ? AND node_id = ?`, source.LoopRunID, metadata.NodeID).Scan(&attentionFlag)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("store: load recovered Loop task attention: %w", err)
	}
	if _, err := exec.ExecContext(ctx, `INSERT INTO loop_node_controls (
		loop_run_id, node_id, attention_flag, attention_reason, death_resume_streak, revision, updated_at
	) VALUES (?, ?, '', '', 0, 1, ?)
	ON CONFLICT(loop_run_id, node_id) DO UPDATE SET
		attention_flag = '', attention_reason = '', death_resume_streak = 0,
		revision = revision + 1, updated_at = excluded.updated_at`,
		source.LoopRunID,
		metadata.NodeID,
		at,
	); err != nil {
		return "", fmt.Errorf("store: clear recovered Loop task attention: %w", err)
	}
	return attentionFlag, nil
}

func appendLoopTaskRecoveryAttempt(
	ctx context.Context,
	exec taskSQLExecutor,
	source taskpkg.Run,
	metadata loopNodeRunMetadata,
	args retryTaskRunArgs,
) error {
	if _, err := exec.ExecContext(ctx, `INSERT INTO loop_node_attempts (
		loop_run_id, generation, node_id, item_index, attempt, failure_code,
		cause, disposition, started_at, ended_at
	) VALUES (?, ?, ?, ?, ?, 'operator_recovery', ?, 'resumed', ?, ?)`,
		source.LoopRunID,
		metadata.Generation,
		metadata.NodeID,
		metadata.ItemIndex,
		metadata.Attempt,
		args.reason,
		args.queuedAt,
		args.queuedAt,
	); err != nil {
		return fmt.Errorf("store: append Loop task recovery attempt: %w", err)
	}
	return nil
}

func appendLoopTaskRecoveryAttentionClearedEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	loopRun looppkg.Run,
	continuationID string,
	metadata loopNodeRunMetadata,
	at time.Time,
) error {
	return appendLoopRunEventWithExecutor(
		ctx,
		exec,
		loopRun.ID,
		loopRun.WorkspaceID,
		loopRunEventNodeAttentionCleared,
		map[string]any{
			loopRunEventPayloadKeyGeneration: metadata.Generation,
			loopRunEventPayloadKeyNodeID:     metadata.NodeID,
			loopRunEventPayloadKeyItemIndex:  metadata.ItemIndex,
			loopRunEventPayloadKeyTaskRunID:  continuationID,
			loopRunEventPayloadKeyReason:     "operator_recovery",
		},
		at,
	)
}

func appendLoopTaskRecoveryResumedEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	loopRun looppkg.Run,
	continuationID string,
	metadata loopNodeRunMetadata,
	args retryTaskRunArgs,
) error {
	return appendLoopRunEventWithExecutor(
		ctx,
		exec,
		loopRun.ID,
		loopRun.WorkspaceID,
		loopRunEventNodeResumed,
		map[string]any{
			loopRunEventPayloadKeyGeneration:  metadata.Generation,
			loopRunEventPayloadKeyNodeID:      metadata.NodeID,
			loopRunEventPayloadKeyItemIndex:   metadata.ItemIndex,
			loopRunEventPayloadKeyTaskRunID:   continuationID,
			loopRunEventPayloadKeyIssuedEpoch: metadata.Epoch + 1,
			loopRunEventPayloadKeyCause:       args.reason,
			loopRunEventPayloadKeyActorKind:   args.origin.Kind.Normalize(),
			loopRunEventPayloadKeyActorID:     args.origin.Ref,
		},
		args.queuedAt,
	)
}
