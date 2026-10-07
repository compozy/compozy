package config

import (
	"errors"
	"path/filepath"
	"reflect"
	"slices"

	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/resources"
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
sources = ["claude"]
custom_sources = ["~/team-skills"]
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
	if got, want := cfg.Skills.Sources, []string{SkillSourceClaude}; !slices.Equal(got, want) {
		t.Fatalf("ApplyConfigOverlayFile() Skills.Sources = %#v, want %#v", got, want)
	}
	if got, want := cfg.Skills.CustomSources, []string{"~/team-skills"}; !slices.Equal(got, want) {
		t.Fatalf("ApplyConfigOverlayFile() Skills.CustomSources = %#v, want %#v", got, want)
	}
	if got, want := cfg.Skills.PollInterval, 9*time.Second; got != want {
		t.Fatalf("ApplyConfigOverlayFile() Skills.PollInterval = %s, want %s", got, want)
	}
	if got, want := cfg.Skills.DisabledSkills, []string{"workspace-skill", "code-review"}; !slices.Equal(got, want) {
		t.Fatalf("ApplyConfigOverlayFile() Skills.DisabledSkills = %#v, want %#v", got, want)
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
}

func TestSkillSourceOverlayLayersReplaceIndependently(t *testing.T) {
	t.Parallel()

	t.Run("Should apply absent replace and present-empty semantics through all four layers", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		workspaceRoot := t.TempDir()
		writeFile(t, homePaths.ConfigFile, `
[skills]
sources = ["agents"]
custom_sources = ["~/user-skills"]
`)
		writeFile(t, filepath.Join(homePaths.ProfilesDir, "marketing", ConfigName), `
[skills]
sources = ["claude"]
`)
		writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[skills]
custom_sources = ["./workspace-skills"]
`)
		writeFile(t, filepath.Join(workspaceRoot, DirName, ProfilesDirName, "marketing", ConfigName), `
[skills]
sources = []
`)

		cfg, err := LoadForHome(
			homePaths,
			WithWorkspaceRoot(workspaceRoot),
			WithProfile("marketing"),
		)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		if len(cfg.Skills.Sources) != 0 {
			t.Fatalf("four-layer sources = %#v, want explicit empty workspace-profile replacement", cfg.Skills.Sources)
		}
		if got, want := cfg.Skills.CustomSources, []string{"./workspace-skills"}; !slices.Equal(got, want) {
			t.Fatalf("four-layer custom_sources = %#v, want %#v", got, want)
		}
	})

	t.Run("Should reject repository workspace-profile writes", func(t *testing.T) {
		t.Parallel()

		err := WriteScope("workspace_profile").Validate()
		if err == nil || !strings.Contains(err.Error(), "invalid write scope") {
			t.Fatalf("WriteScope(workspace_profile).Validate() error = %v, want read-only rejection", err)
		}
	})

	t.Run("Should validate custom paths against the owning overlay layer", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		writeFile(t, homePaths.ConfigFile, `
[skills]
custom_sources = ["./relative-user-source"]
`)

		_, err = LoadForHome(homePaths)
		validation, validationOK := errors.AsType[*SkillSourceValidationError](err)
		if !validationOK || validation.Code != "invalid_source_path" ||
			!strings.Contains(err.Error(), "workspace-relative paths require workspace scope") {
			t.Fatalf("LoadForHome(relative user source) error = %#v, want invalid_source_path", err)
		}
	})
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

func TestApplyConfigOverlayFileAppliesMarketplaceCatalogOverlay(t *testing.T) {
	t.Parallel()
	t.Run("Should load plugin source rows preserve omissions and honor explicit clearing", func(t *testing.T) {
		t.Parallel()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatal(err)
		}
		cfg := DefaultWithHome(homePaths)
		file := filepath.Join(t.TempDir(), "overlay.toml")
		writeFile(t, file, `
[[marketplace.plugin_sources]]
name = "team"
source = "github:team/plugins"
[[marketplace.plugin_sources]]
name = "local"
source = "file:///tmp/plugins"
enabled = false
`)
		if err := ApplyConfigOverlayFile(file, &cfg); err != nil {
			t.Fatal(err)
		}
		if len(cfg.Marketplace.PluginSources) != 2 || cfg.Marketplace.PluginSources[0].Enabled != nil ||
			cfg.Marketplace.PluginSources[1].EffectiveEnabled(true) {
			t.Fatalf("decoded plugin rows = %+v", cfg.Marketplace.PluginSources)
		}
		writeFile(t, file, "[marketplace.catalog]\nttl = '20m'\n")
		if err := ApplyConfigOverlayFile(file, &cfg); err != nil || len(cfg.Marketplace.PluginSources) != 2 {
			t.Fatalf("catalog-only overlay lost plugin rows: %+v, %v", cfg.Marketplace, err)
		}
		writeFile(t, file, "[marketplace]\nplugin_sources = []\n")
		if err := ApplyConfigOverlayFile(file, &cfg); err != nil || len(cfg.Marketplace.PluginSources) != 0 {
			t.Fatalf("explicit clearing retained plugin rows: %+v, %v", cfg.Marketplace, err)
		}
	})

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

func TestApplyConfigOverlayFileAppliesExtensionConfig(t *testing.T) {
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
[extensions.trust]
allow_unverified = false

[extensions.sources.github]
enabled = false
`)

		if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		if cfg.Extensions.Trust.AllowUnverified {
			t.Fatal("Extensions.Trust.AllowUnverified = true, want false")
		}
		if cfg.Extensions.Sources.GitHub.Enabled {
			t.Fatal("Extensions.Sources.GitHub.Enabled = true, want false")
		}
	})
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
		if cfg.Roles.Dream.Model != "model-x" || cfg.Roles.Dream.Enabled {
			t.Fatalf("Roles.Dream = %#v, want model override with disabled default", cfg.Roles.Dream)
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
		if cfg.Roles.Dream.Enabled {
			t.Fatal("Roles.Dream.Enabled = true, want default false")
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
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Roles.Dream.FallbackChain = %#v, want %#v", got, want)
		}
	})

	t.Run("Should replace a route command with a workspace overlay chain", func(t *testing.T) { // UT-004
		t.Parallel()

		cfg := DefaultWithHome(HomePaths{})
		globalPath := filepath.Join(t.TempDir(), "config.toml")
		writeFile(t, globalPath, `
[[roles.auto_title.fallback_chain]]
provider = "claude"
model = "haiku-4-5"
command = "CLAUDE_CONFIG_DIR=/Users/ada/.claude-work claude --acp"
`)
		if err := ApplyConfigOverlayFile(globalPath, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile(global) error = %v", err)
		}
		if got := cfg.Roles.AutoTitle.FallbackChain; len(got) != 1 ||
			got[0].Command != "CLAUDE_CONFIG_DIR=/Users/ada/.claude-work claude --acp" {
			t.Fatalf("global Roles.AutoTitle.FallbackChain = %#v, want the route command", got)
		}
		overlay, err := loadConfigOverlayBytes([]byte(`
[[roles.auto_title.fallback_chain]]
provider = "codex"
model = "gpt-5.6-terra"
`), "workspace.toml")
		if err != nil {
			t.Fatalf("loadConfigOverlayBytes(workspace) error = %v", err)
		}
		if err := applyConfigOverlay(&cfg, &overlay, RoleFieldSourceWorkspace); err != nil {
			t.Fatalf("applyConfigOverlay(workspace) error = %v", err)
		}
		want := []RoleFallback{{Provider: "codex", Model: "gpt-5.6-terra"}}
		if got := cfg.Roles.AutoTitle.FallbackChain; !reflect.DeepEqual(got, want) {
			t.Fatalf("effective Roles.AutoTitle.FallbackChain = %#v, want %#v", got, want)
		}
		if got := cfg.RoleFieldSource(RoleAutoTitle, RoleFieldFallbacks); got != RoleFieldSourceWorkspace {
			t.Fatalf("RoleFieldSource(fallback_chain) = %q, want %q", got, RoleFieldSourceWorkspace)
		}
	})

	t.Run("Should merge role speed and typed ACP options with provenance", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultWithHome(HomePaths{})
		path := filepath.Join(t.TempDir(), "roles.toml")
		writeFile(t, path, `
[roles.dream]
speed = "fast"
acp_options = [{ id = "thinking", bool_value = true }]
`)
		if err := ApplyConfigOverlayFile(path, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		if cfg.Roles.Dream.Speed != "fast" || len(cfg.Roles.Dream.ACPOptions) != 1 {
			t.Fatalf("Roles.Dream runtime = %#v, want speed and one ACP option", cfg.Roles.Dream)
		}
		option := cfg.Roles.Dream.ACPOptions[0]
		if option.ID != "thinking" || option.BoolValue == nil || !*option.BoolValue {
			t.Fatalf("Roles.Dream.ACPOptions = %#v, want thinking=true", cfg.Roles.Dream.ACPOptions)
		}
		if got := cfg.RoleFieldSource(RoleDream, RoleFieldSpeed); got != RoleFieldSourceGlobal {
			t.Fatalf("RoleFieldSource(speed) = %q, want %q", got, RoleFieldSourceGlobal)
		}
		if got := cfg.RoleFieldSource(RoleDream, RoleFieldACPOptions); got != RoleFieldSourceGlobal {
			t.Fatalf("RoleFieldSource(acp_options) = %q, want %q", got, RoleFieldSourceGlobal)
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
					Coordinator: coordinatorRoleOverlay{Enabled: tc.first},
				}
				last := rolesOverlay{
					Coordinator: coordinatorRoleOverlay{Enabled: tc.last},
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

// Invariant: released inactivity timers keep independent zero/equal semantics,
// while current timing keys select the current policy. Owner: config loader suite.
func TestSupervisionTimingCompatibility(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       QuietPolicy
		invalid    bool
	}{
		{name: "legacy warning disabled", body: "inactivity_warning_after = '0s'\ninactivity_timeout = '20m'", want: QuietPolicy{StopAfterQuiet: 20 * time.Minute}},
		{name: "legacy stop disabled", body: "inactivity_warning_after = '5m'\ninactivity_timeout = '0s'", want: QuietPolicy{WarningAfter: 5 * time.Minute}},
		{name: "legacy equal thresholds", body: "inactivity_warning_after = '5m'\ninactivity_timeout = '5m'", want: QuietPolicy{WarningAfter: 5 * time.Minute, StopAfterQuiet: 5 * time.Minute}},
		{name: "legacy partial overlay", body: "inactivity_timeout = '40m'", want: QuietPolicy{WarningAfter: 15 * time.Minute, StopAfterQuiet: 40 * time.Minute}},
		{name: "current disable both", body: "quiet_after = '0s'\nstop_grace = '1m'", want: QuietPolicy{}},
		{name: "current warning only", body: "quiet_after = '5m'\nstop_grace = '0s'", want: QuietPolicy{WarningAfter: 5 * time.Minute}},
		{name: "current keys win", body: "inactivity_warning_after = '1m'\ninactivity_timeout = '2m'\nquiet_after = '3m'\nstop_grace = '4m'", want: QuietPolicy{WarningAfter: 3 * time.Minute, StopGrace: 4 * time.Minute}},
		{name: "legacy invalid order", body: "inactivity_warning_after = '2m'\ninactivity_timeout = '1m'", invalid: true},
		{name: "legacy negative cutoff", body: "inactivity_timeout = '-1s'", invalid: true},
	} {
		t.Run("Should preserve "+tc.name, func(t *testing.T) {
			t.Parallel()
			homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
			if err != nil {
				t.Fatal(err)
			}
			cfg := DefaultWithHome(homePaths)
			path := filepath.Join(t.TempDir(), "config.toml")
			writeFile(t, path, "[session.supervision]\n"+tc.body+"\n")
			if err := ApplyConfigOverlayFile(path, &cfg); err != nil {
				t.Fatal(err)
			}
			err = cfg.Session.Supervision.Validate()
			if (err != nil) != tc.invalid {
				t.Fatalf("validation = %v", err)
			}
			if tc.invalid {
				return
			}
			if got := cfg.Session.Supervision.QuietPolicy(); got != tc.want {
				t.Fatalf("policy = %+v, want %+v", got, tc.want)
			}
			entries := FlattenConfigEntries(RedactedConfigMap(&cfg))
			for _, key := range []string{"quiet_after", "stop_grace", "inactivity_warning_after", "inactivity_timeout"} {
				path := "session.supervision." + key
				if _, ok := EntryByPath(entries, path); !ok {
					t.Fatalf("config read missing %s", path)
				}
				policy, err := ClassifyToolConfigPath(strings.Split(path, "."))
				if err != nil || policy.Denial != ConfigPathAllowed || policy.Kind != ConfigValueDuration {
					t.Fatalf("config write %s: %+v, %v", path, policy, err)
				}
			}
		})
	}
}
