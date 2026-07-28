package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
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
			{name: RoleDream, role: got.Dream},
			{name: RoleCheckpointSummary, role: got.CheckpointSummary},
			{name: RoleMemoryExtractor, role: got.MemoryExtractor},
			{name: RoleAutoTitle, role: got.AutoTitle},
		} {
			if !item.role.Enabled {
				t.Errorf("DefaultRolesConfig().%s.Enabled = false, want true", item.name)
			}
			if item.role.Agent != "" {
				t.Errorf("DefaultRolesConfig().%s.Agent = %q, want empty", item.name, item.role.Agent)
			}
		}
	})

	t.Run("Should preserve memory controller defaults", func(t *testing.T) {
		t.Parallel()

		got := DefaultRolesConfig().MemoryController
		if !got.Enabled {
			t.Fatal("DefaultRolesConfig().MemoryController.Enabled = false, want true")
		}
		if got.Provider != "pi" {
			t.Fatalf("DefaultRolesConfig().MemoryController.Provider = %q, want pi", got.Provider)
		}
		if got.Model != "anthropic/claude-haiku-4" {
			t.Fatalf("DefaultRolesConfig().MemoryController.Model = %q, want anthropic/claude-haiku-4", got.Model)
		}
		if got.Timeout != 250*time.Millisecond {
			t.Fatalf("DefaultRolesConfig().MemoryController.Timeout = %s, want 250ms", got.Timeout)
		}
		if got.TopK != 5 || got.PromptVersion != "v1" || got.MaxTokensOut != 256 {
			t.Fatalf(
				"DefaultRolesConfig().MemoryController = %#v, want top_k=5 prompt_version=v1 max_tokens_out=256",
				got,
			)
		}
	})

	t.Run("Should clone every fallback chain without shared ownership", func(t *testing.T) {
		t.Parallel()

		source := DefaultRolesConfig()
		source.Dream.FallbackChain = []RoleFallback{{Provider: "primary", Model: "dream-model"}}
		source.MemoryController.FallbackChain = []RoleFallback{{Provider: "backup", Model: "controller-model"}}
		cloned := CloneRolesConfig(&source)
		cloned.Dream.FallbackChain[0].Model = "changed-dream"
		cloned.MemoryController.FallbackChain[0].Model = "changed-controller"
		if source.Dream.FallbackChain[0].Model != "dream-model" ||
			source.MemoryController.FallbackChain[0].Model != "controller-model" {
			t.Fatalf("CloneRolesConfig() mutated source fallbacks: %#v", source)
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
		cfg.Dream.FallbackChain = []RoleFallback{{Model: "model-a"}}
		err := cfg.Validate("roles", &Config{})
		if err == nil || !strings.Contains(err.Error(), "roles.dream.fallback_chain[0].provider is required") {
			t.Fatalf("Validate() error = %v, want fallback provider path", err)
		}
	})

	t.Run("Should reject an unknown fallback provider", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultRolesConfig()
		cfg.Dream.FallbackChain = []RoleFallback{{Provider: "missing", Model: "model-a"}}
		err := cfg.Validate("roles", &Config{})
		if err == nil || !strings.Contains(err.Error(), "roles.dream.fallback_chain[0].provider") ||
			!strings.Contains(err.Error(), "missing") {
			t.Fatalf("Validate() error = %v, want wrapped provider-resolution failure", err)
		}
	})

	t.Run("Should reject an invalid reasoning effort", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultRolesConfig()
		cfg.MemoryExtractor.ReasoningEffort = "extreme"
		err := cfg.Validate("roles", &Config{})
		if err == nil || !strings.Contains(err.Error(), "roles.memory_extractor.reasoning_effort") ||
			!strings.Contains(err.Error(), "extreme") {
			t.Fatalf("Validate() error = %v, want reasoning enum error", err)
		}
	})

	t.Run("Should defer a catalog agent model to invocation-time resolution", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultRolesConfig()
		cfg.Dream.Agent = "curator"
		cfg.Dream.Provider = "bare-pi"
		cfg.Dream.Model = ""
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

	t.Run("Should require a positive enabled controller timeout", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultRolesConfig()
		cfg.MemoryController.Timeout = 0
		err := cfg.Validate("roles", &Config{})
		if err == nil || !strings.Contains(err.Error(), "roles.memory_controller.timeout must be positive") {
			t.Fatalf("Validate() error = %v, want controller timeout error", err)
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
