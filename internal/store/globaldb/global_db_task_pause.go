package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

// PauseTask marks one task as paused for future claim eligibility.
func (g *TaskRepo) PauseTask(ctx context.Context, mutation taskpkg.PauseMutation) (taskpkg.Task, error) {
	if err := g.checkReady(ctx, "pause task"); err != nil {
		return taskpkg.Task{}, err
	}
	taskID, err := requireTaskValue(mutation.TaskID, "task id")
	if err != nil {
		return taskpkg.Task{}, err
	}
	actor := strings.TrimSpace(mutation.Actor)
	reason := strings.TrimSpace(mutation.Reason)
	if actor == "" {
		return taskpkg.Task{}, fmt.Errorf("%w: pause actor is required", taskpkg.ErrValidation)
	}
	if reason == "" {
		return taskpkg.Task{}, fmt.Errorf("%w: pause reason is required", taskpkg.ErrValidation)
	}
	pausedAt := mutation.PausedAt.UTC()
	if pausedAt.IsZero() {
		pausedAt = g.now().UTC()
	}

	var updated taskpkg.Task
	if err := g.withTaskImmediateTransaction(ctx, "pause task", func(exec taskSQLExecutor) error {
		if _, err := g.getTaskWithExecutor(ctx, exec, taskID); err != nil {
			return err
		}
		affected, err := sqlcgen.New(exec).PauseTask(ctx, sqlcgen.PauseTaskParams{
			PausedBy:     actor,
			PausedAt:     sql.NullString{String: store.FormatTimestamp(pausedAt), Valid: true},
			PausedReason: reason,
			UpdatedAt:    store.FormatTimestamp(pausedAt),
			ID:           taskID,
		})
		if err != nil {
			return fmt.Errorf("store: pause task %q: %w", taskID, err)
		}
		if affected == 0 {
			return taskpkg.ErrTaskNotFound
		}
		record, err := g.getTaskWithExecutor(ctx, exec, taskID)
		if err != nil {
			return err
		}
		updated = record
		return nil
	}); err != nil {
		return taskpkg.Task{}, err
	}
	return updated, nil
}

// ResumeTask clears one task pause for future claim eligibility.
func (g *TaskRepo) ResumeTask(ctx context.Context, mutation taskpkg.ResumeMutation) (taskpkg.Task, error) {
	if err := g.checkReady(ctx, "resume task"); err != nil {
		return taskpkg.Task{}, err
	}
	taskID, err := requireTaskValue(mutation.TaskID, "task id")
	if err != nil {
		return taskpkg.Task{}, err
	}
	resumedAt := mutation.ResumedAt.UTC()
	if resumedAt.IsZero() {
		resumedAt = g.now().UTC()
	}

	var updated taskpkg.Task
	if err := g.withTaskImmediateTransaction(ctx, "resume task", func(exec taskSQLExecutor) error {
		if _, err := g.getTaskWithExecutor(ctx, exec, taskID); err != nil {
			return err
		}
		affected, err := sqlcgen.New(exec).ResumeTask(ctx, sqlcgen.ResumeTaskParams{
			UpdatedAt: store.FormatTimestamp(resumedAt), ID: taskID,
		})
		if err != nil {
			return fmt.Errorf("store: resume task %q: %w", taskID, err)
		}
		if affected == 0 {
			return taskpkg.ErrTaskNotFound
		}
		record, err := g.getTaskWithExecutor(ctx, exec, taskID)
		if err != nil {
			return err
		}
		updated = record
		return nil
	}); err != nil {
		return taskpkg.Task{}, err
	}
	return updated, nil
}

// CountPausedTasks returns the number of directly paused tasks.

func effectiveTaskPauseExclusionSQL() string {
	return `COALESCE(t.paused, 0) = 0
		AND NOT EXISTS (
			WITH RECURSIVE ancestors(id, parent_task_id, paused) AS (
				SELECT parent.id, parent.parent_task_id, parent.paused
				  FROM tasks parent
				 WHERE parent.id = t.parent_task_id
				UNION ALL
				SELECT parent.id, parent.parent_task_id, parent.paused
				  FROM tasks parent
				  JOIN ancestors a ON parent.id = a.parent_task_id
			)
			SELECT 1 FROM ancestors WHERE COALESCE(paused, 0) = 1
		)`
}

// IsTaskEffectivelyPaused reports whether a task or one of its ancestors is paused.
func (g *TaskRepo) IsTaskEffectivelyPaused(ctx context.Context, taskID string) (bool, string, error) {
	if err := g.checkReady(ctx, "get effective task pause"); err != nil {
		return false, "", err
	}
	trimmedID, err := requireTaskValue(taskID, "task id")
	if err != nil {
		return false, "", err
	}
	var pausedByTaskID string
	// dynamic-sql: sqlc's SQLite analyzer cannot resolve this recursive ancestor walk.
	err = g.db.QueryRowContext(
		ctx,
		`WITH RECURSIVE chain(id, parent_task_id, paused, depth) AS (
			SELECT id, parent_task_id, paused, 0
			  FROM tasks
			 WHERE id = ?
			UNION ALL
			SELECT parent.id, parent.parent_task_id, parent.paused, chain.depth + 1
			  FROM tasks parent
			  JOIN chain ON parent.id = chain.parent_task_id
		)
		SELECT id
		  FROM chain
		 WHERE COALESCE(paused, 0) = 1
		 ORDER BY depth ASC
		 LIMIT 1`,
		trimmedID,
	).Scan(&pausedByTaskID)
	if errors.Is(err, sql.ErrNoRows) {
		if existsErr := g.ensureTaskExists(ctx, trimmedID); existsErr != nil {
			return false, "", existsErr
		}
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("store: get effective task pause for %q: %w", trimmedID, err)
	}
	return true, strings.TrimSpace(pausedByTaskID), nil
}
