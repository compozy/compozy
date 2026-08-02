package loop

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/tools"
)

func TestClassifyNodeFailureShouldApplyClosedPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		evidence FailureEvidence
		want     FailureClass
	}{
		{name: "Should classify transport", evidence: FailureEvidence{Transport: true}, want: FailureTransport},
		{name: "Should classify payload", evidence: FailureEvidence{}, want: FailurePayloadDeclared},
		{
			name:     "Should classify quality rejection",
			evidence: FailureEvidence{QualityRejected: true},
			want:     FailureQualityRejection,
		},
		{name: "Should classify authoring", evidence: FailureEvidence{Authoring: true}, want: FailureAuthoring},
		{
			name:     "Should classify cancellation",
			evidence: FailureEvidence{CancelRequested: true},
			want:     FailureCancellation,
		},
		{
			name:     "Should classify attempt timeout",
			evidence: FailureEvidence{AttemptTimedOut: true},
			want:     FailureAttemptTimeout,
		},
		{
			name:     "Should classify budget exhaustion",
			evidence: FailureEvidence{BudgetExhausted: true},
			want:     FailureBudgetExhausted,
		},
		{
			name:     "Should classify target unavailability",
			evidence: FailureEvidence{TargetUnavailable: true},
			want:     FailureTargetUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ClassifyNodeFailure(tt.evidence)
			if got.Class != tt.want {
				t.Fatalf("ClassifyNodeFailure() class = %q, want %q", got.Class, tt.want)
			}
			if got.RetryEligible != tt.want.RetryEligible() {
				t.Fatalf("RetryEligible = %v, want %v", got.RetryEligible, tt.want.RetryEligible())
			}
		})
	}

	t.Run("Should let cancellation win over payload", func(t *testing.T) {
		t.Parallel()

		payload := NewActionFailure(payloadDeclaredCode, "payload failed", "repair")
		got := ClassifyNodeFailure(FailureEvidence{CancelRequested: true, PayloadFailure: &payload})
		if got.Class != FailureCancellation || got.Code != string(FailureCancellation) ||
			got.Cause != "node execution was canceled" || got.Hint != "" {
			t.Fatalf("ClassifyNodeFailure() = %#v, want cancellation-owned details", got)
		}
	})
}

func TestClassifyNodeFailureShouldMapToolErrorsAndRetryHints(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		code tools.ErrorCode
		want FailureClass
	}{
		{name: "Should map unavailable", code: tools.ErrorCodeUnavailable, want: FailureTransport},
		{name: "Should map backend failed", code: tools.ErrorCodeBackendFailed, want: FailureTransport},
		{name: "Should map timed out", code: tools.ErrorCodeTimedOut, want: FailureTransport},
		{name: "Should map invalid input", code: tools.ErrorCodeInvalidInput, want: FailurePayloadDeclared},
		{name: "Should map denied", code: tools.ErrorCodeDenied, want: FailurePayloadDeclared},
		{name: "Should map canceled", code: tools.ErrorCodeCanceled, want: FailureCancellation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			toolErr := tools.NewToolError(tc.code, "compozy__test", "operator-safe", nil)
			got := ClassifyNodeFailure(FailureEvidence{ToolError: toolErr})
			if got.Class != tc.want || got.Code != string(tc.code) {
				t.Fatalf("ClassifyNodeFailure() = %#v, want class %q code %q", got, tc.want, tc.code)
			}
		})
	}

	t.Run("Should attribute an expired lease to its target", func(t *testing.T) {
		t.Parallel()

		got := ClassifyNodeFailure(FailureEvidence{LeaseExpired: true, Target: "mcp:github"})
		if got.Class != FailureTransport || got.Target != "mcp:github" {
			t.Fatalf("ClassifyNodeFailure() = %#v, want attributed transport failure", got)
		}
	})

	t.Run("Should clamp retry after without aliasing input", func(t *testing.T) {
		t.Parallel()

		retryAfter := 2 * time.Minute
		got := ClassifyNodeFailure(FailureEvidence{
			Transport:  true,
			RetryAfter: &retryAfter,
			BackoffMax: 30 * time.Second,
		})
		if got.RetryAfter == nil || *got.RetryAfter != 30*time.Second {
			t.Fatalf("RetryAfter = %v, want 30s", got.RetryAfter)
		}
		*got.RetryAfter = time.Second
		if retryAfter != 2*time.Minute {
			t.Fatalf("classifier retained input alias; source = %s", retryAfter)
		}
	})
}

func TestClassifyNodeFailureShouldBePureUnderConcurrentCalls(t *testing.T) {
	t.Parallel()
	t.Run("Should return the same owned result under concurrent calls", func(t *testing.T) {
		t.Parallel()

		retryAfter := 45 * time.Second
		evidence := FailureEvidence{
			Transport:  true,
			Code:       "network",
			Cause:      "connection reset",
			RetryAfter: &retryAfter,
		}
		want := ClassifyNodeFailure(evidence)
		const calls = 64
		results := make(chan ClassifiedFailure, calls)
		var group sync.WaitGroup
		for range calls {
			group.Go(func() {
				results <- ClassifyNodeFailure(evidence)
			})
		}
		group.Wait()
		close(results)
		for got := range results {
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("ClassifyNodeFailure() = %#v, want %#v", got, want)
			}
		}
	})
}
