package config

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/diagnosticcontract"
)

func loadEnabledTestConfig(t *testing.T, global string, opts ...LoadOption) (*Config, HomePaths) {
	t.Helper()
	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	writeFile(t, homePaths.ConfigFile, global)
	cfg, err := LoadForHome(homePaths, append([]LoadOption{withoutDotEnv()}, opts...)...)
	if err != nil {
		t.Fatalf("LoadForHome() error = %v", err)
	}
	return &cfg, homePaths
}

func defaultEnabledTestConfig(t *testing.T) *Config {
	t.Helper()
	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	cfg := DefaultWithHome(homePaths)
	return &cfg
}

func TestProviderConfigEffectiveEnabled(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		enabled *bool
		want    bool
	}{
		{name: "Should treat a missing value as enabled", enabled: nil, want: true},
		{name: "Should report an explicit false as disabled", enabled: new(false), want: false},
		{name: "Should report an explicit true as enabled", enabled: new(true), want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := (ProviderConfig{Enabled: tc.enabled}).EffectiveEnabled(); got != tc.want {
				t.Fatalf("EffectiveEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestConfigProviderEnabled(t *testing.T) {
	t.Parallel()

	t.Run("Should accept undefined disabled providers without admitting their sessions", func(t *testing.T) {
		t.Parallel()
		cfg, _ := loadEnabledTestConfig(t, "[providers.ghost]\nenabled = false\n")
		if err := cfg.RequireEnabledProvider("ghost"); !errors.Is(err, ErrProviderDisabled) {
			t.Fatalf("RequireEnabledProvider(ghost) = %v, want ErrProviderDisabled", err)
		}
		if _, err := cfg.ResolveProvider(
			"ghost",
		); !errors.Is(err, ErrProviderUnavailable) ||
			!strings.Contains(err.Error(), `provider "ghost" command is required`) {
			t.Fatalf("ResolveProvider(ghost) = %v, want unavailable missing command", err)
		}
		cfg.Providers["ghost"] = ProviderConfig{Enabled: new(true)}
		if err := cfg.Validate(); !errors.Is(err, ErrProviderUnavailable) ||
			!strings.Contains(err.Error(), `provider "ghost" command is required`) {
			t.Fatalf("Validate() = %v, want unavailable missing command", err)
		}
		cfg.Providers["ghost"] = ProviderConfig{Enabled: new(false)}
		cfg.Defaults.Provider = "ghost"
		if err := cfg.Validate(); !errors.Is(err, ErrProviderUnavailable) ||
			!strings.Contains(err.Error(), `provider "ghost" command is required`) {
			t.Fatalf("Validate() = %v, want unavailable missing default command", err)
		}
	})

	t.Run("Should report a disabled built-in loaded from config", func(t *testing.T) {
		t.Parallel()
		cfg, _ := loadEnabledTestConfig(t, "[providers.gemini]\nenabled = false\n")
		enabled, err := cfg.ProviderEnabled("gemini")
		if err != nil || enabled {
			t.Fatalf("ProviderEnabled(gemini) = (%v, %v), want (false, nil)", enabled, err)
		}
		enabled, err = cfg.ProviderEnabled("codex")
		if err != nil || !enabled {
			t.Fatalf("ProviderEnabled(codex) = (%v, %v), want (true, nil)", enabled, err)
		}
	})

	t.Run("Should report unknown providers as not enabled without an error", func(t *testing.T) {
		t.Parallel()
		cfg := defaultEnabledTestConfig(t)
		enabled, err := cfg.ProviderEnabled("no-such-provider")
		if err != nil || enabled {
			t.Fatalf("ProviderEnabled(unknown) = (%v, %v), want (false, nil)", enabled, err)
		}
	})

	t.Run("Should keep resolving an enabled-only built-in block", func(t *testing.T) {
		t.Parallel()
		cfg, _ := loadEnabledTestConfig(t, "[providers.gemini]\nenabled = false\n")
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		resolved, err := cfg.ResolveProvider("gemini")
		if err != nil {
			t.Fatalf("ResolveProvider(gemini) error = %v", err)
		}
		if strings.TrimSpace(resolved.Command) == "" || !resolved.CommandIsBuiltin {
			t.Fatalf("ResolveProvider(gemini) = %#v, want the built-in command", resolved)
		}
	})

	t.Run("Should not inherit enabled through runtime_provider aliases", func(t *testing.T) {
		t.Parallel()
		cfg, _ := loadEnabledTestConfig(t, `[providers.claude]
enabled = false

[providers.claude-mm1]
command = "claude-agent-acp"
runtime_provider = "claude"

[defaults]
provider = "codex"
`)
		enabled, err := cfg.ProviderEnabled("claude-mm1")
		if err != nil || !enabled {
			t.Fatalf("ProviderEnabled(claude-mm1) = (%v, %v), want (true, nil)", enabled, err)
		}
	})

	t.Run("Should let the profile layer re-enable a globally disabled provider", func(t *testing.T) {
		t.Parallel()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		writeFile(t, homePaths.ConfigFile, "[providers.gemini]\nenabled = false\n")
		writeFile(t, profileConfigFile(homePaths, "marketing"), "[providers.gemini]\nenabled = true\n")
		cfg, err := LoadForHome(homePaths, WithProfile("marketing"), withoutDotEnv())
		if err != nil {
			t.Fatalf("LoadForHome(profile) error = %v", err)
		}
		if enabled, _ := cfg.ProviderEnabled("gemini"); !enabled {
			t.Fatal("ProviderEnabled(gemini) = false, want the profile value to win")
		}
	})

	t.Run("Should reject a non-boolean enabled value at load", func(t *testing.T) {
		t.Parallel()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		writeFile(t, homePaths.ConfigFile, "[providers.gemini]\nenabled = \"no\"\n")
		_, err = LoadForHome(homePaths, withoutDotEnv())
		if err == nil || !strings.Contains(err.Error(), "enabled") {
			t.Fatalf("LoadForHome() error = %v, want a type error naming enabled", err)
		}
	})
}

func TestRequireEnabledProvider(t *testing.T) {
	t.Parallel()

	t.Run("Should return the disabled sentinel with the re-enable hint", func(t *testing.T) {
		t.Parallel()
		cfg, _ := loadEnabledTestConfig(t, "[providers.gemini]\nenabled = false\n")
		err := cfg.RequireEnabledProvider("gemini")
		if !errors.Is(err, ErrProviderDisabled) {
			t.Fatalf("RequireEnabledProvider(gemini) error = %v, want ErrProviderDisabled", err)
		}
		if !strings.Contains(err.Error(), "compozy config set providers.gemini.enabled true") {
			t.Fatalf("RequireEnabledProvider(gemini) error = %q, want the re-enable hint", err)
		}
		carrier, ok := errors.AsType[*ProviderDisabledError](err)
		if !ok || carrier.DiagnosticCode() != diagnosticcontract.CodeProviderDisabled {
			t.Fatalf("RequireEnabledProvider(gemini) error = %#v, want provider_disabled code", err)
		}
	})

	t.Run("Should pass enabled and unknown providers", func(t *testing.T) {
		t.Parallel()
		cfg, _ := loadEnabledTestConfig(t, "[providers.gemini]\nenabled = false\n")
		for _, name := range []string{"codex", "no-such-provider", ""} {
			if err := cfg.RequireEnabledProvider(name); err != nil {
				t.Fatalf("RequireEnabledProvider(%q) error = %v, want nil", name, err)
			}
		}
	})
}

func TestDefaultProviderCannotBeDisabled(t *testing.T) {
	t.Parallel()

	const want = `providers.claude.enabled: cannot disable the default provider "claude"; ` +
		"set defaults.provider to another provider first"

	t.Run("Should reject the combination on validate", func(t *testing.T) {
		t.Parallel()
		cfg := defaultEnabledTestConfig(t)
		cfg.Defaults.Provider = "claude"
		cfg.Providers = map[string]ProviderConfig{"claude": {Enabled: new(false)}}
		err := cfg.Validate()
		assertDefaultDisableRejected(t, err, want)
	})

	t.Run("Should reject the combination on load", func(t *testing.T) {
		t.Parallel()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		writeFile(t, homePaths.ConfigFile, "[defaults]\nprovider = \"claude\"\n\n[providers.claude]\nenabled = false\n")
		_, err = LoadForHome(homePaths, withoutDotEnv())
		assertDefaultDisableRejected(t, err, want)
	})
}

func assertDefaultDisableRejected(t *testing.T, err error, want string) {
	t.Helper()
	if !errors.Is(err, ErrProviderDefaultDisableRejected) {
		t.Fatalf("error = %v, want ErrProviderDefaultDisableRejected", err)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want message %q", err, want)
	}
	carrier, ok := errors.AsType[*ProviderDefaultDisableError](err)
	if !ok || carrier.DiagnosticCode() != diagnosticcontract.CodeProviderDefaultDisableRejected {
		t.Fatalf("error = %#v, want provider_default_disable_rejected code", err)
	}
}

func TestProviderEnabledLayerRestriction(t *testing.T) {
	t.Parallel()

	const want = "providers.gemini.enabled can only be set in the global or profile config"
	const overlay = "[providers.gemini]\nenabled = false\n"

	setup := func(t *testing.T) (HomePaths, string) {
		t.Helper()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		return homePaths, t.TempDir()
	}

	t.Run("Should reject a workspace overlay at load", func(t *testing.T) {
		t.Parallel()
		homePaths, workspaceRoot := setup(t)
		writeFile(t, workspaceConfigFile(workspaceRoot), overlay)
		_, err := LoadForHome(homePaths, WithWorkspaceRoot(workspaceRoot), withoutDotEnv())
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("LoadForHome(workspace) error = %v, want %q", err, want)
		}
	})

	t.Run("Should reject a workspace-profile overlay at load", func(t *testing.T) {
		t.Parallel()
		homePaths, workspaceRoot := setup(t)
		writeFile(t, workspaceProfileConfigFile(workspaceRoot, "marketing"), overlay)
		_, err := LoadForHome(
			homePaths, WithWorkspaceRoot(workspaceRoot), WithProfile("marketing"), withoutDotEnv(),
		)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("LoadForHome(workspace profile) error = %v, want %q", err, want)
		}
	})

	t.Run("Should reject a workspace write", func(t *testing.T) {
		t.Parallel()
		homePaths, workspaceRoot := setup(t)
		target, err := ResolveConfigWriteTarget(homePaths, workspaceRoot, WriteScopeWorkspace, "")
		if err != nil {
			t.Fatalf("ResolveConfigWriteTarget(workspace) error = %v", err)
		}
		_, err = validateEffectiveConfigWrite(homePaths, workspaceRoot, target, []byte(overlay))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("validateEffectiveConfigWrite(workspace) error = %v, want %q", err, want)
		}
	})

	t.Run("Should reject writes while a workspace-profile overlay sets the key", func(t *testing.T) {
		t.Parallel()
		homePaths, workspaceRoot := setup(t)
		writeFile(t, workspaceProfileConfigFile(workspaceRoot, "marketing"), overlay)
		target, err := ResolveConfigWriteTarget(homePaths, workspaceRoot, WriteScopeProfile, "marketing")
		if err != nil {
			t.Fatalf("ResolveConfigWriteTarget(profile) error = %v", err)
		}
		_, err = validateEffectiveConfigWrite(
			homePaths,
			workspaceRoot,
			target,
			[]byte("[defaults]\nagent = \"general\"\n"),
		)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("validateEffectiveConfigWrite(workspace profile) error = %v, want %q", err, want)
		}
	})

	t.Run("Should accept the personal profile layer at load and write", func(t *testing.T) {
		t.Parallel()
		homePaths, workspaceRoot := setup(t)
		writeFile(t, profileConfigFile(homePaths, "marketing"), overlay)
		cfg, err := LoadForHome(
			homePaths, WithWorkspaceRoot(workspaceRoot), WithProfile("marketing"), withoutDotEnv(),
		)
		if err != nil {
			t.Fatalf("LoadForHome(profile) error = %v", err)
		}
		if enabled, _ := cfg.ProviderEnabled("gemini"); enabled {
			t.Fatal("ProviderEnabled(gemini) = true, want the profile value applied")
		}
		target, err := ResolveConfigWriteTarget(homePaths, workspaceRoot, WriteScopeProfile, "marketing")
		if err != nil {
			t.Fatalf("ResolveConfigWriteTarget(profile) error = %v", err)
		}
		if _, err := validateEffectiveConfigWrite(homePaths, workspaceRoot, target, []byte(overlay)); err != nil {
			t.Fatalf("validateEffectiveConfigWrite(profile) error = %v", err)
		}
	})
}

func TestWithProfileProviders(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should keep every inventory provider resolvable when a profile-only provider sets enabled",
		func(t *testing.T) {
			t.Parallel()
			base, homePaths := loadEnabledTestConfig(t, "[defaults]\nprovider = \"codex\"\n")
			writeFile(
				t,
				profileConfigFile(homePaths, "work"),
				"[providers.work]\ncommand = \"work-acp\"\nenabled = false\n\n"+
					"[providers.ghost]\nenabled = false\n\n[providers.gemini]\ncommand = \"profile-gemini\"\nenabled = false\n",
			)
			cfg, err := WithProfileProviders(base, homePaths, "work")
			if err != nil {
				t.Fatalf("WithProfileProviders() error = %v", err)
			}
			for _, name := range ProviderInventoryNames(&cfg) {
				if _, err := cfg.ResolveProvider(name); err != nil {
					t.Fatalf("ResolveProvider(%q) error = %v, want every inventory provider resolvable", name, err)
				}
			}
			if enabled, _ := cfg.ProviderEnabled("gemini"); enabled {
				t.Fatal("ProviderEnabled(gemini) = true, want the profile override applied to the built-in")
			}
			inventory := ProviderInventoryNames(&cfg)
			if !slices.Contains(inventory, "work") {
				t.Fatalf("ProviderInventoryNames() = %v, want the profile-only provider listed", inventory)
			}
			if slices.Contains(inventory, "ghost") {
				t.Fatalf("ProviderInventoryNames() = %v, want the incomplete profile-only entry left out", inventory)
			}
			work, err := cfg.ResolveProvider("work")
			if err != nil {
				t.Fatalf("ResolveProvider(work) error = %v", err)
			}
			if work.Command != "work-acp" || !cfg.IsProviderDisabled("work") {
				t.Fatalf("work = command %q disabled %v, want the profile definition and its disable",
					work.Command, cfg.IsProviderDisabled("work"))
			}
			gemini, err := cfg.ResolveProvider("gemini")
			if err != nil {
				t.Fatalf("ResolveProvider(gemini) error = %v", err)
			}
			if baseGemini, _ := base.ResolveProvider("gemini"); gemini.Command != baseGemini.Command {
				t.Fatalf("gemini command = %q, want the global %q: a known provider takes only enabled",
					gemini.Command, baseGemini.Command)
			}
			if _, listed := base.Providers["work"]; listed {
				t.Fatal("base.Providers gained work, want the base config left unchanged")
			}
		},
	)
}
