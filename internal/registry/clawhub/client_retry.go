package clawhub

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/retry"
)

type clawHubRequestAttemptError struct {
	cause     error
	retryable bool
}

func (e *clawHubRequestAttemptError) Error() string {
	return e.cause.Error()
}

func (e *clawHubRequestAttemptError) Unwrap() error {
	return e.cause
}

func (c *Client) requestRetryPolicy(operation string) retry.Policy {
	return retry.Policy{
		MaxAttempts: c.maxRetries + 1,
		Delay: retry.Exponential(retry.ExponentialConfig{
			BaseDelay: c.initialBackoff,
			MaxDelay:  c.maxBackoff,
		}),
		Sleep: func(ctx context.Context, delay time.Duration) error {
			if err := c.sleep(ctx, delay); err != nil {
				return &clawHubRequestAttemptError{
					cause: fmt.Errorf("clawhub: %s retry wait aborted: %w", operation, err),
				}
			}
			return nil
		},
	}
}

func newClawHubTransportAttemptError(
	ctx context.Context,
	operation string,
	err error,
) *clawHubRequestAttemptError {
	retryable := !errors.Is(err, context.Canceled) &&
		!errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil
	return &clawHubRequestAttemptError{
		cause:     fmt.Errorf("clawhub: %s request failed: %w", operation, err),
		retryable: retryable,
	}
}

func retryClawHubRequest(err error) bool {
	attemptErr, ok := errors.AsType[*clawHubRequestAttemptError](err)
	return ok && attemptErr.retryable
}

func finishClawHubRequest(ctx context.Context, operation string, err error) error {
	attemptErr, ok := errors.AsType[*clawHubRequestAttemptError](err)
	if ok {
		if ctxErr := ctx.Err(); ctxErr != nil && !errors.Is(attemptErr.cause, ctxErr) {
			return errors.Join(
				attemptErr.cause,
				fmt.Errorf("clawhub: %s request failed: %w", operation, ctxErr),
			)
		}
		return attemptErr.cause
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("clawhub: %s request failed: %w", operation, ctxErr)
	}
	return err
}
