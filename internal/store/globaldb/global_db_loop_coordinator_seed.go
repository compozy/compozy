package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
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

func (g *LoopRepo) reserveOrReuseOpenLoopCoordinatorRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	origin taskpkg.Origin,
	now time.Time,
	idempotencyKey string,
) (taskpkg.Run, error) {
	coordinator, _, err := g.reserveLoopCoordinatorRunWithExecutor(
		ctx,
		exec,
		run,
		origin,
		now,
		"",
		idempotencyKey,
	)
	if err == nil {
		return coordinator, nil
	}
	if !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
		return taskpkg.Run{}, err
	}

	taskID := loopCoordinatorTaskID(run.ID)
	openRunID, lookupErr := g.tasks.findOpenRunIDForQueuedRunReservation(ctx, exec, taskID, "")
	if lookupErr != nil {
		return taskpkg.Run{}, lookupErr
	}
	if openRunID == "" {
		return taskpkg.Run{}, err
	}
	openRun, lookupErr := g.tasks.getTaskRunWithExecutor(ctx, exec, openRunID)
	if lookupErr != nil {
		return taskpkg.Run{}, lookupErr
	}
	if openRun.TaskID != taskID ||
		openRun.RunKind.Normalize() != taskpkg.RunKindCoordinator ||
		openRun.LoopRunID != string(run.ID) ||
		openRun.WorkspaceID != string(run.WorkspaceID) ||
		taskpkg.IsTerminalRunStatus(openRun.Status) {
		return taskpkg.Run{}, fmt.Errorf(
			"%w: open run %q is not the active coordinator for loop %q",
			taskpkg.ErrInvalidStatusTransition,
			openRun.ID,
			run.ID,
		)
	}
	return openRun, nil
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
	taskRecord, err := loopCoordinatorTaskRecordForRun(run, taskID, now)
	if err != nil {
		return "", err
	}
	taskRecord, err = g.tasks.normalizeTaskForCreate(taskRecord)
	if err != nil {
		return "", err
	}
	if err := insertTaskWithExecutor(ctx, exec, taskRecord); err != nil {
		return "", fmt.Errorf("store: create loop coordinator task %q: %w", taskRecord.ID, err)
	}
	return taskID, nil
}

func (g *LoopRepo) repairLoopCoordinatorTaskWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	taskID string,
	now time.Time,
) error {
	expectedTaskID := loopCoordinatorTaskID(run.ID)
	if strings.TrimSpace(taskID) != expectedTaskID {
		return fmt.Errorf(
			"%w: coordinator task %q does not own loop run %q",
			taskpkg.ErrValidation,
			taskID,
			run.ID,
		)
	}
	taskRecord, err := g.tasks.getTaskWithExecutor(ctx, exec, expectedTaskID)
	if err != nil {
		return err
	}
	if taskRecord.Status.Normalize() != taskpkg.TaskStatusCanceled || run.Status.Terminal() {
		return nil
	}
	taskRecord.Status = taskpkg.TaskStatusInProgress
	taskRecord.UpdatedAt = now.UTC()
	taskRecord.ClosedAt = time.Time{}
	actor := taskpkg.ActorContext{
		Actor:     taskRecord.CreatedBy,
		Origin:    taskRecord.Origin,
		Authority: taskpkg.FullAccessAuthority(),
	}
	if err := g.tasks.updateTaskWithExecutor(ctx, exec, taskRecord, actor); err != nil {
		return fmt.Errorf("store: reactivate loop coordinator task %q: %w", expectedTaskID, err)
	}
	return nil
}

func loopCoordinatorTaskRecordForRun(run looppkg.Run, taskID string, now time.Time) (taskpkg.Task, error) {
	origin := loopCoordinatorStartOrigin()
	metadata, err := json.Marshal(map[string]string{
		columnLoopRunID: string(run.ID), columnLoopName: run.LoopName,
		watchEventsContentWorkspaceIDKey: string(run.WorkspaceID),
	})
	if err != nil {
		return taskpkg.Task{}, fmt.Errorf("store: marshal Loop coordinator metadata: %w", err)
	}
	return taskpkg.Task{
		ID:                 taskID,
		ProfileID:          run.ProfileID,
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
		Metadata:  metadata,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
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

func loopCoordinatorRecoveryIdempotencyKey(
	loopRunID looppkg.RunID,
	generation int,
	previousRunID string,
) string {
	return fmt.Sprintf(
		"loop.coordinator.recovery.%s.%d.%s",
		strings.TrimSpace(string(loopRunID)),
		generation,
		strings.TrimSpace(previousRunID),
	)
}

func loopCoordinatorStartOrigin() taskpkg.Origin {
	return taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: loopCoordinatorActorRef}
}

func errorsIsTaskNotFound(err error) bool {
	return errors.Is(err, taskpkg.ErrTaskNotFound)
}
