// Package retry provides small context-aware retry and backoff primitives.
package retry

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Policy configures retry attempts and their delay strategy.
type Policy struct {
	MaxAttempts int
	Delay       DelayFunc
	Sleep       func(context.Context, time.Duration) error
	OnRetry     func(context.Context, Attempt) error
}

// Attempt describes one failed attempt that will be retried.
type Attempt struct {
	Number      int
	MaxAttempts int
	Err         error
	Delay       time.Duration
}

// ShouldRetry reports whether an operation error should be retried.
type ShouldRetry func(error) bool

const (
	defaultMaxAttempts = 1
)

// Do retries fn until it succeeds, shouldRetry rejects the error, attempts are
// exhausted, or ctx is canceled.
func Do(ctx context.Context, policy Policy, shouldRetry ShouldRetry, fn func(context.Context) error) error {
	if fn == nil {
		return errors.New("retry: operation is required")
	}
	_, err := DoValue(ctx, policy, shouldRetry, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	})
	return err
}

// DoValue is Do for operations that return a value.
func DoValue[T any](
	ctx context.Context,
	policy Policy,
	shouldRetry ShouldRetry,
	fn func(context.Context) (T, error),
) (T, error) {
	var zero T
	if ctx == nil {
		return zero, errors.New("retry: context is required")
	}
	if fn == nil {
		return zero, errors.New("retry: operation is required")
	}

	policy = normalizePolicy(policy)
	var previousDelay time.Duration
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, fmt.Errorf("retry: context done before attempt %d: %w", attempt, err)
		}

		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return zero, fmt.Errorf("retry: context done after attempt %d: %w", attempt, ctxErr)
		}
		if isContextError(err) || attempt == policy.MaxAttempts || !retryable(shouldRetry, err) {
			return zero, fmt.Errorf("retry: attempt %d failed: %w", attempt, err)
		}

		delay := policy.Delay(DelayInput{
			Attempt:  attempt,
			Previous: previousDelay,
			Err:      err,
		})
		delay = max(delay, 0)
		previousDelay = delay
		if policy.OnRetry != nil {
			if retryErr := policy.OnRetry(ctx, Attempt{
				Number:      attempt,
				MaxAttempts: policy.MaxAttempts,
				Err:         err,
				Delay:       delay,
			}); retryErr != nil {
				return zero, fmt.Errorf(
					"retry: observe attempt %d: %w",
					attempt,
					errors.Join(err, retryErr),
				)
			}
		}
		if err := policy.Sleep(ctx, delay); err != nil {
			return zero, fmt.Errorf("retry: wait before attempt %d: %w", attempt+1, err)
		}
	}

	return zero, nil
}

// Wait blocks for delay or returns early when ctx is canceled.
func Wait(ctx context.Context, delay time.Duration) error {
	if ctx == nil {
		return errors.New("retry: context is required")
	}
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func normalizePolicy(policy Policy) Policy {
	if policy.MaxAttempts <= 0 {
		policy.MaxAttempts = defaultMaxAttempts
	}
	if policy.Delay == nil {
		policy.Delay = DecorrelatedJitter(DecorrelatedJitterConfig{})
	}
	if policy.Sleep == nil {
		policy.Sleep = Wait
	}
	return policy
}

func retryable(shouldRetry ShouldRetry, err error) bool {
	if shouldRetry == nil {
		return true
	}
	return shouldRetry(err)
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
