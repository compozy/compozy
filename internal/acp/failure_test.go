package acp

import (
	"fmt"
	execpkg "os/exec"
	"strings"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/store"
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
		{
			name: "Should classify peer disconnected before response as process exit",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"error": "peer disconnected before response"},
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
