package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

func applyCoordinatorRunStopsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	specs []taskpkg.CoordinatorStopSpec,
	now time.Time,
) error {
	stoppedAny := false
	for _, spec := range specs {
		normalized := spec.Normalize()
		child, err := getLoopRunByIDWithExecutor(ctx, exec, loop.RunID(normalized.LoopRunID))
		if err != nil {
			return err
		}
		if child.Status.Terminal() {
			continue
		}
		if err := updateLoopBoundaryStatusWithExecutor(
			ctx,
			exec,
			child,
			loop.StatusFailed,
			loop.TransitionCauseOperatorStop,
			now,
			child.Generation,
		); err != nil {
			return err
		}
		stoppedAny = true
	}
	if stoppedAny {
		if err := sweepOrphanedLoopOutputBlobsWithExecutor(ctx, exec); err != nil {
			return err
		}
	}
	return nil
}

func (g *TaskRepo) ensureCoordinatorPlanTasksWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	plan taskpkg.CoordinatorCompletionPlan,
	current taskpkg.Run,
	origin taskpkg.Origin,
	now time.Time,
) error {
	if len(plan.NodeTasks) == 0 && len(plan.Dependencies) == 0 {
		return nil
	}
	parentTask, err := g.getTaskWithExecutor(ctx, exec, current.TaskID)
	if err != nil {
		return err
	}
	for _, spec := range plan.NodeTasks {
		if err := g.createCoordinatorTaskIfMissingWithExecutor(ctx, exec, spec, parentTask, origin, now); err != nil {
			return err
		}
	}
	for _, spec := range plan.Dependencies {
		if err := g.createCoordinatorDependencyIfMissingWithExecutor(ctx, exec, spec, now); err != nil {
			return err
		}
	}
	return nil
}

func (g *TaskRepo) createCoordinatorTaskIfMissingWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	spec taskpkg.CoordinatorTaskSpec,
	parent taskpkg.Task,
	origin taskpkg.Origin,
	now time.Time,
) error {
	normalized := spec.Normalize()
	if _, err := g.getTaskWithExecutor(ctx, exec, normalized.TaskID); err == nil {
		return nil
	} else if !errors.Is(err, taskpkg.ErrTaskNotFound) {
		return err
	}
	taskRecord, err := g.normalizeTaskForCreate(
		coordinatorTaskRecord(normalized, parent, origin, now),
	)
	if err != nil {
		return err
	}
	if err := g.ensureCoordinatorTaskCreateReferencesWithExecutor(ctx, exec, taskRecord); err != nil {
		return err
	}
	if err := insertTaskWithExecutor(ctx, exec, taskRecord); err != nil {
		return fmt.Errorf("store: create coordinator node task %q: %w", taskRecord.ID, err)
	}
	return nil
}

func coordinatorTaskRecord(
	spec taskpkg.CoordinatorTaskSpec,
	parent taskpkg.Task,
	origin taskpkg.Origin,
	now time.Time,
) taskpkg.Task {
	return taskpkg.Task{
		ID:                 spec.TaskID,
		Scope:              parent.Scope,
		WorkspaceID:        parent.WorkspaceID,
		ParentTaskID:       parent.ID,
		Title:              spec.Title,
		Description:        spec.Description,
		Priority:           parent.Priority,
		MaxAttempts:        parent.MaxAttempts,
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
		Metadata:  spec.Metadata,
	}
}

func (g *TaskRepo) ensureCoordinatorTaskCreateReferencesWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	record taskpkg.Task,
) error {
	if err := taskpkg.ValidateScopeBinding(record.Scope, record.WorkspaceID, "task", "workspace_id"); err != nil {
		return err
	}
	if record.Scope == taskpkg.ScopeWorkspace {
		if err := g.ensureWorkspaceExistsWithExecutor(ctx, exec, record.WorkspaceID); err != nil {
			return err
		}
	}
	if strings.TrimSpace(record.ParentTaskID) != "" {
		if err := g.ensureTaskExistsWithExecutor(ctx, exec, record.ParentTaskID); err != nil {
			return err
		}
	}
	return nil
}

func (g *TaskRepo) ensureWorkspaceExistsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID string,
) error {
	trimmedID := strings.TrimSpace(workspaceID)
	if trimmedID == "" {
		return nil
	}
	if _, err := sqlcgen.New(exec).GetWorkspaceID(ctx, trimmedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return aghworkspace.ErrWorkspaceNotFound
		}
		return fmt.Errorf("store: check workspace %q exists: %w", trimmedID, err)
	}
	return nil
}

func (g *TaskRepo) createCoordinatorDependencyIfMissingWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	spec taskpkg.CoordinatorDependencySpec,
	now time.Time,
) error {
	normalizedSpec := spec.Normalize()
	dependency, err := g.normalizeTaskDependencyForCreate(taskpkg.Dependency{
		TaskID:          normalizedSpec.TaskID,
		DependsOnTaskID: normalizedSpec.DependsOnTaskID,
		Kind:            normalizedSpec.Kind,
		CreatedAt:       now,
	})
	if err != nil {
		return err
	}
	if err := g.ensureTaskExistsWithExecutor(ctx, exec, dependency.TaskID); err != nil {
		return err
	}
	if err := g.ensureTaskExistsWithExecutor(ctx, exec, dependency.DependsOnTaskID); err != nil {
		return err
	}
	exists, err := g.taskDependencyExists(ctx, exec, dependency)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	count, err := g.countDependenciesWithExecutor(ctx, exec, dependency.TaskID)
	if err != nil {
		return err
	}
	if err := taskpkg.ValidateDependencyCount(count + 1); err != nil {
		return err
	}
	hasPath, err := g.hasDependencyPathWithExecutor(
		ctx,
		exec,
		dependency.DependsOnTaskID,
		dependency.TaskID,
	)
	if err != nil {
		return err
	}
	if hasPath {
		return fmt.Errorf(
			"%w: adding dependency %q -> %q would create a cycle",
			taskpkg.ErrCycleDetected,
			dependency.TaskID,
			dependency.DependsOnTaskID,
		)
	}
	if err := sqlcgen.New(exec).InsertTaskDependency(ctx, sqlcgen.InsertTaskDependencyParams{
		TaskID: dependency.TaskID, DependsOnTaskID: dependency.DependsOnTaskID,
		Kind: string(dependency.Kind), CreatedAt: store.FormatTimestamp(dependency.CreatedAt),
	}); err != nil {
		return fmt.Errorf(
			"store: create coordinator dependency %q -> %q: %w",
			dependency.TaskID,
			dependency.DependsOnTaskID,
			err,
		)
	}
	return nil
}

func completeCoordinatorRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	now time.Time,
) error {
	resultJSON, err := json.Marshal(
		map[string]string{taskRunResultKindKey: taskpkg.RunKindCoordinator.String()},
	)
	if err != nil {
		return fmt.Errorf("store: marshal coordinator result: %w", err)
	}
	affected, err := sqlcgen.New(exec).CompleteCoordinatorRun(ctx, sqlcgen.CompleteCoordinatorRunParams{
		Status:         taskpkg.TaskRunStatusCompleted.String(),
		EndedAt:        nullableTaskTime(now),
		ResultJson:     nullableTaskRawJSON(resultJSON),
		ID:             run.ID,
		ClaimTokenHash: nullableTaskString(run.ClaimTokenHash),
	})
	if err != nil {
		return fmt.Errorf("store: complete coordinator run %q: %w", run.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: coordinator run lease %q: %w", run.ID, taskpkg.ErrTaskRunNotFound)
	}
	return nil
}

func refreshLoopTokensUsedWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	loopRunID string,
) (int64, error) {
	tokensUsed, err := sqlcgen.New(exec).SumLoopRunTaskTokens(ctx, nullableTaskString(loopRunID))
	if err != nil {
		return 0, fmt.Errorf("store: sum loop run %q task tokens: %w", loopRunID, err)
	}
	affected, err := sqlcgen.New(exec).UpdateLoopRunTokens(ctx, sqlcgen.UpdateLoopRunTokensParams{
		TokensUsed: tokensUsed, ID: loopRunID,
	})
	if err != nil {
		return 0, fmt.Errorf("store: refresh loop run %q tokens_used: %w", loopRunID, err)
	}
	if affected == 0 {
		return 0, fmt.Errorf("store: loop run %q: %w", loopRunID, loop.ErrRunNotFound)
	}
	return tokensUsed, nil
}
