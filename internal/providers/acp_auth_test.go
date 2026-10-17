package providers

import (
	"encoding/json"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerauth"
)

// Invariant: native commands own their operation independently; the protocol
// method is selected only when the respective native command is absent.
// Owner: providers auth strategy (UT-003).
func TestACPOperationSelection(t *testing.T) {
	t.Parallel()

	nativeBoth := compozyconfig.ProviderConfig{
		Command:       "provider-cli acp",
		AuthLoginCmd:  "provider-cli login",
		AuthStatusCmd: "provider-cli auth status",
		AuthACPMethod: "oauth-personal",
	}
	nativeLoginOnly := compozyconfig.ProviderConfig{
		Command:       "provider-cli acp",
		AuthLoginCmd:  "provider-cli login",
		AuthACPMethod: "oauth-personal",
	}
	nativeStatusOnly := compozyconfig.ProviderConfig{
		Command:       "provider-cli acp",
		AuthStatusCmd: "provider-cli auth status",
		AuthACPMethod: "oauth-personal",
	}
	methodOnly := compozyconfig.ProviderConfig{
		Command:       "agy_acp_server.par --uid=",
		AuthACPMethod: "oauth-personal",
	}
	neither := compozyconfig.ProviderConfig{Command: "provider-cli acp"}

	for _, tc := range []struct {
		name      string
		provider  compozyconfig.ProviderConfig
		wantLogin bool
		wantProbe bool
	}{
		{name: "Should prefer both native commands over the method", provider: nativeBoth},
		{name: "Should prefer native login and probe through the method", provider: nativeLoginOnly, wantProbe: true},
		{name: "Should log in through the method and prefer native status", provider: nativeStatusOnly, wantLogin: true},
		{name: "Should select the method for both operations", provider: methodOnly, wantLogin: true, wantProbe: true},
		{name: "Should select neither path without commands or method", provider: neither},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := UsesACPLogin(tc.provider); got != tc.wantLogin {
				t.Fatalf("UsesACPLogin() = %t, want %t", got, tc.wantLogin)
			}
			if got := UsesACPProbe(tc.provider); got != tc.wantProbe {
				t.Fatalf("UsesACPProbe() = %t, want %t", got, tc.wantProbe)
			}
		})
	}
}

// Invariant: protocol descriptors carry source auth_acp_method with the
// launch executable basename and presence only.
// Owner: providers safe login metadata (UT-003).
func TestDescribeACPMethodLogin(t *testing.T) {
	t.Parallel()

	provider := compozyconfig.ProviderConfig{
		Command:       "agy_acp_server.par --uid=",
		Harness:       compozyconfig.ProviderHarnessACP,
		AuthACPMethod: "oauth-personal",
	}

	t.Run("Should describe the method source with presence only", func(t *testing.T) {
		t.Parallel()

		descriptor := DescribeProviderLogin(provider, &providerauth.NativeCLIStatus{
			Command: "agy_acp_server.par",
			Present: true,
			Path:    "/test/bin/agy_acp_server.par",
			Source:  providerauth.NativeCLISourceCommand,
		}, Classification{State: ProviderAuthStateUnknown})
		if !descriptor.Configured || descriptor.Source != providerauth.NativeCLISourceAuthACPMethod {
			t.Fatalf("DescribeProviderLogin() = %#v, want configured auth_acp_method source", descriptor)
		}
		if got, want := descriptor.Executable, "agy_acp_server.par"; got != want {
			t.Fatalf("Executable = %q, want %q", got, want)
		}
		if got, want := descriptor.Presence, ProviderLoginPresencePresent; got != want {
			t.Fatalf("Presence = %q, want %q", got, want)
		}
		encoded, err := json.Marshal(descriptor)
		if err != nil {
			t.Fatalf("json.Marshal(descriptor) error = %v", err)
		}
		for _, forbidden := range []string{"--uid=", "/test/bin", "oauth-personal"} {
			if strings.Contains(string(encoded), forbidden) {
				t.Fatalf("descriptor = %s, leaked %q", encoded, forbidden)
			}
		}
	})

	t.Run("Should report a missing launch executable without a path", func(t *testing.T) {
		t.Parallel()

		descriptor := DescribeProviderLogin(provider, &providerauth.NativeCLIStatus{
			Command: "agy_acp_server.par",
			Source:  providerauth.NativeCLISourceCommand,
		}, Classification{State: ProviderAuthStateMissingCLI})
		if got, want := descriptor.Presence, ProviderLoginPresenceMissing; got != want {
			t.Fatalf("Presence = %q, want %q", got, want)
		}
		if got, want := string(descriptor.RecommendedAction), string(ProviderFailureActionInstallCLI); got != want {
			t.Fatalf("RecommendedAction = %q, want %q", got, want)
		}
		if descriptor.Executable != "agy_acp_server.par" {
			t.Fatalf("Executable = %q, want launch basename", descriptor.Executable)
		}
	})

	t.Run("Should stay unconfigured without a login command or method", func(t *testing.T) {
		t.Parallel()

		descriptor := DescribeProviderLogin(
			compozyconfig.ProviderConfig{Command: "provider-cli acp"},
			nil,
			Classification{State: ProviderAuthStateUnknown},
		)
		if descriptor.Configured || descriptor.Source != "" || descriptor.Executable != "" {
			t.Fatalf("DescribeProviderLogin() = %#v, want unconfigured descriptor", descriptor)
		}
	})

	t.Run("Should keep native login precedence with a configured command", func(t *testing.T) {
		t.Parallel()

		withCommand := provider
		withCommand.AuthLoginCmd = "agy login"
		descriptor := DescribeProviderLogin(withCommand, &providerauth.NativeCLIStatus{
			Command: "agy",
			Present: true,
			Source:  providerauth.NativeCLISourceAuthLogin,
		}, Classification{State: ProviderAuthStateNeedsLogin})
		if got, want := descriptor.Source, providerauth.NativeCLISourceAuthLogin; got != want {
			t.Fatalf("Source = %q, want %q", got, want)
		}
		if got, want := descriptor.Executable, "agy"; got != want {
			t.Fatalf("Executable = %q, want %q", got, want)
		}
	})
}

// Invariant: shared free-text needles classify ACP probe errors without
// account or frame details. Owner: providers classifier (IT-004 boundary).
func TestClassifyErrorText(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		text  string
		state ProviderAuthState
	}{
		{name: "Should map rate limiting", text: "ACP session/new failed: HTTP 429 too many requests", state: ProviderAuthStateRateLimited},
		{name: "Should map entitlement denial", text: "ACP session/new failed: 403 not entitled", state: ProviderAuthStatePermissionDenied},
		{name: "Should map transport failure", text: "connection reset by peer", state: ProviderAuthStateTransient},
		{name: "Should map unauthorized output", text: "HTTP 401 unauthorized", state: ProviderAuthStateNeedsLogin},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ClassifyErrorText(tc.text)
			if !ok {
				t.Fatalf("ClassifyErrorText(%q) unclassified, want %q", tc.text, tc.state)
			}
			if got.State != tc.state {
				t.Fatalf("State = %q, want %q", got.State, tc.state)
			}
		})
	}

	t.Run("Should leave unrelated text unclassified", func(t *testing.T) {
		t.Parallel()

		if _, ok := ClassifyErrorText("helper exited with status 1"); ok {
			t.Fatal("ClassifyErrorText() classified generic text")
		}
	})
}
