package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func oldestQueuedLoopRunReadyForPromotion(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID string,
	loopName string,
) (*loopCoordinatorCandidate, error) {
	if workspaceID == "" || loopName == "" {
		return nil, nil
	}
	row, err := sqlcgen.New(exec).OldestQueuedLoopRunReadyForPromotion(
		ctx,
		sqlcgen.OldestQueuedLoopRunReadyForPromotionParams{WorkspaceID: workspaceID, LoopName: loopName},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf(
			"store: find queued loop promotion candidate for %q/%q: %w",
			workspaceID,
			loopName,
			err,
		)
	}
	candidate := loopCoordinatorCandidate{
		loopRunID: row.ID, workspaceID: row.WorkspaceID, loopName: row.LoopName, generation: int(row.Generation),
	}
	return &candidate, nil
}

func (g *LoopRepo) reconcileMissingRunningLoopCoordinators(
	ctx context.Context,
	exec taskSQLExecutor,
	origin taskpkg.Origin,
	now time.Time,
) ([]taskpkg.Run, error) {
	missing, err := loopRunsMissingActiveCoordinator(ctx, exec)
	if err != nil {
		return nil, err
	}
	enqueued := make([]taskpkg.Run, 0, len(missing))
	for _, candidate := range missing {
		current, err := getLoopRunByIDWithExecutor(ctx, exec, loop.RunID(strings.TrimSpace(candidate.loopRunID)))
		if err != nil {
			return nil, err
		}
		generation := candidate.generation + 1
		run, added, err := g.reserveLoopCoordinatorRunWithExecutor(
			ctx,
			exec,
			current,
			origin,
			now,
			loopCoordinatorRunID(current.ID, generation),
			loopCoordinatorIdempotencyKey(current.ID, generation),
		)
		if err != nil {
			return nil, err
		}
		if added {
			enqueued = append(enqueued, run)
		}
	}
	return enqueued, nil
}

func (g *LoopRepo) promoteQueuedLoopCoordinators(
	ctx context.Context,
	exec taskSQLExecutor,
	origin taskpkg.Origin,
	now time.Time,
) ([]taskpkg.Run, error) {
	promotable, err := queuedLoopRunsReadyForPromotion(ctx, exec)
	if err != nil {
		return nil, err
	}
	enqueued := make([]taskpkg.Run, 0, len(promotable))
	for _, candidate := range promotable {
		run, added, err := g.promoteQueuedLoopCoordinator(ctx, exec, candidate, origin, now)
		if err != nil {
			return nil, err
		}
		if added {
			enqueued = append(enqueued, run)
		}
	}
	return enqueued, nil
}

func (g *LoopRepo) promoteQueuedLoopCoordinator(
	ctx context.Context,
	exec taskSQLExecutor,
	candidate loopCoordinatorCandidate,
	origin taskpkg.Origin,
	now time.Time,
) (taskpkg.Run, bool, error) {
	current, err := getLoopRunByIDWithExecutor(ctx, exec, loop.RunID(strings.TrimSpace(candidate.loopRunID)))
	if err != nil {
		return taskpkg.Run{}, false, err
	}
	if err := updateLoopBoundaryStatusWithExecutor(
		ctx,
		exec,
		current,
		loop.StatusRunning,
		loop.TransitionCausePromote,
		now,
		candidate.generation,
	); err != nil {
		return taskpkg.Run{}, false, err
	}
	generation := candidate.generation + 1
	return g.reserveLoopCoordinatorRunWithExecutor(
		ctx,
		exec,
		current,
		origin,
		now,
		loopCoordinatorRunID(current.ID, generation),
		loopCoordinatorIdempotencyKey(current.ID, generation),
	)
}

func (g *LoopRepo) reserveCoordinatorRun(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	loopRunID string,
	runID string,
	idempotencyKey string,
	origin taskpkg.Origin,
	now time.Time,
) (taskpkg.Run, bool, error) {
	reservedRunID := strings.TrimSpace(runID)
	if reservedRunID == "" {
		reservedRunID = store.NewID("run")
	}
	loopRun, err := getLoopRunByIDWithExecutor(ctx, exec, loop.RunID(strings.TrimSpace(loopRunID)))
	if err != nil {
		return taskpkg.Run{}, false, err
	}
	reservation := queuedRunReservationInput{
		taskID:         taskID,
		runID:          reservedRunID,
		runKind:        taskpkg.RunKindCoordinator,
		loopRunID:      loopRunID,
		idempotencyKey: idempotencyKey,
		origin:         origin,
		networkSpec:    loopRun.NetworkSpecSnapshot(),
		queuedAt:       now,
	}
	_, run, existing, err := g.tasks.reserveQueuedRunWithExecutor(ctx, exec, reservation)
	if err != nil {
		return taskpkg.Run{}, false, err
	}
	return run, !existing, nil
}

type loopCoordinatorCandidate struct {
	loopRunID   string
	workspaceID string
	loopName    string
	generation  int
}

func loopRunsMissingActiveCoordinator(
	ctx context.Context,
	exec taskSQLExecutor,
) ([]loopCoordinatorCandidate, error) {
	rows, err := sqlcgen.New(exec).ListLoopRunsMissingActiveCoordinator(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list running loops missing coordinator: %w", err)
	}
	candidates := make([]loopCoordinatorCandidate, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, loopCoordinatorCandidate{loopRunID: row.ID, generation: int(row.Generation)})
	}
	return candidates, nil
}

func queuedLoopRunsReadyForPromotion(
	ctx context.Context,
	exec taskSQLExecutor,
) ([]loopCoordinatorCandidate, error) {
	rows, err := sqlcgen.New(exec).ListQueuedLoopRunsReadyForPromotion(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list queued loops ready for promotion: %w", err)
	}
	candidates := make([]loopCoordinatorCandidate, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, loopCoordinatorCandidate{
			loopRunID: row.ID, workspaceID: row.WorkspaceID, loopName: row.LoopName, generation: int(row.Generation),
		})
	}
	return candidates, nil
}

func lastCoordinatorTaskIDForLoopRun(
	ctx context.Context,
	exec taskSQLExecutor,
	loopRunID string,
) (string, error) {
	taskID, err := sqlcgen.New(exec).GetLastCoordinatorTaskIDForLoopRun(ctx, nullString(loopRunID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		return "", fmt.Errorf("store: find coordinator task for loop run %q: %w", loopRunID, err)
	}
	return taskNullStringValue(taskID), nil
}

func errorsIsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
