package globaldb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

var _ looppkg.DeadNodeResumer = (*LoopRepo)(nil)

// ResumeDeadNode retires one confirmed-dead worker and reserves exactly one continuation.
func (g *LoopRepo) ResumeDeadNode(
	ctx context.Context,
	request looppkg.DeadNodeResumeRequest,
) (looppkg.DeadNodeResumeResult, error) {
	if err := g.checkReady(ctx, "resume dead Loop node"); err != nil {
		return looppkg.DeadNodeResumeResult{}, err
	}
	if err := request.Validate(); err != nil {
		return looppkg.DeadNodeResumeResult{}, err
	}
	var result looppkg.DeadNodeResumeResult
	err := g.withTaskImmediateTransaction(ctx, "resume dead Loop node", func(exec taskSQLExecutor) error {
		loopRun, err := getLoopRunByIDWithExecutor(ctx, exec, request.RunID)
		if err != nil {
			return err
		}
		if loopRun.WorkspaceID != request.WorkspaceID {
			return fmt.Errorf("%w: %s", looppkg.ErrRunNotFound, request.RunID)
		}
		if replay, found, err := g.deadNodeResumeReplay(ctx, exec, request); err != nil {
			return err
		} else if found {
			result = replay
			return nil
		}
		prepared, err := prepareDeadNodeResume(ctx, exec, request)
		if err != nil || prepared.noop {
			result.AttentionRequired = prepared.attention
			return err
		}
		if err := retireDeadTaskRun(ctx, exec, prepared.taskRun, request); err != nil {
			return err
		}
		continuation, err := g.reserveDeadNodeContinuation(ctx, exec, request, &prepared)
		if err != nil {
			return err
		}
		if err := applyDeadNodeContinuation(ctx, exec, request, &prepared, continuation); err != nil {
			return err
		}
		result = looppkg.DeadNodeResumeResult{
			Run: continuation, Continued: true, IssuedEpoch: prepared.nextEpoch,
		}
		return nil
	})
	if err != nil {
		return looppkg.DeadNodeResumeResult{}, err
	}
	return result, nil
}

type deadNodeResumePreparation struct {
	loopRun   looppkg.Run
	taskRun   taskpkg.Run
	attempt   int
	nextEpoch int64
	streak    int
	noop      bool
	attention bool
}

func prepareDeadNodeResume(
	ctx context.Context,
	exec taskSQLExecutor,
	request looppkg.DeadNodeResumeRequest,
) (deadNodeResumePreparation, error) {
	loopRun, err := getLoopRunByIDWithExecutor(ctx, exec, request.RunID)
	if err != nil {
		return deadNodeResumePreparation{}, err
	}
	if loopRun.WorkspaceID != request.WorkspaceID {
		return deadNodeResumePreparation{}, fmt.Errorf("%w: %s", looppkg.ErrRunNotFound, request.RunID)
	}
	if loopRun.Status != looppkg.StatusRunning {
		return deadNodeResumePreparation{loopRun: loopRun, noop: true}, nil
	}
	taskRun, err := (&TaskRepo{}).getTaskRunWithExecutor(ctx, exec, request.TaskRunID)
	if err != nil {
		return deadNodeResumePreparation{}, err
	}
	if taskRun.LoopRunID != string(request.RunID) {
		return deadNodeResumePreparation{}, fmt.Errorf(
			"%w: dead task run belongs to another Loop run",
			looppkg.ErrTransitionConflict,
		)
	}
	var status string
	var attempt int
	err = exec.QueryRowContext(ctx, `SELECT status, attempt FROM loop_generation_outputs
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
		  AND task_run_id = ? AND epoch = ?`, request.RunID, request.Generation, request.NodeID,
		request.ItemIndex, request.TaskRunID, request.ExpectedEpoch).Scan(&status, &attempt)
	if errors.Is(err, sql.ErrNoRows) {
		return deadNodeResumePreparation{loopRun: loopRun, taskRun: taskRun, noop: true}, nil
	}
	if err != nil {
		return deadNodeResumePreparation{}, fmt.Errorf("store: load dead Loop node cell: %w", err)
	}
	if status != loopRunNodeOutputRunning && status != goalGenerationOutputEnqueued &&
		status != loopGenerationOutputAwaitingGoal {
		return deadNodeResumePreparation{loopRun: loopRun, taskRun: taskRun, noop: true}, nil
	}
	var cancelState string
	var paused int64
	var streak int
	err = exec.QueryRowContext(ctx, `SELECT cancel_state, paused, death_resume_streak FROM loop_node_controls
		WHERE loop_run_id = ? AND node_id = ?`, request.RunID, request.NodeID).Scan(&cancelState, &paused, &streak)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return deadNodeResumePreparation{}, fmt.Errorf("store: load dead Loop node control: %w", err)
	}
	if cancelState != "" || paused != 0 {
		return deadNodeResumePreparation{loopRun: loopRun, taskRun: taskRun, noop: true}, nil
	}
	if streak >= request.DeathStreakLimit {
		if err := flagResumeExhausted(ctx, exec, request); err != nil {
			return deadNodeResumePreparation{}, err
		}
		return deadNodeResumePreparation{loopRun: loopRun, taskRun: taskRun, noop: true, attention: true}, nil
	}
	return deadNodeResumePreparation{
		loopRun: loopRun, taskRun: taskRun, attempt: attempt,
		nextEpoch: request.ExpectedEpoch + 1, streak: streak + 1,
	}, nil
}

func retireDeadTaskRun(
	ctx context.Context,
	exec taskSQLExecutor,
	current taskpkg.Run,
	request looppkg.DeadNodeResumeRequest,
) error {
	if !forceFailTaskRunStatusAllowed(current.Status) {
		return fmt.Errorf("%w: dead task run %q is %s", taskpkg.ErrInvalidStatusTransition, current.ID, current.Status)
	}
	next, err := forceFailEligibleTaskRunWithExecutor(
		ctx,
		exec,
		current,
		taskpkg.NewRunMutationFence(current),
		request.Cause,
		request.ConfirmedAt.UTC(),
	)
	if err != nil {
		return err
	}
	return updateTaskCurrentRunProjectionForRunUpdate(ctx, exec, current, next)
}

func (g *LoopRepo) reserveDeadNodeContinuation(
	ctx context.Context,
	exec taskSQLExecutor,
	request looppkg.DeadNodeResumeRequest,
	prepared *deadNodeResumePreparation,
) (taskpkg.Run, error) {
	metadata, err := deadNodeContinuationMetadata(request, prepared)
	if err != nil {
		return taskpkg.Run{}, err
	}
	origin := deadNodeResumeOrigin(request)
	_, run, _, err := g.tasks.reserveQueuedRunWithExecutor(ctx, exec, queuedRunReservationInput{
		taskID: prepared.taskRun.TaskID, runID: deadNodeResumeRunID(request, prepared.nextEpoch),
		runKind: prepared.taskRun.RunKind, loopRunID: string(request.RunID),
		idempotencyKey: deadNodeResumeKey(request, prepared.nextEpoch), origin: origin,
		networkSpec: prepared.taskRun.NetworkSpecSnapshot(), designationGroupID: prepared.taskRun.DesignationGroupID,
		metadata: metadata, queuedAt: request.ConfirmedAt.UTC(),
	})
	return run, err
}

func deadNodeContinuationMetadata(
	request looppkg.DeadNodeResumeRequest,
	prepared *deadNodeResumePreparation,
) (json.RawMessage, error) {
	metadata := make(map[string]any)
	if len(prepared.taskRun.Metadata) > 0 {
		if err := json.Unmarshal(prepared.taskRun.Metadata, &metadata); err != nil {
			return nil, fmt.Errorf("store: decode dead node task metadata: %w", err)
		}
	}
	metadata["generation"] = request.Generation
	metadata["node_id"] = request.NodeID
	metadata["item_index"] = request.ItemIndex
	metadata["attempt"] = prepared.attempt + 1
	metadata["epoch"] = prepared.nextEpoch
	metadata["continuation_kind"] = "death_resume"
	metadata["resume_from_task_run_id"] = request.TaskRunID
	metadata["resume_from_session_id"] = request.SourceSessionID
	delete(metadata, "failure")
	if request.Checkpoint == nil {
		delete(metadata, "death_resume_checkpoint")
	} else {
		metadata["death_resume_checkpoint"] = request.Checkpoint
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("store: marshal dead node continuation metadata: %w", err)
	}
	return encoded, nil
}

func applyDeadNodeContinuation(
	ctx context.Context,
	exec taskSQLExecutor,
	request looppkg.DeadNodeResumeRequest,
	prepared *deadNodeResumePreparation,
	continuation taskpkg.Run,
) error {
	result, err := exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET
		status = 'enqueued', task_run_id = ?, attempt = ?, epoch = ?, next_attempt_at = NULL
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
		  AND task_run_id = ? AND epoch = ?`, continuation.ID, prepared.attempt+1, prepared.nextEpoch, request.RunID,
		request.Generation, request.NodeID, request.ItemIndex, request.TaskRunID, request.ExpectedEpoch)
	if err != nil {
		return fmt.Errorf("store: advance dead Loop node cell: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: inspect dead Loop node cell advance: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: dead Loop node cell changed", looppkg.ErrTransitionConflict)
	}
	if _, err := exec.ExecContext(ctx, `INSERT INTO loop_node_attempts (
		loop_run_id, generation, node_id, item_index, attempt, failure_class, failure_code,
		cause, disposition, started_at, ended_at
	) VALUES (?, ?, ?, ?, ?, 'transport', 'confirmed_session_death', ?, 'resumed', ?, ?)`,
		request.RunID, request.Generation, request.NodeID, request.ItemIndex, prepared.attempt,
		request.Cause, request.ConfirmedAt.UTC(), request.ConfirmedAt.UTC()); err != nil {
		return fmt.Errorf("store: append dead Loop node resume ledger: %w", err)
	}
	if _, err := exec.ExecContext(ctx, `INSERT INTO loop_node_controls (
		loop_run_id, node_id, death_resume_streak, revision, updated_at
	) VALUES (?, ?, ?, 1, ?)
	ON CONFLICT(loop_run_id, node_id) DO UPDATE SET
		death_resume_streak = ?,
		attention_flag = CASE WHEN attention_flag = 'silence' THEN '' ELSE attention_flag END,
		attention_reason = CASE WHEN attention_flag = 'silence' THEN '' ELSE attention_reason END,
		revision = revision + 1, updated_at = excluded.updated_at`, request.RunID, request.NodeID,
		prepared.streak, request.ConfirmedAt.UTC(), prepared.streak); err != nil {
		return fmt.Errorf("store: update dead Loop node resume streak: %w", err)
	}
	if err := appendLoopRunEventWithExecutor(
		ctx,
		exec,
		request.RunID,
		request.WorkspaceID,
		loopRunEventNodeResumed,
		map[string]any{
			loopRunEventPayloadKeyGeneration:  request.Generation,
			loopRunEventPayloadKeyNodeID:      request.NodeID,
			loopRunEventPayloadKeyItemIndex:   request.ItemIndex,
			loopRunEventPayloadKeyTaskRunID:   continuation.ID,
			loopRunEventPayloadKeyIssuedEpoch: prepared.nextEpoch,
			loopRunEventPayloadKeyCause:       request.Cause,
			loopRunEventPayloadKeyActorKind:   request.Actor.Actor.Kind.Normalize(),
			loopRunEventPayloadKeyActorID:     strings.TrimSpace(request.Actor.Actor.Ref),
		},
		request.ConfirmedAt.UTC(),
	); err != nil {
		return err
	}
	if prepared.streak < request.DeathStreakLimit {
		return nil
	}
	return flagResumeExhausted(ctx, exec, request)
}

func flagResumeExhausted(
	ctx context.Context,
	exec taskSQLExecutor,
	request looppkg.DeadNodeResumeRequest,
) error {
	result, err := exec.ExecContext(ctx, `INSERT INTO loop_node_controls (
		loop_run_id, node_id, attention_flag, attention_reason, revision, updated_at
	) VALUES (?, ?, 'resume_exhausted', 'Confirmed death resume limit reached.', 1, ?)
	ON CONFLICT(loop_run_id, node_id) DO UPDATE SET
		attention_flag = 'resume_exhausted', attention_reason = 'Confirmed death resume limit reached.',
		revision = revision + 1, updated_at = excluded.updated_at
	WHERE cancel_state = '' AND attention_flag != 'resume_exhausted'`,
		request.RunID, request.NodeID, request.ConfirmedAt.UTC())
	if err != nil {
		return fmt.Errorf("store: flag dead Loop node resume exhaustion: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return err
	}
	return appendLoopRunEventWithExecutor(
		ctx,
		exec,
		request.RunID,
		request.WorkspaceID,
		loopRunEventNodeAttentionFlagged,
		map[string]any{
			loopRunEventPayloadKeyGeneration: request.Generation,
			loopRunEventPayloadKeyNodeID:     request.NodeID,
			loopRunEventPayloadKeyItemIndex:  request.ItemIndex,
			loopRunEventPayloadKeyReason:     "resume_exhausted",
		},
		request.ConfirmedAt.UTC(),
	)
}

func (g *LoopRepo) deadNodeResumeReplay(
	ctx context.Context,
	exec taskSQLExecutor,
	request looppkg.DeadNodeResumeRequest,
) (looppkg.DeadNodeResumeResult, bool, error) {
	nextEpoch := request.ExpectedEpoch + 1
	record, err := getTaskRunIdempotencyRecord(
		ctx, exec, deadNodeResumeKey(request, nextEpoch), deadNodeResumeOrigin(request),
	)
	if errors.Is(err, taskpkg.ErrTaskRunIdempotencyNotFound) {
		return looppkg.DeadNodeResumeResult{}, false, nil
	}
	if err != nil {
		return looppkg.DeadNodeResumeResult{}, false, err
	}
	run, err := (&TaskRepo{}).getTaskRunWithExecutor(ctx, exec, record.RunID)
	if err != nil {
		return looppkg.DeadNodeResumeResult{}, false, err
	}
	return looppkg.DeadNodeResumeResult{
		Run: run, Continued: true, Replay: true, IssuedEpoch: nextEpoch,
	}, true, nil
}

func deadNodeResumeOrigin(request looppkg.DeadNodeResumeRequest) taskpkg.Origin {
	return taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "loop.death-resume:" + string(request.RunID)}
}

func deadNodeResumeKey(request looppkg.DeadNodeResumeRequest, epoch int64) string {
	return fmt.Sprintf(
		"loop.death-resume.%s.%d.%s.%d.%d",
		request.RunID,
		request.Generation,
		request.NodeID,
		request.ItemIndex,
		epoch,
	)
}

func deadNodeResumeRunID(request looppkg.DeadNodeResumeRequest, epoch int64) string {
	digest := sha256.Sum256([]byte(deadNodeResumeKey(request, epoch)))
	return "run.loop.resume." + hex.EncodeToString(digest[:12])
}
