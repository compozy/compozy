package daemon

import (
	"context"
	"fmt"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func startLoopReconciliation(
	ctx context.Context,
	state *bootState,
	store looppkg.ReconciliationStore,
	cleanup *bootCleanup,
	ready <-chan struct{},
) error {
	if cleanup == nil {
		return fmt.Errorf("daemon: Loop reconciliation cleanup owner is required")
	}
	reconciler := looppkg.NewRunReconciler(store)
	started := time.Now()
	report, err := reconciler.NeutralizeOrphans(ctx)
	if err != nil {
		return fmt.Errorf("daemon: neutralize Loop orphans before recovery: %w", err)
	}
	state.logger.Info("daemon: Loop boot reconciliation complete",
		"runs_examined", report.RunsExamined,
		"records_settled", report.RecordsSettled,
		"orphans_repaired", report.OrphansRepaired,
		"provenance_backfilled", 0,
		"duration_ms", time.Since(started).Milliseconds(),
	)
	interval, err := time.ParseDuration(state.cfg.Loops.ReconcileInterval)
	if err != nil || interval <= 0 {
		return fmt.Errorf("daemon: invalid loops.reconcile_interval %q", state.cfg.Loops.ReconcileInterval)
	}
	runtime := newLoopReconcilerRuntime(reconciler, interval, state.logger)
	if err := runtime.Start(ctx, ready); err != nil {
		return fmt.Errorf("daemon: start Loop reconciler: %w", err)
	}
	cleanup.add(runtime.Shutdown)
	state.runtimeWorkers.loopReconciler = runtime
	return nil
}
