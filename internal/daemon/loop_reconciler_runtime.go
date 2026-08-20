package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
)

type loopReconcilerRuntime struct {
	reconciler looppkg.RunReconciler
	interval   time.Duration
	logger     *slog.Logger
	cancel     context.CancelFunc
	done       chan struct{}
	startOnce  sync.Once
	stopOnce   sync.Once
}

func newLoopReconcilerRuntime(
	reconciler looppkg.RunReconciler,
	interval time.Duration,
	logger *slog.Logger,
) *loopReconcilerRuntime {
	return &loopReconcilerRuntime{reconciler: reconciler, interval: interval, logger: logger, done: make(chan struct{})}
}

func (r *loopReconcilerRuntime) Start(ctx context.Context, ready <-chan struct{}) error {
	if r == nil || r.reconciler == nil || r.interval <= 0 || r.logger == nil {
		return errors.New("daemon: Loop reconciler runtime is incomplete")
	}
	if ready == nil {
		return errors.New("daemon: Loop reconciler readiness barrier is required")
	}
	r.startOnce.Do(func() {
		workerCtx, cancel := context.WithCancel(ctx)
		r.cancel = cancel
		go r.run(workerCtx, ready)
	})
	return nil
}

func (r *loopReconcilerRuntime) run(ctx context.Context, ready <-chan struct{}) {
	defer close(r.done)
	select {
	case <-ctx.Done():
		return
	case <-ready:
	}
	started := time.Now()
	backfilled, err := r.reconciler.BackfillProvenance(ctx)
	if err != nil {
		r.logger.Error("daemon: Loop provenance backfill failed", "error", err,
			"duration_ms", time.Since(started).Milliseconds())
	} else {
		r.logger.Info("daemon: Loop provenance backfill complete", "provenance_backfilled", backfilled,
			"duration_ms", time.Since(started).Milliseconds())
	}
	timer := time.NewTimer(r.interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			r.sweep(ctx)
			timer.Reset(r.interval)
		}
	}
}

func (r *loopReconcilerRuntime) sweep(ctx context.Context) {
	started := time.Now()
	report, err := r.reconciler.SweepOnce(ctx)
	provenanceBackfilled := 0
	if err == nil {
		provenanceBackfilled, err = r.reconciler.BackfillProvenance(ctx)
	}
	attrs := []any{
		"runs_examined", report.RunsExamined,
		"records_settled", report.RecordsSettled,
		"orphans_repaired", report.OrphansRepaired,
		"provenance_backfilled", provenanceBackfilled,
		"duration_ms", time.Since(started).Milliseconds(),
	}
	if err != nil {
		attrs = append(attrs, "error", err)
		r.logger.Error("daemon: Loop reconciliation cycle failed", attrs...)
		return
	}
	r.logger.Info("daemon: Loop reconciliation cycle complete", attrs...)
}

func (r *loopReconcilerRuntime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.stopOnce.Do(func() {
		if r.cancel != nil {
			r.cancel()
		}
	})
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: stop Loop reconciler: %w", ctx.Err())
	}
}
