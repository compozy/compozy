package clawhub

import (
	"context"

	"time"
)

func sleepContext(ctx context.Context, wait time.Duration) error {
	if wait <= 0 {
		return nil
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func nextBackoff(current time.Duration, maxDelay time.Duration) time.Duration {
	if current <= 0 {
		return defaultInitialBackoff
	}
	if maxDelay <= 0 {
		maxDelay = defaultMaxBackoff
	}

	next := current * 2
	if next > maxDelay {
		return maxDelay
	}
	return next
}
