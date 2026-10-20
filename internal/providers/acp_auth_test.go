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
		{name: "Should map a bare rate-limit status", text: "429", state: ProviderAuthStateRateLimited},
		{name: "Should map a status followed by punctuation", text: "HTTP 429.", state: ProviderAuthStateRateLimited},
		{name: "Should map a structured auth status", text: `status:401`, state: ProviderAuthStateNeedsLogin},
		{name: "Should map an overload status", text: `status=529`, state: ProviderAuthStateTransient},
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

	// Numeric substrings in echoed request identities are not HTTP statuses.
	for _, text := range []string{
		"helper exited with status 1",
		"acpmock: no turn matched raw_prompt session_id: sess-dfeb9b429b30bdab",
		"acpmock: no turn matched workspace: /tmp/TestAutoTitle_4290148401/003",
		"request failed for sess-401 and account-403 and model-529",
		"request failed for session_429 and 401.txt and /403/ and 5290",
	} {
		t.Run("Should leave unrelated text unclassified "+text, func(t *testing.T) {
			t.Parallel()
			if got, ok := ClassifyErrorText(text); ok {
				t.Fatalf("ClassifyErrorText(%q) = %#v, want unclassified", text, got)
			}
		})
	}
}
