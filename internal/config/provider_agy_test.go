package config

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Invariant: builtin agy identity and platform launch command.
// Owner: config provider registry. Canonical suite: provider_test.go tables;
// this file owns the platform-parameterized agy rows (UT-001).
func TestAgyACPServerCommand(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		goos string
		want string
	}{
		{name: "Should select the Linux server command with an empty uid argument", goos: "linux", want: "agy_acp_server.par --uid="},
		{name: "Should select the macOS server command without launch arguments", goos: "darwin", want: "agy_acp_server.par"},
		{name: "Should select the Windows server executable", goos: "windows", want: "agy_acp_server.exe"},
		{name: "Should keep a missing-executable limitation off other targets", goos: "freebsd", want: "agy_acp_server.par --uid="},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := agyACPServerCommand(tc.goos); got != tc.want {
				t.Fatalf("agyACPServerCommand(%q) = %q, want %q", tc.goos, got, tc.want)
			}
		})
	}
}

// Invariant: builtin agy resolves without an overlay and carries native auth,
// filtered env, operator home, and the personal method with no static model.
// Owner: config provider registry (UT-001).
func TestBuiltinAgyProviderIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve agy without a custom provider block", func(t *testing.T) {
		t.Parallel()

		cfg := Config{}
		provider, err := cfg.ResolveProvider("agy")
		if err != nil {
			t.Fatalf("ResolveProvider(agy) error = %v", err)
		}
		if got, want := provider.DisplayName, "Google Antigravity"; got != want {
			t.Fatalf("DisplayName = %q, want %q", got, want)
		}
		if got, want := provider.Command, agyACPServerCommand(runtime.GOOS); got != want {
			t.Fatalf("Command = %q, want %q", got, want)
		}
		if strings.Contains(provider.Command, "/usr/local/bin") || strings.Contains(provider.Command, "agy --acp") {
			t.Fatalf("Command = %q, want official server without experiment paths", provider.Command)
		}
		if got := provider.EffectiveHarness(); got != ProviderHarnessACP {
			t.Fatalf("Harness = %q, want acp", got)
		}
		if got := provider.EffectiveAuthMode(); got != ProviderAuthModeNativeCLI {
			t.Fatalf("AuthMode = %q, want native_cli", got)
		}
		if got := provider.EffectiveEnvPolicy(); got != ProviderEnvPolicyFiltered {
			t.Fatalf("EnvPolicy = %q, want filtered", got)
		}
		if got := provider.EffectiveHomePolicy(); got != ProviderHomePolicyOperator {
			t.Fatalf("HomePolicy = %q, want operator", got)
		}
		if got, want := provider.EffectiveACPMethod(), "oauth-personal"; got != want {
			t.Fatalf("AuthACPMethod = %q, want %q", got, want)
		}
		if provider.Models.Default != "" || len(provider.Models.Curated) != 0 {
			t.Fatalf("Models = %#v, want no static model seed", provider.Models)
		}
		if strings.TrimSpace(provider.AuthLoginCmd) != "" || strings.TrimSpace(provider.AuthStatusCmd) != "" {
			t.Fatalf(
				"native commands = (%q, %q), want protocol-owned auth",
				provider.AuthLoginCmd,
				provider.AuthStatusCmd,
			)
		}
	})

	t.Run("Should leave the default provider unchanged", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		if cfg.Defaults.Provider == "agy" {
			t.Fatal("Defaults.Provider = agy, want no default-provider change")
		}
	})

	t.Run("Should let explicit agy overrides win", func(t *testing.T) {
		t.Parallel()

		cfg := Config{Providers: map[string]ProviderConfig{
			"agy": {Command: "/opt/agy/test-server --uid="},
		}}
		provider, err := cfg.ResolveProvider("agy")
		if err != nil {
			t.Fatalf("ResolveProvider(agy) error = %v", err)
		}
		if got, want := provider.Command, "/opt/agy/test-server --uid="; got != want {
			t.Fatalf("Command = %q, want %q", got, want)
		}
		if got, want := provider.EffectiveACPMethod(), "oauth-personal"; got != want {
			t.Fatalf("AuthACPMethod = %q, want inherited %q", got, want)
		}
	})

	t.Run("Should keep an independently named antigravity provider", func(t *testing.T) {
		t.Parallel()

		if got := CanonicalProviderName("antigravity"); got != "antigravity" {
			t.Fatalf("CanonicalProviderName(antigravity) = %q, want no agy alias", got)
		}
		cfg := Config{Providers: map[string]ProviderConfig{
			"antigravity": {Command: "custom-antigravity acp"},
		}}
		provider, err := cfg.ResolveProvider("antigravity")
		if err != nil {
			t.Fatalf("ResolveProvider(antigravity) error = %v", err)
		}
		if got, want := provider.Command, "custom-antigravity acp"; got != want {
			t.Fatalf("Command = %q, want %q", got, want)
		}
		if provider.EffectiveACPMethod() != "" {
			t.Fatalf("AuthACPMethod = %q, want no inherited agy method", provider.AuthACPMethod)
		}
	})
}

// Invariant: explicit full access plus approve-all prefers agy yolo and
// nothing else widens the mode. Owner: config agent resolution (UT-004).
func TestResolveAgentAppliesAgyFullAccessPreference(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	cfg := DefaultWithHome(homePaths)
	cfg.Permissions.ProviderFullAccess = true

	t.Run("Should select yolo for agy under explicit full access", func(t *testing.T) {
		t.Parallel()

		resolved, err := cfg.ResolveAgent(AgentDef{Name: "qa", Provider: "agy", Prompt: "prompt"})
		if err != nil {
			t.Fatalf("ResolveAgent() error = %v", err)
		}
		options := resolved.ACPOptionsValue()
		if len(options) != 1 || options[0].ID != "mode" || options[0].ValueID != "yolo" {
			t.Fatalf("ResolveAgent() ACPOptions = %#v, want mode yolo", options)
		}
	})

	t.Run("Should let an explicit narrower agy mode win", func(t *testing.T) {
		t.Parallel()

		agent := AgentDef{Name: "qa", Provider: "agy", Prompt: "prompt"}
		agent.SetACPOptions([]ACPOptionSelection{{ID: "mode", ValueID: "read-only"}})
		resolved, err := cfg.ResolveAgent(agent)
		if err != nil {
			t.Fatalf("ResolveAgent() error = %v", err)
		}
		options := resolved.ACPOptionsValue()
		if len(options) != 1 || options[0].ValueID != "read-only" {
			t.Fatalf("ResolveAgent() ACPOptions = %#v, want explicit read-only mode", options)
		}
	})

	t.Run("Should keep normal negotiation without approve-all", func(t *testing.T) {
		t.Parallel()

		resolved, err := cfg.ResolveAgent(
			AgentDef{Name: "qa", Provider: "agy", Permissions: "approve-reads", Prompt: "prompt"},
		)
		if err != nil {
			t.Fatalf("ResolveAgent() error = %v", err)
		}
		if options := resolved.ACPOptionsValue(); len(options) != 0 {
			t.Fatalf("ResolveAgent() ACPOptions = %#v, want no widened mode", options)
		}
	})
}

// Invariant: TOML overlays round-trip the agy command override while the
// builtin method shows through, and unrelated custom names stay independent.
// Owner: config load/overlay (IT-006).
func TestAgyConfigOverlayRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("Should inherit the builtin method under an explicit command overlay", func(t *testing.T) {
		t.Parallel()

		overlay, err := loadConfigOverlayBytes([]byte(`
[providers.agy]
command = "/opt/agy/test-server --uid="
`), "inline")
		if err != nil {
			t.Fatalf("loadConfigOverlayBytes() error = %v", err)
		}
		cfg := Config{}
		if err := overlay.Apply(&cfg); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		provider, err := cfg.ResolveProvider("agy")
		if err != nil {
			t.Fatalf("ResolveProvider(agy) error = %v", err)
		}
		if got, want := provider.Command, "/opt/agy/test-server --uid="; got != want {
			t.Fatalf("Command = %q, want %q", got, want)
		}
		if got, want := provider.EffectiveACPMethod(), "oauth-personal"; got != want {
			t.Fatalf("AuthACPMethod = %q, want inherited %q", got, want)
		}
		if cfg.Defaults.Provider == "agy" {
			t.Fatal("Defaults.Provider = agy, want no default-provider change")
		}
	})
}
