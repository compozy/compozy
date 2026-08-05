package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func updateLoopBoundaryStatusWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	current loop.Run,
	to loop.Status,
	cause loop.TransitionCause,
	at time.Time,
	generation int,
) error {
	return updateLoopBoundaryStatusWithEffects(
		ctx,
		exec,
		current,
		to,
		cause,
		at,
		generation,
		nil,
		nil,
	)
}

func updateLoopBoundaryStatusWithFailure(
	ctx context.Context,
	exec taskSQLExecutor,
	current loop.Run,
	to loop.Status,
	cause loop.TransitionCause,
	at time.Time,
	generation int,
	failure *taskpkg.CoordinatorFailure,
) error {
	return updateLoopBoundaryStatusWithEffects(
		ctx, exec, current, to, cause, at, generation, failure, nil,
	)
}

func updateLoopBoundaryStatusWithEffects(
	ctx context.Context,
	exec taskSQLExecutor,
	current loop.Run,
	to loop.Status,
	cause loop.TransitionCause,
	at time.Time,
	generation int,
	failure *taskpkg.CoordinatorFailure,
	effects []loop.RenderedEffectIntent,
) error {
	if current.Status == to {
		return updateLoopGenerationWithExecutor(ctx, exec, string(current.ID), generation)
	}
	if !to.Valid() {
		return fmt.Errorf("%w: loop status is invalid: %q", loop.ErrValidation, to)
	}
	if strings.TrimSpace(string(cause)) == "" {
		return fmt.Errorf("%w: transition cause is required", loop.ErrValidation)
	}
	affected, err := sqlcgen.New(exec).TransitionLoopCoordinatorBoundary(
		ctx,
		sqlcgen.TransitionLoopCoordinatorBoundaryParams{
			ToStatus: string(to), Generation: int64(generation),
			NeedsApprovalStatus: string(loop.StatusNeedsApproval), ID: string(current.ID),
			FromStatus: string(current.Status),
		},
	)
	if err != nil {
		return fmt.Errorf(
			"store: transition loop run %q at coordinator boundary: %w",
			current.ID,
			err,
		)
	}
	if affected == 0 {
		return fmt.Errorf(
			"%w: run_id=%s from=%s to=%s",
			loop.ErrTransitionConflict,
			current.ID,
			current.Status,
			to,
		)
	}
	return appendLoopRunStatusEventWithFailureAndEffects(
		ctx,
		exec,
		current.ID,
		current.WorkspaceID,
		current.Status,
		to,
		cause,
		failure,
		effects,
		at,
	)
}

func updateLoopGenerationWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	loopRunID string,
	generation int,
) error {
	if generation <= 0 {
		return nil
	}
	affected, err := sqlcgen.New(exec).UpdateLoopRunGeneration(ctx, sqlcgen.UpdateLoopRunGenerationParams{
		Generation: int64(generation), ID: loopRunID,
	})
	if err != nil {
		return fmt.Errorf("store: update loop run %q generation: %w", loopRunID, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: loop run %q: %w", loopRunID, loop.ErrRunNotFound)
	}
	return nil
}

func loopStatusIsTerminalOrApproval(status loop.Status) bool {
	switch status {
	case loop.StatusDone,
		loop.StatusNoOp,
		loop.StatusBlocked,
		loop.StatusFailed,
		loop.StatusExhausted,
		loop.StatusStalled,
		loop.StatusCanceled,
		loop.StatusNeedsApproval:
		return true
	default:
		return false
	}
}

func (g *TaskRepo) reserveCoordinatorPlanRunsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	plan taskpkg.CoordinatorCompletionPlan,
	current taskpkg.Run,
	origin taskpkg.Origin,
	queuedAt time.Time,
) ([]taskpkg.Run, error) {
	specs := make([]taskpkg.EnqueueSpec, 0, len(plan.NodeRuns)+1)
	specs = append(specs, plan.NodeRuns...)
	if plan.NextCoordinator != nil {
		next := *plan.NextCoordinator
		if next.RunKind.Normalize() == taskpkg.RunKindUnknown {
			next.RunKind = taskpkg.RunKindCoordinator
		}
		if strings.TrimSpace(next.LoopRunID) == "" {
			next.LoopRunID = current.LoopRunID
		}
		specs = append(specs, next)
	}

	enqueued := make([]taskpkg.Run, 0, len(specs))
	for _, spec := range specs {
		reservation, err := coordinatorPlanRunReservation(spec, current, origin, queuedAt)
		if err != nil {
			return nil, err
		}
		existing, err := g.coordinatorPlanRunReservationExists(ctx, exec, reservation)
		if err != nil {
			return nil, err
		}
		if existing {
			continue
		}
		_, run, existing, err := g.reserveQueuedRunWithExecutor(ctx, exec, reservation)
		if err != nil {
			return nil, err
		}
		if existing {
			continue
		}
		enqueued = append(enqueued, run)
	}
	return enqueued, nil
}

func (g *TaskRepo) coordinatorPlanRunReservationExists(
	ctx context.Context,
	exec taskSQLExecutor,
	reservation queuedRunReservationInput,
) (bool, error) {
	runID := strings.TrimSpace(reservation.runID)
	if runID == "" {
		return false, nil
	}
	existing, err := g.getTaskRunWithExecutor(ctx, exec, runID)
	if errors.Is(err, taskpkg.ErrTaskRunNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(existing.TaskID) != strings.TrimSpace(reservation.taskID) ||
		existing.RunKind.Normalize() != reservation.runKind.Normalize() ||
		strings.TrimSpace(existing.LoopRunID) != strings.TrimSpace(reservation.loopRunID) ||
		strings.TrimSpace(existing.IdempotencyKey) != strings.TrimSpace(reservation.idempotencyKey) {
		return false, fmt.Errorf(
			"%w: coordinator plan run %q conflicts with its existing deterministic identity",
			taskpkg.ErrValidation,
			runID,
		)
	}
	return true, nil
}

func coordinatorPlanRunReservation(
	spec taskpkg.EnqueueSpec,
	current taskpkg.Run,
	origin taskpkg.Origin,
	queuedAt time.Time,
) (queuedRunReservationInput, error) {
	normalized := spec.Normalize()
	networkSpec := current.NetworkSpecSnapshot()
	if normalized.ResolvedNetworkParticipation != nil {
		networkSpec = *normalized.ResolvedNetworkParticipation
	}
	runID := strings.TrimSpace(normalized.RunID)
	if runID == "" {
		generatedID, err := store.NewID("run")
		if err != nil {
			return queuedRunReservationInput{}, fmt.Errorf("store: generate coordinator plan run id: %w", err)
		}
		runID = generatedID
	}
	if strings.TrimSpace(normalized.LoopRunID) == "" {
		normalized.LoopRunID = current.LoopRunID
	}
	return queuedRunReservationInput{
		taskID:             normalized.TaskID,
		runID:              runID,
		runKind:            normalized.RunKind,
		loopRunID:          normalized.LoopRunID,
		idempotencyKey:     normalized.IdempotencyKey,
		origin:             origin,
		networkSpec:        networkSpec,
		designationGroupID: normalized.DesignationGroupID,
		metadata:           normalizeTaskJSON(normalized.Metadata),
		queuedAt:           queuedAt,
	}, nil
}
