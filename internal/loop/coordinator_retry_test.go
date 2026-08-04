package loop

import (
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func TestPlanNodeRetryShouldApplyPinnedLifecyclePolicy(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 2, 18, 0, 0, 0, time.UTC)
	effective := EffectiveConfig{Lifecycle: DefaultLifecycleConfig()}
	transport := ClassifiedFailure{
		Class: FailureTransport, Code: "tool_unavailable", Cause: "offline", RetryEligible: true,
	}

	t.Run("Should schedule a mechanical retry from shipped defaults", func(t *testing.T) {
		t.Parallel()

		firstScheduledAt := now.Add(-time.Second)
		plan, err := planNodeRetry(&nodeRetryPlanInput{
			node:        dsl.Node{ID: "fetch", Class: dsl.NodeClassAction, Kind: "http"},
			effective:   effective,
			output:      GenerationOutput{Attempt: 1, FirstScheduledAt: &firstScheduledAt},
			failure:     transport,
			now:         now,
			randFloat64: func() float64 { return 0 },
		})
		if err != nil {
			t.Fatalf("planNodeRetry() error = %v", err)
		}
		if !plan.retry || plan.nextAttemptAt == nil {
			t.Fatalf("planNodeRetry() = %#v, want retry schedule", plan)
		}
		if got, want := plan.delay, time.Second; got != want {
			t.Fatalf("delay = %s, want %s", got, want)
		}
		if got, want := *plan.nextAttemptAt, now.Add(time.Second); !got.Equal(want) {
			t.Fatalf("next_attempt_at = %v, want %v", got, want)
		}
	})

	t.Run("Should continue decorrelated jitter from the previous durable delay", func(t *testing.T) {
		t.Parallel()

		endedAt := now.Add(-2 * time.Second)
		nextAttemptAt := now
		plan, err := planNodeRetry(&nodeRetryPlanInput{
			node:      dsl.Node{ID: "fetch", Class: dsl.NodeClassAction, Kind: "http"},
			effective: effective,
			output:    GenerationOutput{Attempt: 2},
			failure:   transport,
			attempts: []NodeAttempt{{
				Disposition: AttemptRetried, EndedAt: &endedAt, NextAttemptAt: &nextAttemptAt,
			}},
			now:         now,
			randFloat64: func() float64 { return 1 },
		})
		if err != nil {
			t.Fatalf("planNodeRetry() error = %v", err)
		}
		if got, want := plan.delay, 6*time.Second; got != want {
			t.Fatalf("delay = %s, want %s", got, want)
		}
	})

	t.Run("Should honor retry after within the configured bounds", func(t *testing.T) {
		t.Parallel()

		retryAfter := time.Minute
		failure := transport
		failure.RetryAfter = &retryAfter
		plan, err := planNodeRetry(&nodeRetryPlanInput{
			node: dsl.Node{
				ID: "fetch", Class: dsl.NodeClassAction, Kind: "http",
				Retry: &dsl.RetrySpec{MaxAttempts: 3, Backoff: &dsl.BackoffSpec{Base: "2s", Max: "9s"}},
			},
			effective: effective,
			output:    GenerationOutput{Attempt: 1},
			failure:   failure,
			now:       now,
		})
		if err != nil {
			t.Fatalf("planNodeRetry() error = %v", err)
		}
		if got, want := plan.delay, 9*time.Second; got != want {
			t.Fatalf("delay = %s, want %s", got, want)
		}
	})

	t.Run("Should never retry a non retryable failure class", func(t *testing.T) {
		t.Parallel()

		classes := []FailureClass{
			FailurePayloadDeclared,
			FailureAuthoring,
			FailureCancellation,
			FailureBudgetExhausted,
			FailureTargetUnavailable,
		}
		for _, class := range classes {
			t.Run("Should reject "+string(class), func(t *testing.T) {
				t.Parallel()

				plan, err := planNodeRetry(&nodeRetryPlanInput{
					node:      dsl.Node{ID: "fetch", Class: dsl.NodeClassAction, Kind: "http"},
					effective: effective,
					output:    GenerationOutput{Attempt: 1},
					failure: ClassifiedFailure{
						Class: class, Code: string(class), RetryEligible: class.RetryEligible(),
					},
					now: now,
				})
				if err != nil {
					t.Fatalf("planNodeRetry() error = %v", err)
				}
				if plan.retry {
					t.Fatalf("planNodeRetry() = %#v, want no retry", plan)
				}
			})
		}
	})

	t.Run("Should let a node declaration disable the family default", func(t *testing.T) {
		t.Parallel()

		plan, err := planNodeRetry(&nodeRetryPlanInput{
			node: dsl.Node{
				ID: "fetch", Class: dsl.NodeClassAction, Kind: "http",
				Retry: &dsl.RetrySpec{MaxAttempts: 0},
			},
			effective: effective,
			output:    GenerationOutput{Attempt: 1},
			failure:   transport,
			now:       now,
		})
		if err != nil {
			t.Fatalf("planNodeRetry() error = %v", err)
		}
		if plan.retry {
			t.Fatalf("planNodeRetry() = %#v, want retry disabled", plan)
		}
	})

	t.Run("Should ignore non DSL retry layers for expensive families", func(t *testing.T) {
		t.Parallel()

		for _, kind := range []dsl.ActionKind{dsl.ActionRunAgent, dsl.ActionRunLoop} {
			t.Run("Should reject "+string(kind), func(t *testing.T) {
				t.Parallel()

				plan, err := planNodeRetry(&nodeRetryPlanInput{
					node:      dsl.Node{ID: "expensive", Class: dsl.NodeClassAction, Kind: string(kind)},
					effective: effective,
					output:    GenerationOutput{Attempt: 1},
					failure:   transport,
					now:       now,
				})
				if err != nil {
					t.Fatalf("planNodeRetry() error = %v", err)
				}
				if plan.retry {
					t.Fatalf("planNodeRetry() = %#v, want DSL-only opt-in", plan)
				}
			})
		}
	})

	t.Run("Should schedule an authored sub loop retry", func(t *testing.T) {
		t.Parallel()

		plan, err := planNodeRetry(&nodeRetryPlanInput{
			node: dsl.Node{
				ID: "child", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop),
				Retry: &dsl.RetrySpec{MaxAttempts: 2},
			},
			effective:   effective,
			output:      GenerationOutput{Attempt: 1},
			failure:     transport,
			now:         now,
			randFloat64: func() float64 { return 0 },
		})
		if err != nil {
			t.Fatalf("planNodeRetry() error = %v", err)
		}
		if !plan.retry {
			t.Fatalf("planNodeRetry() = %#v, want authored retry", plan)
		}
	})

	t.Run("Should close as budget exhausted when backoff crosses the deadline", func(t *testing.T) {
		t.Parallel()

		firstScheduledAt := now.Add(-9 * time.Second)
		plan, err := planNodeRetry(&nodeRetryPlanInput{
			node: dsl.Node{
				ID: "fetch", Class: dsl.NodeClassAction, Kind: "http",
				NodeLifecycleState: &dsl.NodeLifecycleState{Deadline: "10s"},
			},
			effective:   effective,
			output:      GenerationOutput{Attempt: 1, FirstScheduledAt: &firstScheduledAt},
			failure:     transport,
			now:         now,
			randFloat64: func() float64 { return 0 },
		})
		if err != nil {
			t.Fatalf("planNodeRetry() error = %v", err)
		}
		if plan.retry || plan.failure.Class != FailureBudgetExhausted {
			t.Fatalf("planNodeRetry() = %#v, want budget exhaustion", plan)
		}
	})

	t.Run("Should reject a retry deadline without a persisted first schedule time", func(t *testing.T) {
		t.Parallel()

		_, err := planNodeRetry(&nodeRetryPlanInput{
			node: dsl.Node{
				ID: "fetch", Class: dsl.NodeClassAction, Kind: "http",
				NodeLifecycleState: &dsl.NodeLifecycleState{Deadline: "10s"},
			},
			effective: effective,
			output:    GenerationOutput{Attempt: 1},
			failure:   transport,
			now:       now,
		})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("planNodeRetry() error = %v, want ErrValidation", err)
		}
	})
}

func TestEffectiveActionRunLimitShouldSeparateAttemptTimeoutAndTotalDeadline(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 2, 19, 0, 0, 0, time.UTC)
	effective := EffectiveConfig{Lifecycle: DefaultLifecycleConfig()}

	tests := []struct {
		name         string
		node         dsl.Node
		first        time.Time
		wantDuration time.Duration
		wantReason   string
	}{
		{
			name:  "attempt timeout wins while total budget remains",
			node:  dsl.Node{Timeout: "5s", NodeLifecycleState: &dsl.NodeLifecycleState{Deadline: "2m"}},
			first: now.Add(-time.Minute), wantDuration: 5 * time.Second,
			wantReason: string(ReasonCodeActionTimeout),
		},
		{
			name:  "total deadline caps a longer attempt",
			node:  dsl.Node{Timeout: "30s", NodeLifecycleState: &dsl.NodeLifecycleState{Deadline: "2m"}},
			first: now.Add(-100 * time.Second), wantDuration: 20 * time.Second,
			wantReason: string(FailureBudgetExhausted),
		},
		{
			name:  "total deadline works without an attempt timeout",
			node:  dsl.Node{NodeLifecycleState: &dsl.NodeLifecycleState{Deadline: "2m"}},
			first: now.Add(-90 * time.Second), wantDuration: 30 * time.Second,
			wantReason: string(FailureBudgetExhausted),
		},
		{
			name:  "expired total deadline wins deterministically",
			node:  dsl.Node{Timeout: "5s", NodeLifecycleState: &dsl.NodeLifecycleState{Deadline: "5s"}},
			first: now.Add(-5 * time.Second), wantDuration: time.Nanosecond,
			wantReason: string(FailureBudgetExhausted),
		},
	}
	for _, test := range tests {
		t.Run("Should select "+test.name, func(t *testing.T) {
			t.Parallel()
			limit, err := effectiveActionRunLimit(
				test.node,
				effective,
				GenerationOutput{FirstScheduledAt: &test.first},
				now,
			)
			if err != nil {
				t.Fatalf("effectiveActionRunLimit() error = %v", err)
			}
			if !limit.enabled || limit.duration != test.wantDuration || limit.reasonCode != test.wantReason {
				t.Fatalf(
					"effectiveActionRunLimit() = %#v, want duration=%s reason=%q",
					limit,
					test.wantDuration,
					test.wantReason,
				)
			}
		})
	}
}
