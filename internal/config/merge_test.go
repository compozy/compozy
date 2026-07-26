package config

import (
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/resources"
)

func TestApplyConfigOverlayFileAppliesSkillsOverlay(t *testing.T) {
	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	cfg := DefaultWithHome(homePaths)
	cfg.Skills.DisabledSkills = []string{"global-skill"}

	overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
	writeFile(t, overlayPath, `
[skills]
enabled = false
disabled_skills = ["workspace-skill", "code-review"]
poll_interval = "9s"
allowed_marketplace_mcp = ["@registry/mcp-a", "@registry/mcp-b"]
allowed_marketplace_hooks = ["@registry/hook-a", "@registry/hook-b"]

[skills.marketplace]
registry = "clawhub"
base_url = "https://registry.example.test/api/v1"
`)

	if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
		t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
	}

	if cfg.Skills.Enabled {
		t.Fatal("ApplyConfigOverlayFile() Skills.Enabled = true, want false")
	}
	if got, want := cfg.Skills.PollInterval, 9*time.Second; got != want {
		t.Fatalf("ApplyConfigOverlayFile() Skills.PollInterval = %s, want %s", got, want)
	}
	if got, want := cfg.Skills.DisabledSkills, []string{"workspace-skill", "code-review"}; !slices.Equal(got, want) {
		t.Fatalf("ApplyConfigOverlayFile() Skills.DisabledSkills = %#v, want %#v", got, want)
	}
	if got, want := cfg.Skills.AllowedMarketplaceMCP, []string{
		"@registry/mcp-a",
		"@registry/mcp-b",
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("ApplyConfigOverlayFile() Skills.AllowedMarketplaceMCP = %#v, want %#v", got, want)
	}
	if got, want := cfg.Skills.AllowedMarketplaceHooks, []string{
		"@registry/hook-a",
		"@registry/hook-b",
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("ApplyConfigOverlayFile() Skills.AllowedMarketplaceHooks = %#v, want %#v", got, want)
	}
	if got, want := cfg.Skills.Marketplace.Registry, "clawhub"; got != want {
		t.Fatalf("ApplyConfigOverlayFile() Skills.Marketplace.Registry = %q, want %q", got, want)
	}
	if got, want := cfg.Skills.Marketplace.BaseURL, "https://registry.example.test/api/v1"; got != want {
		t.Fatalf("ApplyConfigOverlayFile() Skills.Marketplace.BaseURL = %q, want %q", got, want)
	}
}

func TestApplyConfigOverlayFileAppliesDaemonRuntimeHealthSettings(t *testing.T) {
	t.Run("Should apply daemon runtime health settings", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)

		overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
		writeFile(
			t,
			overlayPath,
			"[daemon]\nmemory_report_interval = \"0s\"\nsubprocess_health_escalation_threshold = 7\n",
		)
		if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		if got := cfg.Daemon.MemoryReportInterval; got != 0 {
			t.Fatalf("ApplyConfigOverlayFile() Daemon.MemoryReportInterval = %s, want disabled", got)
		}
		if got, want := cfg.Daemon.SubprocessHealthEscalationThreshold, 7; got != want {
			t.Fatalf("ApplyConfigOverlayFile() threshold = %d, want %d", got, want)
		}
	})
}

func TestApplyConfigOverlayFileAppliesRedactionSnapshotSetting(t *testing.T) {
	t.Run("Should default redaction on and apply an explicit disabled overlay", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		if !cfg.Redact.Enabled {
			t.Fatal("DefaultWithHome().Redact.Enabled = false, want secure default true")
		}

		overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
		writeFile(t, overlayPath, "[redact]\nenabled = false\n")
		if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		if cfg.Redact.Enabled {
			t.Fatal("ApplyConfigOverlayFile() Redact.Enabled = true, want false")
		}
	})
}

func TestApplyConfigOverlayFileLeavesMarketplaceDefaultsWhenOverlayOmitsFields(t *testing.T) {
	t.Run("ShouldLeaveMarketplaceDefaultsWhenOverlayOmitsFields", func(t *testing.T) {
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		cfg := DefaultWithHome(homePaths)
		cfg.Skills.Marketplace = MarketplaceConfig{
			Registry: "clawhub",
			BaseURL:  "https://global.example.test/api/v1",
		}

		overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
		writeFile(t, overlayPath, `
[skills]
enabled = true
`)

		if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}

		if got, want := cfg.Skills.Marketplace.Registry, "clawhub"; got != want {
			t.Fatalf("ApplyConfigOverlayFile() Skills.Marketplace.Registry = %q, want %q", got, want)
		}
		if got, want := cfg.Skills.Marketplace.BaseURL, "https://global.example.test/api/v1"; got != want {
			t.Fatalf("ApplyConfigOverlayFile() Skills.Marketplace.BaseURL = %q, want %q", got, want)
		}
	})
}

func TestApplyConfigOverlayFileAppliesMarketplaceCatalogOverlay(t *testing.T) {
	t.Parallel()

	t.Run("Should apply marketplace catalog overlay", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
		writeFile(t, overlayPath, `
[marketplace.catalog]
base_url = "https://workspace.example.test/catalog"
ttl = "20m"
timeout = "3s"
`)

		if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		if got, want := cfg.Marketplace.Catalog.BaseURL, "https://workspace.example.test/catalog"; got != want {
			t.Fatalf("Marketplace.Catalog.BaseURL = %q, want %q", got, want)
		}
		if got, want := cfg.Marketplace.Catalog.TTL, "20m"; got != want {
			t.Fatalf("Marketplace.Catalog.TTL = %q, want %q", got, want)
		}
		if got, want := cfg.Marketplace.Catalog.Timeout, "3s"; got != want {
			t.Fatalf("Marketplace.Catalog.Timeout = %q, want %q", got, want)
		}
	})
}

func TestApplyConfigOverlayFileAppliesExtensionSideLoadPolicy(t *testing.T) {
	t.Parallel()
	t.Run("Should apply the extension side-load policy from an overlay", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
		writeFile(t, overlayPath, `
[extensions.marketplace]
allow_unverified = true
`)

		if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		if !cfg.Extensions.Marketplace.AllowUnverified {
			t.Fatal("Extensions.Marketplace.AllowUnverified = false, want true")
		}
	})
}

func TestApplyConfigOverlayFileAppliesNetworkOverlay(t *testing.T) {
	t.Parallel()

	t.Run("ShouldApplyNetworkOverlay", func(t *testing.T) {
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		cfg := DefaultWithHome(homePaths)

		overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
		writeFile(t, overlayPath, `
[network]
enabled = true
max_replay_age = 90

[network.live.defaults]
max_wakes = 10
coalesce_window = "750ms"

[network.live.limits]
max_wakes = 72
max_coalesce_window = "6s"
`)

		if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}

		if !cfg.Network.Enabled {
			t.Fatal("ApplyConfigOverlayFile() Network.Enabled = false, want true")
		}
		if got, want := cfg.Network.MaxReplayAge, 90; got != want {
			t.Fatalf("ApplyConfigOverlayFile() Network.MaxReplayAge = %d, want %d", got, want)
		}
		if got, want := cfg.Network.Live.Defaults.MaxWakes, 10; got != want {
			t.Fatalf("ApplyConfigOverlayFile() Network.Live.Defaults.MaxWakes = %d, want %d", got, want)
		}
		if got, want := cfg.Network.Live.Limits.MaxWakes, 72; got != want {
			t.Fatalf("ApplyConfigOverlayFile() Network.Live.Limits.MaxWakes = %d, want %d", got, want)
		}
		if got, want := cfg.Network.Live.Defaults.CoalesceWindow, "750ms"; got != want {
			t.Fatalf("ApplyConfigOverlayFile() Network.Live.Defaults.CoalesceWindow = %s, want %s", got, want)
		}
		if got, want := cfg.Network.Live.Limits.MaxCoalesceWindow, "6s"; got != want {
			t.Fatalf("ApplyConfigOverlayFile() Network.Live.Limits.MaxCoalesceWindow = %s, want %s", got, want)
		}
	})
}

func TestApplyConfigOverlayFileRejectsRemovedNetworkFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field string
		value string
	}{
		{name: "ShouldRejectRemovedGreetInterval", field: "greet_interval", value: "15"},
		{name: "ShouldRejectRemovedMaxQueueDepth", field: "max_queue_depth", value: "12"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}
			cfg := DefaultWithHome(homePaths)
			overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
			writeFile(t, overlayPath, "[network]\n"+tc.field+" = "+tc.value+"\n")

			err = ApplyConfigOverlayFile(overlayPath, &cfg)
			if err == nil {
				t.Fatalf("ApplyConfigOverlayFile() error = nil, want %s rejection", tc.field)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("ApplyConfigOverlayFile() error = %q, want %q", err, tc.field)
			}
		})
	}
}

func TestApplyConfigOverlayFileMergesSandboxProfiles(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	cfg := DefaultWithHome(homePaths)
	cfg.Sandboxes["daytona-dev"] = SandboxProfile{
		Backend:     "daytona",
		SyncMode:    "session-bidirectional",
		Persistence: "reuse",
		RuntimeRoot: "/workspace",
		Env: map[string]string{
			"GLOBAL_ONLY": "true",
			"SHARED":      "global",
		},
		Network: NetworkProfile{
			AllowOutbound: true,
			AllowList:     []string{"api.example.test"},
		},
		Daytona: DaytonaProfile{
			APIURL: "https://app.daytona.io/api",
			Target: "team-default",
			Image:  "ubuntu:24.04",
			Class:  "cpu-2",
		},
	}

	overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
	writeFile(t, overlayPath, `
[defaults]
sandbox = "daytona-dev"

[sandboxes.daytona-dev]
sync_mode = "none"

[sandboxes.daytona-dev.env]
SHARED = "workspace"
WORKSPACE_ONLY = "true"

[sandboxes.daytona-dev.daytona]
snapshot = "snap-workspace"
`)

	if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
		t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
	}

	profile := cfg.Sandboxes["daytona-dev"]
	if got, want := cfg.Defaults.Sandbox, "daytona-dev"; got != want {
		t.Fatalf("Defaults.Sandbox = %q, want %q", got, want)
	}
	if profile.Backend != "daytona" || profile.Persistence != "reuse" || profile.RuntimeRoot != "/workspace" {
		t.Fatalf("sandbox profile base fields not preserved: %#v", profile)
	}
	if profile.SyncMode != "none" {
		t.Fatalf("sandbox SyncMode = %q, want none", profile.SyncMode)
	}
	if profile.Daytona.APIURL != "https://app.daytona.io/api" ||
		profile.Daytona.Image != "ubuntu:24.04" ||
		profile.Daytona.Snapshot != "snap-workspace" {
		t.Fatalf("Daytona overlay did not preserve provider fields: %#v", profile.Daytona)
	}
	if got, want := profile.Env["GLOBAL_ONLY"], "true"; got != want {
		t.Fatalf("Env[GLOBAL_ONLY] = %q, want %q", got, want)
	}
	if got, want := profile.Env["SHARED"], "workspace"; got != want {
		t.Fatalf("Env[SHARED] = %q, want %q", got, want)
	}
	if got, want := profile.Env["WORKSPACE_ONLY"], "true"; got != want {
		t.Fatalf("Env[WORKSPACE_ONLY] = %q, want %q", got, want)
	}
}

func TestApplyConfigOverlayFileAppliesExtensionsResourceOverlay(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	cfg := DefaultWithHome(homePaths)
	cfg.Extensions.Resources.OperatorWriteRateLimit = ExtensionsResourceRateLimitConfig{
		Requests: 12,
		Window:   time.Minute,
		Queue:    0,
	}

	overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
	writeFile(t, overlayPath, `
[extensions.resources]
allowed_kinds = ["tool"]
max_scope = "workspace"

[extensions.resources.snapshot_rate_limit]
requests = 2
window = "8s"
queue = 1
`)

	if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
		t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
	}
	if got, want := cfg.Extensions.Resources.AllowedKinds, []resources.ResourceKind{
		resources.ResourceKind("tool"),
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("ApplyConfigOverlayFile() AllowedKinds = %#v, want %#v", got, want)
	}
	if got, want := cfg.Extensions.Resources.MaxScope, resources.ResourceScopeKindWorkspace; got != want {
		t.Fatalf("ApplyConfigOverlayFile() MaxScope = %q, want %q", got, want)
	}
	if got, want := cfg.Extensions.Resources.SnapshotRateLimit.Requests, 2; got != want {
		t.Fatalf("ApplyConfigOverlayFile() SnapshotRateLimit.Requests = %d, want %d", got, want)
	}
	if got, want := cfg.Extensions.Resources.SnapshotRateLimit.Window, 8*time.Second; got != want {
		t.Fatalf("ApplyConfigOverlayFile() SnapshotRateLimit.Window = %s, want %s", got, want)
	}
	if got, want := cfg.Extensions.Resources.OperatorWriteRateLimit.Requests, 12; got != want {
		t.Fatalf("ApplyConfigOverlayFile() OperatorWriteRateLimit.Requests = %d, want %d", got, want)
	}
	if got, want := cfg.Extensions.Resources.OperatorWriteRateLimit.Window, time.Minute; got != want {
		t.Fatalf("ApplyConfigOverlayFile() OperatorWriteRateLimit.Window = %s, want %s", got, want)
	}
}

func TestValidateWrapsNetworkErrorsWithConfigContext(t *testing.T) {
	t.Parallel()

	t.Run("ShouldWrapNetworkValidationErrorsWithConfigContext", func(t *testing.T) {
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		cfg := DefaultWithHome(homePaths)
		cfg.Network.Live.Defaults.MaxWakes = cfg.Network.Live.Limits.MaxWakes + 1

		err = cfg.Validate()
		if err == nil {
			t.Fatal("Validate() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "validate network config") ||
			!strings.Contains(err.Error(), networkLiveLimitsMaxWakesPath) {
			t.Fatalf("Validate() error = %q, want config and network context", err)
		}
	})
}

func TestValidateRejectsOverflowingNetworkDurations(t *testing.T) {
	t.Parallel()

	if strconv.IntSize < 64 {
		t.Skip("overflow validation requires 64-bit int")
	}

	tests := []struct {
		name   string
		mutate func(*Config)
		field  string
	}{
		{
			name: "ShouldRejectOverflowingReplayAge",
			mutate: func(cfg *Config) {
				cfg.Network.MaxReplayAge = int(maxNetworkDurationSeconds + 1)
			},
			field: "network.max_replay_age",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}

			cfg := DefaultWithHome(homePaths)
			tc.mutate(&cfg)

			err = cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() error = nil, want non-nil for %s", tc.field)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("Validate() error = %q, want field %q", err, tc.field)
			}
		})
	}
}

func TestRolesOverlayPreservesLayeredMergeSemantics(t *testing.T) {
	t.Parallel()

	t.Run("Should change one field without clobbering defaults or sibling roles", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultWithHome(HomePaths{})
		path := filepath.Join(t.TempDir(), "roles.toml")
		writeFile(t, path, "[roles.dream]\nmodel = \"model-x\"\n")
		if err := ApplyConfigOverlayFile(path, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		if cfg.Roles.Dream.Model != "model-x" || !cfg.Roles.Dream.Enabled {
			t.Fatalf("Roles.Dream = %#v, want model override with enabled default", cfg.Roles.Dream)
		}
		if !reflect.DeepEqual(cfg.Roles.CheckpointSummary, DefaultRolesConfig().CheckpointSummary) {
			t.Fatalf("Roles.CheckpointSummary = %#v, want defaults", cfg.Roles.CheckpointSummary)
		}
	})

	t.Run("Should leave enabled intact for an empty role table", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultWithHome(HomePaths{})
		path := filepath.Join(t.TempDir(), "roles.toml")
		writeFile(t, path, "[roles.dream]\n")
		if err := ApplyConfigOverlayFile(path, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		if !cfg.Roles.Dream.Enabled {
			t.Fatal("Roles.Dream.Enabled = false, want default true")
		}
	})

	t.Run("Should replace a fallback chain wholesale", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultWithHome(HomePaths{})
		cfg.Roles.Dream.FallbackChain = []RoleFallback{
			{Provider: "a", Model: "one"},
			{Provider: "b", Model: "two"},
		}
		path := filepath.Join(t.TempDir(), "roles.toml")
		writeFile(t, path, `
[[roles.dream.fallback_chain]]
provider = "c"
model = "three"
`)
		if err := ApplyConfigOverlayFile(path, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		got := cfg.Roles.Dream.FallbackChain
		want := []RoleFallback{{Provider: "c", Model: "three"}}
		if !slices.Equal(got, want) {
			t.Fatalf("Roles.Dream.FallbackChain = %#v, want %#v", got, want)
		}
	})

	t.Run("Should layer workspace values after global values", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultWithHome(HomePaths{})
		cfg.Roles.Dream.Model = "default-model"
		globalPath := filepath.Join(t.TempDir(), "global.toml")
		workspacePath := filepath.Join(t.TempDir(), "workspace.toml")
		writeFile(t, globalPath, "[roles.dream]\nmodel = \"global-model\"\n")
		writeFile(t, workspacePath, "[roles.dream]\nmodel = \"workspace-model\"\n")
		if err := ApplyConfigOverlayFile(globalPath, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile(global) error = %v", err)
		}
		if err := applyWorkspaceConfigOverlayFile(workspacePath, &cfg); err != nil {
			t.Fatalf("applyWorkspaceConfigOverlayFile() error = %v", err)
		}
		if cfg.Roles.Dream.Model != "workspace-model" {
			t.Fatalf("Roles.Dream.Model = %q, want workspace-model", cfg.Roles.Dream.Model)
		}
	})

	t.Run("Should accept roles in workspace overlays", func(t *testing.T) {
		t.Parallel()

		overlay, err := loadConfigOverlayBytes([]byte("[roles.auto_title]\nenabled = false\n"), "workspace.toml")
		if err != nil {
			t.Fatalf("loadConfigOverlayBytes() error = %v", err)
		}
		if err := validateWorkspaceConfigOverlay("workspace.toml", &overlay); err != nil {
			t.Fatalf("validateWorkspaceConfigOverlay() error = %v", err)
		}
	})

	t.Run("Should honor the last explicit coordinator enabled value", func(t *testing.T) {
		t.Parallel()

		falseValue := false
		trueValue := true
		for _, tc := range []struct {
			name  string
			first *bool
			last  *bool
			want  bool
		}{
			{name: "Should end enabled", first: &falseValue, last: &trueValue, want: true},
			{name: "Should end disabled", first: &trueValue, last: &falseValue, want: false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				cfg := DefaultWithHome(HomePaths{})
				first := rolesOverlay{
					Coordinator: coordinatorRoleOverlay{roleOverlay: roleOverlay{Enabled: tc.first}},
				}
				last := rolesOverlay{
					Coordinator: coordinatorRoleOverlay{roleOverlay: roleOverlay{Enabled: tc.last}},
				}
				first.Apply(&cfg.Roles)
				last.Apply(&cfg.Roles)
				if cfg.Roles.Coordinator.Enabled != tc.want {
					t.Fatalf("Roles.Coordinator.Enabled = %t, want %t", cfg.Roles.Coordinator.Enabled, tc.want)
				}
			})
		}
	})
}
