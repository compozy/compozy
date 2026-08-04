package globaldb

import (
	"context"
	"errors"
	"fmt"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func isGoalControlStale(err error) bool {
	var reason *looppkg.ReasonError
	return errors.As(err, &reason) && reason.Code == looppkg.ReasonCodeGoalControlStale
}

func failStaleBindingCreationWithCleanup(
	ctx context.Context,
	exec taskSQLExecutor,
	binding goal.SessionBinding,
	failedAt time.Time,
) (goal.SessionBinding, error) {
	affected, err := sqlcgen.New(exec).FailGoalBindingCreationAttempt(
		ctx,
		sqlcgen.FailGoalBindingCreationAttemptParams{
			FailureCode:  goalNullableString(goalBindingFailureControlRevokedInFlight),
			FailedAt:     store.FormatTimestamp(failedAt),
			LoopRunID:    string(binding.Key.LoopRunID),
			Handle:       binding.Key.Handle,
			BindingEpoch: binding.BindingEpoch,
		},
	)
	if err != nil {
		return goal.SessionBinding{}, fmt.Errorf("store: fail stale Goal binding creation: %w", err)
	}
	if err := requireGoalAffectedCount(affected, "fail stale Goal binding creation"); err != nil {
		return goal.SessionBinding{}, err
	}
	failed, err := getSessionBindingAttemptWithExecutor(ctx, exec, binding.Key, binding.BindingEpoch)
	if err != nil {
		return goal.SessionBinding{}, fmt.Errorf("store: load failed stale Goal binding creation: %w", err)
	}
	if err := enqueueGoalSessionCleanupWithExecutor(
		ctx,
		exec,
		failed,
		goal.SessionCleanupCauseControlRevoked,
		failedAt,
	); err != nil {
		return goal.SessionBinding{}, fmt.Errorf("store: enqueue stale Goal binding cleanup: %w", err)
	}
	return failed, nil
}
