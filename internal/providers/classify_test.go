package providers

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	diagcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/providerauth"
	"github.com/compozy/compozy/internal/testutil"
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

	t.Run("Should treat the Hermes ACP import check as preflight success", func(t *testing.T) {
		t.Parallel()

		provider := compozyconfig.ProviderConfig{
			Command:       "hermes acp",
			AuthStatusCmd: "hermes acp --check",
		}
		got := ClassifyProbeResult(
			provider,
			ProbeOutcome{ExitCode: 0, Stdout: "Hermes ACP check OK"},
			presentEnv(),
		)
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
		resolveCalls := 0
		env.LookPath = func(string) (string, error) {
			resolveCalls++
			if resolveCalls == 1 {
				return "", exec.ErrNotFound
			}
			return "/later/bin/provider-cli", nil
		}
		declared, err := ClassifyDeclared(testutil.Context(t), nativeProvider(), env)
		if err != nil {
			t.Fatalf("ClassifyDeclared() error = %v", err)
		}
		if declared.State != ProviderAuthStateMissingCLI {
			t.Fatalf("declared State = %q, want %q", declared.State, ProviderAuthStateMissingCLI)
		}
		got := ClassifyProbeResult(
			nativeProvider(),
			ProbeOutcome{ExitCode: 0, Stdout: "logged in"},
			env,
		)
		if got.State != ProviderAuthStateMissingCLI {
			t.Fatalf("State = %q, want %q", got.State, ProviderAuthStateMissingCLI)
		}
		if got.Code != diagcontract.CodeProviderCLIMissing {
			t.Fatalf("Code = %q, want %q", got.Code, diagcontract.CodeProviderCLIMissing)
		}
		if got.Action != ProviderFailureActionInstallCLI {
			t.Fatalf("Action = %q, want %q", got.Action, ProviderFailureActionInstallCLI)
		}
		if resolveCalls != 1 {
			t.Fatalf("status resolver calls = %d, want 1", resolveCalls)
		}
	})

	t.Run("Should keep an operational lookup failure unknown through final classification", func(t *testing.T) {
		t.Parallel()

		resolverErr := errors.New("resolver failed token=classification-secret")
		env := presentEnv()
		resolveCalls := 0
		env.LookPath = func(string) (string, error) {
			resolveCalls++
			if resolveCalls == 1 {
				return "", resolverErr
			}
			return "/later/bin/provider-cli", nil
		}
		declared, err := ClassifyDeclared(testutil.Context(t), nativeProvider(), env)
		if err != nil {
			t.Fatalf("ClassifyDeclared() error = %v", err)
		}
		got := ClassifyProbeResult(
			nativeProvider(),
			ProbeOutcome{ExitCode: 0, Stdout: "logged in"},
			env,
		)
		for name, classification := range map[string]Classification{
			"declared": declared,
			"final":    got,
		} {
			if classification.State != ProviderAuthStateUnknown {
				t.Fatalf("%s State = %q, want %q", name, classification.State, ProviderAuthStateUnknown)
			}
			if classification.Code != diagcontract.CodeProviderClassificationUnknown {
				t.Fatalf(
					"%s Code = %q, want %q",
					name,
					classification.Code,
					diagcontract.CodeProviderClassificationUnknown,
				)
			}
			if classification.Action != ProviderFailureActionInspect {
				t.Fatalf("%s Action = %q, want %q", name, classification.Action, ProviderFailureActionInspect)
			}
			if strings.Contains(classification.Message, "classification-secret") {
				t.Fatalf("%s Message = %q, leaked resolver secret", name, classification.Message)
			}
		}
		if resolveCalls != 1 {
			t.Fatalf("status resolver calls = %d, want 1", resolveCalls)
		}
	})

	t.Run("Should return missing credential when required bound secret is absent", func(t *testing.T) {
		t.Parallel()

		provider := compozyconfig.ProviderConfig{
			AuthMode: compozyconfig.ProviderAuthModeBoundSecret,
			CredentialSlots: []compozyconfig.ProviderCredentialSlot{
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

		got, err := ClassifyDeclared(context.Background(), compozyconfig.ProviderConfig{
			AuthMode: compozyconfig.ProviderAuthModeNone,
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

	t.Run("Should include bounded redacted stderr for unknown preflight failures", func(t *testing.T) {
		t.Parallel()

		got := ClassifyProbeResult(
			nativeProvider(),
			ProbeOutcome{ExitCode: 1, Stderr: "MCP setup failed Authorization: Bearer preflight-secret"},
			presentEnv(),
		)
		if !strings.Contains(got.Message, "MCP setup failed") {
			t.Fatalf("Message = %q, want provider stderr detail", got.Message)
		}
		if strings.Contains(got.Message, "preflight-secret") || !strings.Contains(got.Message, "[REDACTED]") {
			t.Fatalf("Message = %q, want redacted provider stderr", got.Message)
		}
	})

	t.Run("Should describe login readiness without retaining command details", func(t *testing.T) {
		t.Parallel()

		provider := nativeProvider()
		provider.AuthLoginCmd = `QA_LOGIN_TOKEN=raw-env-secret "/opt/provider bin/provider-cli" login --token raw-login-secret`
		nativeCLI, err := providerauth.NativeCLIStatusForCommand(
			provider.AuthLoginCmd,
			providerauth.NativeCLISourceAuthLogin,
			func(command string) (string, error) {
				if got, want := command, "/opt/provider bin/provider-cli"; got != want {
					t.Fatalf("lookPath command = %q, want %q", got, want)
				}
				return `C:\Provider Bin\provider-cli.exe`, nil
			},
		)
		if err != nil {
			t.Fatalf("NativeCLIStatusForCommand() error = %v", err)
		}
		descriptor := DescribeProviderLogin(provider, nativeCLI, Classification{
			State:  ProviderAuthStateNeedsLogin,
			Action: ProviderFailureActionLogin,
		})
		if !descriptor.Configured || descriptor.Source != providerauth.NativeCLISourceAuthLogin {
			t.Fatalf("DescribeProviderLogin() = %#v, want configured auth_login source", descriptor)
		}
		if got, want := descriptor.Executable, "provider-cli"; got != want {
			t.Fatalf("Executable = %q, want %q", got, want)
		}
		if got, want := descriptor.Presence, ProviderLoginPresencePresent; got != want {
			t.Fatalf("Presence = %q, want %q", got, want)
		}
		if got, want := descriptor.RecommendedAction, ProviderFailureActionLogin; got != want {
			t.Fatalf("RecommendedAction = %q, want %q", got, want)
		}
		encoded, err := json.Marshal(descriptor)
		if err != nil {
			t.Fatalf("json.Marshal(descriptor) error = %v", err)
		}
		for _, forbidden := range []string{
			"QA_LOGIN_TOKEN",
			"raw-env-secret",
			"raw-login-secret",
			"/opt/provider",
			`C:\Provider Bin`,
			" login ",
		} {
			if strings.Contains(string(encoded), forbidden) {
				t.Fatalf("descriptor leaked %q: %s", forbidden, encoded)
			}
		}
	})
}

func nativeProvider() compozyconfig.ProviderConfig {
	return compozyconfig.ProviderConfig{
		Command:       "provider-cli acp",
		AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
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
