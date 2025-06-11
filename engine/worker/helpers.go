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
	if temporal.IsCanceledError(ctx.Err()) {
		logger.Info("Sleep skipped due to cancellation")
		return workflow.ErrCanceled
	}
	timerDone := false
	timer := workflow.NewTimer(ctx, dur)
	for !timerDone {
		// Check cancellation before each iteration
		if temporal.IsCanceledError(ctx.Err()) {
			logger.Info("Sleep interrupted by cancellation")
			return workflow.ErrCanceled
		}
		sel := workflow.NewSelector(ctx)
		sel.AddFuture(timer, func(workflow.Future) { timerDone = true })
		sel.Select(ctx)
		// Check again after select
		if temporal.IsCanceledError(ctx.Err()) {
			logger.Info("Sleep interrupted by cancellation")
			return workflow.ErrCanceled
		}
	}
	return nil
}

// actHandler - Curry function for cancellation and pause checks
func actHandler[T any](
	errHandler func(err error) error,
	fn func(ctx workflow.Context) (T, error),
) func(ctx workflow.Context) (T, error) {
	return func(ctx workflow.Context) (T, error) {
		var zero T
		if temporal.IsCanceledError(ctx.Err()) {
			return zero, errHandler(workflow.ErrCanceled)
		}

		result, err := fn(ctx)
		if err != nil {
			if temporal.IsCanceledError(err) {
				return zero, errHandler(err)
			}
			// Handle other errors through error handler
			return zero, errHandler(err)
		}
		return result, nil
	}
}
