package daemon

import (
	"context"
	"errors"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

var _ looppkg.CancellationReconciler = (*daemonLoopAPIService)(nil)

func (s *daemonLoopAPIService) ReconcilePendingCancellations(
	ctx context.Context,
	limit int,
	actor taskpkg.ActorContext,
) (int, error) {
	if s == nil || s.aggregate == nil {
		return 0, errors.New("daemon: Loop aggregate is unavailable")
	}
	reconciler, ok := s.aggregate.(looppkg.CancellationReconciler)
	if !ok {
		return 0, fmt.Errorf("%w: Loop cancellation reconciler is unavailable", looppkg.ErrActionDependencyMissing)
	}
	return reconciler.ReconcilePendingCancellations(ctx, limit, actor)
}

func reconcileBootLoopCancellations(ctx context.Context, state *bootState) error {
	if state == nil || state.deps.Loops == nil {
		return nil
	}
	reconciler, ok := state.deps.Loops.(looppkg.CancellationReconciler)
	if !ok {
		return errors.New("daemon: Loop service does not support cancellation reconciliation")
	}
	actor, err := taskpkg.DeriveDaemonActorContext("boot-recovery", "daemon.boot")
	if err != nil {
		return fmt.Errorf("daemon: derive Loop cancellation boot actor: %w", err)
	}
	reconciled, err := reconciler.ReconcilePendingCancellations(
		ctx,
		defaultMechanicalSchedulerSweepLimit,
		actor,
	)
	if err != nil {
		state.logger.Warn("daemon: Loop cancellation boot retry deferred", "error", err)
	}
	if reconciled > 0 {
		state.logger.Info(
			"daemon: Loop cancellation boot reconcile complete",
			"reconciled_cancellations",
			reconciled,
		)
	}
	return nil
}
