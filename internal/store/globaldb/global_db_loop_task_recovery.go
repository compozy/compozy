package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const loopTaskAttentionFlag = "wait_intervention"

func projectLoopTaskRunAttentionWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	diagnostic string,
	at time.Time,
) error {
	metadata, loopRun, err := loadBoundLoopTaskRunCell(ctx, exec, run)
	if err != nil {
		return err
	}
	var previousFlag string
	err = exec.QueryRowContext(
		ctx,
		`SELECT attention_flag FROM loop_node_controls WHERE loop_run_id = ? AND node_id = ?`,
		run.LoopRunID,
		metadata.NodeID,
	).Scan(&previousFlag)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("store: load Loop task attention: %w", err)
	}
	if at.IsZero() {
		at = run.QueuedAt.UTC()
	}
	if _, err := exec.ExecContext(ctx, `INSERT INTO loop_node_controls (
		loop_run_id, node_id, attention_flag, attention_reason, revision, updated_at
	) VALUES (?, ?, ?, ?, 1, ?)
	ON CONFLICT(loop_run_id, node_id) DO UPDATE SET
		attention_flag = CASE
			WHEN attention_flag = '' OR attention_flag = 'silence' THEN excluded.attention_flag
			ELSE attention_flag
		END,
		attention_reason = CASE
			WHEN attention_flag = '' OR attention_flag = 'silence' THEN excluded.attention_reason
			ELSE attention_reason
		END,
		revision = revision + 1,
		updated_at = excluded.updated_at`,
		run.LoopRunID,
		metadata.NodeID,
		loopTaskAttentionFlag,
		diagnostic,
		at.UTC(),
	); err != nil {
		return fmt.Errorf("store: project Loop task attention: %w", err)
	}
	if previousFlag != "" && previousFlag != looppkg.AttentionSilence {
		return nil
	}
	return appendLoopRunEventWithExecutor(
		ctx,
		exec,
		loopRun.ID,
		loopRun.WorkspaceID,
		loopRunEventNodeAttentionFlagged,
		map[string]any{
			loopRunEventPayloadKeyGeneration:    metadata.Generation,
			loopRunEventPayloadKeyNodeID:        metadata.NodeID,
			loopRunEventPayloadKeyItemIndex:     metadata.ItemIndex,
			loopRunEventPayloadKeyTaskRunID:     run.ID,
			loopRunEventPayloadKeyAttentionFlag: loopTaskAttentionFlag,
			loopRunEventPayloadKeyReason:        diagnostic,
		},
		at.UTC(),
	)
}

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
	if loopRun.Status != looppkg.StatusRunning || loopRun.CancelRequested {
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

func loopTaskRecoveryMetadata(
	source json.RawMessage,
	override json.RawMessage,
	current loopNodeRunMetadata,
) (json.RawMessage, error) {
	metadata := make(map[string]any)
	if err := mergeLoopTaskMetadata(metadata, source); err != nil {
		return nil, err
	}
	if err := mergeLoopTaskMetadata(metadata, override); err != nil {
		return nil, err
	}
	metadata["generation"] = current.Generation
	metadata["node_id"] = current.NodeID
	metadata["item_index"] = current.ItemIndex
	metadata["attempt"] = current.Attempt + 1
	metadata["epoch"] = current.Epoch + 1
	for _, key := range []string{
		loopRunEventPayloadKeyFailure,
		"continuation_kind",
		"resume_from_task_run_id",
		"resume_from_session_id",
		"death_resume_checkpoint",
	} {
		delete(metadata, key)
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("store: marshal Loop task recovery metadata: %w", err)
	}
	if err := taskpkg.ValidateMetadataSize(encoded, "recover_run.metadata"); err != nil {
		return nil, err
	}
	return encoded, nil
}

func mergeLoopTaskMetadata(target map[string]any, raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return fmt.Errorf("store: decode Loop task recovery metadata: %w", err)
	}
	if values == nil {
		return fmt.Errorf("%w: Loop task recovery metadata must be an object", taskpkg.ErrValidation)
	}
	maps.Copy(target, values)
	return nil
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
