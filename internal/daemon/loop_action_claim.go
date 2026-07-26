package daemon

import (
	"context"
	"errors"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
)

const loopActionClaimRetryInterval = time.Second

func (r *loopActionRuntime) executeStartedRun(
	ctx context.Context,
	taskRecord taskpkg.Task,
	run taskpkg.Run,
	reason string,
) error {
	for {
		err := r.executeQueuedRunWithSlot(ctx, taskRecord, run, reason)
		if !errors.Is(err, taskpkg.ErrWorkspaceActiveRunCapReached) {
			return err
		}
		timer := time.NewTimer(r.claimRetryInterval)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}

func (r *loopActionRuntime) executeQueuedRunWithSlot(
	ctx context.Context,
	taskRecord taskpkg.Task,
	run taskpkg.Run,
	reason string,
) error {
	if err := r.acquireSlot(ctx); err != nil {
		return err
	}
	defer r.releaseSlot()
	return r.executeQueuedRun(ctx, taskRecord, run, reason)
}

func (r *loopActionRuntime) acquireSlot(ctx context.Context) error {
	select {
	case r.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *loopActionRuntime) releaseSlot() {
	<-r.sem
}
