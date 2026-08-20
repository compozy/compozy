package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (g *TaskRepo) attachTerminalCoordinatorSettlementWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	completion taskpkg.CoordinatorCompletion,
	result *taskpkg.CoordinatorCompletionResult,
	updated taskpkg.Run,
) (taskpkg.CoordinatorCompletionResult, error) {
	if result.Terminal && len(result.EnqueuedRuns) == 0 {
		canceled, err := g.cancelLiveLoopTaskRunsWithExecutor(
			ctx,
			exec,
			result.LoopRunID,
			completion.Actor,
			completion.Now,
		)
		if err != nil {
			return taskpkg.CoordinatorCompletionResult{}, err
		}
		descendants, err := g.cancelOpenLoopTaskDescendantsWithExecutor(
			ctx,
			exec,
			updated.TaskID,
			completion.Actor,
			completion.Now,
		)
		if err != nil {
			return taskpkg.CoordinatorCompletionResult{}, err
		}
		settlement, err := g.settleCompletedTaskHierarchyWithExecutor(
			ctx,
			exec,
			updated.TaskID,
			completion.Actor,
			completion.Now,
		)
		if err != nil {
			return taskpkg.CoordinatorCompletionResult{}, err
		}
		terminalTransitions := make([]taskpkg.StatusTransition, 0, len(canceled)+len(descendants))
		terminalTransitions = append(terminalTransitions, canceled...)
		terminalTransitions = append(terminalTransitions, descendants...)
		settlement.StatusTransitions = append(terminalTransitions, settlement.StatusTransitions...)
		settlement.Run = updated
		result.Settlement = &settlement
		return *result, nil
	}

	currentTask, err := g.getTaskWithExecutor(ctx, exec, updated.TaskID)
	if err != nil {
		return taskpkg.CoordinatorCompletionResult{}, err
	}
	settlement := taskpkg.CompletedRunSettlement{
		Run:  updated,
		Task: currentTask,
	}
	result.Settlement = &settlement
	return *result, nil
}

func (g *TaskRepo) cancelLiveLoopTaskRunsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	loopRunID string,
	actor taskpkg.ActorContext,
	terminalAt time.Time,
) ([]taskpkg.StatusTransition, error) {
	ids, err := liveLoopTaskRunIDs(ctx, exec, loopRunID)
	if err != nil {
		return nil, err
	}
	transitions := make([]taskpkg.StatusTransition, 0, len(ids))
	seenTasks := map[string]struct{}{}
	for _, id := range ids {
		current, err := g.getTaskRunWithExecutor(ctx, exec, id)
		if err != nil {
			return nil, err
		}
		if err := g.terminalizeLoopTaskRunWithExecutor(ctx, exec, current, terminalAt); err != nil {
			return nil, err
		}
		taskID := strings.TrimSpace(current.TaskID)
		if taskID == "" {
			continue
		}
		if _, seen := seenTasks[taskID]; seen {
			continue
		}
		seenTasks[taskID] = struct{}{}
		transition, changed, err := g.cancelLiveLoopTaskWithExecutor(ctx, exec, taskID, actor, terminalAt)
		if err != nil {
			return nil, err
		}
		if changed {
			transitions = append(transitions, transition)
		}
	}
	return transitions, nil
}

func liveLoopTaskRunIDs(ctx context.Context, exec taskSQLExecutor, loopRunID string) ([]string, error) {
	ids, err := sqlcgen.New(exec).ListLiveLoopTaskRunIDs(ctx, sqlcgen.ListLiveLoopTaskRunIDsParams{
		LoopRunID:            sql.NullString{String: strings.TrimSpace(loopRunID), Valid: true},
		QueuedStatus:         taskpkg.TaskRunStatusQueued.String(),
		ClaimedStatus:        taskpkg.TaskRunStatusClaimed.String(),
		StartingStatus:       taskpkg.TaskRunStatusStarting.String(),
		RunningStatus:        taskpkg.TaskRunStatusRunning.String(),
		NeedsAttentionStatus: taskpkg.TaskRunStatusNeedsAttention.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list live Loop task runs: %w", err)
	}
	return ids, nil
}

func (g *TaskRepo) terminalizeLoopTaskRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	current taskpkg.Run,
	terminalAt time.Time,
) error {
	const terminalReason = "Loop run reached a terminal state"
	if current.Status.Normalize() == taskpkg.TaskRunStatusNeedsAttention {
		updated, err := failNeedsAttentionTaskRunForRecoveryWithExecutor(
			ctx,
			exec,
			current,
			taskpkg.NewRunMutationFence(current),
			terminalReason,
			terminalAt,
		)
		if err != nil {
			return err
		}
		return updateTaskCurrentRunProjectionForRunUpdate(ctx, exec, current, updated)
	}
	next := current
	next.Status = taskpkg.TaskRunStatusCanceled
	next.EndedAt = terminalAt.UTC()
	next.Error = terminalReason
	_, err := g.transitionTerminalRunWithExecutor(
		ctx,
		exec,
		taskpkg.NewTerminalRunMutation(current, next),
	)
	return err
}

func (g *TaskRepo) cancelLiveLoopTaskWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	actor taskpkg.ActorContext,
	terminalAt time.Time,
) (taskpkg.StatusTransition, bool, error) {
	current, err := g.getTaskWithExecutor(ctx, exec, taskID)
	if err != nil {
		return taskpkg.StatusTransition{}, false, err
	}
	if current.Status.Normalize() == taskpkg.TaskStatusCompleted ||
		current.Status.Normalize() == taskpkg.TaskStatusFailed ||
		current.Status.Normalize() == taskpkg.TaskStatusCanceled {
		return taskpkg.StatusTransition{}, false, nil
	}
	updated := current
	updated.Status = taskpkg.TaskStatusCanceled
	updated.UpdatedAt = terminalAt.UTC()
	updated.ClosedAt = terminalAt.UTC()
	updated.NeedsAttention = nil
	if err := g.updateTaskWithExecutor(ctx, exec, updated, actor); err != nil {
		return taskpkg.StatusTransition{}, false, err
	}
	return taskpkg.StatusTransition{Task: updated, PreviousStatus: current.Status}, true, nil
}

func (g *TaskRepo) cancelOpenLoopTaskDescendantsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	coordinatorTaskID string,
	actor taskpkg.ActorContext,
	terminalAt time.Time,
) ([]taskpkg.StatusTransition, error) {
	ids, err := openTaskDescendantIDs(ctx, exec, coordinatorTaskID)
	if err != nil {
		return nil, err
	}
	transitions := make([]taskpkg.StatusTransition, 0, len(ids))
	for _, id := range ids {
		transition, changed, err := g.cancelLiveLoopTaskWithExecutor(ctx, exec, id, actor, terminalAt)
		if err != nil {
			return nil, err
		}
		if changed {
			transitions = append(transitions, transition)
		}
	}
	return transitions, nil
}

func openTaskDescendantIDs(
	ctx context.Context,
	exec taskSQLExecutor,
	rootTaskID string,
) ([]string, error) {
	ids, err := sqlcgen.New(exec).ListOpenTaskDescendantIDs(ctx, sqlcgen.ListOpenTaskDescendantIDsParams{
		RootTaskID:      sql.NullString{String: strings.TrimSpace(rootTaskID), Valid: true},
		CompletedStatus: string(taskpkg.TaskStatusCompleted),
		FailedStatus:    string(taskpkg.TaskStatusFailed),
		CanceledStatus:  string(taskpkg.TaskStatusCanceled),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list open Loop task descendants: %w", err)
	}
	return ids, nil
}
