package acp

import (
	"context"
	"fmt"
	execpkg "os/exec"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

func TestFailureFromErrorClassifiesFatalPromptRequestErrorsAsProcessExit(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		err  error
	}{
		{
			name: "Should classify process exited guidance as process exit",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error: The Claude Agent process exited unexpectedly. Please start a new session.",
			},
		},
		{
			name: "Should classify session not found details as process exit",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"details": "Session not found"},
			},
		},
		{
			name: "Should classify resource not found details as process exit",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"details": "Resource not found: sess-dead"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			failure, ok := FailureFromError(tc.err, store.FailurePrompt)
			if !ok {
				t.Fatal("FailureFromError() ok = false, want true")
			}
			if got, want := failure.Kind, store.FailureProcess; got != want {
				t.Fatalf("FailureFromError() kind = %q, want %q", got, want)
			}
		})
	}
}

func TestFailureFromErrorClassifiesPeerDisconnectAsTransport(t *testing.T) {
	t.Parallel()

	t.Run("Should require process evidence before classifying a peer disconnect as a process exit", func(t *testing.T) {
		t.Parallel()

		failure, ok := FailureFromError(&acpsdk.RequestError{
			Code:    -32603,
			Message: "Internal error",
			Data:    map[string]any{"error": "peer disconnected before response"},
		}, store.FailurePrompt)
		if !ok {
			t.Fatal("FailureFromError() ok = false, want true")
		}
		if got, want := failure.Kind, store.FailureTransport; got != want {
			t.Fatalf("FailureFromError() kind = %q, want %q", got, want)
		}
	})
}

func TestFailureFromErrorPreservesGenericPromptErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should keep generic prompt request errors as prompt failures", func(t *testing.T) {
		t.Parallel()

		failure, ok := FailureFromError(&acpsdk.RequestError{
			Code:    -32603,
			Message: "Internal error",
			Data:    map[string]any{"details": "Tool invocation failed"},
		}, store.FailurePrompt)
		if !ok {
			t.Fatal("FailureFromError() ok = false, want true")
		}
		if got, want := failure.Kind, store.FailurePrompt; got != want {
			t.Fatalf("FailureFromError() kind = %q, want %q", got, want)
		}
	})
}

func TestProviderFailureDiagnosticFromErrorClassifiesProviderRecoveryActions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		message  string
		code     string
		action   ProviderFailureAction
		authMode compozyconfig.ProviderAuthMode
	}{
		{name: "Should aggregate rate-limit occurrences without making the session terminal", message: "HTTP 429 rate limit", code: ProviderErrorRateLimited, action: ProviderFailureActionRetry},
		{name: "Should aggregate authentication occurrences without making the session terminal", message: "HTTP 401 authentication required", code: ProviderErrorAuthRequired, action: ProviderFailureActionLogin},
		{name: "Should direct bound-secret authentication failures to credential binding", message: "HTTP 401 authentication required", code: ProviderErrorAuthRequired, action: ProviderFailureActionBindSecret, authMode: compozyconfig.ProviderAuthModeBoundSecret},
		{name: "Should direct no-auth provider failures to configuration inspection", message: "HTTP 401 authentication required", code: ProviderErrorAuthRequired, action: ProviderFailureActionInspect, authMode: compozyconfig.ProviderAuthModeNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			process := &AgentProcess{SessionID: "session", providerName: "provider-a", providerAuthMode: tc.authMode}
			at := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
			err := &acpsdk.RequestError{Code: -32603, Message: tc.message}
			first := process.promptErrorEvent(PromptRequest{TurnID: "turn-1"}, err, at)
			second := process.promptErrorEvent(PromptRequest{TurnID: "turn-2"}, err, at.Add(time.Minute))
			if first.Failure == nil || first.Failure.Kind != store.FailurePrompt ||
				second.Failure.Kind != store.FailurePrompt {
				t.Fatalf(
					"provider failures = %#v / %#v, want nonterminal prompt failures",
					first.Failure,
					second.Failure,
				)
			}
			if first.ProviderError == nil || second.ProviderError == nil {
				t.Fatal("provider diagnostic missing")
			}
			if first.ProviderError.OccurrenceCount != 1 || second.ProviderError.OccurrenceCount != 2 {
				t.Fatalf(
					"occurrences = %d / %d, want immutable first=1 and second=2",
					first.ProviderError.OccurrenceCount,
					second.ProviderError.OccurrenceCount,
				)
			}
			if !strings.Contains(second.Failure.Summary, "next_action="+string(tc.action)) {
				t.Fatalf("legacy summary contradicts scoped recovery: %s", second.Failure.Summary)
			}
			got := second.ProviderError
			if got.Code != tc.code || got.Provider != "provider-a" || got.NextAction != tc.action ||
				got.Guidance == "" {
				t.Fatalf("provider diagnostic = %#v", got)
			}
			if !got.FirstSeenAt.Equal(at) || !got.LastSeenAt.Equal(at.Add(time.Minute)) || second.TurnID != "turn-2" {
				t.Fatalf("provider occurrence context = %#v, turn = %s", got, second.TurnID)
			}
			other := (&AgentProcess{providerName: "provider-a"}).promptErrorEvent(PromptRequest{}, err, at)
			if other.ProviderError.OccurrenceCount != 1 {
				t.Fatal("provider count crossed process ownership")
			}
			canceled := process.promptErrorEvent(PromptRequest{}, context.Canceled, at)
			if canceled.ProviderError != nil || canceled.Failure.Kind != store.FailureCanceled {
				t.Fatalf("cancellation = %#v, want cancellation without provider recovery", canceled)
			}
		})
	}

	t.Parallel()

	testCases := []struct {
		name       string
		err        error
		wantKind   ProviderFailureKind
		wantAction ProviderFailureAction
	}{
		{
			name:       "Should classify missing native CLI as install CLI",
			err:        fmt.Errorf("launch provider: %w", execpkg.ErrNotFound),
			wantKind:   ProviderFailureMissingCLI,
			wantAction: ProviderFailureActionInstallCLI,
		},
		{
			name: "Should classify auth required request errors as login",
			err: &acpsdk.RequestError{
				Code:    -32000,
				Message: "Authentication required",
			},
			wantKind:   ProviderFailureUnauthenticated,
			wantAction: ProviderFailureActionLogin,
		},
		{
			name: "Should classify invalid API keys as login",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"error": "invalid API key"},
			},
			wantKind:   ProviderFailureUnauthenticated,
			wantAction: ProviderFailureActionLogin,
		},
		{
			name: "Should classify unknown models as change model",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"error": "model not found: gpt-does-not-exist"},
			},
			wantKind:   ProviderFailureInvalidModel,
			wantAction: ProviderFailureActionChangeModel,
		},
		{
			name: "Should classify unavailable models as change model",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"error": "model is not available in your region"},
			},
			wantKind:   ProviderFailureModelUnavailable,
			wantAction: ProviderFailureActionChangeModel,
		},
		{
			name: "Should classify entitlement failures as no retry",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"error": "403 forbidden: model entitlement required"},
			},
			wantKind:   ProviderFailurePermissionDenied,
			wantAction: ProviderFailureActionNoRetry,
		},
		{
			name: "Should classify provider quotas as retry",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"status": 429, "error": "rate limit exceeded"},
			},
			wantKind:   ProviderFailureRateLimited,
			wantAction: ProviderFailureActionRetry,
		},
		{
			name: "Should classify overloaded providers as retry",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"status": 529, "error": "provider overloaded"},
			},
			wantKind:   ProviderFailureTransient,
			wantAction: ProviderFailureActionRetry,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diagnostic, ok := ProviderFailureDiagnosticFromError(tc.err)
			if !ok {
				t.Fatal("ProviderFailureDiagnosticFromError() ok = false, want true")
			}
			if got := diagnostic.Kind; got != tc.wantKind {
				t.Fatalf("ProviderFailureDiagnosticFromError() kind = %q, want %q", got, tc.wantKind)
			}
			if got := diagnostic.Action; got != tc.wantAction {
				t.Fatalf("ProviderFailureDiagnosticFromError() action = %q, want %q", got, tc.wantAction)
			}
		})
	}
}

func TestFailureFromErrorAddsProviderRecoveryMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should add provider recovery metadata to public failure summary", func(t *testing.T) {
		t.Parallel()

		err := &acpsdk.RequestError{
			Code:    -32603,
			Message: "Internal error",
			Data:    map[string]any{"status": 429, "error": "rate limit exceeded"},
		}
		failure, ok := FailureFromError(err, store.FailurePrompt)
		if !ok {
			t.Fatal("FailureFromError() ok = false, want true")
		}
		if got, want := failure.Kind, store.FailurePrompt; got != want {
			t.Fatalf("FailureFromError() kind = %q, want %q", got, want)
		}
		for _, want := range []string{
			"provider_failure_kind=rate_limited",
			"next_action=retry",
			"guidance=retry after the provider recovers",
		} {
			if !strings.Contains(failure.Summary, want) {
				t.Fatalf("FailureFromError() summary = %q, want %q", failure.Summary, want)
			}
		}
	})
}

func TestFailureFromErrorClassifiesPromptCancellationRequestErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		err  error
	}{
		{
			name: "Should classify JSON-RPC cancellation code as cancellation",
			err: &acpsdk.RequestError{
				Code:    -32800,
				Message: "Request canceled",
				Data:    map[string]any{"error": "context canceled"},
			},
		},
		{
			name: "Should classify canceled request details as cancellation",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"error": "context canceled"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			failure, ok := FailureFromError(tc.err, store.FailurePrompt)
			if !ok {
				t.Fatal("FailureFromError() ok = false, want true")
			}
			if got, want := failure.Kind, store.FailureCanceled; got != want {
				t.Fatalf("FailureFromError() kind = %q, want %q", got, want)
			}
		})
	}
}
