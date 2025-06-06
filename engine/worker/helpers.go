package worker

import (
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// -----------------------------------------------------------------------------
// Sleep
// -----------------------------------------------------------------------------

func SleepWithPause(ctx workflow.Context, dur time.Duration) error {
	if ctx.Err() == workflow.ErrCanceled {
		logger.Info("Sleep skipped due to cancellation")
		return workflow.ErrCanceled
	}
	timerDone := false
	timer := workflow.NewTimer(ctx, dur)
	for !timerDone {
		// Check cancellation before each iteration
		if ctx.Err() == workflow.ErrCanceled {
			logger.Info("Sleep interrupted by cancellation")
			return workflow.ErrCanceled
		}
		sel := workflow.NewSelector(ctx)
		sel.AddFuture(timer, func(workflow.Future) { timerDone = true })
		sel.Select(ctx)
		// Check again after select
		if ctx.Err() == workflow.ErrCanceled {
			logger.Info("Sleep interrupted by cancellation")
			return workflow.ErrCanceled
		}
	}
	return nil
}

// actHandler - Curry function for cancellation and pause checks
func actHandler[T any](ctx workflow.Context, errHandler func(err error) error, fn func() (T, error)) func() (T, error) {
	return func() (T, error) {
		var zero T
		if ctx.Err() == workflow.ErrCanceled {
			return zero, errHandler(workflow.ErrCanceled)
		}
		result, err := fn()
		if err != nil {
			if err == workflow.ErrCanceled || temporal.IsCanceledError(err) {
				return zero, err
			}
			return zero, errHandler(err)
		}
		return result, nil
	}
}
