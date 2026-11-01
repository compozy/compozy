package acp

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"
	diagcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/store"
)

// Invariant: disposable probe errors map onto the shared auth taxonomy with
// safe messages and no account payload. Owner: acp probe boundary (IT-004).
func TestClassifyProbeError(t *testing.T) {
	t.Parallel()

	authRequired := func() error {
		return WrapFailure(
			store.FailureProtocol,
			"ACP session/new failed",
			&acpsdk.RequestError{Code: -32000, Message: "authentication required"},
		)
	}

	t.Run("Should report readiness on success", func(t *testing.T) {
		t.Parallel()

		got := ClassifyProbeError("agy", nil)
		if got.State != authproviders.ProviderAuthStateAuthenticated {
			t.Fatalf("State = %q, want authenticated", got.State)
		}
		if got.Message != "ACP session creation succeeded." {
			t.Fatalf("Message = %q, want readiness confirmation", got.Message)
		}
	})

	t.Run("Should request local login on auth-required refusal", func(t *testing.T) {
		t.Parallel()

		got := ClassifyProbeError("agy", authRequired())
		if got.State != authproviders.ProviderAuthStateNeedsLogin {
			t.Fatalf("State = %q, want needs_login", got.State)
		}
		if got.Code != diagcontract.CodeProviderNotAuthenticated {
			t.Fatalf("Code = %q, want provider_not_authenticated", got.Code)
		}
		if got.Action != authproviders.ProviderFailureActionLogin {
			t.Fatalf("Action = %q, want login", got.Action)
		}
		if !strings.Contains(got.Message, "compozy provider auth login agy") {
			t.Fatalf("Message = %q, want explicit local login guidance", got.Message)
		}
	})

	t.Run("Should report a missing executable as install guidance", func(t *testing.T) {
		t.Parallel()

		got := ClassifyProbeError("agy", errors.Join(errors.New("launch"), exec.ErrNotFound))
		if got.State != authproviders.ProviderAuthStateMissingCLI {
			t.Fatalf("State = %q, want missing_cli", got.State)
		}
		if got.Code != diagcontract.CodeProviderCLIMissing {
			t.Fatalf("Code = %q, want provider_cli_missing", got.Code)
		}
	})

	t.Run("Should keep unavailable methods inspectable", func(t *testing.T) {
		t.Parallel()

		got := ClassifyProbeError(
			"agy",
			&AuthMethodUnavailableError{MethodID: "oauth-personal", Reason: AuthMethodUnavailableAbsent},
		)
		if got.State != authproviders.ProviderAuthStateUnknown {
			t.Fatalf("State = %q, want unknown", got.State)
		}
		if got.Code != diagcontract.CodeProviderClassificationUnknown {
			t.Fatalf("Code = %q, want provider_classification_unknown", got.Code)
		}
		if !strings.Contains(got.Message, "oauth-personal") {
			t.Fatalf("Message = %q, want method identity without frames", got.Message)
		}
	})

	t.Run("Should reuse shared failure needles", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name  string
			err   string
			state authproviders.ProviderAuthState
		}{
			{name: "rate limit", err: "session/new failed: HTTP 429 slow down", state: authproviders.ProviderAuthStateRateLimited},
			{name: "entitlement", err: "session/new failed: 403 not entitled", state: authproviders.ProviderAuthStatePermissionDenied},
			{name: "transport", err: "connection reset by peer", state: authproviders.ProviderAuthStateTransient},
		} {
			got := ClassifyProbeError("agy", errors.New(tc.err))
			if got.State != tc.state {
				t.Fatalf("%s State = %q, want %q", tc.name, got.State, tc.state)
			}
		}
	})

	t.Run("Should redact unknown failures", func(t *testing.T) {
		t.Parallel()

		got := ClassifyProbeError("agy", errors.New("boom token=secret-value"))
		if got.State != authproviders.ProviderAuthStateUnknown {
			t.Fatalf("State = %q, want unknown", got.State)
		}
		if strings.Contains(got.Message, "secret-value") || !strings.Contains(got.Message, "[REDACTED]") {
			t.Fatalf("Message = %q, want redacted failure", got.Message)
		}
	})
}

// Invariant: startup diagnostics retain recovery guidance through probes and persisted failures.
func TestNPXStartupDiagnostic(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"prepare", "initialize"} {
		t.Run("Should preserve "+stage+" guidance", func(t *testing.T) {
			t.Parallel()
			err := npxDiagnostic(stage, context.DeadlineExceeded)
			classification := ClassifyProbeError("custom", err)
			if classification.Code != diagcontract.CodeProviderTransientFailure ||
				classification.State != authproviders.ProviderAuthStateTransient {
				t.Fatalf("classification = %#v", classification)
			}
			failure, ok := FailureFromError(err, store.FailureHandshake)
			if !ok {
				t.Fatal("missing failure")
			}
			item, ok := NPXStartupDiagnostic(failure.Summary)
			if !ok || !strings.Contains(item.Message, "retry") {
				t.Fatalf("diagnostic = %#v, %t from %q", item, ok, failure.Summary)
			}
			if err := diagcontract.ValidateDiagnosticItem(item); err != nil {
				t.Fatal(err)
			}
		})
	}
}
