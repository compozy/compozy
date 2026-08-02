package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const loopRetryTimerFireTimeout = 10 * time.Second

var _ taskpkg.CoordinatorTimerArmer = (*loopRetryTimerArmer)(nil)
var _ loopRetryTimerStore = (*globaldb.GlobalDB)(nil)

type loopRetryTimerStore interface {
	EnqueueLoopRetryWakeIfCurrent(
		context.Context,
		taskpkg.Origin,
		looppkg.RetryDueCell,
		time.Time,
	) (taskpkg.Run, bool, bool, error)
}

type loopRetryTimerArmer struct {
	ctx    context.Context
	store  loopRetryTimerStore
	logger *slog.Logger
	now    func() time.Time
}

func newLoopRetryTimerArmer(
	ctx context.Context,
	store any,
	logger *slog.Logger,
	now func() time.Time,
) *loopRetryTimerArmer {
	timerStore, ok := store.(loopRetryTimerStore)
	if !ok || ctx == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &loopRetryTimerArmer{ctx: ctx, store: timerStore, logger: logger, now: now}
}

func (a *loopRetryTimerArmer) ArmCoordinatorTimer(
	ctx context.Context,
	spec taskpkg.CoordinatorTimerSpec,
	actor taskpkg.ActorContext,
) error {
	if a == nil || a.store == nil || a.ctx == nil {
		return errors.New("daemon: loop retry timer armer is required")
	}
	if ctx == nil {
		return errors.New("daemon: loop retry timer context is required")
	}
	spec = spec.Normalize()
	if err := spec.Validate("coordinator_timer"); err != nil {
		return err
	}
	cell := retryDueCellFromTimer(spec)
	if got, want := spec.IdempotencyKey, looppkg.RetryWakeIdempotencyKey(cell); got != want {
		return fmt.Errorf("%w: retry timer idempotency key %q does not match %q", taskpkg.ErrValidation, got, want)
	}
	delay := max(spec.FireAt.Sub(a.now().UTC()), 0)
	go a.fireAfter(delay, actor.Origin, cell)
	return nil
}

func (a *loopRetryTimerArmer) fireAfter(delay time.Duration, origin taskpkg.Origin, cell looppkg.RetryDueCell) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-a.ctx.Done():
		return
	case <-timer.C:
	}
	ctx, cancel := context.WithTimeout(a.ctx, loopRetryTimerFireTimeout)
	defer cancel()
	if _, _, _, err := a.store.EnqueueLoopRetryWakeIfCurrent(ctx, origin, cell, a.now().UTC()); err != nil {
		a.logger.Warn(
			"daemon: loop retry timer fire failed; scheduler backstop will recover",
			"loop_run_id", cell.LoopRunID,
			"node_id", cell.NodeID,
			"attempt", cell.Attempt,
			"issued_epoch", cell.Epoch,
			"error", err,
		)
	}
}

func retryDueCellFromTimer(spec taskpkg.CoordinatorTimerSpec) looppkg.RetryDueCell {
	return looppkg.RetryDueCell{
		LoopRunID: looppkg.RunID(spec.LoopRunID), Generation: spec.Generation,
		NodeID: looppkg.NodeID(spec.NodeID), ItemIndex: spec.ItemIndex,
		Attempt: spec.Attempt, Epoch: spec.IssuedEpoch, NextAttemptAt: spec.FireAt.UTC(),
	}
}
