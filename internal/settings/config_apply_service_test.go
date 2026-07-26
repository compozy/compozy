package settings

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	automationmodel "github.com/compozy/agh/internal/automation/model"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestConfigApplyServiceRecordsLiveApplyAndAdvancesGeneration(t *testing.T) {
	t.Parallel()

	t.Run("Should persist applied live record and advance active generation", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(ctx); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})

		service := testService(t, homePaths, Dependencies{
			SkillsRuntime: newFakeSkillsRuntime(testSkill("alpha", false)),
			ApplyRecords:  NewConfigApplyRecordRepository(db.DB(), nil),
		})

		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		cfg.Skills.DisabledSkills = []string{"alpha"}

		result, err := service.ApplySection(WithMutationSource(ctx, "http"), SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionSkills},
			Skills:         &cfg.Skills,
		})
		if err != nil {
			t.Fatalf("ApplySection(skills) error = %v", err)
		}
		if !result.Applied {
			t.Fatal("ApplySection(skills).Applied = false, want true")
		}
		if got, want := result.Record.Lifecycle, lifecycle.Live; got != want {
			t.Fatalf("Lifecycle = %q, want %q", got, want)
		}
		if got, want := result.Record.Status, lifecycle.StatusApplied; got != want {
			t.Fatalf("Status = %q, want %q", got, want)
		}
		if got, want := result.Record.Generation, int64(1); got != want {
			t.Fatalf("Generation = %d, want %d", got, want)
		}
		active, err := service.ActiveConfig(ctx)
		if err != nil {
			t.Fatalf("ActiveConfig() error = %v", err)
		}
		if len(active.Skills.DisabledSkills) != 1 || active.Skills.DisabledSkills[0] != "alpha" {
			t.Fatalf("ActiveConfig().Skills.DisabledSkills = %#v, want [alpha]", active.Skills.DisabledSkills)
		}

		records, err := service.ListApplyRecords(ctx, ApplyRecordFilter{Status: lifecycle.StatusApplied})
		if err != nil {
			t.Fatalf("ListApplyRecords(applied) error = %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("ListApplyRecords(applied) len = %d, want 1", len(records))
		}
		if got, want := records[0].Actor, "http"; got != want {
			t.Fatalf("Actor = %q, want %q", got, want)
		}
	})

	t.Run("Should persist role routing as a live apply with history", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(ctx); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})

		service := testService(t, homePaths, Dependencies{
			ApplyRecords: NewConfigApplyRecordRepository(db.DB(), nil),
		})
		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		roles := cfg.Roles
		roles.MemoryExtractor.Model = "anthropic/claude-sonnet-4"

		result, err := service.ApplySection(WithMutationSource(ctx, "http"), SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionRoles},
			Roles:          &roles,
		})
		if err != nil {
			t.Fatalf("ApplySection(roles) error = %v", err)
		}
		if !result.Applied || result.Record.Lifecycle != lifecycle.Live ||
			result.Record.Status != lifecycle.StatusApplied {
			t.Fatalf("ApplySection(roles) = %#v, want applied live record", result)
		}

		persisted, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(persisted) error = %v", err)
		}
		if got, want := persisted.Roles.MemoryExtractor.Model, "anthropic/claude-sonnet-4"; got != want {
			t.Fatalf("persisted roles.memory_extractor.model = %q, want %q", got, want)
		}
		active, err := service.ActiveConfig(ctx)
		if err != nil {
			t.Fatalf("ActiveConfig() error = %v", err)
		}
		if active.Roles.MemoryExtractor.Model != persisted.Roles.MemoryExtractor.Model {
			t.Fatalf(
				"active roles.memory_extractor.model = %q, want persisted %q",
				active.Roles.MemoryExtractor.Model,
				persisted.Roles.MemoryExtractor.Model,
			)
		}
		active.Roles.MemoryExtractor.FallbackChain = append(
			active.Roles.MemoryExtractor.FallbackChain,
			aghconfig.RoleFallback{Provider: "mutated", Model: "mutated"},
		)
		active.RoleSources[aghconfig.RoleMemoryExtractor]["model"] = "mutated"
		isolated, err := service.ActiveConfig(ctx)
		if err != nil {
			t.Fatalf("ActiveConfig(after caller mutation) error = %v", err)
		}
		if slices.ContainsFunc(isolated.Roles.MemoryExtractor.FallbackChain, func(
			fallback aghconfig.RoleFallback,
		) bool {
			return fallback.Provider == "mutated"
		}) || isolated.RoleSources[aghconfig.RoleMemoryExtractor]["model"] == "mutated" {
			t.Fatalf("ActiveConfig() retained caller-owned role state: %#v", isolated)
		}
		records, err := service.ListApplyRecords(ctx, ApplyRecordFilter{})
		if err != nil {
			t.Fatalf("ListApplyRecords() error = %v", err)
		}
		if len(records) != 1 || records[0].Actor != "http" || records[0].Lifecycle != lifecycle.Live {
			t.Fatalf("role apply records = %#v, want one HTTP live record", records)
		}
	})

	t.Run("Should replace workspace role fallbacks through the scoped section contract", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		workspaceID := "ws-roles"
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(ctx); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})

		service := testService(t, homePaths, Dependencies{
			ApplyRecords: NewConfigApplyRecordRepository(db.DB(), nil),
			WorkspaceResolver: fakeWorkspaceResolver{resolved: map[string]workspacepkg.ResolvedWorkspace{
				workspaceID: {
					Workspace: workspacepkg.Workspace{ID: workspaceID, RootDir: workspaceRoot},
				},
			}},
		})
		cfg, err := aghconfig.LoadForHome(homePaths, aghconfig.WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			t.Fatalf("LoadForHome(workspace) error = %v", err)
		}
		roles := cfg.Roles
		roles.AutoTitle.FallbackChain = []aghconfig.RoleFallback{{
			Provider: "codex", Model: "gpt-5-mini", ReasoningEffort: "medium",
		}}

		result, err := service.ApplySection(WithMutationSource(ctx, "uds"), SectionUpdateRequest{
			SectionRequest: SectionRequest{
				Section: SectionRoles, Scope: ScopeWorkspace, WorkspaceID: workspaceID,
			},
			Roles: &roles,
		})
		if err != nil {
			t.Fatalf("ApplySection(workspace roles) error = %v", err)
		}
		if !result.Applied || result.Scope != ScopeWorkspace || result.WorkspaceID != workspaceID ||
			result.WriteTarget != WriteTargetWorkspaceConfig {
			t.Fatalf("ApplySection(workspace roles) = %#v, want applied workspace config", result)
		}

		persisted, err := aghconfig.LoadForHome(homePaths, aghconfig.WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			t.Fatalf("LoadForHome(persisted workspace) error = %v", err)
		}
		if !reflect.DeepEqual(persisted.Roles.AutoTitle.FallbackChain, roles.AutoTitle.FallbackChain) {
			t.Fatalf(
				"workspace fallback chain = %#v, want %#v",
				persisted.Roles.AutoTitle.FallbackChain,
				roles.AutoTitle.FallbackChain,
			)
		}

		envelope, err := service.GetSection(ctx, SectionRequest{
			Section: SectionRoles, Scope: ScopeWorkspace, WorkspaceID: workspaceID,
		})
		if err != nil {
			t.Fatalf("GetSection(workspace roles) error = %v", err)
		}
		if envelope.Scope != ScopeWorkspace ||
			!reflect.DeepEqual(envelope.AvailableScopes, []ScopeKind{ScopeGlobal, ScopeWorkspace}) {
			t.Fatalf("GetSection(workspace roles) envelope = %#v", envelope)
		}
	})

	t.Run("Should treat nil and empty role fallback chains as unchanged", func(t *testing.T) {
		t.Parallel()

		current := aghconfig.DefaultRolesConfig()
		desired := aghconfig.CloneRolesConfig(&current)
		desired.Dream.FallbackChain = make([]aghconfig.RoleFallback, 0)
		desired.MemoryController.FallbackChain = make([]aghconfig.RoleFallback, 0)
		if changed := diffRolesSettings(&current, &desired); len(changed) != 0 {
			t.Fatalf("diffRolesSettings() = %#v, want no changes", changed)
		}
	})

	t.Run("Should return provider snapshots without shared nested state", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+`

[providers.custom]
command = "custom-acp"
session_mcp = true

[providers.custom.models]
default = "custom-model"

[providers.custom.models.discovery]
enabled = true
command = "custom-acp models"

[providers.custom.models.reasoning]
apply = "acp_option"

[[providers.custom.models.curated]]
id = "custom-model"
reasoning_efforts = ["low", "max"]
hidden = false
release_date = "2026-07-10"
cost_cache_read_per_million = 0.5
cost_cache_write_per_million = 6
cost_reasoning_per_million = 30
`)
		service := testService(t, homePaths, Dependencies{})

		first, err := service.ActiveConfig(ctx)
		if err != nil {
			t.Fatalf("ActiveConfig(first) error = %v", err)
		}
		provider := first.Providers["custom"]
		provider.Models.Curated[0].ReasoningEfforts[0] = "mutated"
		*provider.Models.Curated[0].Hidden = true
		*provider.Models.Curated[0].CostCacheReadPerMillion = 99
		*provider.Models.Curated[0].CostCacheWritePerMillion = 99
		*provider.Models.Curated[0].CostReasoningPerMillion = 99
		*provider.Models.Discovery.Enabled = false
		*provider.SessionMCP = false

		second, err := service.ActiveConfig(ctx)
		if err != nil {
			t.Fatalf("ActiveConfig(second) error = %v", err)
		}
		provider = second.Providers["custom"]
		if got, want := provider.Models.Curated[0].ReasoningEfforts, []string{
			"low",
			string(modelcatalog.ReasoningEffortMax),
		}; !slices.Equal(got, want) {
			t.Fatalf("second reasoning efforts = %#v, want %#v", got, want)
		}
		if provider.Models.Curated[0].Hidden == nil || *provider.Models.Curated[0].Hidden {
			t.Fatalf("second hidden = %#v, want explicit false", provider.Models.Curated[0].Hidden)
		}
		if provider.Models.Curated[0].CostCacheReadPerMillion == nil ||
			*provider.Models.Curated[0].CostCacheReadPerMillion != 0.5 ||
			provider.Models.Curated[0].CostCacheWritePerMillion == nil ||
			*provider.Models.Curated[0].CostCacheWritePerMillion != 6 ||
			provider.Models.Curated[0].CostReasoningPerMillion == nil ||
			*provider.Models.Curated[0].CostReasoningPerMillion != 30 {
			t.Fatalf("second five-rate pricing = %#v, want immutable snapshot", provider.Models.Curated[0])
		}
		if provider.Models.Discovery.Enabled == nil || !*provider.Models.Discovery.Enabled {
			t.Fatalf("second discovery enabled = %#v, want true", provider.Models.Discovery.Enabled)
		}
		if provider.SessionMCP == nil || !*provider.SessionMCP {
			t.Fatalf("second session_mcp = %#v, want true", provider.SessionMCP)
		}
	})

	t.Run("Should live-apply window-manager config without sharing nested state", func(t *testing.T) {
		t.Parallel()

		ctx := WithMutationSource(context.Background(), "http")
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(ctx); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})

		applier := &fakeConfigRuntimeApplier{}
		service := testService(t, homePaths, Dependencies{
			RuntimeApplier: applier,
			ApplyRecords:   NewConfigApplyRecordRepository(db.DB(), nil),
		})
		desired := testWindowManagerConfig()

		result, err := service.ApplySection(ctx, SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionWindowManager},
			WindowManager:  &desired,
		})
		if err != nil {
			t.Fatalf("ApplySection(window-manager) error = %v", err)
		}
		if !result.Applied || result.RestartRequired {
			t.Fatalf("ApplySection(window-manager) = %#v, want live applied result", result)
		}
		if got, want := result.Record.Lifecycle, lifecycle.Live; got != want {
			t.Fatalf("Lifecycle = %q, want %q", got, want)
		}
		if got, want := result.Record.Status, lifecycle.StatusApplied; got != want {
			t.Fatalf("Status = %q, want %q", got, want)
		}
		if got, want := result.Record.Generation, int64(1); got != want {
			t.Fatalf("Generation = %d, want %d", got, want)
		}
		if got, want := applier.calls, 1; got != want {
			t.Fatalf("ApplyActiveConfig() calls = %d, want %d", got, want)
		}
		if got, want := len(applier.snapshots), 1; got != want {
			t.Fatalf("runtime snapshots = %d, want %d", got, want)
		}
		if got := applier.snapshots[0].WindowManager; !reflect.DeepEqual(got, desired) {
			t.Fatalf("runtime WindowManager = %#v, want %#v", got, desired)
		}

		desired.Snap.RepeatRatios[0] = 0.9
		desired.Shortcuts["desktop.switch.next"] = "Meta+ArrowDown"
		first, err := service.ActiveConfig(ctx)
		if err != nil {
			t.Fatalf("ActiveConfig(first) error = %v", err)
		}
		first.WindowManager.Snap.RepeatRatios[0] = 0.8
		first.WindowManager.Shortcuts["desktop.switch.next"] = "Meta+ArrowUp"
		second, err := service.ActiveConfig(ctx)
		if err != nil {
			t.Fatalf("ActiveConfig(second) error = %v", err)
		}
		if got, want := second.WindowManager.Snap.RepeatRatios[0], 0.4; got != want {
			t.Fatalf("active repeat ratio = %v, want %v", got, want)
		}
		if got, want := second.WindowManager.Shortcuts["desktop.switch.next"], "Meta+ArrowRight"; got != want {
			t.Fatalf("active desktop.switch.next shortcut = %q, want %q", got, want)
		}
	})

	t.Run(
		"Should preserve pending restart-required Network fields while applying availability live",
		func(t *testing.T) {
			t.Parallel()

			ctx := WithMutationSource(context.Background(), "http")
			homePaths := testHomePaths(t)
			writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
			db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
			if err != nil {
				t.Fatalf("OpenGlobalDB() error = %v", err)
			}
			t.Cleanup(func() {
				if err := db.Close(ctx); err != nil {
					t.Fatalf("Close() error = %v", err)
				}
			})

			applier := &fakeConfigRuntimeApplier{}
			service := testService(t, homePaths, Dependencies{
				RuntimeApplier: applier,
				ApplyRecords:   NewConfigApplyRecordRepository(db.DB(), nil),
			})
			initialActive, err := service.ActiveConfig(ctx)
			if err != nil {
				t.Fatalf("ActiveConfig(initial) error = %v", err)
			}

			pendingNetwork := initialActive.Network
			pendingNetwork.MaxReplayAge++
			pending, err := service.ApplySection(ctx, SectionUpdateRequest{
				SectionRequest: SectionRequest{Section: SectionNetwork},
				Network:        &pendingNetwork,
			})
			if err != nil {
				t.Fatalf("ApplySection(pending Network config) error = %v", err)
			}
			if pending.Applied || !pending.RestartRequired || pending.Record.Status != lifecycle.StatusBlocked {
				t.Fatalf("pending Network apply = %#v, want blocked restart-required result", pending)
			}

			desired, err := aghconfig.LoadForHome(homePaths)
			if err != nil {
				t.Fatalf("LoadForHome(pending) error = %v", err)
			}
			desired.Network.Enabled = !initialActive.Network.Enabled
			live, err := service.ApplySection(ctx, SectionUpdateRequest{
				SectionRequest: SectionRequest{Section: SectionNetwork},
				Network:        &desired.Network,
			})
			if err != nil {
				t.Fatalf("ApplySection(Network availability) error = %v", err)
			}
			if !live.Applied || live.RestartRequired || live.Record.Lifecycle != lifecycle.Live {
				t.Fatalf("Network availability apply = %#v, want live applied result", live)
			}
			if live.Record.DesiredHash == live.Record.ActiveHash {
				t.Fatalf(
					"Network hashes = desired %q active %q, want pending restart drift",
					live.Record.DesiredHash,
					live.Record.ActiveHash,
				)
			}
			if got, want := applier.calls, 1; got != want {
				t.Fatalf("ApplyActiveConfig() calls = %d, want %d", got, want)
			}
			if got, want := len(applier.snapshots), 1; got != want {
				t.Fatalf("runtime snapshots = %d, want %d", got, want)
			}
			runtimeNetwork := applier.snapshots[0].Network
			if runtimeNetwork.Enabled == initialActive.Network.Enabled {
				t.Fatalf("runtime Network enabled = %t, want toggled value", runtimeNetwork.Enabled)
			}
			if runtimeNetwork.MaxReplayAge != initialActive.Network.MaxReplayAge {
				t.Fatalf(
					"runtime Network max replay age = %d, want prior active %d",
					runtimeNetwork.MaxReplayAge,
					initialActive.Network.MaxReplayAge,
				)
			}

			active, err := service.ActiveConfig(ctx)
			if err != nil {
				t.Fatalf("ActiveConfig(after live toggle) error = %v", err)
			}
			if active.Network != runtimeNetwork {
				t.Fatalf("active Network = %#v, want runtime projection %#v", active.Network, runtimeNetwork)
			}
			desired, err = aghconfig.LoadForHome(homePaths)
			if err != nil {
				t.Fatalf("LoadForHome(after live toggle) error = %v", err)
			}
			if desired.Network.MaxReplayAge != pendingNetwork.MaxReplayAge {
				t.Fatalf(
					"desired Network max replay age = %d, want pending %d",
					desired.Network.MaxReplayAge,
					pendingNetwork.MaxReplayAge,
				)
			}
			reload, err := service.Reload(ctx)
			if err != nil {
				t.Fatalf("Reload() error = %v", err)
			}
			if reload.Applied || !reload.RestartRequired || reload.Record.Status != lifecycle.StatusBlocked {
				t.Fatalf("Reload() = %#v, want pending restart-required result", reload)
			}
		},
	)

	t.Run("Should apply Network availability from a mixed restart-required update", func(t *testing.T) {
		t.Parallel()

		ctx := WithMutationSource(context.Background(), "http")
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(ctx); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})

		applier := &fakeConfigRuntimeApplier{}
		service := testService(t, homePaths, Dependencies{
			RuntimeApplier: applier,
			ApplyRecords:   NewConfigApplyRecordRepository(db.DB(), nil),
		})
		initial, err := service.ActiveConfig(ctx)
		if err != nil {
			t.Fatalf("ActiveConfig(initial) error = %v", err)
		}
		desired := initial.Network
		desired.Enabled = !initial.Network.Enabled
		desired.MaxReplayAge++

		result, err := service.ApplySection(ctx, SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionNetwork},
			Network:        &desired,
		})
		if err != nil {
			t.Fatalf("ApplySection(mixed Network) error = %v", err)
		}
		if result.Applied || !result.RestartRequired || result.Record.Status != lifecycle.StatusBlocked {
			t.Fatalf("ApplySection(mixed Network) = %#v, want remaining restart-required drift", result)
		}
		if got, want := applier.calls, 1; got != want {
			t.Fatalf("ApplyActiveConfig() calls = %d, want %d", got, want)
		}
		active, err := service.ActiveConfig(ctx)
		if err != nil {
			t.Fatalf("ActiveConfig(after mixed update) error = %v", err)
		}
		if active.Network.Enabled != desired.Enabled {
			t.Fatalf("active network.enabled = %t, want %t", active.Network.Enabled, desired.Enabled)
		}
		if active.Network.MaxReplayAge != initial.Network.MaxReplayAge {
			t.Fatalf(
				"active network.max_replay_age = %d, want prior active %d",
				active.Network.MaxReplayAge,
				initial.Network.MaxReplayAge,
			)
		}
	})
}

func TestConfigApplyServiceRecordsRestartRequiredWithoutAdvancingGeneration(t *testing.T) {
	t.Parallel()

	t.Run("Should persist blocked record for restart-required change", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(ctx); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})

		service := testService(t, homePaths, Dependencies{
			ApplyRecords: NewConfigApplyRecordRepository(db.DB(), nil),
		})

		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		general := GeneralSettings{
			Defaults:       cfg.Defaults,
			Limits:         cfg.Limits,
			Permissions:    cfg.Permissions,
			SessionTimeout: cfg.Session.Limits.Timeout,
			HTTP:           cfg.HTTP,
			Daemon:         cfg.Daemon,
		}
		general.HTTP.Port = 2124

		result, err := service.ApplySection(WithMutationSource(ctx, "uds"), SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionGeneral},
			General:        &general,
		})
		if err != nil {
			t.Fatalf("ApplySection(general) error = %v", err)
		}
		if result.Applied {
			t.Fatal("ApplySection(general).Applied = true, want false")
		}
		if got, want := result.Record.Lifecycle, lifecycle.RestartRequired; got != want {
			t.Fatalf("Lifecycle = %q, want %q", got, want)
		}
		if got, want := result.Record.Status, lifecycle.StatusBlocked; got != want {
			t.Fatalf("Status = %q, want %q", got, want)
		}
		if got, want := result.NextAction, lifecycle.NextActionRestartDaemon; got != want {
			t.Fatalf("NextAction = %q, want %q", got, want)
		}
		if got, want := result.Record.Generation, int64(0); got != want {
			t.Fatalf("Generation = %d, want %d", got, want)
		}

		records, err := service.ListApplyRecords(ctx, ApplyRecordFilter{Status: lifecycle.StatusBlocked})
		if err != nil {
			t.Fatalf("ListApplyRecords(blocked) error = %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("ListApplyRecords(blocked) len = %d, want 1", len(records))
		}
		if len(records[0].Diagnostics) == 0 {
			t.Fatal("Diagnostics len = 0, want restart-required diagnostic")
		}
	})
}

func TestConfigApplyServiceProviderOverlayForBuiltinRequiresRestart(t *testing.T) {
	t.Parallel()

	t.Run("Should block active generation when provider overlay replaces builtin provider", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(ctx); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})

		service := testService(t, homePaths, Dependencies{
			ApplyRecords: NewConfigApplyRecordRepository(db.DB(), nil),
		})

		result, err := service.ApplyCollectionItem(
			WithMutationSource(ctx, "http"),
			CollectionItemPutRequest{
				CollectionRequest: CollectionRequest{Collection: CollectionProviders},
				Name:              "codex",
				Provider: &ProviderSettings{
					Command: "codex-browser",
				},
			},
		)
		if err != nil {
			t.Fatalf("ApplyCollectionItem(provider codex) error = %v", err)
		}
		if result.Applied {
			t.Fatal("ApplyCollectionItem(provider codex).Applied = true, want false")
		}
		if !result.RestartRequired {
			t.Fatal("ApplyCollectionItem(provider codex).RestartRequired = false, want true")
		}
		if got, want := result.Record.Lifecycle, lifecycle.RestartRequired; got != want {
			t.Fatalf("Lifecycle = %q, want %q", got, want)
		}
		if got, want := result.Record.Status, lifecycle.StatusBlocked; got != want {
			t.Fatalf("Status = %q, want %q", got, want)
		}
		if got, want := result.Record.Generation, int64(0); got != want {
			t.Fatalf("Generation = %d, want %d", got, want)
		}
	})
}

func TestConfigApplyServiceAppliesProviderModelOnlyChangesLive(t *testing.T) {
	t.Parallel()

	t.Run("Should project five-rate catalog metadata through provider settings", func(t *testing.T) {
		t.Parallel()

		service, _, _, _ := providerModelCurationTestService(t)
		envelope, err := service.ListCollection(context.Background(), CollectionRequest{
			Collection: CollectionProviders,
		})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		provider := mustFindProviderItem(t, envelope.Providers, "codex")
		var model *aghconfig.ProviderModelConfig
		for index := range provider.Settings.Models.Curated {
			if provider.Settings.Models.Curated[index].ID == "gpt-5.6-sol" {
				model = &provider.Settings.Models.Curated[index]
				break
			}
		}
		if model == nil || model.CostInputPerMillion == nil || *model.CostInputPerMillion != 5 ||
			model.CostOutputPerMillion == nil || *model.CostOutputPerMillion != 30 ||
			model.CostCacheReadPerMillion == nil || *model.CostCacheReadPerMillion != 0.5 ||
			model.CostCacheWritePerMillion == nil || *model.CostCacheWritePerMillion != 6 ||
			model.CostReasoningPerMillion == nil || *model.CostReasoningPerMillion != 30 {
			t.Fatalf("provider settings five-rate catalog metadata = %#v", model)
		}
	})

	t.Run("Should skip an unchanged builtin projection without applying runtime config", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(ctx); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})

		applier := &fakeConfigRuntimeApplier{}
		eventStore := &recordingEventSummaryStore{}
		service := testService(t, homePaths, Dependencies{
			ApplyRecords: NewConfigApplyRecordRepository(db.DB(), nil),
			ModelCatalog: &settingsModelCatalogStub{models: map[string][]modelcatalog.Model{
				"claude": {{ProviderID: "claude", ModelID: "claude-sonnet-5", Curated: true}},
			}},
			RuntimeApplier: applier,
			EventSummaries: eventStore,
		})
		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionProviders})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		settings := mustFindProviderItem(t, envelope.Providers, "claude").Settings
		settings.ModelsSet = true
		before := readFile(t, homePaths.ConfigFile)
		result, err := service.ApplyCollectionItem(
			WithMutationSource(ctx, "http"),
			CollectionItemPutRequest{
				CollectionRequest: CollectionRequest{Collection: CollectionProviders},
				Name:              "claude",
				Provider:          &settings,
			},
		)
		if err != nil {
			t.Fatalf("ApplyCollectionItem(unchanged builtin projection) error = %v", err)
		}
		if !result.Applied || result.RestartRequired || !result.Skipped {
			t.Fatalf("apply result = %#v, want applied skipped no-op without restart", result)
		}
		if got, want := result.SkippedReason, configApplyNoChangesReason; got != want {
			t.Fatalf("SkippedReason = %q, want %q", got, want)
		}
		if got := applier.calls; got != 0 {
			t.Fatalf("ApplyActiveConfig() calls = %d, want 0", got)
		}
		summaries, err := eventStore.ListEventSummaries(ctx, store.EventSummaryQuery{})
		if err != nil {
			t.Fatalf("ListEventSummaries() error = %v", err)
		}
		if got := len(summaries); got != 0 {
			t.Fatalf("settings.changed summaries = %d, want 0", got)
		}
		if after := readFile(t, homePaths.ConfigFile); after != before {
			t.Fatalf("unchanged builtin projection rewrote config:\n%s", after)
		}
	})

	t.Run("Should reconcile the catalog without requiring a daemon restart", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(ctx); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})

		applier := &fakeConfigRuntimeApplier{}
		service := testService(t, homePaths, Dependencies{
			ApplyRecords: NewConfigApplyRecordRepository(db.DB(), nil),
			ModelCatalog: &settingsModelCatalogStub{models: map[string][]modelcatalog.Model{
				"codex": {{ProviderID: "codex", ModelID: "gpt-5.6-terra", Curated: true}},
			}},
			RuntimeApplier: applier,
		})
		result, err := service.ApplyCollectionItem(
			WithMutationSource(ctx, "http"),
			CollectionItemPutRequest{
				CollectionRequest: CollectionRequest{Collection: CollectionProviders},
				Name:              "codex",
				Provider: &ProviderSettings{
					ModelsSet: true,
					Models: aghconfig.ProviderModelsConfig{
						Default: "gpt-5.6-terra",
						Reasoning: aghconfig.ProviderReasoningConfig{
							Apply: aghconfig.ReasoningApplyACPOption,
						},
						Curated: []aghconfig.ProviderModelConfig{{
							ID: "gpt-5.6-terra",
							ReasoningEfforts: []string{
								"none",
								"low",
								"medium",
								"high",
								"xhigh",
								string(modelcatalog.ReasoningEffortMax),
							},
							DefaultReasoningEffort: "high",
						}},
					},
				},
			},
		)
		if err != nil {
			t.Fatalf("ApplyCollectionItem(provider models) error = %v", err)
		}
		if !result.Applied || result.RestartRequired {
			t.Fatalf("apply result = %#v, want applied live without restart", result)
		}
		if got, want := result.Record.Lifecycle, lifecycle.Live; got != want {
			t.Fatalf("Lifecycle = %q, want %q", got, want)
		}
		if got, want := result.Record.Status, lifecycle.StatusApplied; got != want {
			t.Fatalf("Status = %q, want %q", got, want)
		}
		if got, want := applier.calls, 1; got != want {
			t.Fatalf("ApplyActiveConfig() calls = %d, want %d", got, want)
		}
		active, err := service.ActiveConfig(ctx)
		if err != nil {
			t.Fatalf("ActiveConfig() error = %v", err)
		}
		provider, err := active.ResolveProvider("codex")
		if err != nil {
			t.Fatalf("ResolveProvider(codex) error = %v", err)
		}
		if got, want := provider.Models.Default, "gpt-5.6-terra"; got != want {
			t.Fatalf("active codex model default = %q, want %q", got, want)
		}
	})
}

func TestConfigApplyServiceAppliesExtensionSideLoadPolicyLive(t *testing.T) {
	t.Parallel()
	t.Run("Should apply extension side-load policy without a restart", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(ctx); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})

		applier := &fakeConfigRuntimeApplier{}
		service := testService(t, homePaths, Dependencies{
			ApplyRecords:   NewConfigApplyRecordRepository(db.DB(), nil),
			RuntimeApplier: applier,
		})
		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		cfg.Extensions.Marketplace.AllowUnverified = true
		result, err := service.ApplySection(WithMutationSource(ctx, "http"), SectionUpdateRequest{
			SectionRequest:  SectionRequest{Section: SectionHooksExtensions},
			HooksExtensions: &cfg.Extensions,
		})
		if err != nil {
			t.Fatalf("ApplySection(hooks-extensions) error = %v", err)
		}
		if !result.Applied || result.RestartRequired {
			t.Fatalf("ApplySection(hooks-extensions) = %#v, want live applied", result)
		}
		if result.Record.Lifecycle != lifecycle.Live || applier.calls != 1 {
			t.Fatalf("Lifecycle = %q, applier calls = %d, want live and one", result.Record.Lifecycle, applier.calls)
		}
		active, err := service.ActiveConfig(ctx)
		if err != nil {
			t.Fatalf("ActiveConfig() error = %v", err)
		}
		if !active.Extensions.Marketplace.AllowUnverified {
			t.Fatal("ActiveConfig() extension side-load policy = false, want true")
		}
	})
}

func TestReloadChangedPaths(t *testing.T) {
	t.Parallel()

	baseProvider := aghconfig.ProviderConfig{
		Command: "codex",
		Models:  aghconfig.ProviderModelsConfig{Default: "gpt-5.6-sol"},
	}
	tests := []struct {
		name            string
		desiredProvider aghconfig.ProviderConfig
		want            []string
	}{
		{
			name: "Should emit only the live model path for model-only changes",
			desiredProvider: aghconfig.ProviderConfig{
				Command: "codex",
				Models:  aghconfig.ProviderModelsConfig{Default: "gpt-5.6-terra"},
			},
			want: []string{"providers.codex.models"},
		},
		{
			name: "Should emit restart and live paths for mixed provider changes",
			desiredProvider: aghconfig.ProviderConfig{
				Command: "codex-browser",
				Models:  aghconfig.ProviderModelsConfig{Default: "gpt-5.6-terra"},
			},
			want: []string{"providers.codex", "providers.codex.models"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			current := &aghconfig.Config{Providers: map[string]aghconfig.ProviderConfig{"codex": baseProvider}}
			desired := &aghconfig.Config{
				Providers: map[string]aghconfig.ProviderConfig{"codex": tt.desiredProvider},
			}
			if got := reloadChangedPaths(current, desired); !slices.Equal(got, tt.want) {
				t.Fatalf("reloadChangedPaths() = %#v, want %#v", got, tt.want)
			}
		})
	}

	t.Run("Should emit every live Marketplace catalog path", func(t *testing.T) {
		t.Parallel()

		current := &aghconfig.Config{Marketplace: aghconfig.DefaultMarketplaceRuntimeConfig()}
		desired := *current
		desired.Marketplace.Catalog.BaseURL = "https://catalog.example.test"
		desired.Marketplace.Catalog.TTL = "30m"
		desired.Marketplace.Catalog.Timeout = "5s"

		want := []string{
			"marketplace.catalog.base_url",
			"marketplace.catalog.ttl",
			"marketplace.catalog.timeout",
		}
		if got := reloadChangedPaths(current, &desired); !slices.Equal(got, want) {
			t.Fatalf("reloadChangedPaths() = %#v, want %#v", got, want)
		}
	})

	t.Run("Should classify role mutations as live", func(t *testing.T) {
		t.Parallel()

		current := aghconfig.DefaultWithHome(aghconfig.HomePaths{})
		desired := current
		desired.Roles.AutoTitle.Enabled = false

		want := []string{"roles.auto_title.enabled"}
		if got := reloadChangedPaths(&current, &desired); !slices.Equal(got, want) {
			t.Fatalf("reloadChangedPaths() = %#v, want %#v", got, want)
		}
		if got := classifyReloadLifecycle(&current, &desired); got != lifecycle.Live {
			t.Fatalf("classifyReloadLifecycle() = %q, want %q", got, lifecycle.Live)
		}
	})

	t.Run("Should emit the restart-required automation suggestion cap path", func(t *testing.T) {
		t.Parallel()

		current := &aghconfig.Config{}
		current.Automation.Suggestions.PendingCap = automationmodel.DefaultSuggestionPendingCap
		desired := *current
		desired.Automation.Suggestions.PendingCap = automationmodel.DefaultSuggestionPendingCap + 1

		want := []string{"automation.suggestions.pending_cap"}
		if got := reloadChangedPaths(current, &desired); !slices.Equal(got, want) {
			t.Fatalf("reloadChangedPaths() = %#v, want %#v", got, want)
		}
		if got := classifyReloadLifecycle(current, &desired); got != lifecycle.RestartRequired {
			t.Fatalf("classifyReloadLifecycle() = %q, want %q", got, lifecycle.RestartRequired)
		}
	})

	t.Run("Should emit a live window manager path", func(t *testing.T) {
		t.Parallel()

		current := &aghconfig.Config{WindowManager: aghconfig.DefaultWindowManagerConfig()}
		desired := *current
		desired.WindowManager.HistoryLimit++

		want := []string{"window_manager.history_limit"}
		if got := reloadChangedPaths(current, &desired); !slices.Equal(got, want) {
			t.Fatalf("reloadChangedPaths() = %#v, want %#v", got, want)
		}
		if got := classifyReloadLifecycle(current, &desired); got != lifecycle.Live {
			t.Fatalf("classifyReloadLifecycle() = %q, want %q", got, lifecycle.Live)
		}
	})
}

func TestConfigApplyServiceCuratesProviderModelsLive(t *testing.T) {
	t.Parallel()

	t.Run("Should persist exact curation and return the reconciled catalog row", func(t *testing.T) {
		t.Parallel()

		service, homePaths, _, applier := providerModelCurationTestService(t)
		hidden := true
		featured := false
		deprecated := false
		defaultEffort := modelcatalog.ReasoningEffortMax
		result, err := service.ApplyProviderModelCuration(
			WithMutationSource(context.Background(), "cli"),
			ProviderModelCurationRequest{
				ProviderID:             "codex",
				ModelID:                "gpt-5.6-sol",
				Hidden:                 &hidden,
				Featured:               &featured,
				Deprecated:             &deprecated,
				DefaultReasoningEffort: &defaultEffort,
			},
		)
		if err != nil {
			t.Fatalf("ApplyProviderModelCuration() error = %v", err)
		}
		if applier.err != nil {
			t.Fatalf("runtime applier reconciliation error = %v", applier.err)
		}
		if got, want := applier.calls, 1; got != want {
			t.Fatalf("ApplyActiveConfig() calls = %d, want %d", got, want)
		}
		if !result.Apply.Applied || result.Apply.RestartRequired {
			t.Fatalf("apply result = %#v, want live applied curation", result.Apply)
		}
		if got, want := result.Apply.Record.Lifecycle, lifecycle.Live; got != want {
			t.Fatalf("Lifecycle = %q, want %q", got, want)
		}
		if !result.Model.Hidden || result.Model.Featured || result.Model.Deprecated {
			t.Fatalf("effective curation flags = %#v, want hidden only", result.Model)
		}
		if result.Model.DefaultReasoningEffort == nil ||
			*result.Model.DefaultReasoningEffort != modelcatalog.ReasoningEffortMax {
			t.Fatalf("DefaultReasoningEffort = %#v, want max", result.Model.DefaultReasoningEffort)
		}

		configText := readFile(t, homePaths.ConfigFile)
		for _, want := range []string{
			`[[providers.codex.models.curated]]`,
			`id = "gpt-5.6-sol"`,
			`hidden = true`,
			`default_reasoning_effort = "max"`,
		} {
			if !strings.Contains(configText, want) {
				t.Fatalf("config is missing %q:\n%s", want, configText)
			}
		}
		curatedTable := tomlTableForTest(t, configText, `[[providers.codex.models.curated]]`)
		if strings.Contains(curatedTable, `featured = true`) || strings.Contains(curatedTable, `deprecated = true`) {
			t.Fatalf("config persisted false curation flags as true:\n%s", configText)
		}
		for _, forbidden := range []string{
			`display_name =`,
			`context_window =`,
			`max_output_tokens =`,
			`supports_tools =`,
			`reasoning_efforts =`,
			`cost_input_per_million =`,
			`cost_output_per_million =`,
			`cost_cache_read_per_million =`,
			`cost_cache_write_per_million =`,
			`cost_reasoning_per_million =`,
			`release_date =`,
		} {
			if strings.Contains(curatedTable, forbidden) {
				t.Fatalf("config froze catalog enrichment %q:\n%s", forbidden, configText)
			}
		}
	})

	t.Run("Should preserve only pre-existing operator-authored model metadata", func(t *testing.T) {
		t.Parallel()

		service, homePaths, _, applier := providerModelCurationTestService(t)
		writeFile(t, homePaths.ConfigFile, readFile(t, homePaths.ConfigFile)+`

[[providers.codex.models.curated]]
id = "gpt-5.6-sol"
display_name = "Operator Sol"
context_window = 4242
reasoning_efforts = ["low", "max"]
featured = true
`)
		hidden := true
		if _, err := service.ApplyProviderModelCuration(
			WithMutationSource(context.Background(), "cli"),
			ProviderModelCurationRequest{
				ProviderID: "codex",
				ModelID:    "gpt-5.6-sol",
				Hidden:     &hidden,
			},
		); err != nil {
			t.Fatalf("ApplyProviderModelCuration() error = %v", err)
		}
		if applier.err != nil {
			t.Fatalf("runtime applier reconciliation error = %v", applier.err)
		}

		configText := readFile(t, homePaths.ConfigFile)
		for _, want := range []string{
			`display_name = "Operator Sol"`,
			`context_window = 4242`,
			`reasoning_efforts = ["low", "max"]`,
			`featured = true`,
			`hidden = true`,
		} {
			if !strings.Contains(configText, want) {
				t.Fatalf("config is missing preserved operator field %q:\n%s", want, configText)
			}
		}
		for _, forbidden := range []string{
			`display_name = "GPT-5.6 Sol"`,
			`max_output_tokens = 128000`,
			`cost_input_per_million = 5`,
			`cost_cache_read_per_million = 0.5`,
			`cost_cache_write_per_million = 6`,
			`cost_reasoning_per_million = 30`,
			`release_date = "2026-06-26"`,
		} {
			if strings.Contains(configText, forbidden) {
				t.Fatalf("config replaced operator row with catalog enrichment %q:\n%s", forbidden, configText)
			}
		}
	})

	t.Run("Should preserve pending restart-required provider fields while applying models live", func(t *testing.T) {
		t.Parallel()

		ctx := WithMutationSource(context.Background(), "cli")
		service, homePaths, _, applier := providerModelCurationTestService(t)
		initialActive, err := service.ActiveConfig(ctx)
		if err != nil {
			t.Fatalf("ActiveConfig(initial) error = %v", err)
		}
		initialProvider, err := initialActive.ResolveProvider("codex")
		if err != nil {
			t.Fatalf("ResolveProvider(initial codex) error = %v", err)
		}

		const pendingCommand = "codex-pending-restart"
		pending, err := service.ApplyCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "codex",
			Provider:          &ProviderSettings{Command: pendingCommand},
		})
		if err != nil {
			t.Fatalf("ApplyCollectionItem(pending command) error = %v", err)
		}
		if pending.Applied || !pending.RestartRequired ||
			pending.Record.Status != lifecycle.StatusBlocked {
			t.Fatalf("pending command apply = %#v, want blocked restart-required result", pending)
		}

		hidden := true
		curated, err := service.ApplyProviderModelCuration(ctx, ProviderModelCurationRequest{
			ProviderID: "codex",
			ModelID:    "gpt-5.6-sol",
			Hidden:     &hidden,
		})
		if err != nil {
			t.Fatalf("ApplyProviderModelCuration() error = %v", err)
		}
		if !curated.Apply.Applied || curated.Apply.RestartRequired ||
			curated.Apply.Record.Lifecycle != lifecycle.Live {
			t.Fatalf("curation apply = %#v, want live applied result", curated.Apply)
		}
		if applier.err != nil {
			t.Fatalf("runtime applier reconciliation error = %v", applier.err)
		}
		if curated.Apply.Record.DesiredHash == curated.Apply.Record.ActiveHash {
			t.Fatalf(
				"curation hashes = desired %q active %q, want pending restart drift",
				curated.Apply.Record.DesiredHash,
				curated.Apply.Record.ActiveHash,
			)
		}
		if len(applier.snapshots) != 1 {
			t.Fatalf("runtime snapshots = %d, want 1", len(applier.snapshots))
		}
		runtimeProvider, err := applier.snapshots[0].ResolveProvider("codex")
		if err != nil {
			t.Fatalf("ResolveProvider(runtime codex) error = %v", err)
		}
		if runtimeProvider.Command != initialProvider.Command {
			t.Fatalf(
				"runtime command = %q, want prior active command %q",
				runtimeProvider.Command,
				initialProvider.Command,
			)
		}
		assertProviderModelHidden(t, runtimeProvider.Models, "gpt-5.6-sol")

		active, err := service.ActiveConfig(ctx)
		if err != nil {
			t.Fatalf("ActiveConfig(after curation) error = %v", err)
		}
		activeProvider, err := active.ResolveProvider("codex")
		if err != nil {
			t.Fatalf("ResolveProvider(active codex) error = %v", err)
		}
		if activeProvider.Command != initialProvider.Command {
			t.Fatalf("active command = %q, want %q", activeProvider.Command, initialProvider.Command)
		}
		assertProviderModelHidden(t, activeProvider.Models, "gpt-5.6-sol")

		desired, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(desired) error = %v", err)
		}
		desiredProvider, err := desired.ResolveProvider("codex")
		if err != nil {
			t.Fatalf("ResolveProvider(desired codex) error = %v", err)
		}
		if desiredProvider.Command != pendingCommand {
			t.Fatalf("desired command = %q, want pending %q", desiredProvider.Command, pendingCommand)
		}
		assertProviderModelHidden(t, desiredProvider.Models, "gpt-5.6-sol")

		reload, err := service.Reload(ctx)
		if err != nil {
			t.Fatalf("Reload() error = %v", err)
		}
		if reload.Applied || !reload.RestartRequired || reload.Record.Status != lifecycle.StatusBlocked {
			t.Fatalf("Reload() = %#v, want pending restart-required result", reload)
		}
	})

	t.Run("Should reject an unknown canonical model without writing config", func(t *testing.T) {
		t.Parallel()

		service, homePaths, _, applier := providerModelCurationTestService(t)
		before := readFile(t, homePaths.ConfigFile)
		_, err := service.ApplyProviderModelCuration(context.Background(), ProviderModelCurationRequest{
			ProviderID: "codex",
			ModelID:    "gpt-sol",
		})
		if err == nil {
			t.Fatal("ApplyProviderModelCuration(unknown) error = nil, want model_not_found")
		}
		item, ok := diagnostics.ItemFromError(err)
		if !ok || item.Code != diagnosticcontract.CodeModelNotFound {
			t.Fatalf("diagnostic = %#v, %v; want model_not_found", item, ok)
		}
		if got := readFile(t, homePaths.ConfigFile); got != before {
			t.Fatalf("unknown-model curation changed config:\n%s", got)
		}
		if applier.calls != 0 {
			t.Fatalf("ApplyActiveConfig() calls = %d, want 0", applier.calls)
		}
	})

	t.Run("Should reject a canonical effort outside the model subset", func(t *testing.T) {
		t.Parallel()

		service, homePaths, _, applier := providerModelCurationTestService(t)
		before := readFile(t, homePaths.ConfigFile)
		unsupported := modelcatalog.ReasoningEffortMinimal
		_, err := service.ApplyProviderModelCuration(context.Background(), ProviderModelCurationRequest{
			ProviderID:             "codex",
			ModelID:                "gpt-5.6-sol",
			DefaultReasoningEffort: &unsupported,
		})
		if err == nil {
			t.Fatal("ApplyProviderModelCuration(minimal) error = nil, want reasoning_effort_unsupported")
		}
		item, ok := diagnostics.ItemFromError(err)
		if !ok || item.Code != diagnosticcontract.CodeReasoningEffortUnsupported {
			t.Fatalf("diagnostic = %#v, %v; want reasoning_effort_unsupported", item, ok)
		}
		if got := readFile(t, homePaths.ConfigFile); got != before {
			t.Fatalf("unsupported-effort curation changed config:\n%s", got)
		}
		if applier.calls != 0 {
			t.Fatalf("ApplyActiveConfig() calls = %d, want 0", applier.calls)
		}
	})
}

func tomlTableForTest(t *testing.T, document string, header string) string {
	t.Helper()

	start := strings.Index(document, header)
	if start < 0 {
		t.Fatalf("TOML document is missing table %q:\n%s", header, document)
	}
	table := document[start:]
	if next := strings.Index(table[len(header):], "\n["); next >= 0 {
		table = table[:len(header)+next]
	}
	return table
}

func TestConfigApplyServiceReloadClassifiesUnknownPathsConservatively(t *testing.T) {
	t.Parallel()

	t.Run("Should block reload when desired config changed outside explicit diff coverage", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(ctx); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})

		service := testService(t, homePaths, Dependencies{
			ApplyRecords: NewConfigApplyRecordRepository(db.DB(), nil),
		})
		initial, err := service.Reload(WithMutationSource(ctx, "boot"))
		if err != nil {
			t.Fatalf("Reload(initial) error = %v", err)
		}
		if !initial.Skipped {
			t.Fatal("Reload(initial).Skipped = false, want true")
		}
		records, err := service.ListApplyRecords(ctx, ApplyRecordFilter{})
		if err != nil {
			t.Fatalf("ListApplyRecords(initial) error = %v", err)
		}
		if len(records) != 0 {
			t.Fatalf("initial apply records len = %d, want 0", len(records))
		}

		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+`
[log]
level = "debug"
`)

		result, err := service.Reload(WithMutationSource(ctx, "cli"))
		if err != nil {
			t.Fatalf("Reload(log-level) error = %v", err)
		}
		if result.Applied {
			t.Fatal("Reload(log-level).Applied = true, want false")
		}
		if got, want := result.Record.Lifecycle, lifecycle.RestartRequired; got != want {
			t.Fatalf("Lifecycle = %q, want %q", got, want)
		}
		if got, want := result.Record.Status, lifecycle.StatusBlocked; got != want {
			t.Fatalf("Status = %q, want %q", got, want)
		}
		if got, want := result.Record.Generation, int64(0); got != want {
			t.Fatalf("Generation = %d, want %d", got, want)
		}
	})
}

func TestConfigApplyServiceReloadUsesBootedConfigAsActiveState(t *testing.T) {
	t.Parallel()

	t.Run("Should not skip restart-required reload after booting a blocked config", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(ctx); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})

		service := testService(t, homePaths, Dependencies{
			SkillsRuntime: newFakeSkillsRuntime(testSkill("alpha", false)),
			ApplyRecords:  NewConfigApplyRecordRepository(db.DB(), nil),
		})

		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(initial) error = %v", err)
		}
		cfg.Skills.DisabledSkills = []string{"alpha"}
		applied, err := service.ApplySection(WithMutationSource(ctx, "http"), SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionSkills},
			Skills:         &cfg.Skills,
		})
		if err != nil {
			t.Fatalf("ApplySection(skills) error = %v", err)
		}
		if !applied.Applied {
			t.Fatal("ApplySection(skills).Applied = false, want true")
		}

		cfg, err = aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(before automation) error = %v", err)
		}
		automation := automationSettingsFromConfig(&cfg)
		automation.Enabled = false
		blocked, err := service.ApplySection(WithMutationSource(ctx, "http"), SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionAutomation},
			Automation:     &automation,
		})
		if err != nil {
			t.Fatalf("ApplySection(automation) error = %v", err)
		}
		if blocked.Applied {
			t.Fatal("ApplySection(automation).Applied = true, want false")
		}

		restarted := testService(t, homePaths, Dependencies{
			SkillsRuntime: newFakeSkillsRuntime(testSkill("alpha", false)),
			ApplyRecords:  NewConfigApplyRecordRepository(db.DB(), nil),
		})
		active, err := restarted.ActiveConfig(ctx)
		if err != nil {
			t.Fatalf("ActiveConfig(after restart) error = %v", err)
		}
		if active.Automation.Enabled {
			t.Fatal("ActiveConfig(after restart).Automation.Enabled = true, want false")
		}
		target, err := aghconfig.ResolveConfigWriteTarget(homePaths, "", aghconfig.WriteScopeGlobal)
		if err != nil {
			t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
		}
		if _, err := aghconfig.EditConfigOverlay(homePaths, "", target, func(editor *aghconfig.OverlayEditor) error {
			return editor.SetValue([]string{"automation", "enabled"}, true)
		}); err != nil {
			t.Fatalf("EditConfigOverlay(automation.enabled=true) error = %v", err)
		}

		result, err := restarted.Reload(WithMutationSource(ctx, "cli"))
		if err != nil {
			t.Fatalf("Reload(after booted blocked config) error = %v", err)
		}
		if result.Applied {
			t.Fatal("Reload(after booted blocked config).Applied = true, want false")
		}
		if !result.RestartRequired {
			t.Fatal("Reload(after booted blocked config).RestartRequired = false, want true")
		}
		if got, want := result.Record.Lifecycle, lifecycle.RestartRequired; got != want {
			t.Fatalf("Lifecycle = %q, want %q", got, want)
		}
		if got, want := result.Record.Status, lifecycle.StatusBlocked; got != want {
			t.Fatalf("Status = %q, want %q", got, want)
		}
	})
}

func TestConfigApplyServiceRecordsRuntimeReconcileFailures(t *testing.T) {
	t.Parallel()

	t.Run("Should fail apply record without advancing generation when runtime reconcile fails", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(ctx); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})

		applier := &fakeConfigRuntimeApplier{
			failures: []ApplyFailure{{
				Subsystem: "mcp",
				Diagnostic: diagnostics.NewItem(
					"config.apply.test_failure",
					diagnosticcontract.CodeConfigPartialFailure,
					diagnosticcontract.CategoryMCP,
					"Test runtime reconcile failed",
					"runtime reconcile failed",
					diagnosticcontract.SeverityError,
					diagnosticcontract.FreshnessLive,
				),
			}},
		}
		service := testService(t, homePaths, Dependencies{
			SkillsRuntime:  newFakeSkillsRuntime(testSkill("alpha", false)),
			ApplyRecords:   NewConfigApplyRecordRepository(db.DB(), nil),
			RuntimeApplier: applier,
		})

		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		cfg.Skills.DisabledSkills = []string{"alpha"}

		result, err := service.ApplySection(WithMutationSource(ctx, "http"), SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionSkills},
			Skills:         &cfg.Skills,
		})
		if err != nil {
			t.Fatalf("ApplySection(skills) error = %v", err)
		}
		if result.Applied {
			t.Fatal("ApplySection(skills).Applied = true, want false")
		}
		if got, want := applier.calls, 1; got != want {
			t.Fatalf("runtime applier calls = %d, want %d", got, want)
		}
		if got, want := result.Record.Status, lifecycle.StatusFailed; got != want {
			t.Fatalf("Status = %q, want %q", got, want)
		}
		if got, want := result.Record.Lifecycle, lifecycle.Live; got != want {
			t.Fatalf("Lifecycle = %q, want %q", got, want)
		}
		if got, want := result.Record.Generation, int64(0); got != want {
			t.Fatalf("Generation = %d, want %d", got, want)
		}
		if result.Record.AppliedAt != nil {
			t.Fatalf("AppliedAt = %v, want nil", result.Record.AppliedAt)
		}
		if len(result.PartialFailures) != 1 {
			t.Fatalf("PartialFailures len = %d, want 1", len(result.PartialFailures))
		}
	})
}

func TestConfigApplyServiceFailedRecordsPreserveLifecycleIntent(t *testing.T) {
	t.Parallel()

	t.Run("Should record skills validation failure as live lifecycle", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(ctx); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})

		service := testService(t, homePaths, Dependencies{
			ApplyRecords: NewConfigApplyRecordRepository(db.DB(), nil),
		})

		result, err := service.ApplySection(WithMutationSource(ctx, "cli"), SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionSkills},
		})
		if err == nil {
			t.Fatal("ApplySection(skills nil payload) error = nil, want validation error")
		}
		if result.Applied {
			t.Fatal("ApplySection(skills nil payload).Applied = true, want false")
		}
		if got, want := result.Record.Status, lifecycle.StatusFailed; got != want {
			t.Fatalf("Status = %q, want %q", got, want)
		}
		if got, want := result.Record.Lifecycle, lifecycle.Live; got != want {
			t.Fatalf("Lifecycle = %q, want %q", got, want)
		}
	})
}

type fakeConfigRuntimeApplier struct {
	failures  []ApplyFailure
	calls     int
	snapshots []aghconfig.Config
}

type providerModelCurationRuntimeApplier struct {
	catalog   *settingsModelCatalogStub
	calls     int
	err       error
	snapshots []aghconfig.Config
}

func (a *providerModelCurationRuntimeApplier) ApplyActiveConfig(
	_ context.Context,
	cfg *aghconfig.Config,
) []ApplyFailure {
	a.calls++
	if cfg == nil {
		return a.fail(fmt.Errorf("active config is nil"))
	}
	a.snapshots = append(a.snapshots, cloneActiveConfig(cfg))
	provider, err := cfg.ResolveProvider("codex")
	if err != nil {
		return a.fail(err)
	}
	var curated *aghconfig.ProviderModelConfig
	for index := range provider.Models.Curated {
		if provider.Models.Curated[index].ID == "gpt-5.6-sol" {
			curated = &provider.Models.Curated[index]
			break
		}
	}
	if curated == nil {
		return a.fail(fmt.Errorf("curated config row gpt-5.6-sol is missing"))
	}
	a.catalog.mu.Lock()
	defer a.catalog.mu.Unlock()
	models := a.catalog.models["codex"]
	for index := range models {
		if models[index].ModelID != curated.ID {
			continue
		}
		if curated.Hidden != nil {
			models[index].Hidden = *curated.Hidden
		}
		if curated.Featured != nil {
			models[index].Featured = *curated.Featured
		}
		if curated.Deprecated != nil {
			models[index].Deprecated = *curated.Deprecated
		}
		models[index].Curated = !models[index].Hidden && !models[index].Deprecated
		if curated.DefaultReasoningEffort != "" {
			effort := modelcatalog.ReasoningEffort(curated.DefaultReasoningEffort)
			models[index].DefaultReasoningEffort = &effort
		}
		a.catalog.models["codex"] = models
		return nil
	}
	return a.fail(fmt.Errorf("catalog row gpt-5.6-sol is missing"))
}

func (a *providerModelCurationRuntimeApplier) fail(err error) []ApplyFailure {
	a.err = err
	return []ApplyFailure{{
		Subsystem: "model_catalog",
		Diagnostic: diagnostics.NewItem(
			"config.apply.model_catalog_sync_failed",
			diagnosticcontract.CodeConfigPartialFailure,
			diagnosticcontract.CategoryConfig,
			"Model catalog sync failed",
			err.Error(),
			diagnosticcontract.SeverityError,
			diagnosticcontract.FreshnessLive,
		),
	}}
}

func assertProviderModelHidden(
	t *testing.T,
	models aghconfig.ProviderModelsConfig,
	modelID string,
) {
	t.Helper()
	for _, model := range models.Curated {
		if model.ID == modelID {
			if model.Hidden == nil || !*model.Hidden {
				t.Fatalf("provider model %q hidden = false, want true", modelID)
			}
			return
		}
	}
	t.Fatalf("provider model %q is missing from curated config", modelID)
}

func providerModelCurationTestService(
	t *testing.T,
) (Service, aghconfig.HomePaths, *settingsModelCatalogStub, *providerModelCurationRuntimeApplier) {
	t.Helper()

	ctx := context.Background()
	homePaths := testHomePaths(t)
	writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
	db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	defaultEffort := modelcatalog.ReasoningEffortMedium
	contextWindow := int64(1_050_000)
	maxOutputTokens := int64(128_000)
	supportsTools := true
	costInput := 5.0
	costOutput := 30.0
	costCacheRead := 0.5
	costCacheWrite := 6.0
	costReasoning := 30.0
	releaseDate := "2026-06-26"
	catalog := &settingsModelCatalogStub{models: map[string][]modelcatalog.Model{
		"codex": {
			{
				ProviderID:  "codex",
				ModelID:     "gpt-5.6-sol",
				DisplayName: "GPT-5.6 Sol",
				ReasoningEfforts: []modelcatalog.ReasoningEffort{
					modelcatalog.ReasoningEffortNone,
					modelcatalog.ReasoningEffortLow,
					modelcatalog.ReasoningEffortMedium,
					modelcatalog.ReasoningEffortHigh,
					modelcatalog.ReasoningEffortXHigh,
					modelcatalog.ReasoningEffortMax,
				},
				DefaultReasoningEffort:   &defaultEffort,
				ContextWindow:            &contextWindow,
				MaxOutputTokens:          &maxOutputTokens,
				SupportsTools:            &supportsTools,
				CostInputPerMillion:      &costInput,
				CostOutputPerMillion:     &costOutput,
				CostCacheReadPerMillion:  &costCacheRead,
				CostCacheWritePerMillion: &costCacheWrite,
				CostReasoningPerMillion:  &costReasoning,
				Featured:                 true,
				Curated:                  true,
				ReleaseDate:              &releaseDate,
			},
		},
	}}
	applier := &providerModelCurationRuntimeApplier{catalog: catalog}
	service := testService(t, homePaths, Dependencies{
		ModelCatalog:   catalog,
		RuntimeApplier: applier,
		ApplyRecords:   NewConfigApplyRecordRepository(db.DB(), nil),
	})
	return service, homePaths, catalog, applier
}

func (f *fakeConfigRuntimeApplier) ApplyActiveConfig(
	_ context.Context,
	cfg *aghconfig.Config,
) []ApplyFailure {
	f.calls++
	if cfg != nil {
		f.snapshots = append(f.snapshots, cloneActiveConfig(cfg))
	}
	return append([]ApplyFailure(nil), f.failures...)
}
