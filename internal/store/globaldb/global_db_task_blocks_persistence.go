package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (g *TaskRepo) insertTaskBlockWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	block taskpkg.TaskBlock,
) (taskpkg.TaskBlock, error) {
	taskRecord, err := g.getTaskWithExecutor(ctx, exec, block.TaskID)
	if err != nil {
		return taskpkg.TaskBlock{}, err
	}
	normalized := block
	normalized.WorkspaceID = taskBlockWorkspaceID(taskRecord)
	if err := validateTaskBlockForInsert(normalized); err != nil {
		return taskpkg.TaskBlock{}, err
	}
	if err := sqlcgen.New(exec).InsertTaskBlock(ctx, sqlcgen.InsertTaskBlockParams{
		ID:            normalized.ID,
		WorkspaceID:   nullableTaskString(normalized.WorkspaceID),
		TaskID:        normalized.TaskID,
		Kind:          string(normalized.Kind),
		Reason:        normalized.Reason,
		DetailsJson:   nullableTaskRawJSON(normalized.Details),
		CreatedByKind: string(normalized.CreatedBy.Kind),
		CreatedByRef:  normalized.CreatedBy.Ref,
		CreatedAt:     store.FormatTimestamp(normalized.CreatedAt),
		ExpiresAt:     nullableTaskTime(normalized.ExpiresAt),
	}); err != nil {
		return taskpkg.TaskBlock{}, fmt.Errorf("store: create task block %q: %w", normalized.ID, err)
	}
	return g.getTaskBlockWithExecutor(ctx, exec, normalized.TaskID, normalized.ID)
}

func (g *TaskRepo) insertTaskBlockWithBreaker(
	ctx context.Context,
	exec taskSQLExecutor,
	block taskpkg.TaskBlock,
	recurrenceLimit int,
	actor taskpkg.ActorContext,
) (taskpkg.BlockMutationResult, error) {
	hadClearedPrior, err := g.hasClearedTaskBlockKindWithExecutor(ctx, exec, block.TaskID, block.Kind)
	if err != nil {
		return taskpkg.BlockMutationResult{}, err
	}
	created, err := g.insertTaskBlockWithExecutor(ctx, exec, block)
	if err != nil {
		return taskpkg.BlockMutationResult{}, err
	}
	result := taskpkg.BlockMutationResult{
		Block: created,
		Recurrence: taskpkg.BlockRecurrence{
			TaskID: created.TaskID,
			Kind:   created.Kind,
		},
	}
	if !hadClearedPrior {
		return result, nil
	}
	recurrence, err := incrementBlockRecurrenceWithExecutor(ctx, exec, created.TaskID, created.Kind, created.CreatedAt)
	if err != nil {
		return taskpkg.BlockMutationResult{}, err
	}
	result.Recurrence = recurrence
	if recurrenceLimit == 0 {
		return result, nil
	}
	if recurrence.Count < recurrenceLimit {
		return result, nil
	}
	escalated, changed, err := g.markTaskNeedsAttentionIfClearWithExecutor(ctx, exec, taskpkg.NeedsAttentionMutation{
		TaskID:   created.TaskID,
		Reason:   blockRecurrenceNeedsAttentionReason(recurrence),
		Actor:    actor.Actor,
		MarkedAt: created.CreatedAt,
		Origin:   actor.Origin,
	})
	if err != nil {
		return taskpkg.BlockMutationResult{}, err
	}
	if changed {
		result.EscalatedTask = &escalated
	}
	return result, nil
}

func (g *TaskRepo) hasClearedTaskBlockKindWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	kind taskpkg.BlockKind,
) (bool, error) {
	taskRecord, err := g.getTaskWithExecutor(ctx, exec, taskID)
	if err != nil {
		return false, err
	}
	exists, err := sqlcgen.New(exec).HasClearedTaskBlockKind(ctx, sqlcgen.HasClearedTaskBlockKindParams{
		TaskID:      strings.TrimSpace(taskID),
		Kind:        string(kind.Normalize()),
		WorkspaceID: store.NullableString(taskBlockWorkspaceID(taskRecord)),
	})
	if err != nil {
		return false, fmt.Errorf(
			"store: check cleared task block recurrence prior for task %q kind %q: %w",
			taskID,
			kind,
			err,
		)
	}
	return exists, nil
}

func blockRecurrenceNeedsAttentionReason(recurrence taskpkg.BlockRecurrence) string {
	return fmt.Sprintf(
		"task re-blocked %d times with %q blocks",
		recurrence.Count,
		recurrence.Kind.Normalize(),
	)
}

type taskBlockExpiryCandidate struct {
	TaskID  string
	BlockID string
}

func (g *TaskRepo) listExpiredTaskBlockCandidates(
	ctx context.Context,
	now time.Time,
) ([]taskBlockExpiryCandidate, error) {
	rows, err := g.queries.ListExpiredTaskBlockCandidates(ctx, nullableTaskTime(now.UTC()))
	if err != nil {
		return nil, fmt.Errorf("store: query expired task blocks: %w", err)
	}
	candidates := make([]taskBlockExpiryCandidate, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, taskBlockExpiryCandidate{TaskID: row.TaskID, BlockID: row.ID})
	}
	return candidates, nil
}

func uniqueTaskBlockCandidateTaskIDs(candidates []taskBlockExpiryCandidate) []string {
	seen := make(map[string]struct{}, len(candidates))
	taskIDs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		taskID := strings.TrimSpace(candidate.TaskID)
		if taskID == "" {
			continue
		}
		if _, ok := seen[taskID]; ok {
			continue
		}
		seen[taskID] = struct{}{}
		taskIDs = append(taskIDs, taskID)
	}
	return taskIDs
}

func (g *TaskRepo) expireTaskBlocksForTaskWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	mutation taskpkg.ExpireTaskBlocksMutation,
) ([]taskpkg.TaskBlock, error) {
	taskRecord, err := g.getTaskWithExecutor(ctx, exec, taskID)
	if err != nil {
		return nil, err
	}
	workspaceID := store.NullableString(taskBlockWorkspaceID(taskRecord))
	blockIDs, err := expiredTaskBlockIDsForTaskWithExecutor(
		ctx,
		exec,
		taskID,
		workspaceID,
		mutation.Now,
	)
	if err != nil {
		return nil, err
	}

	expired := make([]taskpkg.TaskBlock, 0, len(blockIDs))
	for _, blockID := range blockIDs {
		affected, err := sqlcgen.New(exec).ExpireTaskBlock(ctx, sqlcgen.ExpireTaskBlockParams{
			ClearedAt:     nullableTaskTime(mutation.Now),
			ClearedByKind: nullableTaskString(string(mutation.ClearedBy.Kind)),
			ClearedByRef:  nullableTaskString(mutation.ClearedBy.Ref),
			ID:            blockID,
			TaskID:        strings.TrimSpace(taskID),
			Now:           nullableTaskTime(mutation.Now),
			WorkspaceID:   workspaceID,
		})
		if err != nil {
			return nil, fmt.Errorf("store: expire task block %q: %w", blockID, err)
		}
		if affected == 0 {
			continue
		}
		block, err := g.getTaskBlockWithExecutor(ctx, exec, taskID, blockID)
		if err != nil {
			return nil, err
		}
		expired = append(expired, block)
	}
	return expired, nil
}

func expiredTaskBlockIDsForTaskWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	workspaceID any,
	now time.Time,
) ([]string, error) {
	rows, err := sqlcgen.New(exec).ListExpiredTaskBlockIDs(ctx, sqlcgen.ListExpiredTaskBlockIDsParams{
		TaskID:      strings.TrimSpace(taskID),
		Now:         nullableTaskTime(now),
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("store: query expired task blocks for task %q: %w", taskID, err)
	}
	return rows, nil
}

func normalizeCreateTaskBlockMutation(
	mutation taskpkg.CreateTaskBlockMutation,
	defaultNow time.Time,
) (taskpkg.CreateTaskBlockMutation, error) {
	normalized := mutation
	if normalized.RecurrenceLimit < 0 {
		return taskpkg.CreateTaskBlockMutation{}, fmt.Errorf(
			"%w: task_block.recurrence_limit cannot be negative: %d",
			taskpkg.ErrValidation,
			normalized.RecurrenceLimit,
		)
	}
	block, err := normalizeTaskBlockForCreate(normalized.Block, defaultNow)
	if err != nil {
		return taskpkg.CreateTaskBlockMutation{}, err
	}
	if err := normalized.Actor.Validate(); err != nil {
		return taskpkg.CreateTaskBlockMutation{}, err
	}
	normalized.Block = block
	return normalized, nil
}

func normalizeExpireTaskBlocksMutation(
	mutation taskpkg.ExpireTaskBlocksMutation,
	defaultNow time.Time,
) (taskpkg.ExpireTaskBlocksMutation, error) {
	normalized := mutation
	if normalized.Now.IsZero() {
		normalized.Now = defaultNow.UTC()
	} else {
		normalized.Now = normalized.Now.UTC()
	}
	normalized.ClearedBy.Kind = normalized.ClearedBy.Kind.Normalize()
	normalized.ClearedBy.Ref = strings.TrimSpace(normalized.ClearedBy.Ref)
	if err := normalized.ClearedBy.Validate("task_block.expired_by"); err != nil {
		return taskpkg.ExpireTaskBlocksMutation{}, err
	}
	return normalized, nil
}

func (g *TaskRepo) getTaskBlockWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	blockID string,
) (taskpkg.TaskBlock, error) {
	trimmedTaskID, err := requireTaskValue(taskID, "task id")
	if err != nil {
		return taskpkg.TaskBlock{}, err
	}
	trimmedBlockID, err := requireTaskValue(blockID, "task block id")
	if err != nil {
		return taskpkg.TaskBlock{}, err
	}
	taskRecord, err := g.getTaskWithExecutor(ctx, exec, trimmedTaskID)
	if err != nil {
		return taskpkg.TaskBlock{}, err
	}
	row, err := sqlcgen.New(exec).GetTaskBlock(ctx, sqlcgen.GetTaskBlockParams{
		ID: trimmedBlockID, TaskID: trimmedTaskID,
		WorkspaceID: store.NullableString(taskBlockWorkspaceID(taskRecord)),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.TaskBlock{}, taskpkg.ErrTaskBlockNotFound
		}
		return taskpkg.TaskBlock{}, fmt.Errorf("store: get task block %q: %w", trimmedBlockID, err)
	}
	return taskBlockFromGenerated(row)
}

func upsertBlockRecurrenceWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	recurrence taskpkg.BlockRecurrence,
) (taskpkg.BlockRecurrence, error) {
	if err := sqlcgen.New(exec).UpsertTaskBlockRecurrence(ctx, sqlcgen.UpsertTaskBlockRecurrenceParams{
		TaskID: recurrence.TaskID, Kind: string(recurrence.Kind),
		Count: int64(recurrence.Count), UpdatedAt: store.FormatTimestamp(recurrence.UpdatedAt),
	}); err != nil {
		return taskpkg.BlockRecurrence{}, fmt.Errorf(
			"store: upsert task block recurrence for task %q kind %q: %w",
			recurrence.TaskID,
			recurrence.Kind,
			err,
		)
	}
	return getBlockRecurrenceWithExecutor(ctx, exec, recurrence.TaskID, recurrence.Kind)
}

func incrementBlockRecurrenceWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	kind taskpkg.BlockKind,
	updatedAt time.Time,
) (taskpkg.BlockRecurrence, error) {
	if err := sqlcgen.New(exec).IncrementTaskBlockRecurrence(ctx, sqlcgen.IncrementTaskBlockRecurrenceParams{
		TaskID: taskID, Kind: string(kind), UpdatedAt: store.FormatTimestamp(updatedAt),
	}); err != nil {
		return taskpkg.BlockRecurrence{}, fmt.Errorf(
			"store: increment task block recurrence for task %q kind %q: %w",
			taskID,
			kind,
			err,
		)
	}
	return getBlockRecurrenceWithExecutor(ctx, exec, taskID, kind)
}

func resetTaskBlockRecurrencesWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
) error {
	err := sqlcgen.New(exec).DeleteTaskBlockRecurrences(ctx, taskID)
	if err != nil {
		return fmt.Errorf("store: reset task block recurrences for task %q: %w", taskID, err)
	}
	return nil
}

func getBlockRecurrenceWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	kind taskpkg.BlockKind,
) (taskpkg.BlockRecurrence, error) {
	row, err := sqlcgen.New(exec).GetTaskBlockRecurrence(ctx, sqlcgen.GetTaskBlockRecurrenceParams{
		TaskID: taskID, Kind: string(kind),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.BlockRecurrence{
				TaskID: taskID,
				Kind:   kind,
			}, nil
		}
		return taskpkg.BlockRecurrence{}, fmt.Errorf("store: get task block recurrence: %w", err)
	}
	return taskBlockRecurrenceFromGenerated(row)
}

func (g *TaskRepo) updateTaskNeedsAttention(
	ctx context.Context,
	mutation taskpkg.NeedsAttentionMutation,
) (taskpkg.Task, error) {
	var updated taskpkg.Task
	if err := g.withTaskImmediateTransaction(ctx, "mark task needs attention", func(exec taskSQLExecutor) error {
		var changed bool
		var err error
		updated, changed, err = g.markTaskNeedsAttentionIfClearWithExecutor(ctx, exec, mutation)
		if err != nil {
			return err
		}
		if !changed {
			var loadErr error
			updated, loadErr = g.getTaskWithExecutor(ctx, exec, mutation.TaskID)
			return loadErr
		}
		return nil
	}); err != nil {
		return taskpkg.Task{}, err
	}
	return updated, nil
}

func (g *TaskRepo) markTaskNeedsAttentionIfClearWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation taskpkg.NeedsAttentionMutation,
) (taskpkg.Task, bool, error) {
	affected, err := sqlcgen.New(exec).MarkTaskNeedsAttentionIfClear(
		ctx,
		sqlcgen.MarkTaskNeedsAttentionIfClearParams{
			Reason:    nullableTaskString(mutation.Reason),
			MarkedAt:  nullableTaskTime(mutation.MarkedAt),
			ActorKind: nullableTaskString(string(mutation.Actor.Kind)),
			ActorRef:  nullableTaskString(mutation.Actor.Ref),
			ID:        mutation.TaskID,
		},
	)
	if err != nil {
		return taskpkg.Task{}, false, fmt.Errorf("store: mark task needs attention %q: %w", mutation.TaskID, err)
	}
	if affected == 0 {
		if _, loadErr := g.getTaskWithExecutor(ctx, exec, mutation.TaskID); loadErr != nil {
			return taskpkg.Task{}, false, loadErr
		}
		return taskpkg.Task{}, false, nil
	}
	updated, err := g.getTaskWithExecutor(ctx, exec, mutation.TaskID)
	if err != nil {
		return taskpkg.Task{}, false, err
	}
	return updated, true, nil
}
