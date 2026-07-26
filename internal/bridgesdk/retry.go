package bridgesdk

import (
	"context"
	"errors"
	"math/rand"
	"time"

	retrypkg "github.com/compozy/agh/internal/retry"
)

// RetryBackoff bounds one provider failure class's delay sequence.
type RetryBackoff struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

// RetryAttempt describes one bridge operation failure that will be retried.
type RetryAttempt struct {
	Number      int
	MaxAttempts int
	Delay       time.Duration
	Classified  ClassifiedError
}

// RetryConfig configures bridge-provider retry classification and backoff.
type RetryConfig struct {
	Attempts          int
	StandardBackoff   RetryBackoff
	OverloadedBackoff RetryBackoff
	RandFloat         func() float64
	Sleep             func(context.Context, time.Duration) error
	OnRetry           func(context.Context, RetryAttempt) error
}

const (
	defaultRetryAttempts     = 3
	defaultRetryBaseDelay    = 300 * time.Millisecond
	defaultOverloadBaseDelay = time.Second
	defaultRetryMaxDelay     = 30 * time.Second
)

// DefaultRetryConfig returns the bridge-sdk default retry policy.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		Attempts: defaultRetryAttempts,
		StandardBackoff: RetryBackoff{
			BaseDelay: defaultRetryBaseDelay,
			MaxDelay:  defaultRetryMaxDelay,
		},
		OverloadedBackoff: RetryBackoff{
			BaseDelay: defaultOverloadBaseDelay,
			MaxDelay:  defaultRetryMaxDelay,
		},
		RandFloat: rand.Float64,
		Sleep:     retrypkg.Wait,
	}
}

// RetryDo retries a first-party bridge operation through the shared retry runner.
func RetryDo[T any](ctx context.Context, config RetryConfig, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if ctx == nil {
		return zero, errors.New("bridgesdk: retry context is required")
	}
	if fn == nil {
		return zero, errors.New("bridgesdk: retry function is required")
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	config = normalizeRetryConfig(config)
	return retrypkg.DoValue(
		ctx,
		retrypkg.Policy{
			MaxAttempts: config.Attempts,
			Delay:       bridgeRetryDelay(config),
			Sleep:       config.Sleep,
			OnRetry: func(retryCtx context.Context, attempt retrypkg.Attempt) error {
				if config.OnRetry == nil {
					return nil
				}
				return config.OnRetry(retryCtx, RetryAttempt{
					Number:      attempt.Number,
					MaxAttempts: attempt.MaxAttempts,
					Delay:       attempt.Delay,
					Classified:  ClassifyError(attempt.Err),
				})
			},
		},
		func(err error) bool {
			return ClassifyError(err).Recovery().Retry
		},
		fn,
	)
}

func normalizeRetryConfig(config RetryConfig) RetryConfig {
	if config.Attempts <= 0 {
		config.Attempts = 1
	}
	config.StandardBackoff = normalizeRetryBackoff(config.StandardBackoff, defaultRetryBaseDelay)
	config.OverloadedBackoff = normalizeRetryBackoff(config.OverloadedBackoff, defaultOverloadBaseDelay)
	if config.RandFloat == nil {
		config.RandFloat = rand.Float64
	}
	if config.Sleep == nil {
		config.Sleep = retrypkg.Wait
	}
	return config
}

func normalizeRetryBackoff(backoff RetryBackoff, defaultBase time.Duration) RetryBackoff {
	if backoff.BaseDelay <= 0 {
		backoff.BaseDelay = defaultBase
	}
	if backoff.MaxDelay <= 0 {
		backoff.MaxDelay = max(defaultRetryMaxDelay, backoff.BaseDelay)
	} else {
		backoff.MaxDelay = max(backoff.MaxDelay, backoff.BaseDelay)
	}
	return backoff
}

func bridgeRetryDelay(config RetryConfig) retrypkg.DelayFunc {
	standard := newDecorrelatedDelay(config.StandardBackoff, config.RandFloat)
	overloaded := newDecorrelatedDelay(config.OverloadedBackoff, config.RandFloat)

	return func(input retrypkg.DelayInput) time.Duration {
		classified := ClassifyError(input.Err)
		recovery := classified.Recovery()
		if recovery.RetryAfter > 0 {
			return recovery.RetryAfter
		}
		if classified.Class == ErrorClassOverloaded {
			return overloaded(input)
		}
		return standard(input)
	}
}

func newDecorrelatedDelay(backoff RetryBackoff, random func() float64) retrypkg.DelayFunc {
	return retrypkg.DecorrelatedJitter(retrypkg.DecorrelatedJitterConfig{
		BaseDelay:   backoff.BaseDelay,
		MaxDelay:    backoff.MaxDelay,
		RandFloat64: random,
	})
}
