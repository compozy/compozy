package config

import (
	"path/filepath"
	"strings"
	"testing"
)

// Invariant: builtin agy resolves without an overlay, launches the native CLI
// with its acp argument, and carries native auth, filtered env, operator home,
// the personal method, and native file tools with no static model. Owner:
// config provider registry (UT-001).
func TestBuiltinAgyProviderIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve the native CLI without a custom provider block", func(t *testing.T) {
		t.Parallel()

		cfg := Config{}
		provider, err := cfg.ResolveProvider("agy")
		if err != nil {
			t.Fatalf("ResolveProvider(agy) error = %v", err)
		}
		if got, want := provider.DisplayName, "Google Antigravity"; got != want {
			t.Fatalf("DisplayName = %q, want %q", got, want)
		}
		if got, want := provider.Command, "agy acp"; got != want {
			t.Fatalf("Command = %q, want %q", got, want)
		}
		for _, obsolete := range []string{"agy_acp_server", ".par", ".exe", "--uid=", "agy --acp"} {
			if strings.Contains(provider.Command, obsolete) {
				t.Fatalf("Command = %q, want no %q standalone launch", provider.Command, obsolete)
			}
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
		if !provider.DisableACPClientFS {
			t.Fatal("DisableACPClientFS = false, want true")
		}
		if got := provider.Models.Reasoning.Apply; got != ReasoningApplyACPOption {
			t.Fatalf("Models.Reasoning.Apply = %q, want %q", got, ReasoningApplyACPOption)
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
			"agy": {Command: "/opt/agy/bin/agy acp --process-per-session"},
		}}
		provider, err := cfg.ResolveProvider("agy")
		if err != nil {
			t.Fatalf("ResolveProvider(agy) error = %v", err)
		}
		if got, want := provider.Command, "/opt/agy/bin/agy acp --process-per-session"; got != want {
			t.Fatalf("Command = %q, want %q", got, want)
		}
		if got, want := provider.EffectiveACPMethod(), "oauth-personal"; got != want {
			t.Fatalf("AuthACPMethod = %q, want inherited %q", got, want)
		}
		if !provider.DisableACPClientFS {
			t.Fatal("DisableACPClientFS = false, want inherited true")
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

// Invariant: config authors no ACP mode for agy at any permission level; the
// advertised-mode negotiation owns the choice, and explicit selections stay
// exactly as authored. Owner: config agent resolution (UT-002).
func TestResolveAgentLeavesAgyModeToNegotiation(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	cfg := DefaultWithHome(homePaths)

	for _, permissions := range []string{
		"",
		string(PermissionModeDenyAll),
		string(PermissionModeApproveReads),
		string(PermissionModeApproveEdits),
		string(PermissionModeAuto),
		string(PermissionModeApproveAll),
	} {
		t.Run("Should author no agy mode for permissions "+permissions, func(t *testing.T) {
			t.Parallel()

			resolved, err := cfg.ResolveAgent(
				AgentDef{Name: "qa", Provider: "agy", Permissions: permissions, Prompt: "prompt"},
			)
			if err != nil {
				t.Fatalf("ResolveAgent() error = %v", err)
			}
			if options := resolved.ACPOptionsValue(); len(options) != 0 {
				t.Fatalf("ResolveAgent() ACPOptions = %#v, want no injected mode", options)
			}
		})
	}

	for _, option := range []ACPOptionSelection{
		{ID: "mode", ValueID: "plan"},
		{ID: "mode", ValueID: "accept-edits"},
	} {
		t.Run("Should keep an explicit agy mode "+option.ValueID, func(t *testing.T) {
			t.Parallel()

			agent := AgentDef{Name: "qa", Provider: "agy", Prompt: "prompt"}
			agent.SetACPOptions([]ACPOptionSelection{option})
			resolved, err := cfg.ResolveAgent(agent)
			if err != nil {
				t.Fatalf("ResolveAgent() error = %v", err)
			}
			options := resolved.ACPOptionsValue()
			if len(options) != 1 || options[0] != option {
				t.Fatalf("ResolveAgent() ACPOptions = %#v, want the authored %#v", options, option)
			}
		})
	}

	t.Run("Should scope exact mode advertisement to the native runtime", func(t *testing.T) {
		t.Parallel()

		for _, provider := range []string{"agy", "AGY", " agy "} {
			if !RequiresAdvertisedACPMode(provider) {
				t.Fatalf("RequiresAdvertisedACPMode(%q) = false, want exact validation", provider)
			}
		}
		for _, provider := range []string{"codex", "claude", "copilot", "custom", ""} {
			if RequiresAdvertisedACPMode(provider) {
				t.Fatalf("RequiresAdvertisedACPMode(%q) = true, want the compatibility skip", provider)
			}
		}
	})

	t.Run("Should keep an authored agent or fallback command", func(t *testing.T) {
		t.Parallel()

		const authored = "/opt/agy/bin/agy acp --process-per-session"
		agent := AgentDef{Name: "qa", Provider: "agy", Command: authored, Prompt: "prompt"}
		resolved, err := cfg.ResolveAgent(agent)
		if err != nil {
			t.Fatalf("ResolveAgent() error = %v", err)
		}
		if got := resolved.Command; got != authored {
			t.Fatalf("Command = %q, want the authored agent command %q", got, authored)
		}

		const fallback = "/fallback/agy acp"
		fallbackResolved, err := cfg.ResolveSessionAgentWithRuntime(
			AgentDef{Name: "qa", Provider: "agy", Prompt: "prompt"},
			RuntimeOverrides{Command: fallback},
		)
		if err != nil {
			t.Fatalf("ResolveSessionAgentWithRuntime() error = %v", err)
		}
		if got := fallbackResolved.Command; got != fallback {
			t.Fatalf("Command = %q, want the fallback command %q", got, fallback)
		}
	})

	t.Run("Should leave a custom provider routed to agy without a mode", func(t *testing.T) {
		t.Parallel()

		cfg := Config{Providers: map[string]ProviderConfig{
			"agy-custom": {
				Command:         "custom-agy acp",
				Harness:         ProviderHarnessACP,
				RuntimeProvider: "agy",
			},
		}}
		resolved, err := cfg.ResolveAgent(
			AgentDef{Name: "qa", Provider: "agy-custom", Prompt: "prompt"},
		)
		if err != nil {
			t.Fatalf("ResolveAgent() error = %v", err)
		}
		if options := resolved.ACPOptionsValue(); len(options) != 0 {
			t.Fatalf("ResolveAgent() ACPOptions = %#v, want no agy-derived mode", options)
		}
	})
}

// Invariant: TOML overlays round-trip the agy command override while the
// builtin method shows through, and unrelated custom names stay independent.
// Owner: config load/overlay (UT-001).
func TestAgyConfigOverlayRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("Should inherit the builtin method under an explicit command overlay", func(t *testing.T) {
		t.Parallel()

		overlay, err := loadConfigOverlayBytes([]byte(`
[providers.agy]
command = "/opt/agy/bin/agy acp --process-per-session"
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
		if got, want := provider.Command, "/opt/agy/bin/agy acp --process-per-session"; got != want {
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

// Invariant: only the native Antigravity CLI ACP launch is recognizable from the
// authored command; standalone servers, wrappers, and non-ACP agy invocations stay
// outside the native-policy boundary, while path, case, env, and Windows shim
// variants of the native launch are still recognized. Owner: config launch policy.
func TestNativeAgyACPCommand(t *testing.T) {
	t.Parallel()

	native := []string{
		"agy acp",
		"agy ACP",
		"AGY acp",
		"/usr/local/share/antigravity/bin/agy acp",
		"AGY_HOME=/tmp agy acp",
		"agy acp --process-per-session",
		"agy --sandbox acp",
		"agy --flag=value acp",
		"C:/tools/agy.exe acp",
		"agy.cmd acp",
		"antigravity acp",
		"/opt/antigravity/bin/antigravity --gemini-dir=/tmp/g acp",
		"C:/tools/antigravity.exe acp",
	}
	for _, command := range native {
		t.Run("Should recognize native command "+command, func(t *testing.T) {
			t.Parallel()

			if !IsNativeAgyACPCommand(command) {
				t.Fatalf("IsNativeAgyACPCommand(%q) = false, want the native CLI launch", command)
			}
		})
	}
	for _, command := range []string{
		"",
		"agy",
		"agy --acp",
		"agy run acp",
		"agy run --sandbox acp",
		"agy_acp_server.par",
		"/opt/agy/bin/agy_acp_server.par --uid=1",
		"agy-wrapper acp",
		"antigravity",
		"antigravity-ide acp",
		"npx agy acp",
		"env AGY_HOME=/tmp agy acp",
		"bash -c 'agy acp'",
	} {
		t.Run("Should ignore non-native command "+command, func(t *testing.T) {
			t.Parallel()

			if IsNativeAgyACPCommand(command) {
				t.Fatalf("IsNativeAgyACPCommand(%q) = true, want a non-native launch", command)
			}
		})
	}
}
