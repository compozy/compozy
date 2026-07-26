package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

const taskRunReleaseReasonBlocked = "blocked"

var _ taskpkg.BlockStore = (*TaskRepo)(nil)

// CreateTaskBlock inserts one task block, stamping workspace_id from the owning task.
func (g *TaskRepo) CreateTaskBlock(
	ctx context.Context,
	mutation taskpkg.CreateTaskBlockMutation,
) (taskpkg.BlockMutationResult, error) {
	if err := g.checkReady(ctx, "create task block"); err != nil {
		return taskpkg.BlockMutationResult{}, err
	}
	normalized, err := normalizeCreateTaskBlockMutation(mutation, g.now())
	if err != nil {
		return taskpkg.BlockMutationResult{}, err
	}

	var result taskpkg.BlockMutationResult
	if err := g.withTaskImmediateTransaction(ctx, "create task block", func(exec taskSQLExecutor) error {
		var createErr error
		result, createErr = g.insertTaskBlockWithBreaker(
			ctx,
			exec,
			normalized.Block,
			normalized.RecurrenceLimit,
			normalized.Actor,
		)
		if createErr != nil {
			return createErr
		}
		if err := appendTaskBlockedWatchEvent(ctx, exec, result.Block, normalized.Actor, "", ""); err != nil {
			return err
		}
		return appendNeedsAttentionWatchEventIfEscalated(ctx, exec, result, normalized.Actor)
	}); err != nil {
		return taskpkg.BlockMutationResult{}, err
	}
	return result, nil
}

// GetTaskBlock returns one task block when it belongs to the supplied task.
func (g *TaskRepo) GetTaskBlock(ctx context.Context, taskID string, blockID string) (taskpkg.TaskBlock, error) {
	if err := g.checkReady(ctx, "get task block"); err != nil {
		return taskpkg.TaskBlock{}, err
	}
	return g.getTaskBlockWithExecutor(ctx, g.db, taskID, blockID)
}

// ClearTaskBlock stamps one open block as cleared and rejects repeated clears as conflicts.
func (g *TaskRepo) ClearTaskBlock(
	ctx context.Context,
	mutation taskpkg.ClearTaskBlockMutation,
) (taskpkg.TaskBlock, error) {
	if err := g.checkReady(ctx, "clear task block"); err != nil {
		return taskpkg.TaskBlock{}, err
	}
	normalized, err := normalizeClearTaskBlockMutation(mutation, g.now())
	if err != nil {
		return taskpkg.TaskBlock{}, err
	}

	var cleared taskpkg.TaskBlock
	if err := g.withTaskImmediateTransaction(ctx, "clear task block", func(exec taskSQLExecutor) error {
		taskRecord, err := g.getTaskWithExecutor(ctx, exec, normalized.TaskID)
		if err != nil {
			return err
		}
		affected, err := sqlcgen.New(exec).ClearTaskBlock(ctx, sqlcgen.ClearTaskBlockParams{
			ClearedAt:     nullableTaskTime(normalized.ClearedAt),
			ClearedByKind: nullableTaskString(string(normalized.ClearedBy.Kind)),
			ClearedByRef:  nullableTaskString(normalized.ClearedBy.Ref),
			ClearNote:     nullableTaskString(normalized.ClearNote),
			ID:            normalized.BlockID,
			TaskID:        normalized.TaskID,
			WorkspaceID:   store.NullableString(taskBlockWorkspaceID(taskRecord)),
		})
		if err != nil {
			return fmt.Errorf("store: clear task block %q: %w", normalized.BlockID, err)
		}
		if affected == 0 {
			current, loadErr := g.getTaskBlockWithExecutor(ctx, exec, normalized.TaskID, normalized.BlockID)
			if loadErr != nil {
				return loadErr
			}
			if !current.ClearedAt.IsZero() {
				return fmt.Errorf("%w: task block %q is already cleared", taskpkg.ErrConflict, normalized.BlockID)
			}
			return fmt.Errorf("store: task block %q: %w", normalized.BlockID, taskpkg.ErrTaskBlockNotFound)
		}
		cleared, err = g.getTaskBlockWithExecutor(ctx, exec, normalized.TaskID, normalized.BlockID)
		if err != nil {
			return err
		}
		return appendTaskUnblockedWatchEvent(ctx, exec, cleared, normalized.Actor)
	}); err != nil {
		return taskpkg.TaskBlock{}, err
	}
	return cleared, nil
}

// ExpireTaskBlocks finalizes expired transient blocks as daemon-cleared rows, grouped one transaction per task.
func (g *TaskRepo) ExpireTaskBlocks(
	ctx context.Context,
	mutation taskpkg.ExpireTaskBlocksMutation,
) (taskpkg.ExpireTaskBlocksResult, error) {
	if err := g.checkReady(ctx, "expire task blocks"); err != nil {
		return taskpkg.ExpireTaskBlocksResult{}, err
	}
	normalized, err := normalizeExpireTaskBlocksMutation(mutation, g.now())
	if err != nil {
		return taskpkg.ExpireTaskBlocksResult{}, err
	}
	candidates, err := g.listExpiredTaskBlockCandidates(ctx, normalized.Now)
	if err != nil {
		return taskpkg.ExpireTaskBlocksResult{}, err
	}
	blocks := make([]taskpkg.TaskBlock, 0, len(candidates))
	for _, taskID := range uniqueTaskBlockCandidateTaskIDs(candidates) {
		var taskBlocks []taskpkg.TaskBlock
		if err := g.withTaskImmediateTransaction(ctx, "expire task blocks", func(exec taskSQLExecutor) error {
			var expireErr error
			taskBlocks, expireErr = g.expireTaskBlocksForTaskWithExecutor(ctx, exec, taskID, normalized)
			return expireErr
		}); err != nil {
			return taskpkg.ExpireTaskBlocksResult{}, err
		}
		blocks = append(blocks, taskBlocks...)
	}
	return taskpkg.ExpireTaskBlocksResult{Blocks: blocks}, nil
}

// ListTaskBlocks returns task blocks for one task, open-only by default.
func (g *TaskRepo) ListTaskBlocks(
	ctx context.Context,
	taskID string,
	includeCleared bool,
) ([]taskpkg.TaskBlock, error) {
	if err := g.checkReady(ctx, "list task blocks"); err != nil {
		return nil, err
	}
	return g.listTaskBlocksWithExecutor(ctx, g.db, taskID, includeCleared, g.now())
}

func (g *TaskRepo) listTaskBlocksWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	includeCleared bool,
	now time.Time,
) ([]taskpkg.TaskBlock, error) {
	trimmedTaskID, err := requireTaskValue(taskID, "task id")
	if err != nil {
		return nil, err
	}
	taskRecord, err := g.getTaskWithExecutor(ctx, exec, trimmedTaskID)
	if err != nil {
		return nil, err
	}
	queries := sqlcgen.New(exec)
	workspaceID := store.NullableString(taskBlockWorkspaceID(taskRecord))
	var rows []sqlcgen.TaskBlock
	if includeCleared {
		rows, err = queries.ListAllTaskBlocks(ctx, sqlcgen.ListAllTaskBlocksParams{
			TaskID: trimmedTaskID, WorkspaceID: workspaceID,
		})
	} else {
		rows, err = queries.ListOpenTaskBlocks(ctx, sqlcgen.ListOpenTaskBlocksParams{
			TaskID: trimmedTaskID, WorkspaceID: workspaceID, Now: nullableTaskTime(now.UTC()),
		})
	}
	if err != nil {
		return nil, fmt.Errorf("store: query task blocks for task %q: %w", trimmedTaskID, err)
	}
	return taskBlocksFromGenerated(rows)
}

// HasOpenTaskBlocks returns whether a task currently has any open, non-expired block.
func (g *TaskRepo) HasOpenTaskBlocks(ctx context.Context, taskID string) (bool, error) {
	if err := g.checkReady(ctx, "check open task blocks"); err != nil {
		return false, err
	}
	return g.hasOpenTaskBlocksWithExecutor(ctx, g.db, taskID, g.now())
}

func (g *TaskRepo) hasOpenTaskBlocksWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	now time.Time,
) (bool, error) {
	trimmedTaskID, err := requireTaskValue(taskID, "task id")
	if err != nil {
		return false, err
	}
	taskRecord, err := g.getTaskWithExecutor(ctx, exec, trimmedTaskID)
	if err != nil {
		return false, err
	}
	exists, err := sqlcgen.New(exec).HasOpenTaskBlocks(ctx, sqlcgen.HasOpenTaskBlocksParams{
		TaskID:      trimmedTaskID,
		WorkspaceID: store.NullableString(taskBlockWorkspaceID(taskRecord)),
		Now:         nullableTaskTime(now.UTC()),
	})
	if err != nil {
		return false, fmt.Errorf("store: check open task blocks for task %q: %w", trimmedTaskID, err)
	}
	return exists, nil
}

// UpsertTaskBlockRecurrence sets the persisted counter for one task and block kind.
func (g *TaskRepo) UpsertTaskBlockRecurrence(
	ctx context.Context,
	recurrence taskpkg.BlockRecurrence,
) (taskpkg.BlockRecurrence, error) {
	if err := g.checkReady(ctx, "upsert task block recurrence"); err != nil {
		return taskpkg.BlockRecurrence{}, err
	}
	normalized, err := normalizeBlockRecurrence(recurrence, g.now())
	if err != nil {
		return taskpkg.BlockRecurrence{}, err
	}

	var stored taskpkg.BlockRecurrence
	if err := g.withTaskImmediateTransaction(ctx, "upsert task block recurrence", func(exec taskSQLExecutor) error {
		if err := g.ensureTaskExistsWithExecutor(ctx, exec, normalized.TaskID); err != nil {
			return err
		}
		stored, err = upsertBlockRecurrenceWithExecutor(ctx, exec, normalized)
		return err
	}); err != nil {
		return taskpkg.BlockRecurrence{}, err
	}
	return stored, nil
}

// IncrementTaskBlockRecurrence increments and returns the counter for one task and block kind.
func (g *TaskRepo) IncrementTaskBlockRecurrence(
	ctx context.Context,
	taskID string,
	kind taskpkg.BlockKind,
	updatedAt time.Time,
) (taskpkg.BlockRecurrence, error) {
	if err := g.checkReady(ctx, "increment task block recurrence"); err != nil {
		return taskpkg.BlockRecurrence{}, err
	}
	normalizedTaskID, normalizedKind, now, err := normalizeBlockRecurrenceKey(taskID, kind, updatedAt, g.now())
	if err != nil {
		return taskpkg.BlockRecurrence{}, err
	}

	var stored taskpkg.BlockRecurrence
	if err := g.withTaskImmediateTransaction(ctx, "increment task block recurrence", func(exec taskSQLExecutor) error {
		if err := g.ensureTaskExistsWithExecutor(ctx, exec, normalizedTaskID); err != nil {
			return err
		}
		stored, err = incrementBlockRecurrenceWithExecutor(ctx, exec, normalizedTaskID, normalizedKind, now)
		return err
	}); err != nil {
		return taskpkg.BlockRecurrence{}, err
	}
	return stored, nil
}

// GetTaskBlockRecurrence returns the counter for one task and block kind, or a zero counter when absent.
func (g *TaskRepo) GetTaskBlockRecurrence(
	ctx context.Context,
	taskID string,
	kind taskpkg.BlockKind,
) (taskpkg.BlockRecurrence, error) {
	if err := g.checkReady(ctx, "get task block recurrence"); err != nil {
		return taskpkg.BlockRecurrence{}, err
	}
	normalizedTaskID, normalizedKind, _, err := normalizeBlockRecurrenceKey(taskID, kind, time.Time{}, g.now())
	if err != nil {
		return taskpkg.BlockRecurrence{}, err
	}
	if err := g.ensureTaskExistsWithExecutor(ctx, g.db, normalizedTaskID); err != nil {
		return taskpkg.BlockRecurrence{}, err
	}
	return getBlockRecurrenceWithExecutor(ctx, g.db, normalizedTaskID, normalizedKind)
}

// ResetTaskBlockRecurrences clears all breaker counters for one task.
func (g *TaskRepo) ResetTaskBlockRecurrences(ctx context.Context, taskID string) error {
	if err := g.checkReady(ctx, "reset task block recurrences"); err != nil {
		return err
	}
	trimmedTaskID, err := requireTaskValue(taskID, "task id")
	if err != nil {
		return err
	}
	return g.withTaskImmediateTransaction(ctx, "reset task block recurrences", func(exec taskSQLExecutor) error {
		if err := g.ensureTaskExistsWithExecutor(ctx, exec, trimmedTaskID); err != nil {
			return err
		}
		return resetTaskBlockRecurrencesWithExecutor(ctx, exec, trimmedTaskID)
	})
}

// MarkTaskNeedsAttention writes the task-level escalation metadata columns.
func (g *TaskRepo) MarkTaskNeedsAttention(
	ctx context.Context,
	mutation taskpkg.NeedsAttentionMutation,
) (taskpkg.Task, error) {
	if err := g.checkReady(ctx, "mark task needs attention"); err != nil {
		return taskpkg.Task{}, err
	}
	normalized, err := normalizeNeedsAttentionMutation(mutation, g.now())
	if err != nil {
		return taskpkg.Task{}, err
	}
	return g.updateTaskNeedsAttention(ctx, normalized)
}

// SetTaskWakeCreator writes the per-task creator wake opt-in flag.
func (g *TaskRepo) SetTaskWakeCreator(
	ctx context.Context,
	mutation taskpkg.WakeCreatorMutation,
) (taskpkg.Task, error) {
	if err := g.checkReady(ctx, "set task wake creator"); err != nil {
		return taskpkg.Task{}, err
	}
	trimmedTaskID, err := requireTaskValue(mutation.TaskID, "task id")
	if err != nil {
		return taskpkg.Task{}, err
	}
	updatedAt := mutation.UpdatedAt.UTC()
	if updatedAt.IsZero() {
		updatedAt = g.now().UTC()
	}

	var updated taskpkg.Task
	if err := g.withTaskImmediateTransaction(ctx, "set task wake creator", func(exec taskSQLExecutor) error {
		affected, err := sqlcgen.New(exec).SetTaskWakeCreator(ctx, sqlcgen.SetTaskWakeCreatorParams{
			WakeCreator: int64(taskBoolToInt(mutation.WakeCreator)),
			UpdatedAt:   store.FormatTimestamp(updatedAt),
			ID:          trimmedTaskID,
		})
		if err != nil {
			return fmt.Errorf("store: set task wake creator %q: %w", trimmedTaskID, err)
		}
		if affected == 0 {
			return fmt.Errorf("store: task %q: %w", trimmedTaskID, taskpkg.ErrTaskNotFound)
		}
		updated, err = g.getTaskWithExecutor(ctx, exec, trimmedTaskID)
		return err
	}); err != nil {
		return taskpkg.Task{}, err
	}
	return updated, nil
}

// BlockTaskAndReleaseRun inserts a block, evaluates breaker accounting, and releases the active run atomically.
func (g *TaskRepo) BlockTaskAndReleaseRun(
	ctx context.Context,
	mutation taskpkg.BlockTaskAndReleaseRunMutation,
) (taskpkg.BlockTaskAndReleaseRunResult, error) {
	if err := g.checkReady(ctx, "block task and release run"); err != nil {
		return taskpkg.BlockTaskAndReleaseRunResult{}, err
	}
	normalized, err := normalizeBlockTaskAndReleaseRunMutation(mutation, g.now())
	if err != nil {
		return taskpkg.BlockTaskAndReleaseRunResult{}, err
	}

	var result taskpkg.BlockTaskAndReleaseRunResult
	if err := g.withTaskImmediateTransaction(ctx, "block task and release run", func(exec taskSQLExecutor) error {
		current, err := g.getTaskRunWithExecutor(ctx, exec, normalized.RunID)
		if err != nil {
			return err
		}
		if err := requireCurrentRunLease(current, normalized.ClaimToken, normalized.Now); err != nil {
			return err
		}
		if strings.TrimSpace(current.TaskID) != strings.TrimSpace(normalized.Block.TaskID) {
			return fmt.Errorf(
				"%w: task run %q belongs to task %q, not %q",
				taskpkg.ErrInvalidStatusTransition,
				current.ID,
				current.TaskID,
				normalized.Block.TaskID,
			)
		}

		blockResult, err := g.insertTaskBlockWithBreaker(
			ctx,
			exec,
			normalized.Block,
			normalized.RecurrenceLimit,
			normalized.Actor,
		)
		if err != nil {
			return err
		}
		if err := requeueLeasedRun(ctx, exec, current.ID); err != nil {
			return err
		}
		if err := clearTaskCurrentRunProjection(ctx, exec, current.TaskID, current.ID); err != nil {
			return err
		}
		updatedRun, err := g.getTaskRunWithExecutor(ctx, exec, current.ID)
		if err != nil {
			return err
		}
		result = taskpkg.BlockTaskAndReleaseRunResult{
			Block:          blockResult.Block,
			Run:            updatedRun,
			Recurrence:     blockResult.Recurrence,
			EscalatedTask:  blockResult.EscalatedTask,
			ReleaseReason:  taskRunReleaseReasonBlocked,
			PreviousRun:    current,
			ClaimTokenHash: current.ClaimTokenHash,
		}
		if err := appendTaskBlockedWatchEvent(
			ctx,
			exec,
			result.Block,
			normalized.Actor,
			updatedRun.ID,
			current.ClaimTokenHash,
		); err != nil {
			return err
		}
		return appendNeedsAttentionWatchEventIfEscalated(ctx, exec, blockResult, normalized.Actor)
	}); err != nil {
		return taskpkg.BlockTaskAndReleaseRunResult{}, err
	}
	return result, nil
}
