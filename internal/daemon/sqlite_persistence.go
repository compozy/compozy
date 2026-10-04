package daemon

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// Retry only the persistence operation; the caller retains the already-executed result.
func retrySQLitePersistence(
	ctx context.Context,
	attemptTimeout time.Duration,
	persist func(context.Context) error,
) error {
	if attemptTimeout <= 0 {
		attemptTimeout = defaultTaskCancelGrace
	}
	delay := 100 * time.Millisecond
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		err := persist(attemptCtx)
		cancel()
		if err == nil {
			return nil
		}
		contention, hasContention := errors.AsType[*store.WriteContentionError](err)
		retryable := store.IsSQLiteBusy(err) || (hasContention && contention != nil) ||
			errors.Is(err, context.DeadlineExceeded)
		if !retryable {
			return err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(err, ctx.Err())
		case <-timer.C:
		}
		delay = min(2*delay, time.Second)
	}
}

func loopActionSettlementContext(ctx context.Context) (context.Context, context.CancelFunc) {
	settleCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	stop := context.AfterFunc(ctx, func() {
		timer := time.NewTimer(defaultTaskCancelGrace)
		defer timer.Stop()
		select {
		case <-timer.C:
			cancel()
		case <-settleCtx.Done():
		}
	})
	return settleCtx, func() {
		stop()
		cancel()
	}
}
