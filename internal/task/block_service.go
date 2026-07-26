package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"strings"
	"time"
)

const taskBlockIDPrefix = "block"

// BlockTask creates a runtime-declared task block and parks the active run when supplied.
func (m *Service) BlockTask(ctx context.Context, req BlockRequest, actor ActorContext) (TaskBlock, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return TaskBlock{}, err
	}
	block, runID, claimToken, err := m.taskBlockFromRequest(req, actor)
	if err != nil {
		return TaskBlock{}, err
	}
	if _, err := m.loadAuthorizedTask(ctx, m.store, block.TaskID, actor); err != nil {
		return TaskBlock{}, err
	}
	if runID != "" || claimToken != "" {
		if runID == "" {
			return TaskBlock{}, fmt.Errorf("%w: task_block.run_id is required when claim_token is set", ErrValidation)
		}
		if claimToken == "" {
			return TaskBlock{}, fmt.Errorf("%w: task_block.claim_token is required when run_id is set", ErrValidation)
		}
		result, blockErr := m.store.BlockTaskAndReleaseRun(ctx, BlockTaskAndReleaseRunMutation{
			Block:           block,
			RunID:           runID,
			ClaimToken:      claimToken,
			Now:             block.CreatedAt,
			RecurrenceLimit: m.blockRecurrenceLimit,
			Actor:           actor,
		})
		if blockErr != nil {
			return TaskBlock{}, blockErr
		}
		reconciled, reconcileErr := m.reconcileTaskCascade(ctx, result.Block.TaskID, actor)
		if reconcileErr != nil {
			return TaskBlock{}, reconcileErr
		}
		m.recordTaskBlockCreated(ctx, result.Block, reconciled, actor, &result)
		m.recordReleasedRunEvent(ctx, &result, reconciled, actor)
		m.dispatchTaskBlocked(ctx, result.Block, reconciled, actor, &result)
		m.dispatchBlockedWake(ctx, reconciled, result.Block, actor, &result)
		m.recordTaskNeedsAttention(
			ctx,
			result.Block,
			result.EscalatedTask,
			reconciled,
			actor,
			&result,
		)
		m.dispatchTaskRunReleased(ctx, result.Run, reconciled, actor, result.PreviousRun, result.ReleaseReason)
		return result.Block, nil
	}

	if err := m.requireAgentSessionTaskLease(ctx, block.TaskID, actor); err != nil {
		return TaskBlock{}, err
	}

	created, err := m.store.CreateTaskBlock(ctx, CreateTaskBlockMutation{
		Block:           block,
		RecurrenceLimit: m.blockRecurrenceLimit,
		Actor:           actor,
	})
	if err != nil {
		return TaskBlock{}, err
	}
	reconciled, err := m.reconcileTaskCascade(ctx, created.Block.TaskID, actor)
	if err != nil {
		return TaskBlock{}, err
	}
	m.recordTaskBlockCreated(ctx, created.Block, reconciled, actor, nil)
	m.dispatchTaskBlocked(ctx, created.Block, reconciled, actor, nil)
	m.dispatchBlockedWake(ctx, reconciled, created.Block, actor, nil)
	m.recordTaskNeedsAttention(ctx, created.Block, created.EscalatedTask, reconciled, actor, nil)
	return created.Block, nil
}

func (m *Service) recordReleasedRunEvent(
	ctx context.Context,
	result *BlockTaskAndReleaseRunResult,
	reconciled Task,
	actor ActorContext,
) {
	if result == nil {
		return
	}
	if eventErr := m.recordTaskEvent(
		ctx,
		result.Run.TaskID,
		result.Run.ID,
		taskEventRunReleased,
		actor,
		releasedRunPayload{
			PreviousStatus: result.PreviousRun.Status,
			Status:         result.Run.Status,
			TaskStatus:     reconciled.Status,
			Reason:         result.ReleaseReason,
			SessionID:      result.PreviousRun.SessionID,
		},
	); eventErr != nil {
		slog.Error(
			"task: run release event failed after committed block-and-release",
			"error", eventErr,
			"event_type", taskEventRunReleased,
			"task_id", result.Run.TaskID,
			"run_id", result.Run.ID,
		)
	}
}

func (m *Service) recordTaskBlockCreated(
	ctx context.Context,
	block TaskBlock,
	reconciled Task,
	actor ActorContext,
	release *BlockTaskAndReleaseRunResult,
) {
	runID := ""
	claimTokenHash := ""
	if release != nil {
		runID = strings.TrimSpace(release.Run.ID)
		claimTokenHash = strings.TrimSpace(release.ClaimTokenHash)
	}
	if eventErr := m.recordTaskEvent(ctx, block.TaskID, runID, taskEventBlockCreated, actor, taskBlockCreatedPayload{
		Status:         reconciled.Status,
		BlockID:        strings.TrimSpace(block.ID),
		BlockKind:      block.Kind.Normalize(),
		Reason:         redactTaskSecretText(strings.TrimSpace(block.Reason)),
		ExpiresAt:      block.ExpiresAt,
		ClaimTokenHash: claimTokenHash,
	}); eventErr != nil {
		m.logTaskBlockEventFailure(eventErr, taskEventBlockCreated, block, runID)
	}
}

func (m *Service) recordTaskBlockCleared(
	ctx context.Context,
	block TaskBlock,
	reconciled Task,
	actor ActorContext,
) {
	if eventErr := m.recordTaskEvent(ctx, block.TaskID, "", taskEventBlockCleared, actor, taskBlockClearedPayload{
		Status:    reconciled.Status,
		BlockID:   strings.TrimSpace(block.ID),
		BlockKind: block.Kind.Normalize(),
		Reason:    redactTaskSecretText(strings.TrimSpace(block.Reason)),
		ClearNote: redactTaskSecretText(strings.TrimSpace(block.ClearNote)),
		ClearedAt: block.ClearedAt,
	}); eventErr != nil {
		m.logTaskBlockEventFailure(eventErr, taskEventBlockCleared, block, "")
	}
}

func (m *Service) recordTaskBlockExpired(
	ctx context.Context,
	block TaskBlock,
	reconciled Task,
	actor ActorContext,
) {
	if eventErr := m.recordTaskEvent(ctx, block.TaskID, "", taskEventBlockExpired, actor, taskBlockExpiredPayload{
		Status:    reconciled.Status,
		BlockID:   strings.TrimSpace(block.ID),
		BlockKind: block.Kind.Normalize(),
		Reason:    redactTaskSecretText(strings.TrimSpace(block.Reason)),
		ExpiresAt: block.ExpiresAt,
		ClearedAt: block.ClearedAt,
	}); eventErr != nil {
		m.logTaskBlockEventFailure(eventErr, taskEventBlockExpired, block, "")
	}
}

func (m *Service) logTaskBlockEventFailure(err error, eventType string, block TaskBlock, runID string) {
	slog.Error(
		"task: task block event failed after committed mutation",
		"error", err,
		"event_type", eventType,
		"task_id", block.TaskID,
		"run_id", strings.TrimSpace(runID),
		"block_id", block.ID,
	)
}

func (m *Service) requireAgentSessionTaskLease(ctx context.Context, taskID string, actor ActorContext) error {
	if actor.Actor.Kind.Normalize() != ActorKindAgentSession {
		return nil
	}
	sessionID := strings.TrimSpace(actor.Actor.Ref)
	normalizedTaskID := strings.TrimSpace(taskID)
	if sessionID == "" {
		return autonomyError(AutonomySessionRequired, ErrPermissionDenied, "agent session identity is required")
	}
	if normalizedTaskID == "" {
		return fmt.Errorf("%w: task.id is required", ErrValidation)
	}
	leaseStore, ok := m.store.(AutonomyLeaseStore)
	if !ok {
		return errors.New("task: autonomy lease lookup store is unavailable")
	}
	handles, err := leaseStore.ListAutonomyLeaseHandles(ctx, sessionID)
	if err != nil {
		return err
	}
	activeCount := 0
	matched := false
	now := m.now().UTC()
	for _, handle := range handles {
		normalized := normalizeAutonomyLeaseHandle(handle)
		if !isScopedTaskLeaseActive(normalized, sessionID, now) {
			continue
		}
		activeCount++
		if normalized.TaskID == normalizedTaskID {
			matched = true
		}
	}
	switch {
	case activeCount > 1:
		return autonomyError(
			AutonomyLeaseAlreadyHeld,
			ErrActiveRunLease,
			"session %q owns multiple active task-run leases",
			sessionID,
		)
	case matched:
		return nil
	case activeCount == 1:
		return autonomyError(
			AutonomyForeignRun,
			ErrPermissionDenied,
			"task %q is not leased by session %q",
			normalizedTaskID,
			sessionID,
		)
	default:
		return autonomyError(
			AutonomyNoActiveLease,
			ErrInvalidClaimToken,
			"session %q has no active task-run lease",
			sessionID,
		)
	}
}

func isScopedTaskLeaseActive(handle AutonomyLeaseHandle, sessionID string, now time.Time) bool {
	return handle.SessionID == sessionID &&
		isAutonomyLeaseStatusActive(handle.Status) &&
		!handle.LeaseUntil.IsZero() &&
		handle.LeaseUntil.After(now) &&
		handle.ClaimTokenHash != ""
}
