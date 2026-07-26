package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestDefaultWithHomeIncludesAutonomyDefaults(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the block breaker and scheduler defaults", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultWithHome(HomePaths{})
		if got, want := cfg.Autonomy.BlockRecurrenceLimit, DefaultBlockRecurrenceLimit; got != want {
			t.Fatalf("DefaultWithHome() Autonomy.BlockRecurrenceLimit = %d, want %d", got, want)
		}
		if got, want := cfg.Autonomy.Scheduler, DefaultSchedulerConfig(); got != want {
			t.Fatalf("DefaultWithHome() Autonomy.Scheduler = %#v, want %#v", got, want)
		}
	})
}

func TestAutonomyConfigValidatesBlockRecurrenceLimit(t *testing.T) {
	t.Parallel()

	t.Run("Should accept zero as breaker disabled", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultWithHome(HomePaths{})
		cfg.Autonomy.BlockRecurrenceLimit = 0
		if err := cfg.Autonomy.Validate(); err != nil {
			t.Fatalf("Autonomy.Validate(block_recurrence_limit=0) error = %v", err)
		}
	})

	t.Run("Should reject a negative block recurrence limit", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultWithHome(HomePaths{})
		cfg.Autonomy.BlockRecurrenceLimit = -1
		err := cfg.Autonomy.Validate()
		if err == nil || !strings.Contains(err.Error(), "autonomy.block_recurrence_limit") {
			t.Fatalf("Autonomy.Validate() error = %v, want block_recurrence_limit path", err)
		}
	})
}

func TestSchedulerConfigValidateMonotonic(t *testing.T) {
	t.Parallel()

	t.Run("Should accept the monotonic defaults", func(t *testing.T) {
		t.Parallel()

		if err := DefaultSchedulerConfig().Validate("autonomy.scheduler"); err != nil {
			t.Fatalf("DefaultSchedulerConfig().Validate() error = %v, want nil", err)
		}
	})

	base := DefaultSchedulerConfig()
	for _, tc := range []struct {
		name            string
		wantErrContains string
		mutate          func(*SchedulerConfig)
	}{
		{
			name:            "Should reject non-positive fan out",
			wantErrContains: "fan_out_after must be positive",
			mutate:          func(c *SchedulerConfig) { c.FanOutAfter = 0 },
		},
		{
			name:            "Should reject spawn before fan out",
			wantErrContains: "spawn_after must be >= fan_out_after",
			mutate:          func(c *SchedulerConfig) { c.SpawnAfter = c.FanOutAfter - 1 },
		},
		{
			name:            "Should reject event before spawn",
			wantErrContains: "event_after must be >= spawn_after",
			mutate:          func(c *SchedulerConfig) { c.EventAfter = c.SpawnAfter - 1 },
		},
		{
			name:            "Should reject needs attention before event",
			wantErrContains: "needs_attention_after must be >= event_after",
			mutate:          func(c *SchedulerConfig) { c.NeedsAttentionAfter = c.EventAfter - 1 },
		},
		{
			name:            "Should reject non-positive minimum queued age",
			wantErrContains: "min_queued_age must be positive",
			mutate:          func(c *SchedulerConfig) { c.MinQueuedAge = 0 },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := base
			tc.mutate(&cfg)
			err := cfg.Validate("autonomy.scheduler")
			if err == nil || !strings.Contains(err.Error(), tc.wantErrContains) {
				t.Fatalf("Validate() error = %v, want %q", err, tc.wantErrContains)
			}
		})
	}
}

func TestLoadLayersAutonomyAndRolesWithoutClobberingOtherSections(t *testing.T) {
	// not parallel: mutates AGH_HOME through the config test harness.
	workspaceRoot, homePaths := prepareAutonomyConfigTestEnv(t)

	writeFile(t, homePaths.ConfigFile, `
[providers.claude]
auth_mode = "bound_secret"
[providers.claude.models]
default = "global-model"
[[providers.claude.credential_slots]]
name = "api_key"
target_env = "GLOBAL_KEY"
secret_ref = "env:GLOBAL_KEY"
kind = "api_key"
required = true

[[hooks.declarations]]
name = "shared"
event = "tool.pre_call"
mode = "sync"
timeout = "5s"
[hooks.declarations.executor]
command = "/bin/global"

[network]
enabled = true
max_replay_age = 600

[memory]
enabled = true
[memory.dream]
min_hours = 36
min_sessions = 4
check_interval = "20m"

[skills]
enabled = true
disabled_skills = ["global-skill"]
poll_interval = "4s"

[roles.coordinator]
enabled = true
provider = "claude"
model = "global-coordinator"
ttl = "2h"
max_children = 5
max_active_sessions_per_workspace = 5
`)
	writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[providers.claude]
auth_mode = "bound_secret"
[[providers.claude.credential_slots]]
name = "api_key"
target_env = "WORKSPACE_KEY"
secret_ref = "env:WORKSPACE_KEY"
kind = "api_key"
required = true

[[hooks.declarations]]
name = "workspace-only"
event = "tool.pre_call"
mode = "sync"
[hooks.declarations.executor]
command = "/bin/workspace"

[network]
max_replay_age = 900
[memory.dream]
min_sessions = 6
[skills]
poll_interval = "9s"
[roles.coordinator]
model = "workspace-coordinator"
ttl = "3h"
max_children = 2
`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	claude, err := cfg.ResolveProvider("claude")
	if err != nil {
		t.Fatalf("ResolveProvider(claude) error = %v", err)
	}
	if claude.Models.Default != "global-model" {
		t.Fatalf("ResolveProvider(claude).Models.Default = %q, want global-model", claude.Models.Default)
	}
	if slots := claude.EffectiveCredentialSlots(); len(slots) != 1 || slots[0].TargetEnv != "WORKSPACE_KEY" {
		t.Fatalf("ResolveProvider(claude) CredentialSlots = %#v, want workspace slot", slots)
	}
	decls, err := HookDeclarations(cfg.Hooks, nil)
	if err != nil {
		t.Fatalf("HookDeclarations() error = %v", err)
	}
	if len(decls) != 2 || !cfg.Network.Enabled || cfg.Network.MaxReplayAge != 900 {
		t.Fatalf("Load() hooks/network = %d/%#v, want two hooks and layered network", len(decls), cfg.Network)
	}
	if cfg.Memory.Dream.MinHours != 36 || cfg.Memory.Dream.MinSessions != 6 {
		t.Fatalf("Load() Memory.Dream = %#v, want layered dream policy", cfg.Memory.Dream)
	}
	if got, want := cfg.Skills.DisabledSkills, []string{"global-skill"}; !slices.Equal(got, want) {
		t.Fatalf("Load() Skills.DisabledSkills = %#v, want %#v", got, want)
	}
	if cfg.Skills.PollInterval != 9*time.Second {
		t.Fatalf("Load() Skills.PollInterval = %s, want 9s", cfg.Skills.PollInterval)
	}
	if cfg.Roles.Coordinator.Model != "workspace-coordinator" || cfg.Roles.Coordinator.TTL != 3*time.Hour ||
		cfg.Roles.Coordinator.MaxChildren != 2 {
		t.Fatalf("Load() Roles.Coordinator = %#v, want workspace values", cfg.Roles.Coordinator)
	}
}

func TestLoadAutonomyDoesNotUseAmbientWorkspaceOrMutateEnv(t *testing.T) {
	// not parallel: changes the process working directory and AGH_HOME.
	workspaceRoot, homePaths := prepareAutonomyConfigTestEnv(t)
	ambientWorkspace := t.TempDir()
	writeFile(t, homePaths.ConfigFile, "[roles.coordinator]\nmodel = \"global-coordinator\"\n")
	writeFile(
		t,
		filepath.Join(workspaceRoot, DirName, ConfigName),
		"[roles.coordinator]\nmodel = \"target-workspace\"\n",
	)
	writeFile(
		t,
		filepath.Join(ambientWorkspace, DirName, ConfigName),
		"[roles.coordinator]\nmodel = \"ambient-workspace\"\n",
	)

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(ambientWorkspace); err != nil {
		t.Fatalf("Chdir(%q) error = %v", ambientWorkspace, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
	beforeHome := os.Getenv("AGH_HOME")

	cfg, err := LoadForHome(homePaths, WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("LoadForHome(target workspace) error = %v", err)
	}
	if cfg.Roles.Coordinator.Model != "target-workspace" {
		t.Fatalf("LoadForHome() coordinator model = %q, want target-workspace", cfg.Roles.Coordinator.Model)
	}
	if got := os.Getenv("AGH_HOME"); got != beforeHome {
		t.Fatalf("LoadForHome() mutated AGH_HOME = %q, want %q", got, beforeHome)
	}
	globalOnly, err := LoadForHome(homePaths)
	if err != nil {
		t.Fatalf("LoadForHome(global only) error = %v", err)
	}
	if globalOnly.Roles.Coordinator.Model != "global-coordinator" {
		t.Fatalf("LoadForHome(global only) model = %q, want global-coordinator", globalOnly.Roles.Coordinator.Model)
	}
}

func TestLoadWorkspaceOverridesAutonomyBlockRecurrenceLimit(t *testing.T) {
	// not parallel: mutates AGH_HOME through the config test harness.
	workspaceRoot, homePaths := prepareAutonomyConfigTestEnv(t)
	writeFile(t, homePaths.ConfigFile, "[autonomy]\nblock_recurrence_limit = 3\n")
	writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), "[autonomy]\nblock_recurrence_limit = 0\n")

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Autonomy.BlockRecurrenceLimit != 0 {
		t.Fatalf("Load() BlockRecurrenceLimit = %d, want 0", cfg.Autonomy.BlockRecurrenceLimit)
	}
}

func TestLoadRejectsInvalidAutonomyBlockRecurrenceLimit(t *testing.T) {
	// not parallel: mutates AGH_HOME through the config test harness.
	workspaceRoot, homePaths := prepareAutonomyConfigTestEnv(t)
	writeFile(t, homePaths.ConfigFile, "[autonomy]\nblock_recurrence_limit = -1\n")

	_, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err == nil || !strings.Contains(err.Error(), "autonomy.block_recurrence_limit") {
		t.Fatalf("Load() error = %v, want block_recurrence_limit path", err)
	}
}

func TestLoadRejectsUnknownAutonomyConfigKeys(t *testing.T) {
	// not parallel: mutates AGH_HOME through the config test harness.
	workspaceRoot, homePaths := prepareAutonomyConfigTestEnv(t)
	writeFile(t, homePaths.ConfigFile, "[autonomy]\nunknown = true\n")

	_, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err == nil || !strings.Contains(err.Error(), "autonomy.unknown") {
		t.Fatalf("Load() error = %v, want autonomy.unknown", err)
	}
}

func TestRootExampleConfigIncludesBlockRecurrenceLimit(t *testing.T) {
	// not parallel: mutates AGH_HOME through the config test harness.
	examplePath := filepath.Join("..", "..", "config.toml")
	contents, err := os.ReadFile(examplePath)
	if err != nil {
		t.Fatalf("ReadFile(config.toml) error = %v", err)
	}
	if !strings.Contains(string(contents), "block_recurrence_limit = 2") {
		t.Fatal("config.toml missing block_recurrence_limit example")
	}
	workspaceRoot, homePaths := prepareAutonomyConfigTestEnv(t)
	writeFile(t, homePaths.ConfigFile, string(contents))
	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load(config.toml example) error = %v", err)
	}
	if cfg.Autonomy.BlockRecurrenceLimit != 2 {
		t.Fatalf("Load(config.toml example) BlockRecurrenceLimit = %d, want 2", cfg.Autonomy.BlockRecurrenceLimit)
	}
}

func prepareAutonomyConfigTestEnv(t *testing.T) (string, HomePaths) {
	t.Helper()

	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")
	t.Setenv("AGH_HOME", homeRoot)
	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	return workspaceRoot, homePaths
}
