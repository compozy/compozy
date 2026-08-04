package loop_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/loop"
)

func TestPredicateFailurePolicyShouldDistinguishContinuationAndRouting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		kind        loop.PredicateKind
		override    *loop.PredicateErrorPolicy
		wantPolicy  loop.PredicateErrorPolicy
		wantExit    bool
		wantFailure bool
	}{
		{
			name:       "Should fail a broken continuation open to an exit",
			kind:       loop.PredicateContinuation,
			wantPolicy: loop.PredicateErrorExit,
			wantExit:   true,
		},
		{
			name:        "Should fail a broken routing predicate closed",
			kind:        loop.PredicateRouting,
			wantPolicy:  loop.PredicateErrorRoute,
			wantFailure: true,
		},
		{
			name:        "Should honor an explicit continuation route override",
			kind:        loop.PredicateContinuation,
			override:    new(loop.PredicateErrorRoute),
			wantPolicy:  loop.PredicateErrorRoute,
			wantFailure: true,
		},
		{
			name:       "Should honor an explicit routing exit override",
			kind:       loop.PredicateRouting,
			override:   new(loop.PredicateErrorExit),
			wantPolicy: loop.PredicateErrorExit,
			wantExit:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := loop.ApplyPredicateFailurePolicy(
				"contract.stop_when",
				tt.kind,
				tt.override,
				errors.New("no such key"),
			)
			if err != nil {
				t.Fatalf("ApplyPredicateFailurePolicy() error = %v", err)
			}
			if result.Policy != tt.wantPolicy || (result.ExitReason != "") != tt.wantExit ||
				(result.Failure != nil) != tt.wantFailure {
				t.Fatalf(
					"result = %#v, want policy %q exit=%v failure=%v",
					result,
					tt.wantPolicy,
					tt.wantExit,
					tt.wantFailure,
				)
			}
			if result.Diagnostic.Predicate != "contract.stop_when" ||
				result.Diagnostic.Outcome != tt.wantPolicy || !strings.Contains(result.Diagnostic.Cause, "no such key") {
				t.Fatalf("diagnostic = %#v, want predicate, cause, and outcome", result.Diagnostic)
			}
			if result.Failure != nil &&
				(result.Failure.Class != loop.FailureAuthoring || result.Failure.RetryEligible) {
				t.Fatalf("failure = %#v, want non-retryable authoring class", result.Failure)
			}
		})
	}
}
