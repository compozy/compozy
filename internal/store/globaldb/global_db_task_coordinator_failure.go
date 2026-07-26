package globaldb

import (
	"context"
	"errors"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	taskpkg "github.com/compozy/agh/internal/task"
)

func settleCoordinatorFailureLoopWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	failure taskpkg.LeaseFailure,
) error {
	if run.RunKind.Normalize() != taskpkg.RunKindCoordinator {
		return nil
	}
	loopRunID := strings.TrimSpace(run.LoopRunID)
	if loopRunID == "" {
		return nil
	}
	loopRun, err := getLoopRunByIDWithExecutor(ctx, exec, looppkg.RunID(loopRunID))
	if errors.Is(err, looppkg.ErrRunNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if loopRun.Status.Terminal() {
		return nil
	}
	details, ok := taskpkg.CoordinatorFailureFromRunFailure(failure.Failure)
	if !ok {
		return nil
	}
	return updateLoopBoundaryStatusWithFailure(
		ctx,
		exec,
		loopRun,
		looppkg.StatusFailed,
		looppkg.TransitionCauseCoordinatorFailure,
		failure.Now,
		loopRun.Generation,
		&details,
	)
}
