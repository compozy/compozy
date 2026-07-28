package task

import (
	"context"

	"fmt"
	"log/slog"

	"strings"
)

func (m *Service) taskBlockFromRequest(req BlockRequest, actor ActorContext) (TaskBlock, string, string, error) {
	now := m.now().UTC()
	taskID := strings.TrimSpace(req.TaskID)
	if taskID == "" {
		return TaskBlock{}, "", "", fmt.Errorf("%w: task_block.task_id is required", ErrValidation)
	}
	kind := req.Kind.Normalize()
	if err := kind.Validate("task_block.kind"); err != nil {
		return TaskBlock{}, "", "", err
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return TaskBlock{}, "", "", fmt.Errorf("%w: task_block.reason is required", ErrValidation)
	}
	if err := rejectTaskSecretText("task_block.reason", reason); err != nil {
		return TaskBlock{}, "", "", err
	}
	details := cloneRawJSON(req.Details)
	if err := rejectTaskSecretJSON("task_block.details", details); err != nil {
		return TaskBlock{}, "", "", err
	}
	expiresAt := req.ExpiresAt
	if !expiresAt.IsZero() {
		expiresAt = expiresAt.UTC()
		if kind != BlockKindTransient {
			return TaskBlock{}, "", "", fmt.Errorf(
				"%w: task_block.expires_at is only valid for %q blocks",
				ErrValidation,
				BlockKindTransient,
			)
		}
	}
	runID := strings.TrimSpace(req.RunID)
	claimToken := strings.TrimSpace(req.ClaimToken)

	return TaskBlock{
		ID:        m.newID(taskBlockIDPrefix),
		TaskID:    taskID,
		Kind:      kind,
		Reason:    reason,
		Details:   details,
		CreatedBy: ActorIdentity{Kind: actor.Actor.Kind.Normalize(), Ref: strings.TrimSpace(actor.Actor.Ref)},
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}, runID, claimToken, nil
}

func (m *Service) recordTaskNeedsAttention(
	ctx context.Context,
	block TaskBlock,
	escalatedTask *Task,
	reconciled Task,
	actor ActorContext,
	release *BlockTaskAndReleaseRunResult,
) {
	if escalatedTask == nil {
		return
	}
	needsAttention := reconciled.NeedsAttention
	if needsAttention == nil {
		needsAttention = escalatedTask.NeedsAttention
	}
	if needsAttention == nil {
		slog.Error(
			"task: recurrence escalation missing needs_attention metadata",
			"task_id", reconciled.ID,
			"block_id", block.ID,
		)
		return
	}
	m.dispatchNeedsAttentionWake(ctx, reconciled, block, needsAttention.Reason, actor, release)
	m.dispatchTaskNeedsAttention(ctx, reconciled, actor, needsAttention.Reason, needsAttention.At, release)
}
