package globaldb

import (
	"context"
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
	taskpkg "github.com/compozy/agh/internal/task"
)

func appendTaskBlockedWatchEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	block taskpkg.TaskBlock,
	actor taskpkg.ActorContext,
	runID string,
	claimTokenHash string,
) error {
	return appendTaskEventPayloadWithExecutor(
		ctx,
		exec,
		block.TaskID,
		runID,
		string(hookspkg.HookTaskBlocked),
		actor,
		block.CreatedAt,
		taskBlockWatchEventPayload{
			BlockID:        strings.TrimSpace(block.ID),
			BlockKind:      block.Kind.Normalize(),
			Reason:         strings.TrimSpace(block.Reason),
			ExpiresAt:      block.ExpiresAt,
			ClaimTokenHash: strings.TrimSpace(claimTokenHash),
		},
	)
}

func appendTaskUnblockedWatchEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	block taskpkg.TaskBlock,
	actor taskpkg.ActorContext,
) error {
	return appendTaskEventPayloadWithExecutor(
		ctx,
		exec,
		block.TaskID,
		"",
		string(hookspkg.HookTaskUnblocked),
		actor,
		block.ClearedAt,
		taskBlockWatchEventPayload{
			BlockID:   strings.TrimSpace(block.ID),
			BlockKind: block.Kind.Normalize(),
			Reason:    strings.TrimSpace(block.Reason),
			ClearNote: strings.TrimSpace(block.ClearNote),
			ClearedAt: block.ClearedAt,
		},
	)
}

func appendNeedsAttentionWatchEventIfEscalated(
	ctx context.Context,
	exec taskSQLExecutor,
	result taskpkg.BlockMutationResult,
	actor taskpkg.ActorContext,
) error {
	if result.EscalatedTask == nil {
		return nil
	}
	needsAttention := result.EscalatedTask.NeedsAttention
	if needsAttention == nil {
		return nil
	}
	return appendTaskEventPayloadWithExecutor(
		ctx,
		exec,
		result.EscalatedTask.ID,
		"",
		string(hookspkg.HookTaskNeedsAttention),
		actor,
		needsAttention.At,
		taskAttentionWatchEventPayload{
			Status:          taskpkg.TaskStatusNeedsAttention,
			Reason:          strings.TrimSpace(needsAttention.Reason),
			At:              needsAttention.At,
			BlockID:         strings.TrimSpace(result.Block.ID),
			BlockKind:       result.Block.Kind.Normalize(),
			RecurrenceCount: result.Recurrence.Count,
		},
	)
}
