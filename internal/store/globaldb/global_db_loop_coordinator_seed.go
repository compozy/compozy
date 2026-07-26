package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (g *LoopRepo) reserveLoopCoordinatorRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	origin taskpkg.Origin,
	now time.Time,
	runID string,
	idempotencyKey string,
) (taskpkg.Run, bool, error) {
	taskID, err := g.ensureLoopCoordinatorTaskWithExecutor(ctx, exec, run, now)
	if err != nil {
		return taskpkg.Run{}, false, err
	}
	return g.reserveCoordinatorRun(
		ctx,
		exec,
		taskID,
		string(run.ID),
		strings.TrimSpace(runID),
		strings.TrimSpace(idempotencyKey),
		origin,
		now,
	)
}

func (g *LoopRepo) ensureLoopCoordinatorTaskWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	now time.Time,
) (string, error) {
	taskID := loopCoordinatorTaskID(run.ID)
	if _, err := g.tasks.getTaskWithExecutor(ctx, exec, taskID); err == nil {
		return taskID, nil
	} else if !errorsIsTaskNotFound(err) {
		return "", err
	}
	taskRecord, err := g.tasks.normalizeTaskForCreate(loopCoordinatorTaskRecordForRun(run, taskID, now))
	if err != nil {
		return "", err
	}
	if err := insertTaskWithExecutor(ctx, exec, taskRecord); err != nil {
		return "", fmt.Errorf("store: create loop coordinator task %q: %w", taskRecord.ID, err)
	}
	return taskID, nil
}

func loopCoordinatorTaskRecordForRun(run looppkg.Run, taskID string, now time.Time) taskpkg.Task {
	origin := loopCoordinatorStartOrigin()
	return taskpkg.Task{
		ID:                 taskID,
		Scope:              taskpkg.ScopeWorkspace,
		WorkspaceID:        string(run.WorkspaceID),
		Title:              fmt.Sprintf("Loop coordinator %s", strings.TrimSpace(run.LoopName)),
		Priority:           taskpkg.DefaultPriority,
		Status:             taskpkg.TaskStatusReady,
		ApprovalPolicy:     taskpkg.ApprovalPolicyNone,
		ApprovalState:      taskpkg.ApprovalStateNotRequired,
		AutoEnqueueOnReady: false,
		CreatedBy: taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKindDaemon,
			Ref:  loopCoordinatorActorRef,
		},
		Origin:    origin,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func loopCoordinatorTaskID(loopRunID looppkg.RunID) string {
	return fmt.Sprintf("loop.%s.coordinator", strings.TrimSpace(string(loopRunID)))
}

func loopCoordinatorRunID(loopRunID looppkg.RunID, generation int) string {
	return fmt.Sprintf("run.loop.%s.g%d.coordinator", strings.TrimSpace(string(loopRunID)), generation)
}

func loopCoordinatorIdempotencyKey(loopRunID looppkg.RunID, generation int) string {
	return fmt.Sprintf("loop.coordinator.%s.%d", strings.TrimSpace(string(loopRunID)), generation)
}

func loopCoordinatorStartOrigin() taskpkg.Origin {
	return taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: loopCoordinatorActorRef}
}

func errorsIsTaskNotFound(err error) bool {
	return errors.Is(err, taskpkg.ErrTaskNotFound)
}
