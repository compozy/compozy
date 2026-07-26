package providers

import (
	"context"
	"os/exec"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	diagcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/testutil"
)

func TestClassifyProviderAuth(t *testing.T) {
	t.Parallel()

	t.Run("Should return authenticated when status probe succeeds", func(t *testing.T) {
		t.Parallel()

		got := ClassifyProbeResult(nativeProvider(), ProbeOutcome{ExitCode: 0, Stdout: "logged in"}, presentEnv())
		if got.State != ProviderAuthStateAuthenticated {
			t.Fatalf("State = %q, want %q", got.State, ProviderAuthStateAuthenticated)
		}
		if got.Code != "" {
			t.Fatalf("Code = %q, want empty", got.Code)
		}
	})

	t.Run("Should return needs login when probe reports unauthorized", func(t *testing.T) {
		t.Parallel()

		got := ClassifyProbeResult(
			nativeProvider(),
			ProbeOutcome{ExitCode: 1, Stderr: "HTTP 401 unauthorized"},
			presentEnv(),
		)
		if got.State != ProviderAuthStateNeedsLogin {
			t.Fatalf("State = %q, want %q", got.State, ProviderAuthStateNeedsLogin)
		}
		if got.Code != diagcontract.CodeProviderNotAuthenticated {
			t.Fatalf("Code = %q, want %q", got.Code, diagcontract.CodeProviderNotAuthenticated)
		}
	})

	t.Run("Should return missing CLI when lookup fails", func(t *testing.T) {
		t.Parallel()

		env := presentEnv()
		env.LookPath = func(string) (string, error) {
			return "", exec.ErrNotFound
		}
		got := ClassifyProbeResult(nativeProvider(), ProbeOutcome{ExitCode: 1}, env)
		if got.State != ProviderAuthStateMissingCLI {
			t.Fatalf("State = %q, want %q", got.State, ProviderAuthStateMissingCLI)
		}
		if got.Code != diagcontract.CodeProviderCLIMissing {
			t.Fatalf("Code = %q, want %q", got.Code, diagcontract.CodeProviderCLIMissing)
		}
	})

	t.Run("Should return missing credential when required bound secret is absent", func(t *testing.T) {
		t.Parallel()

		provider := aghconfig.ProviderConfig{
			AuthMode: aghconfig.ProviderAuthModeBoundSecret,
			CredentialSlots: []aghconfig.ProviderCredentialSlot{
				{Name: "api_key", TargetEnv: "TEST_API_KEY", SecretRef: "env:TEST_API_KEY", Required: true},
			},
		}
		got, err := ClassifyDeclared(testutil.Context(t), provider, &ProbeEnv{
			LookupEnv: func(string) (string, bool) { return "", false },
		})
		if err != nil {
			t.Fatalf("ClassifyDeclared() error = %v", err)
		}
		if got.State != ProviderAuthStateMissingCredential {
			t.Fatalf("State = %q, want %q", got.State, ProviderAuthStateMissingCredential)
		}
		if got.Code != diagcontract.CodeProviderCredentialUnresolved {
			t.Fatalf("Code = %q, want %q", got.Code, diagcontract.CodeProviderCredentialUnresolved)
		}
	})

	t.Run("Should return explicit none when auth mode is none", func(t *testing.T) {
		t.Parallel()

		got, err := ClassifyDeclared(context.Background(), aghconfig.ProviderConfig{
			AuthMode: aghconfig.ProviderAuthModeNone,
		}, nil)
		if err != nil {
			t.Fatalf("ClassifyDeclared(none) error = %v", err)
		}
		if got.State != ProviderAuthStateNone {
			t.Fatalf("State = %q, want %q", got.State, ProviderAuthStateNone)
		}
		if got.Message != "No auth required." {
			t.Fatalf("Message = %q, want no-auth wording", got.Message)
		}
	})

	t.Run("Should return rate limited before generic auth failures", func(t *testing.T) {
		t.Parallel()

		got := ClassifyProbeResult(nativeProvider(), ProbeOutcome{
			ExitCode: 1,
			Stderr:   "HTTP 429 unauthorized quota exhausted",
		}, presentEnv())
		if got.State != ProviderAuthStateRateLimited {
			t.Fatalf("State = %q, want %q", got.State, ProviderAuthStateRateLimited)
		}
		if got.Code != diagcontract.CodeProviderRateLimited {
			t.Fatalf("Code = %q, want %q", got.Code, diagcontract.CodeProviderRateLimited)
		}
	})

	t.Run("Should return permission denied for forbidden probe output", func(t *testing.T) {
		t.Parallel()

		got := ClassifyProbeResult(
			nativeProvider(),
			ProbeOutcome{ExitCode: 1, Stderr: "HTTP 403 forbidden"},
			presentEnv(),
		)
		if got.State != ProviderAuthStatePermissionDenied {
			t.Fatalf("State = %q, want %q", got.State, ProviderAuthStatePermissionDenied)
		}
	})

	t.Run("Should return transient for timeout output", func(t *testing.T) {
		t.Parallel()

		got := ClassifyProbeResult(
			nativeProvider(),
			ProbeOutcome{ExitCode: 1, Stderr: "connection refused"},
			presentEnv(),
		)
		if got.State != ProviderAuthStateTransient {
			t.Fatalf("State = %q, want %q", got.State, ProviderAuthStateTransient)
		}
	})

	t.Run("Should return unknown for unrecognized probe output", func(t *testing.T) {
		t.Parallel()

		got := ClassifyProbeResult(
			nativeProvider(),
			ProbeOutcome{ExitCode: 1, Stderr: "unexpected provider text"},
			presentEnv(),
		)
		if got.State != ProviderAuthStateUnknown {
			t.Fatalf("State = %q, want %q", got.State, ProviderAuthStateUnknown)
		}
		if got.Code != diagcontract.CodeProviderClassificationUnknown {
			t.Fatalf("Code = %q, want %q", got.Code, diagcontract.CodeProviderClassificationUnknown)
		}
	})
}

func nativeProvider() aghconfig.ProviderConfig {
	return aghconfig.ProviderConfig{
		Command:       "provider-cli acp",
		AuthMode:      aghconfig.ProviderAuthModeNativeCLI,
		AuthStatusCmd: "provider-cli auth status",
		AuthLoginCmd:  "provider-cli login",
	}
}

func presentEnv() *ProbeEnv {
	return &ProbeEnv{
		ProviderName: "test",
		LookPath: func(name string) (string, error) {
			return "/bin/" + name, nil
		},
	}
}
