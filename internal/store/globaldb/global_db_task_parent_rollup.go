package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	taskpkg "github.com/compozy/agh/internal/task"
)

const parentRollupResultKind = "children_completed"

type parentRollupRunResult struct {
	Kind string `json:"kind"`
}

func (g *TaskRepo) settleCompletedTaskHierarchyWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	completedTaskID string,
	actor taskpkg.ActorContext,
	settledAt time.Time,
) (taskpkg.CompletedRunSettlement, error) {
	settlement := taskpkg.CompletedRunSettlement{
		StatusTransitions: make([]taskpkg.StatusTransition, 0, 2),
		RolledUpRuns:      make([]taskpkg.Run, 0, 1),
	}

	completedTask, changed, err := g.completeTaskForRollupWithExecutor(
		ctx,
		exec,
		completedTaskID,
		actor,
		settledAt,
	)
	if err != nil {
		return taskpkg.CompletedRunSettlement{}, err
	}
	settlement.Task = completedTask.Task
	if changed {
		settlement.StatusTransitions = append(settlement.StatusTransitions, completedTask)
	}

	rollupChild := completedTask.Task
	parentTaskID := strings.TrimSpace(rollupChild.ParentTaskID)
	for parentTaskID != "" {
		allCompleted, err := g.allDirectChildrenCompletedWithExecutor(ctx, exec, parentTaskID)
		if err != nil {
			return taskpkg.CompletedRunSettlement{}, err
		}
		if !allCompleted {
			break
		}

		parent, transition, rolledUpRuns, eligible, err := g.rollUpCompletedParentWithExecutor(
			ctx,
			exec,
			parentTaskID,
			rollupChild,
			actor,
			settledAt,
		)
		if err != nil {
			return taskpkg.CompletedRunSettlement{}, err
		}
		if !eligible {
			break
		}
		settlement.RolledUpRuns = append(settlement.RolledUpRuns, rolledUpRuns...)
		if transition != nil {
			settlement.StatusTransitions = append(settlement.StatusTransitions, *transition)
		}
		rollupChild = parent
		parentTaskID = strings.TrimSpace(parent.ParentTaskID)
	}

	return settlement, nil
}

func (g *TaskRepo) rollUpCompletedParentWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	parentTaskID string,
	rollupChild taskpkg.Task,
	actor taskpkg.ActorContext,
	settledAt time.Time,
) (taskpkg.Task, *taskpkg.StatusTransition, []taskpkg.Run, bool, error) {
	parent, err := g.getTaskWithExecutor(ctx, exec, parentTaskID)
	if err != nil {
		return taskpkg.Task{}, nil, nil, false, err
	}
	if err := validateParentRollupScope(parent, rollupChild); err != nil {
		return taskpkg.Task{}, nil, nil, false, err
	}
	if parent.Status.Normalize() == taskpkg.TaskStatusFailed ||
		parent.Status.Normalize() == taskpkg.TaskStatusCanceled {
		return parent, nil, nil, false, nil
	}
	if parent.Status.Normalize() == taskpkg.TaskStatusCompleted {
		return parent, nil, nil, true, nil
	}

	rolledUpRuns, eligible, err := g.settleParentRollupRunsWithExecutor(ctx, exec, parent, actor, settledAt)
	if err != nil || !eligible {
		return parent, nil, nil, eligible, err
	}
	transition, changed, err := g.completeTaskForRollupWithExecutor(ctx, exec, parent.ID, actor, settledAt)
	if err != nil {
		return taskpkg.Task{}, nil, nil, false, err
	}
	if !changed {
		return transition.Task, nil, rolledUpRuns, true, nil
	}
	return transition.Task, &transition, rolledUpRuns, true, nil
}

func (g *TaskRepo) completeTaskForRollupWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	actor taskpkg.ActorContext,
	settledAt time.Time,
) (taskpkg.StatusTransition, bool, error) {
	current, err := g.getTaskWithExecutor(ctx, exec, taskID)
	if err != nil {
		return taskpkg.StatusTransition{}, false, err
	}
	if current.Status.Normalize() == taskpkg.TaskStatusCompleted {
		return taskpkg.StatusTransition{Task: current, PreviousStatus: current.Status}, false, nil
	}
	updated := current
	updated.Status = taskpkg.TaskStatusCompleted
	updated.UpdatedAt = settledAt.UTC()
	updated.ClosedAt = settledAt.UTC()
	updated.NeedsAttention = nil
	if err := g.updateTaskWithExecutor(ctx, exec, updated, actor); err != nil {
		return taskpkg.StatusTransition{}, false, err
	}
	return taskpkg.StatusTransition{Task: updated, PreviousStatus: current.Status}, true, nil
}

func validateParentRollupScope(parent taskpkg.Task, child taskpkg.Task) error {
	if parent.Scope.Normalize() != taskpkg.ScopeWorkspace {
		return nil
	}
	if child.Scope.Normalize() != taskpkg.ScopeWorkspace ||
		strings.TrimSpace(child.WorkspaceID) != strings.TrimSpace(parent.WorkspaceID) {
		return fmt.Errorf(
			"%w: workspace parent %q cannot aggregate child %q outside workspace %q",
			taskpkg.ErrInvalidScopeBinding,
			parent.ID,
			child.ID,
			parent.WorkspaceID,
		)
	}
	return nil
}

func (g *TaskRepo) allDirectChildrenCompletedWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	parentTaskID string,
) (bool, error) {
	var childCount int
	var completedCount int
	// dynamic-sql: this transactional aggregate is intentionally local to the
	// parent-rollup boundary; no public query shape or generated contract changes.
	err := exec.QueryRowContext(
		ctx,
		`SELECT COUNT(1), COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0)
		   FROM tasks
		  WHERE parent_task_id = ?`,
		string(taskpkg.TaskStatusCompleted),
		strings.TrimSpace(parentTaskID),
	).Scan(&childCount, &completedCount)
	if err != nil {
		return false, fmt.Errorf("store: aggregate direct children for parent rollup %q: %w", parentTaskID, err)
	}
	return childCount > 0 && completedCount == childCount, nil
}

func (g *TaskRepo) settleParentRollupRunsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	parent taskpkg.Task,
	actor taskpkg.ActorContext,
	settledAt time.Time,
) ([]taskpkg.Run, bool, error) {
	runs, err := g.listTaskRunsWithExecutor(ctx, exec, taskpkg.RunQuery{TaskID: parent.ID})
	if err != nil {
		return nil, false, err
	}
	for _, run := range runs {
		status := run.Status.Normalize()
		if status == taskpkg.TaskRunStatusCompleted ||
			status == taskpkg.TaskRunStatusFailed ||
			status == taskpkg.TaskRunStatusCanceled {
			continue
		}
		if status != taskpkg.TaskRunStatusNeedsAttention {
			return nil, false, nil
		}
	}

	rolledUp := make([]taskpkg.Run, 0, 1)
	for _, run := range runs {
		if run.Status.Normalize() != taskpkg.TaskRunStatusNeedsAttention {
			continue
		}
		updated, err := completedParentRollupRun(run, settledAt)
		if err != nil {
			return nil, false, err
		}
		if err := updateTaskRunRecordWithSnapshotCAS(ctx, exec, run, updated); err != nil {
			return nil, false, err
		}
		if err := updateTaskCurrentRunProjectionForRunUpdate(ctx, exec, run, updated); err != nil {
			return nil, false, err
		}
		if err := appendTaskEventPayloadWithExecutor(
			ctx,
			exec,
			parent.ID,
			updated.ID,
			string(hookspkg.HookTaskRunCompleted),
			actor,
			settledAt,
			taskRunCompletedWatchEventPayload{
				Status: taskpkg.TaskRunStatusCompleted,
				Result: updated.Result,
			},
		); err != nil {
			return nil, false, err
		}
		rolledUp = append(rolledUp, updated)
	}
	return rolledUp, true, nil
}

func completedParentRollupRun(run taskpkg.Run, settledAt time.Time) (taskpkg.Run, error) {
	result, err := json.Marshal(parentRollupRunResult{Kind: parentRollupResultKind})
	if err != nil {
		return taskpkg.Run{}, fmt.Errorf("store: marshal parent rollup run result: %w", err)
	}
	updated := run
	updated.Status = taskpkg.TaskRunStatusCompleted
	updated.Result = result
	updated.Error = ""
	updated.EndedAt = settledAt.UTC()
	updated.LeaseUntil = time.Time{}
	updated.HeartbeatAt = time.Time{}
	return updated, nil
}
