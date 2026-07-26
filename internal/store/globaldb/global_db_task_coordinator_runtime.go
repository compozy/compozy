package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
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
	return updateLoopBoundaryStatusWithFailure(
		ctx,
		exec,
		current,
		to,
		cause,
		at,
		generation,
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
	return appendLoopRunStatusEventWithFailure(
		ctx,
		exec,
		current.ID,
		current.WorkspaceID,
		current.Status,
		to,
		cause,
		failure,
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
		reservation := coordinatorPlanRunReservation(spec, current, origin, queuedAt)
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

func coordinatorPlanRunReservation(
	spec taskpkg.EnqueueSpec,
	current taskpkg.Run,
	origin taskpkg.Origin,
	queuedAt time.Time,
) queuedRunReservationInput {
	normalized := spec.Normalize()
	networkSpec := current.NetworkSpecSnapshot()
	if normalized.ResolvedNetworkParticipation != nil {
		networkSpec = *normalized.ResolvedNetworkParticipation
	}
	runID := strings.TrimSpace(normalized.RunID)
	if runID == "" {
		runID = store.NewID("run")
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
	}
}
