package loop

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	retrypkg "github.com/compozy/compozy/internal/retry"
)

type nodeRetryPolicy struct {
	maxAttempts  int
	baseDelay    time.Duration
	maxDelay     time.Duration
	deadline     *time.Duration
	nonRetryable map[string]struct{}
}

type nodeRetryPlanInput struct {
	node        dsl.Node
	effective   EffectiveConfig
	output      GenerationOutput
	failure     ClassifiedFailure
	attempts    []NodeAttempt
	now         time.Time
	randFloat64 func() float64
}

type nodeRetryPlan struct {
	retry         bool
	nextAttemptAt *time.Time
	failure       ClassifiedFailure
	delay         time.Duration
}

func planNodeRetry(input *nodeRetryPlanInput) (nodeRetryPlan, error) {
	if input == nil {
		return nodeRetryPlan{}, fmt.Errorf("%w: node retry input is required", ErrValidation)
	}
	policy, err := resolveNodeRetryPolicy(input.node, input.effective)
	if err != nil {
		return nodeRetryPlan{}, err
	}
	plan := nodeRetryPlan{failure: input.failure}
	if !nodeFailureRetryEligible(policy, input.output.Attempt, input.failure) {
		return plan, nil
	}
	delay := retryDelay(policy, input.failure, input.attempts, input.randFloat64)
	now := input.now.UTC()
	nextAttemptAt := now.Add(delay)
	if policy.deadline != nil && input.output.FirstScheduledAt != nil {
		deadlineAt := input.output.FirstScheduledAt.UTC().Add(*policy.deadline)
		if !nextAttemptAt.Before(deadlineAt) {
			plan.failure = ClassifyNodeFailure(FailureEvidence{
				BudgetExhausted: true,
				Code:            string(FailureBudgetExhausted),
				Cause:           "node retry backoff would exceed its deadline",
				Hint:            input.failure.Hint,
				Target:          input.failure.Target,
			})
			return plan, nil
		}
	}
	plan.retry = true
	plan.delay = delay
	plan.nextAttemptAt = &nextAttemptAt
	return plan, nil
}

func resolveNodeRetryPolicy(node dsl.Node, effective EffectiveConfig) (nodeRetryPolicy, error) {
	resolved, err := ResolveNodeLifecycleConfig(node, nil, effective.Lifecycle)
	if err != nil {
		return nodeRetryPolicy{}, err
	}
	policy := nodeRetryPolicy{
		maxAttempts:  resolved.RetryMaxAttempts,
		baseDelay:    resolved.RetryBackoffBase,
		maxDelay:     resolved.RetryBackoffMax,
		deadline:     resolved.Deadline,
		nonRetryable: make(map[string]struct{}, len(resolved.RetryNonRetryable)),
	}
	for _, value := range resolved.RetryNonRetryable {
		if normalized := strings.TrimSpace(value); normalized != "" {
			policy.nonRetryable[normalized] = struct{}{}
		}
	}
	if expensiveRetryFamily(node) && node.Retry == nil {
		policy.maxAttempts = 0
	}
	if dsl.ActionKind(node.Kind) == dsl.ActionGoal {
		policy.maxAttempts = 0
	}
	return policy, nil
}

func expensiveRetryFamily(node dsl.Node) bool {
	switch dsl.ActionKind(node.Kind) {
	case dsl.ActionRunAgent, dsl.ActionRunLoop:
		return true
	default:
		return false
	}
}

func nodeFailureRetryEligible(
	policy nodeRetryPolicy,
	attempt int,
	failure ClassifiedFailure,
) bool {
	if policy.maxAttempts <= 0 || attempt < 1 || attempt >= policy.maxAttempts || !failure.RetryEligible {
		return false
	}
	for _, value := range []string{string(failure.Class), strings.TrimSpace(failure.Code)} {
		if _, blocked := policy.nonRetryable[value]; blocked {
			return false
		}
	}
	return true
}

func retryDelay(
	policy nodeRetryPolicy,
	failure ClassifiedFailure,
	attempts []NodeAttempt,
	randFloat64 func() float64,
) time.Duration {
	if failure.RetryAfter != nil {
		return min(max(*failure.RetryAfter, policy.baseDelay), policy.maxDelay)
	}
	delayFor := retrypkg.DecorrelatedJitter(retrypkg.DecorrelatedJitterConfig{
		BaseDelay:   policy.baseDelay,
		MaxDelay:    policy.maxDelay,
		RandFloat64: randFloat64,
	})
	return delayFor(retrypkg.DelayInput{
		Attempt:  max(1, len(attempts)+1),
		Previous: previousRetryDelay(attempts),
	})
}

func previousRetryDelay(attempts []NodeAttempt) time.Duration {
	for _, attempt := range slices.Backward(attempts) {
		if attempt.Disposition != AttemptRetried || attempt.EndedAt == nil || attempt.NextAttemptAt == nil {
			continue
		}
		delay := attempt.NextAttemptAt.Sub(*attempt.EndedAt)
		if delay > 0 {
			return delay
		}
	}
	return 0
}

func validateRetrySchedule(output GenerationOutput) error {
	if output.Status != generationOutputRetrying || output.NextAttemptAt == nil {
		return fmt.Errorf("%w: retry schedule requires a retrying cell and due time", ErrValidation)
	}
	if output.Attempt < 1 || output.Epoch < 1 {
		return fmt.Errorf("%w: retry schedule requires a positive attempt and epoch", ErrValidation)
	}
	return nil
}
