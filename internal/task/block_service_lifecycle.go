package task

import (
	"context"

	"fmt"

	"sort"
	"strings"
	"time"
)

// ClearTaskBlock clears one open task block through the service-owned transition.
func (m *Service) ClearTaskBlock(
	ctx context.Context,
	taskID string,
	blockID string,
	note string,
	actor ActorContext,
) (TaskBlock, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return TaskBlock{}, err
	}
	normalizedTaskID := strings.TrimSpace(taskID)
	normalizedBlockID := strings.TrimSpace(blockID)
	if normalizedTaskID == "" {
		return TaskBlock{}, fmt.Errorf("%w: task_block.task_id is required", ErrValidation)
	}
	if normalizedBlockID == "" {
		return TaskBlock{}, fmt.Errorf("%w: task_block.id is required", ErrValidation)
	}
	if _, err := m.loadAuthorizedTask(ctx, m.store, normalizedTaskID, actor); err != nil {
		return TaskBlock{}, err
	}
	normalizedNote := strings.TrimSpace(note)
	if err := rejectTaskSecretText("task_block.clear_note", normalizedNote); err != nil {
		return TaskBlock{}, err
	}
	if err := m.requireAgentSessionTaskLease(ctx, normalizedTaskID, actor); err != nil {
		return TaskBlock{}, err
	}

	cleared, err := m.store.ClearTaskBlock(ctx, ClearTaskBlockMutation{
		TaskID:    normalizedTaskID,
		BlockID:   normalizedBlockID,
		ClearedBy: actor.Actor,
		ClearedAt: m.now().UTC(),
		ClearNote: normalizedNote,
		Actor:     actor,
	})
	if err != nil {
		return TaskBlock{}, err
	}
	reconciled, err := m.reconcileTaskCascade(ctx, cleared.TaskID, actor)
	if err != nil {
		return TaskBlock{}, err
	}
	m.recordTaskBlockCleared(ctx, cleared, reconciled, actor)
	m.dispatchTaskUnblocked(ctx, cleared, reconciled, actor)
	m.autoEnqueueReadyTaskDetached(ctx, cleared.TaskID, autoEnqueueTrigger{
		Kind: autoEnqueueTriggerBlockClear,
		Ref:  cleared.ID,
	}, actor)
	return cleared, nil
}

// RecoverTask clears task-level needs_attention escalation through the service-owned transition.
func (m *Service) RecoverTask(ctx context.Context, id string, note string, actor ActorContext) (*Task, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, fmt.Errorf("%w: task.id is required", ErrValidation)
	}
	if actor.Actor.Kind.Normalize() == ActorKindAgentSession {
		return nil, autonomyError(
			AutonomyForeignRun,
			ErrPermissionDenied,
			"agent session %q cannot recover task %q without operator scope",
			strings.TrimSpace(actor.Actor.Ref),
			trimmedID,
		)
	}
	if _, err := m.loadAuthorizedTask(ctx, m.store, trimmedID, actor); err != nil {
		return nil, err
	}
	normalizedNote := strings.TrimSpace(note)
	if err := rejectTaskSecretText("task.recover_note", normalizedNote); err != nil {
		return nil, err
	}
	recoveredAt := m.now().UTC()
	cleared, err := m.store.ClearTaskNeedsAttention(ctx, NeedsAttentionClearMutation{
		TaskID:    trimmedID,
		Note:      normalizedNote,
		Actor:     actor,
		ClearedAt: recoveredAt,
	})
	if err != nil {
		return nil, err
	}
	reconciled, err := m.publishAndReconcileRecoveredTask(ctx, &cleared, actor)
	if err != nil {
		return nil, err
	}
	m.dispatchTaskRecovered(ctx, reconciled, actor, normalizedNote, recoveredAt)
	m.autoEnqueueReadyTaskDetached(ctx, reconciled.ID, autoEnqueueTrigger{
		Kind: autoEnqueueTriggerRecover,
		Ref:  reconciled.ID,
	}, actor)
	return &reconciled, nil
}

// ExpireTaskBlocks finalizes expired transient blocks through service-owned transitions.
func (m *Service) ExpireTaskBlocks(
	ctx context.Context,
	now time.Time,
	actor ActorContext,
) (ExpireTaskBlocksResult, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return ExpireTaskBlocksResult{}, err
	}
	expireAt := now.UTC()
	if expireAt.IsZero() {
		expireAt = m.now().UTC()
	}
	result, err := m.store.ExpireTaskBlocks(ctx, ExpireTaskBlocksMutation{
		Now:       expireAt,
		ClearedBy: actor.Actor,
	})
	if err != nil {
		return ExpireTaskBlocksResult{}, err
	}
	blocksByTask := make(map[string][]TaskBlock)
	taskIDs := make([]string, 0)
	for _, block := range result.Blocks {
		if _, ok := blocksByTask[block.TaskID]; !ok {
			taskIDs = append(taskIDs, block.TaskID)
		}
		blocksByTask[block.TaskID] = append(blocksByTask[block.TaskID], block)
	}
	for _, taskID := range taskIDs {
		blocks := blocksByTask[taskID]
		reconciled, reconcileErr := m.reconcileTaskCascade(ctx, taskID, actor)
		if reconcileErr != nil {
			return ExpireTaskBlocksResult{}, reconcileErr
		}
		for _, block := range blocks {
			m.recordTaskBlockExpired(ctx, block, reconciled, actor)
			m.dispatchTaskUnblocked(ctx, block, reconciled, actor)
		}
		m.autoEnqueueReadyTaskDetached(ctx, taskID, autoEnqueueTrigger{
			Kind: autoEnqueueTriggerTransientExpiry,
			Ref:  transientExpiryTriggerRef(blocks),
		}, actor)
	}
	return result, nil
}

func transientExpiryTriggerRef(blocks []TaskBlock) string {
	if len(blocks) == 1 {
		return blocks[0].ID
	}
	ids := make([]string, 0, len(blocks))
	for _, block := range blocks {
		ids = append(ids, block.ID)
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

// ListTaskBlocks returns task block rows after enforcing read authority and task existence.
func (m *Service) ListTaskBlocks(
	ctx context.Context,
	taskID string,
	includeCleared bool,
	actor ActorContext,
) ([]TaskBlock, error) {
	if err := requireReadAuthority(actor); err != nil {
		return nil, err
	}
	normalizedTaskID := strings.TrimSpace(taskID)
	if normalizedTaskID == "" {
		return nil, fmt.Errorf("%w: task_block.task_id is required", ErrValidation)
	}
	if err := m.requireAgentSessionTaskLease(ctx, normalizedTaskID, actor); err != nil {
		return nil, err
	}
	if _, err := m.loadAuthorizedTask(ctx, m.store, normalizedTaskID, actor); err != nil {
		return nil, err
	}
	return m.store.ListTaskBlocks(ctx, normalizedTaskID, includeCleared)
}
