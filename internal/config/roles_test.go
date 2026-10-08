package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	speedpkg "github.com/compozy/compozy/internal/speed"
)

func TestDefaultRolesConfigPreservesRoleBehavior(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve coordinator defaults", func(t *testing.T) {
		t.Parallel()

		got := DefaultRolesConfig().Coordinator
		if got.Enabled {
			t.Fatal("DefaultRolesConfig().Coordinator.Enabled = true, want false")
		}
		if got.Agent != "" {
			t.Fatalf("DefaultRolesConfig().Coordinator.Agent = %q, want empty", got.Agent)
		}
		if got.TTL != 2*time.Hour {
			t.Fatalf("DefaultRolesConfig().Coordinator.TTL = %s, want 2h", got.TTL)
		}
		if got.MaxChildren != 5 {
			t.Fatalf("DefaultRolesConfig().Coordinator.MaxChildren = %d, want 5", got.MaxChildren)
		}
		if got.MaxActiveSessionsPerWorkspace != 5 {
			t.Fatalf(
				"DefaultRolesConfig().Coordinator.MaxActiveSessionsPerWorkspace = %d, want 5",
				got.MaxActiveSessionsPerWorkspace,
			)
		}
	})

	t.Run("Should preserve session-backed role defaults", func(t *testing.T) {
		t.Parallel()

		got := DefaultRolesConfig()
		for _, item := range []struct {
			name RoleName
			role RoleConfig
		}{
			{name: RoleAutoTitle, role: got.AutoTitle},
		} {
			if want := true; item.role.Enabled != want {
				t.Errorf("DefaultRolesConfig().%s.Enabled = %t, want %t", item.name, item.role.Enabled, want)
			}
			if item.role.Agent != "" {
				t.Errorf("DefaultRolesConfig().%s.Agent = %q, want empty", item.name, item.role.Agent)
			}
		}
	})

	t.Run("Should clone every fallback chain without shared ownership", func(t *testing.T) {
		t.Parallel()

		source := DefaultRolesConfig()
		source.AutoTitle.FallbackChain = []RoleFallback{{Provider: "primary", Model: "title-model"}}
		source.Coordinator.FallbackChain = []RoleFallback{{Provider: "backup", Model: "coordinator-model"}}
		source.AutoTitle.Speed = speedpkg.SpeedFast
		source.AutoTitle.ACPOptions = []ACPOptionSelection{{ID: "thinking", BoolValue: new(true)}}
		source.AutoTitle.FallbackChain[0].ACPOptions = []ACPOptionSelection{{ID: "context", ValueID: "1m"}}
		cloned := CloneRolesConfig(&source)
		cloned.AutoTitle.FallbackChain[0].Model = "changed-title"
		cloned.Coordinator.FallbackChain[0].Model = "changed-coordinator"
		cloned.AutoTitle.Speed = speedpkg.SpeedNormal
		*cloned.AutoTitle.ACPOptions[0].BoolValue = false
		cloned.AutoTitle.FallbackChain[0].ACPOptions[0].ValueID = "128k"
		if source.AutoTitle.FallbackChain[0].Model != "title-model" ||
			source.Coordinator.FallbackChain[0].Model != "coordinator-model" ||
			source.AutoTitle.Speed != speedpkg.SpeedFast ||
			source.AutoTitle.ACPOptions[0].BoolValue == nil || !*source.AutoTitle.ACPOptions[0].BoolValue ||
			source.AutoTitle.FallbackChain[0].ACPOptions[0].ValueID != "1m" {
			t.Fatalf("CloneRolesConfig() mutated source fallbacks: %#v", source)
		}
	})

	t.Run("Should clone and validate a coordinator route command", func(t *testing.T) { // UT-002
		t.Parallel()

		const command = "CLAUDE_CONFIG_DIR=/Users/ada/.claude-work claude --acp"
		source := DefaultRolesConfig()
		source.Coordinator.FallbackChain = []RoleFallback{{
			Provider: "claude", Model: "haiku-4-5", Command: command,
		}}
		if err := source.Validate("roles", &Config{}); err != nil {
			t.Fatalf("Validate(coordinator route command) error = %v", err)
		}
		cloned := CloneRolesConfig(&source)
		if got := cloned.Coordinator.FallbackChain[0].Command; got != command {
			t.Fatalf("cloned route command = %q, want %q", got, command)
		}
		cloned.Coordinator.FallbackChain[0].Command = "changed"
		if got := source.Coordinator.FallbackChain[0].Command; got != command {
			t.Fatalf("source route command = %q after clone mutation, want %q", got, command)
		}
	})
}

func TestRolesConfigValidateEnforcesBoundsAndRoutes(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a coordinator TTL below the floor", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultRolesConfig()
		cfg.Coordinator.TTL = 30 * time.Second
		err := cfg.Validate("roles", &Config{})
		if err == nil || !strings.Contains(err.Error(), "roles.coordinator.ttl") ||
			!strings.Contains(err.Error(), "1m0s") || !strings.Contains(err.Error(), "24h0m0s") {
			t.Fatalf("Validate() error = %v, want coordinator TTL path and bounds", err)
		}
	})

	t.Run("Should accept both coordinator TTL boundaries", func(t *testing.T) {
		t.Parallel()

		for _, ttl := range []time.Duration{time.Minute, 24 * time.Hour} {
			cfg := DefaultRolesConfig()
			cfg.Coordinator.TTL = ttl
			if err := cfg.Validate("roles", &Config{}); err != nil {
				t.Fatalf("Validate(ttl=%s) error = %v", ttl, err)
			}
		}
	})

	t.Run("Should reject a coordinator TTL above the ceiling", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultRolesConfig()
		cfg.Coordinator.TTL = 24*time.Hour + time.Second
		err := cfg.Validate("roles", &Config{})
		if err == nil || !strings.Contains(err.Error(), "roles.coordinator.ttl") {
			t.Fatalf("Validate() error = %v, want coordinator TTL error", err)
		}
	})

	t.Run("Should reject coordinator child counts outside the safe range", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			value   int
			message string
		}{
			{value: 0, message: "must be positive"},
			{value: 6, message: "must be <= 5"},
		} {
			cfg := DefaultRolesConfig()
			cfg.Coordinator.MaxChildren = tc.value
			err := cfg.Validate("roles", &Config{})
			if err == nil || !strings.Contains(err.Error(), "roles.coordinator.max_children") ||
				!strings.Contains(err.Error(), tc.message) {
				t.Errorf("Validate(max_children=%d) error = %v, want %q", tc.value, err, tc.message)
			}
		}
	})

	t.Run("Should reject non-positive coordinator active session caps", func(t *testing.T) {
		t.Parallel()

		for _, value := range []int{0, -1} {
			cfg := DefaultRolesConfig()
			cfg.Coordinator.MaxActiveSessionsPerWorkspace = value
			err := cfg.Validate("roles", &Config{})
			if err == nil || !strings.Contains(err.Error(), "roles.coordinator.max_active_sessions_per_workspace") ||
				!strings.Contains(err.Error(), "must be positive") {
				t.Errorf("Validate(max_active_sessions_per_workspace=%d) error = %v, want positive bound", value, err)
			}
		}
	})

	t.Run("Should require a provider for every fallback", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultRolesConfig()
		cfg.AutoTitle.FallbackChain = []RoleFallback{{Model: "model-a"}}
		err := cfg.Validate("roles", &Config{})
		if err == nil || !strings.Contains(err.Error(), "roles.auto_title.fallback_chain[0].provider is required") {
			t.Fatalf("Validate() error = %v, want fallback provider path", err)
		}
	})

	t.Run("Should reject an unknown fallback provider", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultRolesConfig()
		cfg.AutoTitle.FallbackChain = []RoleFallback{{Provider: "missing", Model: "model-a"}}
		err := cfg.Validate("roles", &Config{})
		assertErrorContains(t, err, "roles.auto_title.fallback_chain[0].provider")
		assertErrorContains(t, err, "missing")
	})

	t.Run("Should accept a route command and parse it with the launch grammar", func(t *testing.T) { // UT-001
		t.Parallel()

		cfg := DefaultRolesConfig()
		cfg.AutoTitle.FallbackChain = []RoleFallback{{
			Provider: "codex", Model: "gpt-5.6-terra", Command: "CODEX_HOME=/Users/ada/.codex-work codex acp",
		}}
		if err := cfg.Validate("roles", &Config{}); err != nil {
			t.Fatalf("Validate(route command) error = %v", err)
		}
		parsed, err := ParseLaunchCommand(cfg.AutoTitle.FallbackChain[0].Command)
		if err != nil {
			t.Fatalf("ParseLaunchCommand() error = %v", err)
		}
		if parsed.Executable != "codex" || !reflect.DeepEqual(parsed.Args, []string{"acp"}) ||
			!reflect.DeepEqual(parsed.Environment, []string{"CODEX_HOME=/Users/ada/.codex-work"}) {
			t.Fatalf("ParseLaunchCommand() = %#v, want codex [acp] with CODEX_HOME env", parsed)
		}
		literal, err := ParseLaunchCommand("CODEX_HOME=~/.codex-work codex acp")
		if err != nil {
			t.Fatalf("ParseLaunchCommand(tilde) error = %v", err)
		}
		if !reflect.DeepEqual(literal.Environment, []string{"CODEX_HOME=~/.codex-work"}) {
			t.Fatalf("ParseLaunchCommand(tilde) env = %#v, want the literal ~ value", literal.Environment)
		}
	})

	t.Run(
		"Should validate route commands by parsing only, with no count or length limit",
		func(t *testing.T) { // UT-003
			t.Parallel()

			longChain := make([]RoleFallback, 20)
			for index := range longChain {
				longChain[index] = RoleFallback{Provider: "claude", Model: "haiku-4-5"}
			}
			longChain[19].Command = "claude --acp " + strings.Repeat("x", 10*1024)
			for _, testCase := range []struct {
				name    string
				chain   []RoleFallback
				wantErr string
			}{
				{
					name:    "missing executable",
					chain:   []RoleFallback{{Provider: "claude", Model: "haiku-4-5", Command: "FOO=bar"}},
					wantErr: "roles.auto_title.fallback_chain[0].command: command is missing an executable",
				},
				{
					name:    "unterminated quote",
					chain:   []RoleFallback{{Provider: "claude", Model: "haiku-4-5", Command: "claude 'unterminated"}},
					wantErr: "roles.auto_title.fallback_chain[0].command: parse command: ",
				},
				{
					name:  "whitespace inherits",
					chain: []RoleFallback{{Provider: "claude", Model: "haiku-4-5", Command: "   "}},
				},
				{name: "twenty routes and a ten kilobyte command", chain: longChain},
			} {
				t.Run("Should handle "+testCase.name, func(t *testing.T) {
					t.Parallel()

					cfg := DefaultRolesConfig()
					cfg.AutoTitle.FallbackChain = testCase.chain
					err := cfg.Validate("roles", &Config{})
					if testCase.wantErr == "" {
						if err != nil {
							t.Fatalf("Validate() error = %v, want accepted", err)
						}
						return
					}
					assertErrorContains(t, err, testCase.wantErr)
				})
			}
		},
	)

	t.Run("Should reject an invalid reasoning effort", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultRolesConfig()
		cfg.AutoTitle.ReasoningEffort = "invalid effort"
		err := cfg.Validate("roles", &Config{})
		assertErrorContains(t, err, "roles.auto_title.reasoning_effort")
		assertErrorContains(t, err, "invalid effort")
	})

	t.Run("Should reject an invalid role speed", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultRolesConfig()
		cfg.AutoTitle.Speed = "burst"
		err := cfg.Validate("roles", &Config{})
		assertErrorContains(t, err, "roles.auto_title.speed")
	})

	t.Run("Should reject role speed duplicated by its ACP option", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultRolesConfig()
		cfg.AutoTitle.Speed = speedpkg.SpeedFast
		cfg.AutoTitle.ACPOptions = []ACPOptionSelection{{ID: "speed", ValueID: "fast"}}
		err := cfg.Validate("roles", &Config{})
		assertErrorContains(t, err, "duplicates speed")
	})

	t.Run("Should reject an invalid optional agent reference when configured", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultRolesConfig()
		cfg.AutoTitle.Agent = "audio designer"
		err := cfg.Validate("roles", &Config{})
		validationErr, validationErrOK := errors.AsType[ValidationError](err)
		if !validationErrOK {
			t.Fatalf("RolesConfig.Validate() error = %T, want ValidationError", err)
		}
		if got, want := validationErr.Path, "roles.auto_title.agent"; got != want {
			t.Fatalf("RolesConfig.Validate() path = %q, want %q", got, want)
		}
		const wantMessage = `agent name "audio designer" must start with a lowercase letter, use only lowercase letters, numbers, hyphens, or underscores, and be at most 106 characters`
		if got := validationErr.Message; got != wantMessage {
			t.Fatalf("RolesConfig.Validate() message = %q, want %q", got, wantMessage)
		}
	})

	t.Run("Should defer a catalog agent model to invocation-time resolution", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultRolesConfig()
		cfg.AutoTitle.Agent = "curator"
		cfg.AutoTitle.Provider = "bare-pi"
		cfg.AutoTitle.Model = ""
		providers := &Config{Providers: map[string]ProviderConfig{
			"bare-pi": {
				Command:         "pi-acp",
				Harness:         ProviderHarnessPiACP,
				RuntimeProvider: "anthropic",
			},
		}}
		if err := cfg.Validate("roles", providers); err != nil {
			t.Fatalf("Validate(catalog agent supplies model) error = %v", err)
		}
	})
}

func TestLoadAllowsDirectACPCoordinatorWithoutModel(t *testing.T) {
	t.Parallel()

	t.Run("Should allow a direct ACP coordinator without a model", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		writeFile(t, homePaths.ConfigFile, "[roles.coordinator]\nprovider = \"opencode\"\n")

		cfg, err := LoadForHome(homePaths, WithWorkspaceRoot(t.TempDir()))
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		if got := cfg.Roles.Coordinator.Model; got != "" {
			t.Fatalf("LoadForHome() coordinator model = %q, want empty for direct ACP", got)
		}
	})
}

func TestLoadKeepsPreReleaseFallbackChains(t *testing.T) {
	t.Parallel()

	t.Run("Should load a twelve-route chain written before route commands existed", func(t *testing.T) { // UT-003
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		var fixture strings.Builder
		for index := range 12 {
			fmt.Fprintf(
				&fixture,
				"[[roles.auto_title.fallback_chain]]\nprovider = \"claude\"\nmodel = \"model-%02d\"\n",
				index,
			)
		}
		writeFile(t, homePaths.ConfigFile, fixture.String())

		cfg, err := LoadForHome(homePaths, WithWorkspaceRoot(t.TempDir()))
		if err != nil {
			t.Fatalf("LoadForHome(pre-release chain) error = %v", err)
		}
		chain := cfg.Roles.AutoTitle.FallbackChain
		if len(chain) != 12 || chain[0].Model != "model-00" || chain[11].Model != "model-11" ||
			chain[11].Command != "" {
			t.Fatalf("Roles.AutoTitle.FallbackChain = %#v, want 12 unchanged routes", chain)
		}
	})
}

func TestRolesCoordinatorOverlayDecodesEmbeddedFields(t *testing.T) {
	t.Parallel()

	t.Run("Should decode shared fields and coordinator extras from one table", func(t *testing.T) {
		t.Parallel()

		overlay, err := loadConfigOverlayBytes([]byte(`
[roles.coordinator]
enabled = true
model = "model-a"
ttl = "1h"
`), "roles.toml")
		if err != nil {
			t.Fatalf("loadConfigOverlayBytes() error = %v", err)
		}
		cfg := DefaultWithHome(HomePaths{})
		if err := overlay.Apply(&cfg); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		if !cfg.Roles.Coordinator.Enabled || cfg.Roles.Coordinator.Model != "model-a" ||
			cfg.Roles.Coordinator.TTL != time.Hour {
			t.Fatalf("Config.Roles.Coordinator = %#v, want enabled/model/ttl overlay", cfg.Roles.Coordinator)
		}
	})
}
