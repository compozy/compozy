package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

type normalizedNeedsAttentionClear struct {
	taskID    string
	note      string
	clearedAt time.Time
	actor     taskpkg.ActorContext
}

// ClearTaskNeedsAttention clears escalation metadata and returns its exact
// co-committed recovery event.
func (g *TaskRepo) ClearTaskNeedsAttention(
	ctx context.Context,
	mutation taskpkg.NeedsAttentionClearMutation,
) (taskpkg.NeedsAttentionClearResult, error) {
	if err := g.checkReady(ctx, "clear task needs attention"); err != nil {
		return taskpkg.NeedsAttentionClearResult{}, err
	}
	normalized, err := normalizeNeedsAttentionClear(mutation, g.now())
	if err != nil {
		return taskpkg.NeedsAttentionClearResult{}, err
	}

	var cleared taskpkg.NeedsAttentionClearResult
	if err := g.withTaskImmediateTransaction(ctx, "clear task needs attention", func(exec taskSQLExecutor) error {
		affected, err := sqlcgen.New(exec).ClearTaskNeedsAttention(ctx, sqlcgen.ClearTaskNeedsAttentionParams{
			UpdatedAt: store.FormatTimestamp(normalized.clearedAt),
			ID:        normalized.taskID,
		})
		if err != nil {
			return fmt.Errorf("store: clear task needs attention %q: %w", normalized.taskID, err)
		}
		if affected == 0 {
			if _, loadErr := g.getTaskWithExecutor(ctx, exec, normalized.taskID); loadErr != nil {
				return loadErr
			}
			return fmt.Errorf(
				"%w: task %q is not escalated",
				taskpkg.ErrInvalidStatusTransition,
				normalized.taskID,
			)
		}

		event, err := appendTaskEventPayloadRecordWithExecutor(
			ctx,
			exec,
			normalized.taskID,
			"",
			string(hookspkg.HookTaskRecovered),
			normalized.actor,
			normalized.clearedAt,
			taskRecoveredWatchEventPayload{
				Status: taskpkg.TaskStatusReady,
				Note:   normalized.note,
				At:     normalized.clearedAt,
			},
		)
		if err != nil {
			return err
		}
		updated, err := g.getTaskWithExecutor(ctx, exec, normalized.taskID)
		if err != nil {
			return err
		}
		cleared = taskpkg.NeedsAttentionClearResult{Task: updated, Event: event}
		return nil
	}); err != nil {
		return taskpkg.NeedsAttentionClearResult{}, err
	}
	return cleared, nil
}

func normalizeNeedsAttentionClear(
	mutation taskpkg.NeedsAttentionClearMutation,
	now time.Time,
) (normalizedNeedsAttentionClear, error) {
	taskID, err := requireTaskValue(mutation.TaskID, "task id")
	if err != nil {
		return normalizedNeedsAttentionClear{}, err
	}
	clearedAt := mutation.ClearedAt.UTC()
	if clearedAt.IsZero() {
		clearedAt = now.UTC()
	}
	actor := mutation.Actor
	actor.Actor.Kind = actor.Actor.Kind.Normalize()
	actor.Actor.Ref = strings.TrimSpace(actor.Actor.Ref)
	actor.Origin.Kind = actor.Origin.Kind.Normalize()
	actor.Origin.Ref = strings.TrimSpace(actor.Origin.Ref)
	actor.Scope.SessionID = strings.TrimSpace(actor.Scope.SessionID)
	actor.Scope.WorkspaceID = strings.TrimSpace(actor.Scope.WorkspaceID)
	if err := actor.Validate(); err != nil {
		return normalizedNeedsAttentionClear{}, err
	}
	if !actor.Authority.Write {
		return normalizedNeedsAttentionClear{}, fmt.Errorf(
			"%w: task needs-attention clear requires write authority",
			taskpkg.ErrPermissionDenied,
		)
	}
	return normalizedNeedsAttentionClear{
		taskID:    taskID,
		note:      strings.TrimSpace(mutation.Note),
		clearedAt: clearedAt,
		actor:     actor,
	}, nil
}
