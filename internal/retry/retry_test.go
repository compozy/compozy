package retry

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestDoValue(t *testing.T) {
	t.Parallel()

	t.Run("ShouldRetryUntilSuccessWithDeterministicJitter", func(t *testing.T) {
		t.Parallel()

		attempts := 0
		delays := make([]time.Duration, 0, 2)
		errTransient := errors.New("transient")
		result, err := DoValue(
			context.Background(),
			Policy{
				MaxAttempts: 3,
				Delay: DecorrelatedJitter(DecorrelatedJitterConfig{
					BaseDelay:   10 * time.Millisecond,
					MaxDelay:    50 * time.Millisecond,
					RandFloat64: sequenceRand(t, 1, 0),
				}),
				Sleep: func(_ context.Context, delay time.Duration) error {
					delays = append(delays, delay)
					return nil
				},
			},
			func(err error) bool {
				return errors.Is(err, errTransient)
			},
			func(context.Context) (string, error) {
				attempts++
				if attempts < 3 {
					return "", errTransient
				}
				return "ok", nil
			},
		)
		if err != nil {
			t.Fatalf("DoValue() error = %v", err)
		}
		if result != "ok" {
			t.Fatalf("DoValue() result = %q, want ok", result)
		}
		if attempts != 3 {
			t.Fatalf("attempts = %d, want 3", attempts)
		}
		if got, want := delays, []time.Duration{
			30 * time.Millisecond,
			10 * time.Millisecond,
		}; !equalDurations(
			t,
			got,
			want,
		) {
			t.Fatalf("delays = %#v, want %#v", got, want)
		}
	})

	t.Run("ShouldStopOnNonRetryableError", func(t *testing.T) {
		t.Parallel()

		errPermanent := errors.New("permanent")
		attempts := 0
		_, err := DoValue(
			context.Background(),
			Policy{MaxAttempts: 3},
			func(error) bool { return false },
			func(context.Context) (string, error) {
				attempts++
				return "", errPermanent
			},
		)
		if !errors.Is(err, errPermanent) {
			t.Fatalf("DoValue(nonretryable) error = %v, want permanent", err)
		}
		if attempts != 1 {
			t.Fatalf("attempts = %d, want 1", attempts)
		}
	})

	t.Run("ShouldStopOnContextErrorReturnedByOperation", func(t *testing.T) {
		t.Parallel()

		_, err := DoValue(
			context.Background(),
			Policy{MaxAttempts: 3},
			nil,
			func(context.Context) (string, error) {
				return "", context.DeadlineExceeded
			},
		)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("DoValue(context error) error = %v, want context.DeadlineExceeded", err)
		}
	})

	t.Run("Should stop before sleeping when retry observation fails", func(t *testing.T) {
		t.Parallel()

		providerErr := errors.New("provider unavailable")
		observerErr := errors.New("report retry state")
		attempts := 0
		sleepCalls := 0
		_, err := DoValue(
			t.Context(),
			Policy{
				MaxAttempts: 2,
				Sleep: func(context.Context, time.Duration) error {
					sleepCalls++
					return nil
				},
				OnRetry: func(_ context.Context, attempt Attempt) error {
					if !errors.Is(attempt.Err, providerErr) {
						t.Fatalf("retry attempt error = %v, want provider error", attempt.Err)
					}
					return observerErr
				},
			},
			func(err error) bool { return errors.Is(err, providerErr) },
			func(context.Context) (string, error) {
				attempts++
				return "", providerErr
			},
		)
		if !errors.Is(err, providerErr) || !errors.Is(err, observerErr) {
			t.Fatalf("DoValue(observer failure) error = %v, want provider and observer errors", err)
		}
		if attempts != 1 {
			t.Fatalf("attempts = %d, want 1", attempts)
		}
		if sleepCalls != 0 {
			t.Fatalf("sleep calls = %d, want 0", sleepCalls)
		}
	})
}

func TestDo(t *testing.T) {
	t.Parallel()

	t.Run("ShouldStopAtMaxAttemptsAndAvoidSleepAfterFinalFailure", func(t *testing.T) {
		t.Parallel()

		attempts := 0
		sleepCalls := 0
		errBoom := errors.New("boom")
		err := Do(
			context.Background(),
			Policy{
				MaxAttempts: 2,
				Delay:       func(DelayInput) time.Duration { return time.Millisecond },
				Sleep: func(context.Context, time.Duration) error {
					sleepCalls++
					return nil
				},
			},
			nil,
			func(context.Context) error {
				attempts++
				return errBoom
			},
		)
		if !errors.Is(err, errBoom) {
			t.Fatalf("Do() error = %v, want errBoom", err)
		}
		if attempts != 2 {
			t.Fatalf("attempts = %d, want 2", attempts)
		}
		if sleepCalls != 1 {
			t.Fatalf("sleepCalls = %d, want 1", sleepCalls)
		}
	})

	t.Run("ShouldHonorContextCancellationBeforeFirstAttempt", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		called := false
		err := Do(ctx, Policy{MaxAttempts: 3}, nil, func(context.Context) error {
			called = true
			return errors.New("should not run")
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Do(canceled) error = %v, want context.Canceled", err)
		}
		if called {
			t.Fatal("operation called for canceled context")
		}
	})

	t.Run("ShouldRejectNilInputs", func(t *testing.T) {
		t.Parallel()

		err := Do(nilRetryContext(t), Policy{}, nil, func(context.Context) error { return nil })
		if err == nil || !strings.Contains(err.Error(), "context is required") {
			t.Fatalf("Do(nil context) error = %v, want context required", err)
		}
		err = Do(context.Background(), Policy{}, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "operation is required") {
			t.Fatalf("Do(nil operation) error = %v, want operation required", err)
		}
	})
}

func TestWait(t *testing.T) {
	t.Parallel()

	t.Run("ShouldHonorContextCancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := Wait(ctx, time.Hour); !errors.Is(err, context.Canceled) {
			t.Fatalf("Wait(canceled) error = %v, want context.Canceled", err)
		}
	})

	t.Run("ShouldHandleNilContextAndNonPositiveDelay", func(t *testing.T) {
		t.Parallel()

		if err := Wait(nilRetryContext(t), 0); err == nil || !strings.Contains(err.Error(), "context is required") {
			t.Fatalf("Wait(nil context) error = %v, want context required", err)
		}
		if err := Wait(context.Background(), 0); err != nil {
			t.Fatalf("Wait(zero delay) error = %v", err)
		}
	})
}

func TestDecorrelatedJitterBoundaries(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		config   DecorrelatedJitterConfig
		previous time.Duration
		want     time.Duration
	}{
		{
			name: "ShouldApplyBaseAtLowBound",
			config: DecorrelatedJitterConfig{
				BaseDelay:   100 * time.Millisecond,
				MaxDelay:    time.Second,
				RandFloat64: func() float64 { return 0 },
			},
			want: 100 * time.Millisecond,
		},
		{
			name: "ShouldApplyThreeTimesPreviousAtHighBound",
			config: DecorrelatedJitterConfig{
				BaseDelay:   100 * time.Millisecond,
				MaxDelay:    time.Second,
				RandFloat64: func() float64 { return 1 },
			},
			previous: 200 * time.Millisecond,
			want:     600 * time.Millisecond,
		},
		{
			name: "ShouldCapDelayAtMaxDelay",
			config: DecorrelatedJitterConfig{
				BaseDelay:   100 * time.Millisecond,
				MaxDelay:    time.Second,
				RandFloat64: func() float64 { return 1 },
			},
			previous: 750 * time.Millisecond,
			want:     time.Second,
		},
		{
			name: "ShouldClampMaximumToBaseDelay",
			config: DecorrelatedJitterConfig{
				BaseDelay:   2 * time.Second,
				MaxDelay:    time.Second,
				RandFloat64: func() float64 { return 1 },
			},
			want: 2 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			delayFor := DecorrelatedJitter(tc.config)
			if got := delayFor(DelayInput{Attempt: 1, Previous: tc.previous}); got != tc.want {
				t.Fatalf("DecorrelatedJitter() = %s, want %s", got, tc.want)
			}
		})
	}
}

// Invariant: decorrelated jitter stays bounded without collapsing into a fixed multiplier sequence.
// Owning layer: shared transient retry infrastructure. Canonical suite: retry_test.go.
func TestDecorrelatedJitterStaysBoundedAndVariesWithFixedSeed(t *testing.T) {
	t.Parallel()

	const samples = 128
	baseDelay := 100 * time.Millisecond
	maxDelay := 5 * time.Second
	random := rand.New(rand.NewSource(42))
	delayFor := DecorrelatedJitter(DecorrelatedJitterConfig{
		BaseDelay:   baseDelay,
		MaxDelay:    maxDelay,
		RandFloat64: random.Float64,
	})

	var (
		previous  time.Duration
		increases int
		decreases int
		nonDouble int
	)
	for attempt := 1; attempt <= samples; attempt++ {
		delay := delayFor(DelayInput{
			Attempt:  attempt,
			Previous: previous,
			Err:      errors.New("provider unavailable"),
		})
		upperPrevious := max(previous, baseDelay)
		upper := min(maxDelay, upperPrevious*3)
		if delay < baseDelay || delay > upper {
			t.Fatalf(
				"attempt %d delay = %s, want within [%s, %s] after %s",
				attempt,
				delay,
				baseDelay,
				upper,
				previous,
			)
		}
		if previous > 0 {
			switch {
			case delay > previous:
				increases++
			case delay < previous:
				decreases++
			}
			if delay != previous*2 {
				nonDouble++
			}
		}
		previous = delay
	}

	if increases == 0 || decreases == 0 {
		t.Fatalf("delay movement = increases:%d decreases:%d, want both directions", increases, decreases)
	}
	if nonDouble < samples/2 {
		t.Fatalf("non-double transitions = %d, want at least %d", nonDouble, samples/2)
	}
}

func sequenceRand(t *testing.T, values ...float64) func() float64 {
	t.Helper()

	index := 0
	return func() float64 {
		if index >= len(values) {
			return values[len(values)-1]
		}
		value := values[index]
		index++
		return value
	}
}

func equalDurations(t *testing.T, left, right []time.Duration) bool {
	t.Helper()

	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func nilRetryContext(t *testing.T) context.Context {
	t.Helper()

	return nil
}
