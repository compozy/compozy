package globaldb

import (
	"context"
	"errors"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (g *TaskRepo) settleCoordinatorFailureLoopWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	failure taskpkg.LeaseFailure,
) ([]taskpkg.StatusTransition, error) {
	if run.RunKind.Normalize() != taskpkg.RunKindCoordinator {
		return nil, nil
	}
	loopRunID := strings.TrimSpace(run.LoopRunID)
	if loopRunID == "" {
		return nil, nil
	}
	loopRun, err := getLoopRunByIDWithExecutor(ctx, exec, looppkg.RunID(loopRunID))
	if errors.Is(err, looppkg.ErrRunNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if loopRun.Status.Terminal() {
		return nil, nil
	}
	details, ok := taskpkg.CoordinatorFailureFromRunFailure(failure.Failure)
	if !ok {
		return nil, nil
	}
	transitions, err := updateLoopBoundaryStatusWithFailure(
		ctx,
		exec,
		loopRun,
		looppkg.StatusFailed,
		looppkg.TransitionCauseCoordinatorFailure,
		failure.Now,
		loopRun.Generation,
		&details,
	)
	if err != nil {
		return nil, err
	}
	return transitions, nil
}
