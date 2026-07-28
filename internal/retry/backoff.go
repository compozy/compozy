package retry

import (
	"math/rand"
	"time"
)

// DelayInput describes the failed attempt for which a retry delay is needed.
type DelayInput struct {
	Attempt  int
	Previous time.Duration
	Err      error
}

// DelayFunc computes the delay before the next attempt.
type DelayFunc func(DelayInput) time.Duration

// DecorrelatedJitterConfig bounds a decorrelated-jitter delay sequence.
type DecorrelatedJitterConfig struct {
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	RandFloat64 func() float64
}

const (
	defaultBaseDelay = 100 * time.Millisecond
	defaultMaxDelay  = 30 * time.Second
)

// DecorrelatedJitter returns a per-operation delay strategy whose next value
// is sampled between the base delay and three times the previous delay.
func DecorrelatedJitter(config DecorrelatedJitterConfig) DelayFunc {
	config = normalizeDecorrelatedJitter(config)

	return func(input DelayInput) time.Duration {
		previous := max(input.Previous, config.BaseDelay)
		upper := config.MaxDelay
		if previous <= config.MaxDelay/3 {
			upper = previous * 3
		}
		upper = min(max(upper, config.BaseDelay), config.MaxDelay)
		if upper == config.BaseDelay {
			return config.BaseDelay
		}

		random := min(max(config.RandFloat64(), 0), 1)
		delay := float64(config.BaseDelay) + random*float64(upper-config.BaseDelay)
		return min(time.Duration(delay), config.MaxDelay)
	}
}

func normalizeDecorrelatedJitter(config DecorrelatedJitterConfig) DecorrelatedJitterConfig {
	if config.BaseDelay <= 0 {
		config.BaseDelay = defaultBaseDelay
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = max(defaultMaxDelay, config.BaseDelay)
	} else {
		config.MaxDelay = max(config.MaxDelay, config.BaseDelay)
	}
	if config.RandFloat64 == nil {
		config.RandFloat64 = rand.Float64
	}
	return config
}
