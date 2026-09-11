package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// RecoverSupervisedRun rechecks ownership and terminal intent inside the task transaction before spending an attempt.
func (s *taskMutationTxStore) RecoverSupervisedRun(
	ctx context.Context, mutation *taskpkg.SupervisedRunRecoveryMutation,
) (taskpkg.SupervisedRunRecoveryResult, error) {
	if mutation == nil {
		return taskpkg.SupervisedRunRecoveryResult{}, fmt.Errorf(
			"%w: supervised recovery mutation is required",
			taskpkg.ErrValidation,
		)
	}
	source, err := s.tasks.getTaskRunWithExecutor(ctx, s.exec, mutation.Source.ID)
	if errors.Is(err, taskpkg.ErrTaskRunNotFound) {
		return taskpkg.SupervisedRunRecoveryResult{}, nil
	}
	if err != nil {
		return taskpkg.SupervisedRunRecoveryResult{}, err
	}
	if !taskpkg.NewRunMutationFence(mutation.Source).Matches(source) {
		return taskpkg.SupervisedRunRecoveryResult{}, nil
	}
	switch source.Status {
	case taskpkg.TaskRunStatusClaimed, taskpkg.TaskRunStatusStarting, taskpkg.TaskRunStatusRunning:
	default:
		return taskpkg.SupervisedRunRecoveryResult{}, nil
	}
	if source.SessionID != mutation.Stop.SessionID ||
		(source.WorkspaceID != "" && source.WorkspaceID != mutation.Stop.WorkspaceID) ||
		mutation.Stop.EventID == "" {
		return taskpkg.SupervisedRunRecoveryResult{}, fmt.Errorf(
			"%w: supervised recovery scope mismatch",
			taskpkg.ErrValidation,
		)
	}
	_, err = sqlcgen.New(s.exec).GetTaskRunTerminalCommandByScope(ctx, sqlcgen.GetTaskRunTerminalCommandByScopeParams{
		RunID: source.ID, TaskID: source.TaskID, WorkspaceID: source.WorkspaceID,
	})
	if err == nil {
		return taskpkg.SupervisedRunRecoveryResult{}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return taskpkg.SupervisedRunRecoveryResult{}, err
	}
	taskRecord, err := s.tasks.getTaskWithExecutor(ctx, s.exec, source.TaskID)
	if err != nil {
		return taskpkg.SupervisedRunRecoveryResult{}, err
	}
	// Task profiles are authoritative; task-run projections do not hydrate ProfileID.
	if taskRecord.ProfileID != mutation.Stop.ProfileID {
		return taskpkg.SupervisedRunRecoveryResult{}, nil
	}
	if taskRecord.Status == taskpkg.TaskStatusCompleted {
		return taskpkg.SupervisedRunRecoveryResult{}, nil
	}
	if err := validateTaskForQueuedRunReservation(taskRecord); err != nil {
		return taskpkg.SupervisedRunRecoveryResult{}, nil
	}
	return s.recoverSupervisedRun(ctx, source, taskRecord, mutation)
}

// recoverSupervisedRun atomically settles the stopped attempt and its successor, including any owned Loop cell.
func (s *taskMutationTxStore) recoverSupervisedRun(
	ctx context.Context, source taskpkg.Run, taskRecord taskpkg.Task, mutation *taskpkg.SupervisedRunRecoveryMutation,
) (taskpkg.SupervisedRunRecoveryResult, error) {
	metadata, loopRun, allowed, err := loadSupervisedLoopRecovery(ctx, s.exec, source)
	if err != nil || !allowed {
		return taskpkg.SupervisedRunRecoveryResult{}, err
	}
	nextAttempt, err := nextTaskRunAttemptNumberWithExecutor(ctx, s.exec, taskRecord)
	if err != nil {
		return taskpkg.SupervisedRunRecoveryResult{}, err
	}
	consumed := int64(nextAttempt) + int64(source.RecoveryCount)
	if consumed > int64(normalizeStoredTaskMaxAttempts(taskRecord.MaxAttempts)) {
		attention, attentionErr := s.MarkRunNeedsAttentionMutation(ctx,
			taskpkg.NewRunNeedsAttentionCommand(source, taskpkg.SupervisedSilenceExhaustedReason, mutation.At))
		return taskpkg.SupervisedRunRecoveryResult{
			Previous: source,
			Run:      attention.Run,
			Applied:  attention.Applied,
		}, attentionErr
	}
	failed := source
	failed.Status, failed.Error, failed.EndedAt = taskpkg.TaskRunStatusFailed, taskpkg.SupervisedSilenceReason, mutation.At
	if err := transitionTerminalRunRecordWithExecutor(
		ctx,
		s.exec,
		&failed,
		taskpkg.NewRunMutationFence(source),
	); err != nil {
		return taskpkg.SupervisedRunRecoveryResult{}, err
	}
	if err := updateTaskCurrentRunProjectionForRunUpdate(ctx, s.exec, source, failed); err != nil {
		return taskpkg.SupervisedRunRecoveryResult{}, err
	}
	openRunID, err := s.tasks.findOpenRunIDForQueuedRunReservation(
		ctx,
		s.exec,
		source.TaskID,
		source.DesignationGroupID,
	)
	if err != nil {
		return taskpkg.SupervisedRunRecoveryResult{}, err
	}
	if openRunID != "" {
		return taskpkg.SupervisedRunRecoveryResult{}, fmt.Errorf(
			"%w: task has another open run",
			taskpkg.ErrInvalidStatusTransition,
		)
	}
	if err := requireNoRetryChildWithExecutor(ctx, s.exec, source.ID); err != nil {
		return taskpkg.SupervisedRunRecoveryResult{}, err
	}
	continuation, err := s.insertSupervisedContinuation(ctx, source, metadata, mutation, nextAttempt)
	if err != nil {
		return taskpkg.SupervisedRunRecoveryResult{}, err
	}
	if source.IsLoopWorker() {
		if err := closeTerminalRunAgentBinding(ctx, s.exec, source, mutation.At); err != nil {
			return taskpkg.SupervisedRunRecoveryResult{}, err
		}
		args := retryTaskRunArgs{origin: source.Origin, queuedAt: mutation.At, reason: taskpkg.SupervisedSilenceReason}
		if err := applyLoopTaskRecovery(ctx, s.exec, loopRun, source, continuation, metadata, args); err != nil {
			return taskpkg.SupervisedRunRecoveryResult{}, err
		}
	}
	return taskpkg.SupervisedRunRecoveryResult{Previous: source, Run: continuation, Applied: true, Requeued: true}, nil
}

// insertSupervisedContinuation preserves execution context while issuing a fresh run
// without live session or lease state.
func (s *taskMutationTxStore) insertSupervisedContinuation(
	ctx context.Context, source taskpkg.Run, metadata loopNodeRunMetadata,
	mutation *taskpkg.SupervisedRunRecoveryMutation, attempt int,
) (taskpkg.Run, error) {
	continuation := source
	continuation.ID, continuation.PreviousRunID = mutation.NewRunID, source.ID
	var err error
	continuation.Attempt, err = taskRunAttemptFromInt(attempt)
	if err != nil {
		return taskpkg.Run{}, err
	}
	continuation.Status = taskpkg.TaskRunStatusQueued
	continuation.SessionID, continuation.ClaimTokenHash, continuation.IdempotencyKey = "", "", ""
	continuation.ClaimedBy = nil
	continuation.ClaimedAt, continuation.StartedAt, continuation.EndedAt = time.Time{}, time.Time{}, time.Time{}
	continuation.LeaseUntil, continuation.HeartbeatAt = time.Time{}, time.Time{}
	continuation.QueuedAt, continuation.TokensUsed = mutation.At, 0
	continuation.Error, continuation.FailureKind = "", ""
	continuation.RunResultState = nil
	continuation.SetNetworkState(source.NetworkSpecSnapshot(), "", "", "")
	if source.IsLoopWorker() {
		continuation.Metadata, err = loopTaskRecoveryMetadata(source.Metadata, nil, metadata)
		if err != nil {
			return taskpkg.Run{}, err
		}
	}
	continuation, err = s.tasks.normalizeTaskRunForCreate(continuation)
	if err != nil {
		return taskpkg.Run{}, err
	}
	if err := insertQueuedTaskRun(ctx, s.exec, continuation); err != nil {
		return taskpkg.Run{}, err
	}
	return continuation, nil
}
