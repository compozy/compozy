package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

// SetLoopRunPauseRequested updates the durable operator pause intent and its authenticated actor.
func (g *LoopRepo) SetLoopRunPauseRequested(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	runID looppkg.RunID,
	requested bool,
	actor taskpkg.ActorContext,
) error {
	if err := g.checkReady(ctx, "set loop run pause requested"); err != nil {
		return err
	}
	if err := actor.Validate(); err != nil {
		return fmt.Errorf("%w: pause actor: %w", looppkg.ErrValidation, err)
	}
	workspaceID := strings.TrimSpace(string(ws))
	trimmedRunID := strings.TrimSpace(string(runID))
	if workspaceID == "" || trimmedRunID == "" {
		return fmt.Errorf("%w: workspace_id and run_id are required", looppkg.ErrValidation)
	}
	now := g.now()
	return g.withTaskImmediateTransaction(ctx, "set loop run pause requested", func(exec taskSQLExecutor) error {
		changed, err := setLoopRunPauseState(
			ctx,
			exec,
			workspaceID,
			trimmedRunID,
			requested,
			actor,
			now,
		)
		if err != nil || !changed {
			return err
		}
		if err := sqlcgen.New(exec).ProjectLoopPauseToGoalCheckpoints(
			ctx,
			sqlcgen.ProjectLoopPauseToGoalCheckpointsParams{
				Requested: boolToInt(requested), ActorKind: nullString(string(actor.Actor.Kind.Normalize())),
				ActorID: nullString(strings.TrimSpace(actor.Actor.Ref)), RequestedAt: nullableLoopTime(now),
				UpdatedAt: now, LoopRunID: trimmedRunID,
			},
		); err != nil {
			return fmt.Errorf("store: project loop pause intent to Goal checkpoints: %w", err)
		}
		return nil
	})
}

func setLoopRunPauseState(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID string,
	runID string,
	requested bool,
	actor taskpkg.ActorContext,
	now time.Time,
) (bool, error) {
	expectedPause := 1
	if requested {
		expectedPause = 0
	}
	rows, err := sqlcgen.New(exec).SetLoopRunPauseState(ctx, sqlcgen.SetLoopRunPauseStateParams{
		Requested: int64(boolToInt(requested)), ActorKind: nullString(string(actor.Actor.Kind.Normalize())),
		ActorID: nullString(strings.TrimSpace(actor.Actor.Ref)), RequestedAt: nullableLoopTime(now),
		WorkspaceID: workspaceID, ID: runID, ExpectedPause: int64(expectedPause),
	})
	if err != nil {
		return false, fmt.Errorf("store: set loop run %q pause_requested: %w", runID, err)
	}
	if rows == 1 {
		return true, nil
	}
	return equivalentLoopPauseState(ctx, exec, workspaceID, runID, requested, actor)
}

func equivalentLoopPauseState(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID string,
	runID string,
	requested bool,
	actor taskpkg.ActorContext,
) (bool, error) {
	row, err := sqlcgen.New(exec).GetLoopRunPauseState(ctx, sqlcgen.GetLoopRunPauseStateParams{
		WorkspaceID: workspaceID, ID: runID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("%w: %s", looppkg.ErrRunNotFound, runID)
		}
		return false, fmt.Errorf("store: load loop run %q pause state: %w", runID, err)
	}
	if row.Status != string(looppkg.StatusRunning) {
		return false, fmt.Errorf(
			"%w: loop run %q status changed to %q",
			looppkg.ErrTransitionConflict,
			runID,
			row.Status,
		)
	}
	if !requested && row.PauseRequested == 0 {
		return false, nil
	}
	if requested && row.PauseRequested == 1 && row.ControlActorKind.Valid && row.ControlActorID.Valid &&
		strings.TrimSpace(row.ControlActorKind.String) == string(actor.Actor.Kind.Normalize()) &&
		strings.TrimSpace(row.ControlActorID.String) == strings.TrimSpace(actor.Actor.Ref) {
		return false, nil
	}
	return false, fmt.Errorf("%w: loop run %q pause writer changed", looppkg.ErrTransitionConflict, runID)
}
