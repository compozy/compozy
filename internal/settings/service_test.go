package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	automationmodel "github.com/compozy/agh/internal/automation/model"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/marketplace"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/resources"
	skillspkg "github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/vault"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestGetSectionBuildsSupportedSections(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := testHomePaths(t)
	writeFile(t, homePaths.ConfigFile, baseSettingsConfig())

	service := testService(t, homePaths, Dependencies{
		GeneralRuntime: fakeGeneralRuntimeProvider{
			status: DaemonRuntimeStatus{
				Available:      true,
				Status:         "running",
				PID:            1234,
				UptimeSeconds:  99,
				ActiveSessions: 4,
				ActiveAgents:   3,
				TotalSessions:  6,
				Version:        "1.2.3",
			},
		},
		MemoryRuntime: fakeMemoryRuntimeProvider{
			status: MemoryHealthStatus{
				Available:          true,
				FileCount:          5,
				DreamEnabled:       true,
				LastConsolidatedAt: new(time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)),
			},
		},
		SkillsRuntime: newFakeSkillsRuntime(
			testSkill("alpha", false),
			testSkill("beta", false),
			testSkill("gamma", true),
		),
		AutomationRuntime: fakeAutomationRuntimeProvider{
			status: AutomationRuntimeStatus{
				Available:        true,
				Running:          true,
				SchedulerRunning: true,
				JobTotal:         4,
				JobEnabled:       3,
				TriggerTotal:     2,
				TriggerEnabled:   1,
				NextFire:         new(time.Date(2026, 4, 17, 13, 0, 0, 0, time.UTC)),
			},
		},
		NetworkRuntime: fakeNetworkRuntimeProvider{
			status: NetworkRuntimeStatus{
				Available: true,
				Enabled:   true,
				Status:    "ready",
			},
		},
		ObservabilityRuntime: fakeObservabilityRuntimeProvider{
			status: ObservabilityRuntimeStatus{
				Available:          true,
				Status:             "ok",
				GlobalDBSizeBytes:  2048,
				SessionDBSizeBytes: 4096,
			},
		},
		Extensions: fakeExtensionStatusProvider{
			items: []InstalledExtension{{
				Name:    "linear",
				Version: "1.0.0",
				Enabled: true,
				State:   "active",
				Health:  "healthy",
			}},
		},
		TransportParity: fakeTransportParityProvider{
			status: TransportParityStatus{
				Known:          true,
				SettingsHTTP:   true,
				SettingsUDS:    true,
				ExtensionsHTTP: true,
				ExtensionsUDS:  true,
			},
		},
		RestartActionAvailable:     true,
		ConsolidateActionAvailable: true,
		LogTailAvailable:           true,
	})

	tests := []struct {
		name   SectionName
		label  string
		assert func(t *testing.T, envelope SectionEnvelope)
	}{
		{
			name: SectionGeneral,
			assert: func(t *testing.T, envelope SectionEnvelope) {
				t.Helper()
				if envelope.General == nil {
					t.Fatal("General section = nil")
				}
				if got, want := envelope.General.Settings.Defaults.Agent, "writer"; got != want {
					t.Fatalf("General defaults agent = %q, want %q", got, want)
				}
				if got, want := envelope.General.Runtime.PID, 1234; got != want {
					t.Fatalf("General runtime PID = %d, want %d", got, want)
				}
				if !envelope.General.Actions.Restart.Available {
					t.Fatal("General restart action unavailable")
				}
			},
		},
		{
			name: SectionMemory,
			assert: func(t *testing.T, envelope SectionEnvelope) {
				t.Helper()
				if envelope.Memory == nil {
					t.Fatal("Memory section = nil")
				}
				if got, want := envelope.Memory.Config.Dream.MinHours, 12.0; got != want {
					t.Fatalf("Memory dream minimum hours = %v, want %v", got, want)
				}
				if got, want := envelope.Memory.Health.FileCount, 5; got != want {
					t.Fatalf("Memory file count = %d, want %d", got, want)
				}
			},
		},
		{
			name:  SectionRoles,
			label: "Should build an ownership-safe roles section",
			assert: func(t *testing.T, envelope SectionEnvelope) {
				t.Helper()
				if envelope.Roles == nil || !envelope.Roles.Config.Dream.Enabled {
					t.Fatalf("Roles section = %#v, want enabled Dream role", envelope.Roles)
				}
				envelope.Roles.Config.AutoTitle.FallbackChain = append(
					envelope.Roles.Config.AutoTitle.FallbackChain,
					aghconfig.RoleFallback{Provider: "mutated", Model: "mutated"},
				)
				reloaded, err := service.GetSection(ctx, SectionRequest{Section: SectionRoles})
				if err != nil {
					t.Fatalf("GetSection(roles after mutation) error = %v", err)
				}
				if slices.ContainsFunc(reloaded.Roles.Config.AutoTitle.FallbackChain, func(
					fallback aghconfig.RoleFallback,
				) bool {
					return fallback.Provider == "mutated"
				}) {
					t.Fatal("Roles section retained a caller-owned fallback mutation")
				}
			},
		},
		{
			name: SectionSkills,
			assert: func(t *testing.T, envelope SectionEnvelope) {
				t.Helper()
				if envelope.Skills == nil {
					t.Fatal("Skills section = nil")
				}
				if got, want := envelope.Skills.Config.Marketplace.Registry, "clawhub"; got != want {
					t.Fatalf("Skills marketplace registry = %q, want %q", got, want)
				}
				if got, want := envelope.Skills.DiscoveredCount, 3; got != want {
					t.Fatalf("Skills discovered count = %d, want %d", got, want)
				}
				if got, want := envelope.Skills.DisabledCount, 2; got != want {
					t.Fatalf("Skills disabled count = %d, want %d", got, want)
				}
				if got, want := envelope.Skills.Links, []OperationalLink{{
					Label: "skills",
					Path:  "/marketplace/skills?tab=installed",
				}}; !reflect.DeepEqual(got, want) {
					t.Fatalf("Skills links = %#v, want %#v", got, want)
				}
			},
		},
		{
			name: SectionAutomation,
			assert: func(t *testing.T, envelope SectionEnvelope) {
				t.Helper()
				if envelope.Automation == nil {
					t.Fatal("Automation section = nil")
				}
				if got, want := envelope.Automation.Config.Timezone, "UTC"; got != want {
					t.Fatalf("Automation timezone = %q, want %q", got, want)
				}
				if got, want := envelope.Automation.Runtime.JobTotal, 4; got != want {
					t.Fatalf("Automation jobs = %d, want %d", got, want)
				}
			},
		},
		{
			name: SectionNetwork,
			assert: func(t *testing.T, envelope SectionEnvelope) {
				t.Helper()
				if envelope.Network == nil {
					t.Fatal("Network section = nil")
				}
				if got, want := envelope.Network.Config.Live.Defaults.MaxWakes, 12; got != want {
					t.Fatalf("Network Live default max wakes = %d, want %d", got, want)
				}
				if got, want := envelope.Network.Runtime.Status, "ready"; got != want {
					t.Fatalf("Network runtime status = %q, want %q", got, want)
				}
			},
		},
		{
			name:  SectionWindowManager,
			label: "Should build the window-manager section",
			assert: func(t *testing.T, envelope SectionEnvelope) {
				t.Helper()
				if envelope.WindowManager == nil {
					t.Fatal("WindowManager section = nil")
				}
				if got, want := envelope.WindowManager.Config.HistoryLimit, 50; got != want {
					t.Fatalf("WindowManager history limit = %d, want %d", got, want)
				}
				if got, want := envelope.WindowManager.Config.Bindings.TopCenter, "zoom"; got != want {
					t.Fatalf("WindowManager top-center binding = %q, want %q", got, want)
				}
			},
		},
		{
			name: SectionObservability,
			assert: func(t *testing.T, envelope SectionEnvelope) {
				t.Helper()
				if envelope.Observability == nil {
					t.Fatal("Observability section = nil")
				}
				if got, want := envelope.Observability.Config.Transcripts.SegmentBytes, 512; got != want {
					t.Fatalf("Observability transcripts segment bytes = %d, want %d", got, want)
				}
				if got, want := envelope.Observability.Runtime.GlobalDBSizeBytes, int64(2048); got != want {
					t.Fatalf("Observability global db bytes = %d, want %d", got, want)
				}
				if !envelope.Observability.LogTailSupport.Available {
					t.Fatal("Log tail capability unavailable")
				}
			},
		},
		{
			name: SectionHooksExtensions,
			assert: func(t *testing.T, envelope SectionEnvelope) {
				t.Helper()
				if envelope.HooksExtensions == nil {
					t.Fatal("HooksExtensions section = nil")
				}
				if got, want := len(envelope.HooksExtensions.Hooks), 1; got != want {
					t.Fatalf("Hooks count = %d, want %d", got, want)
				}
				if got, want := len(envelope.HooksExtensions.Installed), 1; got != want {
					t.Fatalf("Installed extensions count = %d, want %d", got, want)
				}
				if !envelope.HooksExtensions.TransportParity.ExtensionsHTTP {
					t.Fatal("Extensions HTTP parity = false, want true")
				}
			},
		},
	}

	for _, tt := range tests {
		name := string(tt.name)
		if tt.label != "" {
			name = tt.label
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			envelope, err := service.GetSection(ctx, SectionRequest{Section: tt.name})
			if err != nil {
				t.Fatalf("GetSection(%q) error = %v", tt.name, err)
			}
			tt.assert(t, envelope)
		})
	}
}

func TestInvalidScopeCombinationsReturnDescriptiveError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := testHomePaths(t)
	service := testService(t, homePaths, Dependencies{})

	t.Run("Should section workspace unsupported", func(t *testing.T) {
		t.Parallel()
		_, err := service.GetSection(ctx, SectionRequest{
			Section:     SectionGeneral,
			Scope:       ScopeWorkspace,
			WorkspaceID: "ws-1",
		})
		if err == nil || !strings.Contains(err.Error(), "does not support workspace scope") {
			t.Fatalf("GetSection(workspace) error = %v, want unsupported workspace scope", err)
		}
	})

	t.Run("Should providers workspace unsupported", func(t *testing.T) {
		t.Parallel()
		_, err := service.ListCollection(ctx, CollectionRequest{
			Collection:  CollectionProviders,
			Scope:       ScopeWorkspace,
			WorkspaceID: "ws-1",
		})
		if err == nil || !strings.Contains(err.Error(), "does not support workspace scope") {
			t.Fatalf("ListCollection(providers workspace) error = %v, want unsupported workspace scope", err)
		}
	})

	t.Run("Should workspace mcp requires workspace id", func(t *testing.T) {
		t.Parallel()
		_, err := service.ListCollection(ctx, CollectionRequest{
			Collection: CollectionMCPServers,
			Scope:      ScopeWorkspace,
		})
		if err == nil || !strings.Contains(err.Error(), "requires a workspace_id") {
			t.Fatalf("ListCollection(mcp workspace) error = %v, want workspace_id error", err)
		}
	})
}

func TestListMCPServersIncludesPrecedenceMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := testHomePaths(t)
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")

	writeFile(t, homePaths.ConfigFile, `
[[mcp_servers]]
name = "alpha"
command = "global-config"
`)
	writeFile(t, filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName), `{
  "mcpServers": {
    "alpha": {
      "command": "global-sidecar",
      "catalog_entry": "alpha-catalog",
      "catalog_version": "1.2.3"
    },
    "beta": { "command": "beta-sidecar" }
  }
}`)
	writeFile(t, filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.ConfigName), `
[[mcp_servers]]
name = "alpha"
command = "workspace-config"
`)
	writeFile(t, filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.MCPJSONName), `{
  "mcpServers": {
    "alpha": { "command": "workspace-sidecar" }
  }
}`)

	service := testService(t, homePaths, Dependencies{
		WorkspaceResolver: fakeWorkspaceResolver{
			resolved: map[string]workspacepkg.ResolvedWorkspace{
				"ws-1": {
					Workspace: workspacepkg.Workspace{ID: "ws-1", RootDir: workspaceRoot},
				},
			},
		},
	})

	globalEnvelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionMCPServers})
	if err != nil {
		t.Fatalf("ListCollection(global mcp) error = %v", err)
	}
	globalAlpha := findMCPItem(t, globalEnvelope.MCPServers, "alpha")
	if got, want := globalAlpha.SourceMetadata.EffectiveSource.Kind, SourceKindGlobalMCPSidecar; got != want {
		t.Fatalf("global alpha effective source = %q, want %q", got, want)
	}
	if got, want := globalAlpha.CatalogEntry, "alpha-catalog"; got != want {
		t.Fatalf("global alpha catalog entry = %q, want %q", got, want)
	}
	if got, want := globalAlpha.CatalogVersion, "1.2.3"; got != want {
		t.Fatalf("global alpha catalog version = %q, want %q", got, want)
	}
	if got, want := len(globalAlpha.SourceMetadata.ShadowedSources), 1; got != want {
		t.Fatalf("global alpha shadowed sources = %d, want %d", got, want)
	}
	if got, want := globalAlpha.SourceMetadata.ShadowedSources[0].Kind, SourceKindGlobalConfig; got != want {
		t.Fatalf("global alpha shadowed source = %q, want %q", got, want)
	}
	if got, want := globalAlpha.SourceMetadata.AvailableTargets, []WriteTargetKind{
		WriteTargetGlobalConfig,
		WriteTargetGlobalMCPSidecar,
	}; !equalWriteTargets(got, want) {
		t.Fatalf("global alpha available targets = %#v, want %#v", got, want)
	}

	workspaceEnvelope, err := service.ListCollection(ctx, CollectionRequest{
		Collection:  CollectionMCPServers,
		Scope:       ScopeWorkspace,
		WorkspaceID: "ws-1",
	})
	if err != nil {
		t.Fatalf("ListCollection(workspace mcp) error = %v", err)
	}
	workspaceAlpha := findMCPItem(t, workspaceEnvelope.MCPServers, "alpha")
	if got, want := workspaceAlpha.SourceMetadata.EffectiveSource.Kind, SourceKindWorkspaceMCPSidecar; got != want {
		t.Fatalf("workspace alpha effective source = %q, want %q", got, want)
	}
	if got, want := len(workspaceAlpha.SourceMetadata.ShadowedSources), 3; got != want {
		t.Fatalf("workspace alpha shadowed sources = %d, want %d", got, want)
	}
	if got, want := workspaceAlpha.SourceMetadata.AvailableTargets, []WriteTargetKind{
		WriteTargetWorkspaceConfig,
		WriteTargetWorkspaceMCPSidecar,
	}; !equalWriteTargets(got, want) {
		t.Fatalf("workspace alpha available targets = %#v, want %#v", got, want)
	}
}

func TestMCPTargetAutoSelectsExistingSourceAndDefaultsNewEntriesToSidecar(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := testHomePaths(t)
	writeFile(t, homePaths.ConfigFile, `
[[mcp_servers]]
name = "alpha"
command = "before"
`)

	service := testService(t, homePaths, Dependencies{})

	result, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
		Name:              "alpha",
		Target:            TargetAuto,
		MCPServer: &aghconfig.MCPServer{
			Command: "after",
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(existing alpha) error = %v", err)
	}
	if got, want := result.WriteTarget, WriteTargetGlobalConfig; got != want {
		t.Fatalf("existing alpha write target = %q, want %q", got, want)
	}
	configPayload := readFile(t, homePaths.ConfigFile)
	if !strings.Contains(configPayload, `command = "after"`) {
		t.Fatalf("config payload missing updated alpha command:\n%s", configPayload)
	}

	result, err = service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
		Name:              "beta",
		Target:            TargetAuto,
		MCPServer: &aghconfig.MCPServer{
			Command: "beta-command",
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(new beta) error = %v", err)
	}
	if got, want := result.WriteTarget, WriteTargetGlobalMCPSidecar; got != want {
		t.Fatalf("new beta write target = %q, want %q", got, want)
	}
	sidecarPayload := readFile(t, filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName))
	if !strings.Contains(sidecarPayload, `"beta"`) || !strings.Contains(sidecarPayload, `"beta-command"`) {
		t.Fatalf("sidecar payload missing beta server:\n%s", sidecarPayload)
	}
}

func TestUpdateSectionGeneralReturnsRestartRequired(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := testHomePaths(t)
	writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
	service := testService(t, homePaths, Dependencies{})

	result, err := service.UpdateSection(ctx, SectionUpdateRequest{
		SectionRequest: SectionRequest{Section: SectionGeneral},
		General: &GeneralSettings{
			Defaults: aghconfig.DefaultsConfig{
				Agent:    "editor",
				Provider: "codex",
				Sandbox:  "dev",
			},
			Limits: aghconfig.LimitsConfig{
				MaxConcurrentAgents: 11,
			},
			Permissions:    aghconfig.PermissionsConfig{Mode: aghconfig.PermissionModeApproveReads},
			SessionTimeout: 45 * time.Minute,
			HTTP:           aghconfig.HTTPConfig{Host: "127.0.0.1", Port: 9001},
			Daemon: aghconfig.DaemonConfig{
				Socket:               "/tmp/agh.sock",
				MemoryReportInterval: aghconfig.DefaultDaemonMemoryReportInterval,
			},
			Redact: aghconfig.RedactConfig{Enabled: false},
		},
	})
	if err != nil {
		t.Fatalf("UpdateSection(general) error = %v", err)
	}
	if got, want := result.Behavior, MutationBehaviorRestartRequired; got != want {
		t.Fatalf("general behavior = %q, want %q", got, want)
	}
	if !result.RestartRequired {
		t.Fatal("general restart_required = false, want true")
	}
	if got, want := result.WriteTarget, WriteTargetGlobalConfig; got != want {
		t.Fatalf("general write target = %q, want %q", got, want)
	}
	contents := readFile(t, homePaths.ConfigFile)
	if !strings.Contains(contents, "[redact]") || !strings.Contains(contents, "enabled = false") {
		t.Fatalf("config contents missing disabled redaction gate:\n%s", contents)
	}
}

func TestUpdateSectionWindowManager(t *testing.T) {
	t.Parallel()

	t.Run("Should round-trip the complete validated global config", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{})
		desired := testWindowManagerConfig()

		result, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionWindowManager},
			WindowManager:  &desired,
		})
		if err != nil {
			t.Fatalf("UpdateSection(window-manager) error = %v", err)
		}
		if got, want := result.Behavior, MutationBehaviorAppliedNow; got != want {
			t.Fatalf("window-manager behavior = %q, want %q", got, want)
		}
		if !result.Applied || result.RestartRequired {
			t.Fatalf("window-manager result = %#v, want live applied mutation", result)
		}
		if got, want := result.WriteTarget, WriteTargetGlobalConfig; got != want {
			t.Fatalf("window-manager write target = %q, want %q", got, want)
		}

		loaded, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(updated window-manager) error = %v", err)
		}
		if !reflect.DeepEqual(loaded.WindowManager, desired) {
			t.Fatalf("loaded WindowManager = %#v, want %#v", loaded.WindowManager, desired)
		}
	})

	t.Run("Should leave config unchanged when validation fails", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		before := readFile(t, homePaths.ConfigFile)
		service := testService(t, homePaths, Dependencies{})
		invalid := testWindowManagerConfig()
		invalid.HistoryLimit = 0

		_, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionWindowManager},
			WindowManager:  &invalid,
		})
		if err == nil {
			t.Fatal("UpdateSection(invalid window-manager) error = nil, want validation error")
		}
		var validationError aghconfig.ValidationError
		if !errors.As(err, &validationError) {
			t.Fatalf("UpdateSection(invalid window-manager) error = %T %v, want config.ValidationError", err, err)
		}
		if got, want := validationError.Path, "window_manager.history_limit"; got != want {
			t.Fatalf("UpdateSection(invalid window-manager) validation path = %q, want %q", got, want)
		}
		if after := readFile(t, homePaths.ConfigFile); after != before {
			t.Fatalf("config changed after validation failure\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})
}

func TestUpdateSectionGeneralMemoryReportIntervalRequiresRestart(t *testing.T) {
	t.Run("Should require a restart when disabling memory reports", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{})
		envelope, err := service.GetSection(ctx, SectionRequest{Section: SectionGeneral})
		if err != nil {
			t.Fatalf("GetSection(general) error = %v", err)
		}
		desired := envelope.General.Settings
		desired.Daemon.MemoryReportInterval = 0

		result, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionGeneral},
			General:        &desired,
		})
		if err != nil {
			t.Fatalf("UpdateSection(memory report interval) error = %v", err)
		}
		if result.Behavior != MutationBehaviorRestartRequired || !result.RestartRequired ||
			result.Lifecycle != lifecycle.RestartRequired {
			t.Fatalf("UpdateSection(memory report interval) result = %#v, want daemon restart", result)
		}
		contents := readFile(t, homePaths.ConfigFile)
		if !strings.Contains(contents, `memory_report_interval = "0s"`) {
			t.Fatalf("config contents missing disabled memory interval:\n%s", contents)
		}
	})
}

func TestUpdateSectionSkillsAppliesDisabledSkillsNow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := testHomePaths(t)
	writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
	skillsRuntime := newFakeSkillsRuntime(
		testSkill("alpha", false),
		testSkill("beta", false),
		testSkill("gamma", true),
	)
	service := testService(t, homePaths, Dependencies{SkillsRuntime: skillsRuntime})

	result, err := service.UpdateSection(ctx, SectionUpdateRequest{
		SectionRequest: SectionRequest{Section: SectionSkills},
		Skills: &aghconfig.SkillsConfig{
			Enabled:                 true,
			DisabledSkills:          []string{"beta"},
			PollInterval:            30 * time.Minute,
			AllowedMarketplaceMCP:   []string{"ctx"},
			AllowedMarketplaceHooks: []string{"market"},
			Marketplace: aghconfig.MarketplaceConfig{
				Registry: "clawhub",
				BaseURL:  "https://skills.example",
			},
		},
	})
	if err != nil {
		t.Fatalf("UpdateSection(skills disabled) error = %v", err)
	}
	if got, want := result.Behavior, MutationBehaviorAppliedNow; got != want {
		t.Fatalf("skills behavior = %q, want %q", got, want)
	}
	if !result.Applied {
		t.Fatal("skills applied = false, want true")
	}
	if result.RestartRequired {
		t.Fatal("skills restart_required = true, want false")
	}
	if got, want := skillsRuntime.enabled["alpha"], true; got != want {
		t.Fatalf("alpha enabled = %v, want %v", got, want)
	}
	if got, want := skillsRuntime.enabled["beta"], false; got != want {
		t.Fatalf("beta enabled = %v, want %v", got, want)
	}
}

func TestUpdateSectionSkillsWithoutRuntimeDoesNotPersistChanges(t *testing.T) {
	t.Parallel()

	t.Run("Should leave config unchanged when disabled skills need a runtime apply", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		before := readFile(t, homePaths.ConfigFile)
		service := testService(t, homePaths, Dependencies{})

		_, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionSkills},
			Skills: &aghconfig.SkillsConfig{
				Enabled:                 true,
				DisabledSkills:          []string{"beta"},
				PollInterval:            30 * time.Minute,
				AllowedMarketplaceMCP:   []string{"ctx"},
				AllowedMarketplaceHooks: []string{"market"},
				Marketplace: aghconfig.MarketplaceConfig{
					Registry: "clawhub",
					BaseURL:  "https://skills.example",
				},
			},
		})
		if err == nil {
			t.Fatal("UpdateSection(skills disabled without runtime) error = nil, want runtime failure")
		}
		if got, want := err.Error(), "settings: skills runtime is required to apply skills.disabled_skills"; got != want {
			t.Fatalf("UpdateSection(skills disabled without runtime) error = %q, want %q", got, want)
		}
		if after := readFile(t, homePaths.ConfigFile); after != before {
			t.Fatalf("config changed on runtime failure\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})
}

func TestClassifyMutationDominatesMixedLifecycleWithRestartRequired(t *testing.T) {
	t.Parallel()

	classification, err := ClassifyMutation(MutationDescriptor{
		Section:       SectionSkills,
		ChangedFields: []string{"skills.disabled_skills", "skills.poll_interval"},
	})
	if err != nil {
		t.Fatalf("ClassifyMutation(mixed skills lifecycle) error = %v", err)
	}
	if got, want := classification.Lifecycle, lifecycle.RestartRequired; got != want {
		t.Fatalf("Lifecycle = %q, want %q", got, want)
	}
	if !classification.RestartRequired {
		t.Fatal("RestartRequired = false, want true")
	}
}

func TestClassifyMutationReturnsMatrixBehavior(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		descriptor MutationDescriptor
		want       MutationBehavior
	}{
		{
			name: "applied now",
			descriptor: MutationDescriptor{
				Section:       SectionSkills,
				ChangedFields: []string{"skills.disabled_skills"},
			},
			want: MutationBehaviorAppliedNow,
		},
		{
			name: "restart required",
			descriptor: MutationDescriptor{
				Section:       SectionGeneral,
				ChangedFields: []string{"defaults.agent"},
			},
			want: MutationBehaviorRestartRequired,
		},
		{
			name: "action trigger",
			descriptor: MutationDescriptor{
				Section: SectionMemory,
				Action:  "consolidate",
			},
			want: MutationBehaviorActionTrigger,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			classification, err := ClassifyMutation(tt.descriptor)
			if err != nil {
				t.Fatalf("ClassifyMutation() error = %v", err)
			}
			if got, want := classification.Behavior, tt.want; got != want {
				t.Fatalf("behavior = %q, want %q", got, want)
			}
		})
	}
}

func TestClassifyMutationSupportsActionTriggers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		descriptor MutationDescriptor
	}{
		{
			name: "general restart",
			descriptor: MutationDescriptor{
				Section: SectionGeneral,
				Action:  "restart",
			},
		},
		{
			name: "hooks extensions install",
			descriptor: MutationDescriptor{
				Section: SectionHooksExtensions,
				Action:  "extension-install",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			classification, err := ClassifyMutation(tt.descriptor)
			if err != nil {
				t.Fatalf("ClassifyMutation() error = %v", err)
			}
			if got, want := classification.Behavior, MutationBehaviorActionTrigger; got != want {
				t.Fatalf("behavior = %q, want %q", got, want)
			}
			if !classification.Applied {
				t.Fatal("classification.Applied = false, want true")
			}
		})
	}
}

func TestClassifyMutationSupportsCollectionFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		descriptor MutationDescriptor
		want       MutationBehavior
	}{
		{
			descriptor: MutationDescriptor{
				Section:       SectionName(CollectionProviders),
				ChangedFields: []string{"providers.custom.command"},
			},
			want: MutationBehaviorRestartRequired,
		},
		{
			descriptor: MutationDescriptor{
				Section:       SectionName(CollectionMCPServers),
				ChangedFields: []string{"mcp-servers.alpha.command"},
			},
			want: MutationBehaviorRestartRequired,
		},
		{
			descriptor: MutationDescriptor{
				Section:       SectionName(CollectionSandboxes),
				ChangedFields: []string{"sandboxes.dev.backend"},
			},
			want: MutationBehaviorAppliedNow,
		},
		{
			descriptor: MutationDescriptor{
				Section:       SectionName(CollectionHooks),
				ChangedFields: []string{"hooks.audit.command"},
			},
			want: MutationBehaviorRestartRequired,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.descriptor.Section), func(t *testing.T) {
			t.Parallel()

			classification, err := ClassifyMutation(tt.descriptor)
			if err != nil {
				t.Fatalf("ClassifyMutation() error = %v", err)
			}
			if got, want := classification.Behavior, tt.want; got != want {
				t.Fatalf("behavior = %q, want %q", got, want)
			}
		})
	}
}

func TestListCollectionBuildsProvidersSandboxesAndHooks(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := testHomePaths(t)
	writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+`

[providers.codex]
[providers.codex.models]
default = "gpt-5"
[[providers.codex.models.curated]]
id = "gpt-5"
display_name = "GPT-5"
[[providers.codex.models.curated]]
id = "gpt-5-mini"
display_name = "GPT-5 Mini"

	[providers.custom]
	command = "custom-acp --stdio"
	[providers.custom.models]
	default = "custom-model"
	[[providers.custom.credential_slots]]
	name = "api_key"
	target_env = "CUSTOM_API_KEY"
	secret_ref = "env:CUSTOM_API_KEY"
	kind = "api_key"
	required = true

	[sandboxes.staging]
backend = "local"

[[hooks.declarations]]
name = "ship"
event = "session.post_create"
mode = "async"
command = "/bin/ship"
`)

	service := testService(t, homePaths, Dependencies{
		ModelCatalog: &settingsModelCatalogStub{models: map[string][]modelcatalog.Model{
			"codex": {
				{ProviderID: "codex", ModelID: "gpt-5", DisplayName: "GPT-5", Curated: true},
				{ProviderID: "codex", ModelID: "gpt-5-mini", DisplayName: "GPT-5 Mini", Curated: true},
			},
		}},
		CommandLookPath: func(command string) (string, error) {
			if strings.HasPrefix(command, "custom-acp") {
				return "", os.ErrNotExist
			}
			return "/bin/" + command, nil
		},
		LookupEnv: func(key string) (string, bool) {
			if key == "OPENAI_API_KEY" {
				return "token", true
			}
			return "", false
		},
		WorkspaceResolver: fakeWorkspaceResolver{
			listed: []workspacepkg.Workspace{
				{ID: "ws-dev", SandboxRef: "dev"},
				{ID: "ws-stage-a", SandboxRef: "staging"},
				{ID: "ws-stage-b", SandboxRef: "staging"},
			},
		},
	})

	providers, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionProviders})
	if err != nil {
		t.Fatalf("ListCollection(providers) error = %v", err)
	}
	codex := mustFindProviderItem(t, providers.Providers, "codex")
	if got, want := codex.Settings.Models.Default, "gpt-5"; got != want {
		t.Fatalf("codex default model = %q, want %q", got, want)
	}
	if got, want := len(codex.Settings.Models.Curated), 2; got != want {
		t.Fatalf("codex curated model count = %d, want %d", got, want)
	}
	if got, want := codex.Settings.Models.Curated[0].ID, "gpt-5"; got != want {
		t.Fatalf("codex curated[0].ID = %q, want %q", got, want)
	}
	if got, want := codex.Settings.Models.Curated[1].ID, "gpt-5-mini"; got != want {
		t.Fatalf("codex curated[1].ID = %q, want %q", got, want)
	}
	if !codex.Default {
		t.Fatal("codex default = false, want true")
	}
	if got, want := codex.SourceMetadata.EffectiveSource.Kind, SourceKindGlobalConfig; got != want {
		t.Fatalf("codex effective source = %q, want %q", got, want)
	}
	if codex.Fallback == nil || codex.Fallback.Source.Kind != SourceKindBuiltinProvider {
		t.Fatalf("codex fallback = %#v, want builtin fallback", codex.Fallback)
	}
	custom := mustFindProviderItem(t, providers.Providers, "custom")
	if got, want := custom.SourceMetadata.EffectiveSource.Kind, SourceKindGlobalConfig; got != want {
		t.Fatalf("custom effective source = %q, want %q", got, want)
	}
	if custom.CommandAvailable {
		t.Fatal("custom command available = true, want false")
	}
	if len(custom.Credentials) != 1 || custom.Credentials[0].Present {
		t.Fatalf("custom credentials = %#v, want one missing credential status", custom.Credentials)
	}
	claude := mustFindProviderItem(t, providers.Providers, "claude")
	if got, want := claude.SourceMetadata.EffectiveSource.Kind, SourceKindBuiltinProvider; got != want {
		t.Fatalf("claude effective source = %q, want %q", got, want)
	}

	sandboxes, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionSandboxes})
	if err != nil {
		t.Fatalf("ListCollection(sandboxes) error = %v", err)
	}
	dev := findSandboxItem(t, sandboxes.Sandboxes, "dev")
	if got, want := dev.WorkspaceUsageCount, 1; got != want {
		t.Fatalf("dev workspace usage = %d, want %d", got, want)
	}
	staging := findSandboxItem(t, sandboxes.Sandboxes, "staging")
	if got, want := staging.WorkspaceUsageCount, 2; got != want {
		t.Fatalf("staging workspace usage = %d, want %d", got, want)
	}

	hooks, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionHooks})
	if err != nil {
		t.Fatalf("ListCollection(hooks) error = %v", err)
	}
	if got, want := hooks.Hooks[0].Name, "audit"; got != want {
		t.Fatalf("hooks[0].Name = %q, want %q", got, want)
	}
	if got, want := hooks.Hooks[1].Name, "ship"; got != want {
		t.Fatalf("hooks[1].Name = %q, want %q", got, want)
	}
	if got, want := hooks.Hooks[1].SourceMetadata.EffectiveSource.Kind, SourceKindGlobalConfig; got != want {
		t.Fatalf("hook effective source = %q, want %q", got, want)
	}
}

func TestCollectionMutationsProviderSandboxAndHook(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := testHomePaths(t)
	writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
	service := testService(t, homePaths, Dependencies{
		ModelCatalog: &settingsModelCatalogStub{models: map[string][]modelcatalog.Model{
			"codex": {
				{ProviderID: "codex", ModelID: "gpt-5.6-sol", Curated: true},
				{ProviderID: "codex", ModelID: "gpt-5.6-terra", Curated: true},
				{ProviderID: "codex", ModelID: "gpt-5.6-luna", Curated: true},
			},
			"custom": {
				{ProviderID: "custom", ModelID: "custom-model", Curated: true},
				{ProviderID: "custom", ModelID: "custom-fast", Curated: true},
			},
		}},
	})

	providerResult, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionProviders},
		Name:              "custom",
		Provider: &ProviderSettings{
			Command:   "custom-acp --stdio",
			ModelsSet: true,
			Models: aghconfig.ProviderModelsConfig{
				Default: "custom-model",
				Curated: []aghconfig.ProviderModelConfig{
					{
						ID:                     "custom-model",
						DisplayName:            "Custom Model",
						SupportsReasoning:      new(true),
						ReasoningEfforts:       []string{"low", "high"},
						DefaultReasoningEffort: "high",
						SupportsTools:          new(true),
					},
					{ID: "custom-fast", DisplayName: "Custom Fast"},
				},
			},
			CredentialSlots: []aghconfig.ProviderCredentialSlot{
				{
					Name:      "api_key",
					TargetEnv: "CUSTOM_API_KEY",
					SecretRef: "env:CUSTOM_API_KEY",
					Kind:      "api_key",
					Required:  true,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(provider) error = %v", err)
	}
	if got, want := providerResult.WriteTarget, WriteTargetGlobalConfig; got != want {
		t.Fatalf("provider write target = %q, want %q", got, want)
	}
	if got, want := providerResult.Behavior, MutationBehaviorRestartRequired; got != want {
		t.Fatalf("provider behavior = %q, want %q", got, want)
	}
	configPayload := readFile(t, homePaths.ConfigFile)
	if !strings.Contains(configPayload, "[providers.custom]") ||
		!strings.Contains(configPayload, "[providers.custom.models]") ||
		!strings.Contains(configPayload, `default = "custom-model"`) ||
		!strings.Contains(configPayload, `[[providers.custom.models.curated]]`) ||
		!strings.Contains(configPayload, `id = "custom-model"`) ||
		!strings.Contains(configPayload, `reasoning_efforts = ["low", "high"]`) {
		t.Fatalf("config payload missing provider overlay:\n%s", configPayload)
	}
	emptyCuratedResult, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionProviders},
		Name:              "codex",
		Provider: &ProviderSettings{
			ModelsSet: true,
			Models: aghconfig.ProviderModelsConfig{
				Curated: []aghconfig.ProviderModelConfig{},
			},
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(explicit empty curated) error = %v", err)
	}
	if got, want := emptyCuratedResult.WriteTarget, WriteTargetGlobalConfig; got != want {
		t.Fatalf("empty curated write target = %q, want %q", got, want)
	}
	loadedConfig, err := aghconfig.LoadForHome(homePaths)
	if err != nil {
		t.Fatalf("LoadForHome(after empty curated) error = %v", err)
	}
	codexOverlay := loadedConfig.Providers["codex"]
	if got, want := len(codexOverlay.Models.Curated), 3; got != want {
		t.Fatalf("codex curation row count after explicit empty membership = %d, want %d", got, want)
	}
	for _, model := range codexOverlay.Models.Curated {
		if model.Hidden == nil || !*model.Hidden {
			t.Fatalf("codex curation row %q hidden = %v, want true", model.ID, model.Hidden)
		}
	}
	emptyEffortsResult, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionProviders},
		Name:              "custom",
		Provider: &ProviderSettings{
			Command:   "custom-acp --stdio",
			ModelsSet: true,
			Models: aghconfig.ProviderModelsConfig{
				Curated: []aghconfig.ProviderModelConfig{
					{
						ID:               "custom-model",
						ReasoningEfforts: []string{},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(explicit empty reasoning efforts) error = %v", err)
	}
	if got, want := emptyEffortsResult.WriteTarget, WriteTargetGlobalConfig; got != want {
		t.Fatalf("empty reasoning efforts write target = %q, want %q", got, want)
	}
	loadedConfig, err = aghconfig.LoadForHome(homePaths)
	if err != nil {
		t.Fatalf("LoadForHome(after empty reasoning efforts) error = %v", err)
	}
	custom := loadedConfig.Providers["custom"]
	customModel := requireConfiguredProviderModel(t, custom.Models.Curated, "custom-model")
	if got, want := len(customModel.ReasoningEfforts), 2; got != want {
		t.Fatalf(
			"custom reasoning effort count after membership-only write = %d, want preserved %d",
			got,
			want,
		)
	}
	customFast := requireConfiguredProviderModel(t, custom.Models.Curated, "custom-fast")
	if customFast.Hidden == nil || !*customFast.Hidden {
		t.Fatalf("custom-fast hidden = %v, want true after removal from membership", customFast.Hidden)
	}
	blankIDResult, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionProviders},
		Name:              "custom",
		Provider: &ProviderSettings{
			Command:   "custom-acp --stdio",
			ModelsSet: true,
			Models: aghconfig.ProviderModelsConfig{
				Curated: []aghconfig.ProviderModelConfig{
					{ID: "   ", DisplayName: "Ignored Blank"},
					{ID: "custom-valid", DisplayName: "Custom Valid"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(blank curated id) error = %v", err)
	}
	if got, want := blankIDResult.WriteTarget, WriteTargetGlobalConfig; got != want {
		t.Fatalf("blank curated id write target = %q, want %q", got, want)
	}
	loadedConfig, err = aghconfig.LoadForHome(homePaths)
	if err != nil {
		t.Fatalf("LoadForHome(after blank curated id) error = %v", err)
	}
	custom = loadedConfig.Providers["custom"]
	visible := make([]string, 0, len(custom.Models.Curated))
	for _, model := range custom.Models.Curated {
		if model.Hidden == nil || !*model.Hidden {
			visible = append(visible, model.ID)
		}
	}
	if got, want := len(visible), 1; got != want {
		t.Fatalf("visible custom curation rows after blank id filtering = %#v, want %d", visible, want)
	}
	if got, want := visible[0], "custom-valid"; got != want {
		t.Fatalf("visible custom model after blank curated id = %q, want %q", got, want)
	}
	clearModelsResult, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionProviders},
		Name:              "custom",
		Provider: &ProviderSettings{
			Command:   "custom-acp --stdio",
			ModelsSet: true,
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(clear provider models) error = %v", err)
	}
	if got, want := clearModelsResult.WriteTarget, WriteTargetGlobalConfig; got != want {
		t.Fatalf("clear provider models write target = %q, want %q", got, want)
	}
	configPayload = readFile(t, homePaths.ConfigFile)
	if strings.Contains(configPayload, "[providers.custom.models]") ||
		strings.Contains(configPayload, `default = "custom-model"`) ||
		strings.Contains(configPayload, `[[providers.custom.models.curated]]`) {
		t.Fatalf("config payload still contains provider model overlay after clear:\n%s", configPayload)
	}
	if _, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionProviders},
		Name:              "custom",
	}); err != nil {
		t.Fatalf("DeleteCollectionItem(provider) error = %v", err)
	}
	configPayload = readFile(t, homePaths.ConfigFile)
	if strings.Contains(configPayload, "[providers.custom]") {
		t.Fatalf("provider overlay still present after delete:\n%s", configPayload)
	}

	sandboxResult, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionSandboxes},
		Name:              "staging",
		Sandbox: &aghconfig.SandboxProfile{
			Backend:     "local",
			SyncMode:    "session-bidirectional",
			Persistence: "transient",
			RuntimeRoot: "/tmp/staging",
			Env: map[string]string{
				"QA_VISIBLE": "yes",
			},
			Network: aghconfig.NetworkProfile{
				AllowOutbound: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(sandbox) error = %v", err)
	}
	if got, want := sandboxResult.WriteTarget, WriteTargetGlobalConfig; got != want {
		t.Fatalf("sandbox write target = %q, want %q", got, want)
	}
	configPayload = readFile(t, homePaths.ConfigFile)
	if !strings.Contains(configPayload, "[sandboxes.staging]") ||
		!strings.Contains(configPayload, `runtime_root = "/tmp/staging"`) ||
		!strings.Contains(configPayload, `[sandboxes.staging.env]`) ||
		!strings.Contains(configPayload, `QA_VISIBLE = "yes"`) ||
		!strings.Contains(configPayload, `[sandboxes.staging.network]`) ||
		!strings.Contains(configPayload, "allow_outbound = true") {
		t.Fatalf("config payload missing sandbox overlay:\n%s", configPayload)
	}
	_, err = service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionSandboxes},
		Name:              "staging",
		Sandbox: &aghconfig.SandboxProfile{
			Backend: "local",
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(replace sandbox) error = %v", err)
	}
	configPayload = readFile(t, homePaths.ConfigFile)
	if strings.Contains(configPayload, `runtime_root = "/tmp/staging"`) ||
		strings.Contains(configPayload, `[sandboxes.staging.env]`) ||
		strings.Contains(configPayload, `QA_VISIBLE = "yes"`) ||
		strings.Contains(configPayload, `[sandboxes.staging.network]`) ||
		strings.Contains(configPayload, "allow_outbound = true") {
		t.Fatalf("config payload still contains stale sandbox fields after replace:\n%s", configPayload)
	}
	if _, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionSandboxes},
		Name:              "staging",
	}); err != nil {
		t.Fatalf("DeleteCollectionItem(sandbox) error = %v", err)
	}
	configPayload = readFile(t, homePaths.ConfigFile)
	if strings.Contains(configPayload, "[sandboxes.staging]") {
		t.Fatalf("sandbox overlay still present after delete:\n%s", configPayload)
	}

	hookResult, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionHooks},
		Name:              "ship",
		Hook: &hookspkg.HookDecl{
			Event:   hookspkg.HookToolPreCall,
			Mode:    hookspkg.HookModeAsync,
			Command: "/bin/ship",
			Args:    []string{"--fast"},
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(hook) error = %v", err)
	}
	if got, want := hookResult.WriteTarget, WriteTargetGlobalConfig; got != want {
		t.Fatalf("hook write target = %q, want %q", got, want)
	}
	configPayload = readFile(t, homePaths.ConfigFile)
	if !strings.Contains(configPayload, `name = "ship"`) || !strings.Contains(configPayload, `args = ["--fast"]`) {
		t.Fatalf("config payload missing hook declaration:\n%s", configPayload)
	}
	if _, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionHooks},
		Name:              "ship",
	}); err != nil {
		t.Fatalf("DeleteCollectionItem(hook) error = %v", err)
	}
	configPayload = readFile(t, homePaths.ConfigFile)
	if strings.Contains(configPayload, `name = "ship"`) {
		t.Fatalf("hook declaration still present after delete:\n%s", configPayload)
	}
}

func TestProviderSettingsUsesMergedCatalogProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should leave a builtin provider untouched after an unchanged projected round trip", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		catalog := &settingsModelCatalogStub{models: map[string][]modelcatalog.Model{
			"claude": {
				{ProviderID: "claude", ModelID: "claude-sonnet-5", Curated: true},
			},
		}}
		service := testService(t, homePaths, Dependencies{ModelCatalog: catalog})
		envelope, err := service.ListCollection(context.Background(), CollectionRequest{
			Collection: CollectionProviders,
		})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		claude := mustFindProviderItem(t, envelope.Providers, "claude")
		if got, want := claude.SourceMetadata.EffectiveSource.Kind, SourceKindBuiltinProvider; got != want {
			t.Fatalf("claude source before round trip = %q, want %q", got, want)
		}
		before := readFile(t, homePaths.ConfigFile)
		settings := claude.Settings
		settings.ModelsSet = true
		result, err := service.PutCollectionItem(context.Background(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "claude",
			Provider:          &settings,
		})
		if err != nil {
			t.Fatalf("PutCollectionItem(unchanged builtin projection) error = %v", err)
		}
		if result.RestartRequired {
			t.Fatalf("unchanged builtin projection restart_required = true, want false")
		}
		if after := readFile(t, homePaths.ConfigFile); after != before {
			t.Fatalf("unchanged builtin projection rewrote config:\n%s", after)
		}
		envelope, err = service.ListCollection(context.Background(), CollectionRequest{
			Collection: CollectionProviders,
		})
		if err != nil {
			t.Fatalf("ListCollection(providers after round trip) error = %v", err)
		}
		claude = mustFindProviderItem(t, envelope.Providers, "claude")
		if got, want := claude.SourceMetadata.EffectiveSource.Kind, SourceKindBuiltinProvider; got != want {
			t.Fatalf("claude source after round trip = %q, want %q", got, want)
		}
	})

	t.Run("Should replace raw curated config rows with the curated catalog view", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+`

[providers.custom]
command = "custom-acp"

[providers.custom.models]
default = "merged-model"

[providers.custom.models.reasoning]
apply = "acp_option"

[[providers.custom.models.curated]]
id = "raw-only"
display_name = "Raw config row"
`)
		releaseDate := "2026-07-09"
		catalog := &settingsModelCatalogStub{models: map[string][]modelcatalog.Model{
			"custom": {
				{
					ProviderID:        "custom",
					ModelID:           "merged-model",
					DisplayName:       "Merged model",
					SupportsTools:     new(true),
					SupportsReasoning: new(true),
					ReasoningEfforts: []modelcatalog.ReasoningEffort{
						modelcatalog.ReasoningEffortHigh,
						modelcatalog.ReasoningEffortMax,
					},
					Curated:         true,
					Featured:        true,
					ReleaseDate:     &releaseDate,
					ReasoningSource: modelcatalog.ReasoningSourceACP,
				},
			},
		}}
		service := testService(t, homePaths, Dependencies{ModelCatalog: catalog})
		envelope, err := service.ListCollection(context.Background(), CollectionRequest{
			Collection: CollectionProviders,
		})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		custom := mustFindProviderItem(t, envelope.Providers, "custom")
		if got, want := len(custom.Settings.Models.Curated), 1; got != want {
			t.Fatalf("merged curated model count = %d, want %d", got, want)
		}
		model := custom.Settings.Models.Curated[0]
		if got, want := model.ID, "merged-model"; got != want {
			t.Fatalf("merged model id = %q, want %q", got, want)
		}
		if model.ID == "raw-only" {
			t.Fatal("settings exposed the raw curated config row")
		}
		if model.Featured == nil || !*model.Featured || model.ReleaseDate != releaseDate {
			t.Fatalf("merged model metadata = %#v, want featured release metadata", model)
		}
		if got, want := custom.Settings.Models.Reasoning.Apply, aghconfig.ReasoningApplyACPOption; got != want {
			t.Fatalf("reasoning apply = %q, want %q", got, want)
		}
		if !catalog.sawCuratedView("custom") {
			t.Fatal("settings did not request the curated model catalog view")
		}

		settings := custom.Settings
		settings.ModelsSet = true
		if _, err := service.PutCollectionItem(context.Background(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "custom",
			Provider:          &settings,
		}); err != nil {
			t.Fatalf("PutCollectionItem(unchanged merged projection) error = %v", err)
		}
		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after unchanged merged projection) error = %v", err)
		}
		rawRows := cfg.Providers["custom"].Models.Curated
		if got, want := len(rawRows), 1; got != want {
			t.Fatalf("raw curated row count after unchanged round trip = %d, want %d", got, want)
		}
		if got, want := rawRows[0].ID, "raw-only"; got != want {
			t.Fatalf("raw curated model after unchanged round trip = %q, want %q", got, want)
		}
		if got, want := rawRows[0].DisplayName, "Raw config row"; got != want {
			t.Fatalf("raw display name after unchanged round trip = %q, want %q", got, want)
		}

		defaultEffort := modelcatalog.ReasoningEffortMax
		if _, err := service.PutCollectionItem(context.Background(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "custom",
			Provider:          &settings,
			ProviderModelCuration: &ProviderModelCurationRequest{
				ModelID:                "merged-model",
				DefaultReasoningEffort: &defaultEffort,
			},
		}); err != nil {
			t.Fatalf("PutCollectionItem(explicit default effort intent) error = %v", err)
		}
		cfg, err = aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after explicit default effort intent) error = %v", err)
		}
		curated := requireConfiguredProviderModel(t, cfg.Providers["custom"].Models.Curated, "merged-model")
		if got, want := curated.DefaultReasoningEffort, string(modelcatalog.ReasoningEffortMax); got != want {
			t.Fatalf("explicit default reasoning effort = %q, want %q", got, want)
		}
		if curated.DisplayName != "" || curated.ReleaseDate != "" || curated.Featured != nil ||
			curated.SupportsReasoning != nil || curated.ReasoningEfforts != nil {
			t.Fatalf("explicit curation materialized merged enrichment: %#v", curated)
		}
		envelope, err = service.ListCollection(context.Background(), CollectionRequest{
			Collection: CollectionProviders,
		})
		if err != nil {
			t.Fatalf("ListCollection(providers after explicit curation) error = %v", err)
		}
		custom = mustFindProviderItem(t, envelope.Providers, "custom")
		model = custom.Settings.Models.Curated[0]
		if got, want := model.DefaultReasoningEffort, string(modelcatalog.ReasoningEffortMax); got != want {
			t.Fatalf("projected desired default reasoning effort = %q, want %q", got, want)
		}
		if got, want := model.DisplayName, "Merged model"; got != want {
			t.Fatalf("projected catalog display name = %q, want %q", got, want)
		}

		updatedReleaseDate := "2026-07-10"
		catalog.mu.Lock()
		updated := catalog.models["custom"][0]
		updated.DisplayName = "Merged model refreshed"
		updated.ReleaseDate = &updatedReleaseDate
		catalog.models["custom"] = []modelcatalog.Model{updated}
		catalog.mu.Unlock()
		envelope, err = service.ListCollection(context.Background(), CollectionRequest{
			Collection: CollectionProviders,
		})
		if err != nil {
			t.Fatalf("ListCollection(providers after catalog refresh) error = %v", err)
		}
		custom = mustFindProviderItem(t, envelope.Providers, "custom")
		model = custom.Settings.Models.Curated[0]
		if got, want := model.DisplayName, "Merged model refreshed"; got != want {
			t.Fatalf("refreshed merged display name = %q, want %q", got, want)
		}
		if got, want := model.ReleaseDate, updatedReleaseDate; got != want {
			t.Fatalf("refreshed merged release date = %q, want %q", got, want)
		}
	})

	t.Run("Should preserve the raw models overlay when a partial provider PUT omits models", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+`

[providers.custom]
command = "custom-acp"

[providers.custom.models]
default = "raw-model"

[providers.custom.models.discovery]
enabled = true
command = "custom-models --json"

[providers.custom.models.reasoning]
apply = "acp_option"

[[providers.custom.models.curated]]
id = "raw-model"
display_name = "Raw model"
featured = true
`)
		beforeConfig, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(before partial provider PUT) error = %v", err)
		}
		beforeModels := cloneProviderModelsConfig(beforeConfig.Providers["custom"].Models)
		service := testService(t, homePaths, Dependencies{})
		if _, err := service.PutCollectionItem(context.Background(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "custom",
			Provider:          &ProviderSettings{Command: "custom-acp-v2"},
		}); err != nil {
			t.Fatalf("PutCollectionItem(partial provider PUT) error = %v", err)
		}

		afterConfig, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after partial provider PUT) error = %v", err)
		}
		afterProvider := afterConfig.Providers["custom"]
		if got, want := afterProvider.Command, "custom-acp-v2"; got != want {
			t.Fatalf("provider command after partial PUT = %q, want %q", got, want)
		}
		if !reflect.DeepEqual(afterProvider.Models, beforeModels) {
			t.Fatalf(
				"provider models after partial PUT = %#v, want preserved %#v",
				afterProvider.Models,
				beforeModels,
			)
		}
	})

	t.Run("Should materialize exact membership when fallback curation receives its first edit", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+`

[providers.custom]
command = "custom-acp"
`)
		catalog := &settingsModelCatalogStub{models: map[string][]modelcatalog.Model{
			"custom": {
				{ProviderID: "custom", ModelID: "alpha", Curated: true},
				{ProviderID: "custom", ModelID: "beta", Curated: true},
				{ProviderID: "custom", ModelID: "gamma", Curated: true},
			},
		}}
		service := testService(t, homePaths, Dependencies{ModelCatalog: catalog})
		envelope, err := service.ListCollection(context.Background(), CollectionRequest{
			Collection: CollectionProviders,
		})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		custom := mustFindProviderItem(t, envelope.Providers, "custom")
		settings := custom.Settings
		settings.ModelsSet = true
		settings.Models.Curated = settings.Models.Curated[1:]
		if _, err := service.PutCollectionItem(context.Background(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "custom",
			Provider:          &settings,
		}); err != nil {
			t.Fatalf("PutCollectionItem(first explicit membership edit) error = %v", err)
		}

		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after first explicit membership edit) error = %v", err)
		}
		rows := cfg.Providers["custom"].Models.Curated
		if got, want := len(rows), 3; got != want {
			t.Fatalf("materialized curation row count = %d, want %d", got, want)
		}
		alpha := requireConfiguredProviderModel(t, rows, "alpha")
		if alpha.Hidden == nil || !*alpha.Hidden {
			t.Fatalf("alpha hidden = %v, want true", alpha.Hidden)
		}
		for _, modelID := range []string{"beta", "gamma"} {
			row := requireConfiguredProviderModel(t, rows, modelID)
			if row.Hidden != nil && *row.Hidden {
				t.Fatalf("%s hidden = %v, want visible explicit membership", modelID, row.Hidden)
			}
		}
	})

	t.Run("Should explicitly clear lower-source exclusion when adding a model to membership", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+`

[providers.custom]
command = "custom-acp"

[[providers.custom.models.curated]]
id = "excluded"
hidden = true
deprecated = true
`)
		catalog := &settingsModelCatalogStub{models: map[string][]modelcatalog.Model{
			"custom": {{ProviderID: "custom", ModelID: "visible", Curated: true}},
		}}
		service := testService(t, homePaths, Dependencies{ModelCatalog: catalog})
		envelope, err := service.ListCollection(context.Background(), CollectionRequest{
			Collection: CollectionProviders,
		})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		custom := mustFindProviderItem(t, envelope.Providers, "custom")
		settings := custom.Settings
		settings.ModelsSet = true
		settings.Models.Curated = append(
			settings.Models.Curated,
			aghconfig.ProviderModelConfig{ID: "excluded"},
		)
		if _, err := service.PutCollectionItem(context.Background(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "custom",
			Provider:          &settings,
		}); err != nil {
			t.Fatalf("PutCollectionItem(add excluded model) error = %v", err)
		}

		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after adding excluded model) error = %v", err)
		}
		excluded := requireConfiguredProviderModel(t, cfg.Providers["custom"].Models.Curated, "excluded")
		if excluded.Hidden == nil || *excluded.Hidden || excluded.Deprecated == nil || *excluded.Deprecated {
			t.Fatalf("excluded model overrides = %#v, want explicit hidden=false deprecated=false", excluded)
		}
	})

	t.Run("Should reject curation intent for an unresolved provider without writing config", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{})
		before := readFile(t, homePaths.ConfigFile)
		defaultEffort := modelcatalog.ReasoningEffortMax
		_, err := service.PutCollectionItem(context.Background(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "missing",
			Provider: &ProviderSettings{
				Command:   "missing-acp",
				ModelsSet: true,
				Models: aghconfig.ProviderModelsConfig{
					Curated: []aghconfig.ProviderModelConfig{{ID: "missing-model"}},
				},
			},
			ProviderModelCuration: &ProviderModelCurationRequest{
				ModelID:                "missing-model",
				DefaultReasoningEffort: &defaultEffort,
			},
		})
		if err == nil {
			t.Fatal("PutCollectionItem(unresolved provider curation) error = nil")
		}
		item, ok := diagnostics.ItemFromError(err)
		if !ok || item.Code != diagnosticcontract.CodeModelNotFound {
			t.Fatalf("curation diagnostic = %#v, want model_not_found; err=%v", item, err)
		}
		if after := readFile(t, homePaths.ConfigFile); after != before {
			t.Fatalf("config changed after rejected curation intent:\n%s", after)
		}
	})

	t.Run("Should reject curation intent without a model catalog before writing config", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+`

[providers.custom]
command = "custom-acp"

[providers.custom.models]
default = "custom-model"

[[providers.custom.models.curated]]
id = "custom-model"
`)
		service := testService(t, homePaths, Dependencies{})
		before := readFile(t, homePaths.ConfigFile)
		defaultEffort := modelcatalog.ReasoningEffortMax
		settings := ProviderSettings{
			Command:   "custom-acp",
			ModelsSet: true,
			Models: aghconfig.ProviderModelsConfig{
				Default: "custom-model",
			},
		}
		_, err := service.PutCollectionItem(context.Background(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "custom",
			Provider:          &settings,
			ProviderModelCuration: &ProviderModelCurationRequest{
				ModelID:                "custom-model",
				DefaultReasoningEffort: &defaultEffort,
			},
		})
		if !errors.Is(err, modelcatalog.ErrAllSourcesFailed) {
			t.Fatalf("PutCollectionItem(curation without catalog) error = %v, want ErrAllSourcesFailed", err)
		}
		if after := readFile(t, homePaths.ConfigFile); after != before {
			t.Fatalf("config changed after unavailable curation intent:\n%s", after)
		}
	})

	t.Run("Should keep providers readable when optional catalog sources have no usable rows", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		catalog := &settingsModelCatalogStub{
			models: map[string][]modelcatalog.Model{},
			errs: map[string]error{
				"blackbox": modelcatalog.ErrAllSourcesFailed,
			},
		}
		service := testService(t, homePaths, Dependencies{ModelCatalog: catalog})
		envelope, err := service.ListCollection(context.Background(), CollectionRequest{
			Collection: CollectionProviders,
		})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		blackbox := mustFindProviderItem(t, envelope.Providers, "blackbox")
		if blackbox.Settings.Models.Curated != nil {
			t.Fatalf("blackbox curated models = %#v, want nil fallback", blackbox.Settings.Models.Curated)
		}
	})

	t.Run("Should preserve raw membership when a degraded projection is round-tripped", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+`

[providers.custom]
command = "custom-acp"

[providers.custom.models]
default = "raw-model"

[[providers.custom.models.curated]]
id = "raw-model"
display_name = "Raw model"
featured = true
`)
		catalog := &settingsModelCatalogStub{
			models: map[string][]modelcatalog.Model{},
			errs: map[string]error{
				"custom": modelcatalog.ErrAllSourcesFailed,
			},
		}
		service := testService(t, homePaths, Dependencies{ModelCatalog: catalog})
		envelope, err := service.ListCollection(context.Background(), CollectionRequest{
			Collection: CollectionProviders,
		})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		custom := mustFindProviderItem(t, envelope.Providers, "custom")
		if custom.Settings.Models.Curated != nil {
			t.Fatalf("degraded projection curated = %#v, want omitted membership", custom.Settings.Models.Curated)
		}
		settings := custom.Settings
		settings.ModelsSet = true
		if _, err := service.PutCollectionItem(context.Background(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "custom",
			Provider:          &settings,
		}); err != nil {
			t.Fatalf("PutCollectionItem(degraded catalog round trip) error = %v", err)
		}
		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after degraded round trip) error = %v", err)
		}
		raw := requireConfiguredProviderModel(t, cfg.Providers["custom"].Models.Curated, "raw-model")
		if raw.DisplayName != "Raw model" || raw.Featured == nil || !*raw.Featured {
			t.Fatalf("raw model after degraded round trip = %#v, want original metadata", raw)
		}
		if raw.Hidden != nil && *raw.Hidden {
			t.Fatalf("raw model hidden = %v, want membership preserved", raw.Hidden)
		}
	})

	t.Run("Should reject an explicit membership clear when the catalog is unavailable", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+`

[providers.custom]
command = "custom-acp"

[providers.custom.models]
default = "raw-model"

[[providers.custom.models.curated]]
id = "raw-model"
`)
		catalog := &settingsModelCatalogStub{
			models: map[string][]modelcatalog.Model{},
			errs: map[string]error{
				"custom": modelcatalog.ErrAllSourcesFailed,
			},
		}
		service := testService(t, homePaths, Dependencies{ModelCatalog: catalog})
		settings := ProviderSettings{
			Command:   "custom-acp",
			ModelsSet: true,
			Models: aghconfig.ProviderModelsConfig{
				Default: "raw-model",
				Curated: []aghconfig.ProviderModelConfig{},
			},
		}
		before := readFile(t, homePaths.ConfigFile)
		_, err := service.PutCollectionItem(context.Background(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "custom",
			Provider:          &settings,
		})
		if !errors.Is(err, modelcatalog.ErrAllSourcesFailed) {
			t.Fatalf("PutCollectionItem(explicit clear during outage) error = %v, want ErrAllSourcesFailed", err)
		}
		if after := readFile(t, homePaths.ConfigFile); after != before {
			t.Fatalf("config changed after rejected explicit clear:\n%s", after)
		}
	})

	t.Run("Should reject an explicit membership clear when the catalog dependency is absent", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+`

[providers.custom]
command = "custom-acp"

[providers.custom.models]
default = "raw-model"

[[providers.custom.models.curated]]
id = "raw-model"
`)
		service := testService(t, homePaths, Dependencies{})
		settings := ProviderSettings{
			Command:   "custom-acp",
			ModelsSet: true,
			Models: aghconfig.ProviderModelsConfig{
				Default: "raw-model",
				Curated: []aghconfig.ProviderModelConfig{},
			},
		}
		before := readFile(t, homePaths.ConfigFile)
		_, err := service.PutCollectionItem(context.Background(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "custom",
			Provider:          &settings,
		})
		if !errors.Is(err, modelcatalog.ErrAllSourcesFailed) {
			t.Fatalf("PutCollectionItem(explicit clear without catalog) error = %v, want ErrAllSourcesFailed", err)
		}
		if after := readFile(t, homePaths.ConfigFile); after != before {
			t.Fatalf("config changed after rejected explicit clear without catalog:\n%s", after)
		}
	})

	t.Run("Should omit models when the curated catalog view has no candidates", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		catalog := &settingsModelCatalogStub{models: map[string][]modelcatalog.Model{
			"blackbox": {},
		}}
		service := testService(t, homePaths, Dependencies{ModelCatalog: catalog})
		envelope, err := service.ListCollection(context.Background(), CollectionRequest{
			Collection: CollectionProviders,
		})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		blackbox := mustFindProviderItem(t, envelope.Providers, "blackbox")
		if blackbox.Settings.Models.Curated != nil {
			t.Fatalf("blackbox curated models = %#v, want nil projection", blackbox.Settings.Models.Curated)
		}
	})
}

// TestCollectionMutationsCodexNativeProviderOverlay verifies Codex onboarding persistence.
func TestCollectionMutationsCodexNativeProviderOverlay(t *testing.T) {
	t.Parallel()

	t.Run("Should accept native CLI Codex overlay from onboarding", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+`

	[providers.codex.models]
	default = "gpt-5.5"
	`)
		service := testService(t, homePaths, Dependencies{})

		result, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "codex",
			Provider: &ProviderSettings{
				Command:         "npx -y @agentclientprotocol/codex-acp@latest",
				DisplayName:     "Codex",
				Harness:         aghconfig.ProviderHarnessACP,
				RuntimeProvider: "codex",
				AuthMode:        aghconfig.ProviderAuthModeNativeCLI,
				EnvPolicy:       aghconfig.ProviderEnvPolicyFiltered,
				HomePolicy:      aghconfig.ProviderHomePolicyOperator,
				AuthLoginCmd:    "codex login",
				Models: aghconfig.ProviderModelsConfig{
					Default: "gpt-5.5",
					Curated: []aghconfig.ProviderModelConfig{
						{
							ID:                     "gpt-5.4",
							DisplayName:            "GPT-5.4",
							SupportsTools:          new(true),
							SupportsReasoning:      new(true),
							ReasoningEfforts:       []string{"minimal", "low", "medium", "high", "xhigh"},
							DefaultReasoningEffort: "medium",
						},
						{
							ID:                     "gpt-5.4-mini",
							DisplayName:            "GPT-5.4 Mini",
							SupportsTools:          new(true),
							SupportsReasoning:      new(true),
							ReasoningEfforts:       []string{"minimal", "low", "medium", "high", "xhigh"},
							DefaultReasoningEffort: "medium",
						},
						{ID: "gpt-5.3", DisplayName: "GPT-5.3"},
						{ID: "gpt-5.3-mini", DisplayName: "GPT-5.3 Mini"},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("PutCollectionItem(codex native overlay) error = %v", err)
		}
		if got, want := result.WriteTarget, WriteTargetGlobalConfig; got != want {
			t.Fatalf("codex native overlay write target = %q, want %q", got, want)
		}
	})
}

func TestProviderSecretOnlyMutationStoresVaultSecret(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := testHomePaths(t)
	writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
	secretStore := newFakeProviderSecretStore()
	service := testService(t, homePaths, Dependencies{ProviderSecrets: secretStore})
	before := readFile(t, homePaths.ConfigFile)

	result, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionProviders},
		Name:              "openrouter",
		Provider:          &ProviderSettings{},
		ProviderSecrets: []ProviderSecretWrite{
			{
				Name:      "api_key",
				SecretRef: "vault:providers/openrouter/api-key",
				Kind:      "api_key",
				Value:     "openrouter-token",
			},
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(provider secret only) error = %v", err)
	}
	if got, want := result.WriteTarget, WriteTargetGlobalConfig; got != want {
		t.Fatalf("provider secret write target = %q, want %q", got, want)
	}
	if got, want := result.Behavior, MutationBehaviorRestartRequired; got != want {
		t.Fatalf("provider secret behavior = %q, want %q", got, want)
	}
	if got := secretStore.plaintext["vault:providers/openrouter/api-key"]; got != "openrouter-token" {
		t.Fatalf("stored provider secret = %q, want openrouter-token", got)
	}
	if after := readFile(t, homePaths.ConfigFile); after != before {
		t.Fatalf("config changed for secret-only mutation:\n%s", after)
	}
}

func TestProviderSecretMutationRejectsCrossProviderRefs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := testHomePaths(t)
	writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
	secretStore := newFakeProviderSecretStore()
	service := testService(t, homePaths, Dependencies{ProviderSecrets: secretStore})

	_, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionProviders},
		Name:              "openrouter",
		Provider:          &ProviderSettings{},
		ProviderSecrets: []ProviderSecretWrite{{
			Name:      "api_key",
			SecretRef: "vault:providers/anthropic/api-key",
			Kind:      "api_key",
			Value:     "openrouter-token",
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "must be scoped under vault:providers/openrouter") {
		t.Fatalf("PutCollectionItem(cross-provider secret ref) error = %v", err)
	}
	if len(secretStore.plaintext) != 0 {
		t.Fatalf("secret store writes = %#v, want none after validation failure", secretStore.plaintext)
	}
}

func TestProviderSecretMutationRejectsInvalidProviderConfigWithoutStoringSecrets(t *testing.T) {
	t.Run(
		"Should leave the secret store untouched when provider validation fails after ref checks",
		func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			homePaths := testHomePaths(t)
			writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
			secretStore := newFakeProviderSecretStore()
			service := testService(t, homePaths, Dependencies{ProviderSecrets: secretStore})
			before := readFile(t, homePaths.ConfigFile)

			_, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
				CollectionRequest: CollectionRequest{Collection: CollectionProviders},
				Name:              "openrouter",
				Provider: &ProviderSettings{
					AuthMode: aghconfig.ProviderAuthModeNone,
					CredentialSlots: []aghconfig.ProviderCredentialSlot{{
						Name:      "api_key",
						TargetEnv: "OPENROUTER_API_KEY",
						SecretRef: "vault:providers/openrouter/api-key",
						Kind:      "api_key",
						Required:  true,
					}},
				},
				ProviderSecrets: []ProviderSecretWrite{{
					Name:      "api_key",
					SecretRef: "vault:providers/openrouter/api-key",
					Kind:      "api_key",
					Value:     "openrouter-token",
				}},
			})
			if err == nil || !strings.Contains(err.Error(), "credential_slots cannot be set when auth_mode is none") {
				t.Fatalf("PutCollectionItem(invalid provider config) error = %v", err)
			}
			if len(secretStore.plaintext) != 0 {
				t.Fatalf(
					"secret store writes = %#v, want none after provider validation failure",
					secretStore.plaintext,
				)
			}
			if after := readFile(t, homePaths.ConfigFile); after != before {
				t.Fatalf("config changed after provider validation failure:\n%s", after)
			}
		},
	)
}

func TestMCPSecretValuesStoreVaultSecrets(t *testing.T) {
	t.Run("Should store stdio secret env values without writing plaintext config", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		secretStore := newFakeProviderSecretStore()
		service := testService(t, homePaths, Dependencies{ProviderSecrets: secretStore})

		result, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "github",
			Target:            TargetAuto,
			MCPServer: &aghconfig.MCPServer{
				Command: "npx",
			},
			MCPSecrets: MCPSecretValues{
				SecretEnv: map[string]string{"GITHUB_TOKEN": "ghp-secret"},
			},
		})
		if err != nil {
			t.Fatalf("PutCollectionItem(MCP stdio secret) error = %v", err)
		}
		if got, want := result.WriteTarget, WriteTargetGlobalMCPSidecar; got != want {
			t.Fatalf("MCP secret write target = %q, want %q", got, want)
		}
		if got, want := secretStore.plaintext["vault:mcp/global/github/env/GITHUB_TOKEN"], "ghp-secret"; got != want {
			t.Fatalf("stored MCP secret_env = %q, want %q", got, want)
		}
		sidecarPayload := readFile(t, filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName))
		if !strings.Contains(sidecarPayload, "vault:mcp/global/github/env/GITHUB_TOKEN") {
			t.Fatalf("sidecar payload missing secret ref:\n%s", sidecarPayload)
		}
		if strings.Contains(sidecarPayload, "ghp-secret") {
			t.Fatalf("sidecar payload leaked plaintext secret:\n%s", sidecarPayload)
		}
		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionMCPServers})
		if err != nil {
			t.Fatalf("ListCollection(MCP refs) error = %v", err)
		}
		item := findMCPItem(t, envelope.MCPServers, "github")
		assertMCPSecretKeys(t, item, "GITHUB_TOKEN")
		if item.Auth.ClientSecretRef != "" {
			t.Fatalf("settings read exposed OAuth client secret ref %q", item.Auth.ClientSecretRef)
		}
		if _, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "github",
			Target:            TargetAuto,
			MCPServer: &aghconfig.MCPServer{
				Command: "npx-updated",
			},
			MCPSecretPreservation: MCPSecretPreservation{
				SecretEnv: []string{"GITHUB_TOKEN"},
			},
		}); err != nil {
			t.Fatalf("PutCollectionItem(preserve MCP secret env) error = %v", err)
		}
		envelope, err = service.ListCollection(ctx, CollectionRequest{Collection: CollectionMCPServers})
		if err != nil {
			t.Fatalf("ListCollection(preserved MCP refs) error = %v", err)
		}
		item = findMCPItem(t, envelope.MCPServers, "github")
		if got, want := item.Command, "npx-updated"; got != want {
			t.Fatalf("preserved MCP command = %q, want %q", got, want)
		}
		assertMCPSecretKeys(t, item, "GITHUB_TOKEN")
		if got, want := secretStore.plaintext["vault:mcp/global/github/env/GITHUB_TOKEN"], "ghp-secret"; got != want {
			t.Fatalf("preserved MCP secret plaintext = %q, want %q", got, want)
		}
		if _, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "github",
		}); err != nil {
			t.Fatalf("DeleteCollectionItem(non-catalog MCP) error = %v", err)
		}
		deletedRef := "vault:mcp/global/github/env/GITHUB_TOKEN"
		_, err = secretStore.GetMetadata(ctx, deletedRef)
		if !errors.Is(err, vault.ErrSecretNotFound) {
			t.Fatalf("GetMetadata(deleted non-catalog MCP secret) error = %v, want ErrSecretNotFound", err)
		}
	})

	t.Run(
		"Should roll back definition and secret replacement when post-mutation auth invalidation fails",
		func(t *testing.T) {
			t.Parallel()

			homePaths := testHomePaths(t)
			writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
			secretStore := newFakeProviderSecretStore()
			initialService := testService(t, homePaths, Dependencies{ProviderSecrets: secretStore})
			request := CollectionItemPutRequest{
				CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
				Name:              "github",
				MCPServer:         &aghconfig.MCPServer{Command: "old-command"},
				MCPSecrets: MCPSecretValues{
					SecretEnv: map[string]string{"GITHUB_TOKEN": "old-secret"},
				},
			}
			if _, err := initialService.PutCollectionItem(t.Context(), request); err != nil {
				t.Fatalf("PutCollectionItem(initial) error = %v", err)
			}

			invalidateErr := errors.New("auth invalidation unavailable")
			runtime := &recordingMCPAuthRuntime{invalidateErrs: map[int]error{2: invalidateErr}}
			service := testService(t, homePaths, Dependencies{
				MCPAuth:         runtime,
				ProviderSecrets: secretStore,
			})
			request.MCPServer = &aghconfig.MCPServer{Command: "new-command"}
			request.MCPSecrets.SecretEnv["GITHUB_TOKEN"] = "new-secret"

			_, err := service.PutCollectionItem(t.Context(), request)
			if !errors.Is(err, invalidateErr) {
				t.Fatalf("PutCollectionItem(replacement) error = %v, want auth invalidation failure", err)
			}
			envelope, err := service.ListCollection(t.Context(), CollectionRequest{Collection: CollectionMCPServers})
			if err != nil {
				t.Fatalf("ListCollection() error = %v", err)
			}
			item := findMCPItem(t, envelope.MCPServers, "github")
			if item.Command != "old-command" {
				t.Fatalf("restored command = %q, want old-command", item.Command)
			}
			ref := "vault:mcp/global/github/env/GITHUB_TOKEN"
			if got := secretStore.plaintext[ref]; got != "old-secret" {
				t.Fatalf("restored secret = %q, want old-secret", got)
			}
			if got, want := len(runtime.invalidated), 3; got != want {
				t.Fatalf(
					"auth invalidation calls = %d, want pre-mutation, failed post-mutation, and post-rollback",
					got,
				)
			}
		},
	)

	t.Run(
		"Should retain committed replacement secrets when definition rollback fails",
		func(t *testing.T) {
			t.Parallel()

			homePaths := testHomePaths(t)
			writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
			secretStore := newFakeProviderSecretStore()
			initialService := testService(t, homePaths, Dependencies{ProviderSecrets: secretStore})
			request := CollectionItemPutRequest{
				CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
				Name:              "github",
				MCPServer:         &aghconfig.MCPServer{Command: "old-command"},
				MCPSecrets: MCPSecretValues{
					SecretEnv: map[string]string{"GITHUB_TOKEN": "old-secret"},
				},
			}
			if _, err := initialService.PutCollectionItem(t.Context(), request); err != nil {
				t.Fatalf("PutCollectionItem(initial) error = %v", err)
			}

			invalidateErr := errors.New("auth invalidation unavailable")
			rollbackErr := errors.New("definition rollback unavailable")
			runtime := &recordingMCPAuthRuntime{invalidateErrs: map[int]error{2: invalidateErr}}
			writeCalls := 0
			writer := func(
				home aghconfig.HomePaths,
				root string,
				name string,
				target aghconfig.WriteTarget,
				server aghconfig.MCPServer,
			) error {
				writeCalls++
				if writeCalls == 2 {
					return rollbackErr
				}
				return writeMCPDefinition(home, root, name, target, server)
			}
			service := testService(t, homePaths, Dependencies{
				MCPAuth:             runtime,
				ProviderSecrets:     secretStore,
				MCPDefinitionWriter: writer,
			})
			request.MCPServer = &aghconfig.MCPServer{Command: "new-command"}
			request.MCPSecrets.SecretEnv["GITHUB_TOKEN"] = "new-secret"

			_, err := service.PutCollectionItem(t.Context(), request)
			if !errors.Is(err, invalidateErr) || !errors.Is(err, rollbackErr) {
				t.Fatalf("PutCollectionItem(replacement) error = %v, want invalidation and rollback failures", err)
			}
			envelope, err := service.ListCollection(t.Context(), CollectionRequest{Collection: CollectionMCPServers})
			if err != nil {
				t.Fatalf("ListCollection() error = %v", err)
			}
			if got := findMCPItem(t, envelope.MCPServers, "github").Command; got != "new-command" {
				t.Fatalf("committed command = %q, want new-command", got)
			}
			ref := "vault:mcp/global/github/env/GITHUB_TOKEN"
			if got := secretStore.plaintext[ref]; got != "new-secret" {
				t.Fatalf("committed secret = %q, want new-secret", got)
			}
		},
	)

	t.Run("Should preserve a canonical secret referenced by a surviving source", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		secretStore := newFakeProviderSecretStore()
		service := testService(t, homePaths, Dependencies{ProviderSecrets: secretStore})
		canonicalRef := "vault:mcp/global/shared/env/TOKEN"
		if _, err := service.PutCollectionItem(t.Context(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "shared",
			Target:            TargetConfig,
			MCPServer:         &aghconfig.MCPServer{Command: "config-command"},
			MCPSecrets: MCPSecretValues{
				SecretEnv: map[string]string{"TOKEN": "shared-secret"},
			},
		}); err != nil {
			t.Fatalf("PutCollectionItem(config source) error = %v", err)
		}
		if _, err := service.PutCollectionItem(t.Context(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "shared",
			Target:            TargetSidecar,
			MCPServer: &aghconfig.MCPServer{
				Command:   "sidecar-command",
				SecretEnv: map[string]string{"TOKEN": canonicalRef},
			},
		}); err != nil {
			t.Fatalf("PutCollectionItem(sidecar source) error = %v", err)
		}
		if _, err := service.DeleteCollectionItem(t.Context(), CollectionItemDeleteRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "shared",
			Target:            TargetSidecar,
		}); err != nil {
			t.Fatalf("DeleteCollectionItem(sidecar source) error = %v", err)
		}
		if got := secretStore.plaintext[canonicalRef]; got != "shared-secret" {
			t.Fatalf("surviving source secret = %q, want shared-secret", got)
		}
		envelope, err := service.ListCollection(t.Context(), CollectionRequest{Collection: CollectionMCPServers})
		if err != nil {
			t.Fatalf("ListCollection() error = %v", err)
		}
		if got := findMCPItem(t, envelope.MCPServers, "shared").Command; got != "config-command" {
			t.Fatalf("surviving source command = %q, want config-command", got)
		}
	})

	t.Run("Should preserve plain env only for the exact existing target", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{})
		if _, err := service.PutCollectionItem(t.Context(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "plain-env",
			Target:            TargetSidecar,
			MCPServer: &aghconfig.MCPServer{
				Command: "old-command",
				Env:     map[string]string{"PROJECT": "agh"},
			},
		}); err != nil {
			t.Fatalf("PutCollectionItem(initial plain env) error = %v", err)
		}

		if _, err := service.PutCollectionItem(t.Context(), CollectionItemPutRequest{
			CollectionRequest:  CollectionRequest{Collection: CollectionMCPServers},
			Name:               "plain-env",
			Target:             TargetSidecar,
			MCPServer:          &aghconfig.MCPServer{Command: "new-command"},
			MCPEnvPreservation: []string{"PROJECT"},
		}); err != nil {
			t.Fatalf("PutCollectionItem(preserve plain env) error = %v", err)
		}
		servers, err := aghconfig.LoadMCPServersJSONFile(filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName))
		if err != nil {
			t.Fatalf("LoadMCPServersJSONFile() error = %v", err)
		}
		var persisted aghconfig.MCPServer
		for _, candidate := range servers {
			if candidate.Name == "plain-env" {
				persisted = candidate
				break
			}
		}
		if got, want := persisted.Env["PROJECT"], "agh"; got != want {
			t.Fatalf("preserved PROJECT = %q, want %q", got, want)
		}

		_, err = service.PutCollectionItem(t.Context(), CollectionItemPutRequest{
			CollectionRequest:  CollectionRequest{Collection: CollectionMCPServers},
			Name:               "plain-env",
			Target:             TargetConfig,
			MCPServer:          &aghconfig.MCPServer{Command: "config-command"},
			MCPEnvPreservation: []string{"PROJECT"},
		})
		if err == nil || !strings.Contains(err.Error(), "has no existing env values") {
			t.Fatalf("PutCollectionItem(cross-target preserve) error = %v, want exact-target rejection", err)
		}
	})

	t.Run("Should store OAuth client secret values without writing plaintext config", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		secretStore := newFakeProviderSecretStore()
		service := testService(t, homePaths, Dependencies{ProviderSecrets: secretStore})
		clientSecret := "oauth-client-secret"

		_, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "linear",
			Target:            TargetAuto,
			MCPServer: &aghconfig.MCPServer{
				Transport: aghconfig.MCPServerTransportSSE,
				URL:       "https://mcp.linear.app/sse",
				Auth: aghconfig.MCPAuthConfig{
					Type:             aghconfig.MCPAuthTypeOAuth2PKCE,
					AuthorizationURL: "https://linear.app/oauth/authorize",
					TokenURL:         "https://api.linear.app/oauth/token",
					ClientID:         "agh-client",
				},
			},
			MCPSecrets: MCPSecretValues{OAuthClientSecret: &clientSecret},
		})
		if err != nil {
			t.Fatalf("PutCollectionItem(MCP OAuth secret) error = %v", err)
		}
		if got, want := secretStore.plaintext["vault:mcp/global/linear/oauth/client-secret"], clientSecret; got != want {
			t.Fatalf("stored MCP OAuth secret = %q, want %q", got, want)
		}
		sidecarPayload := readFile(t, filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName))
		if !strings.Contains(sidecarPayload, "vault:mcp/global/linear/oauth/client-secret") {
			t.Fatalf("sidecar payload missing client secret ref:\n%s", sidecarPayload)
		}
		if strings.Contains(sidecarPayload, clientSecret) {
			t.Fatalf("sidecar payload leaked OAuth client secret:\n%s", sidecarPayload)
		}
		if _, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "linear",
			Target:            TargetAuto,
			MCPServer: &aghconfig.MCPServer{
				Transport: aghconfig.MCPServerTransportSSE,
				URL:       "https://mcp.linear.app/sse-v2",
				Auth: aghconfig.MCPAuthConfig{
					Type:             aghconfig.MCPAuthTypeOAuth2PKCE,
					AuthorizationURL: "https://linear.app/oauth/authorize",
					TokenURL:         "https://api.linear.app/oauth/token",
					ClientID:         "agh-client",
				},
			},
			MCPSecretPreservation: MCPSecretPreservation{OAuthClientSecret: true},
		}); err != nil {
			t.Fatalf("PutCollectionItem(preserve MCP OAuth secret) error = %v", err)
		}
		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionMCPServers})
		if err != nil {
			t.Fatalf("ListCollection(preserved MCP OAuth secret) error = %v", err)
		}
		item := findMCPItem(t, envelope.MCPServers, "linear")
		if got, want := item.URL, "https://mcp.linear.app/sse-v2"; got != want {
			t.Fatalf("preserved MCP URL = %q, want %q", got, want)
		}
		if !item.ClientSecretConfigured || item.Auth.ClientSecretRef != "" {
			t.Fatalf(
				"preserved MCP OAuth presence = %t with ref %q, want true without ref",
				item.ClientSecretConfigured,
				item.Auth.ClientSecretRef,
			)
		}
		if got := secretStore.plaintext["vault:mcp/global/linear/oauth/client-secret"]; got != clientSecret {
			t.Fatalf("preserved MCP OAuth plaintext = %q, want original value", got)
		}
	})

	t.Run("Should reject preservation when the exact target has no existing binding", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{ProviderSecrets: newFakeProviderSecretStore()})
		_, err := service.PutCollectionItem(t.Context(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "github",
			Target:            TargetConfig,
			MCPServer:         &aghconfig.MCPServer{Command: "npx"},
			MCPSecretPreservation: MCPSecretPreservation{
				SecretEnv: []string{"GITHUB_TOKEN"},
			},
		})
		if err == nil || !strings.Contains(err.Error(), "has no existing bindings") {
			t.Fatalf("PutCollectionItem(missing preserved binding) error = %v, want exact-target rejection", err)
		}
	})

	t.Run("Should preserve the complete OAuth contract in the config target", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		secretStore := newFakeProviderSecretStore()
		service := testService(t, homePaths, Dependencies{ProviderSecrets: secretStore})
		clientSecret := "full-oauth-client-secret"
		wantAuth := aghconfig.MCPAuthConfig{
			Type:             aghconfig.MCPAuthTypeOAuth2PKCE,
			IssuerURL:        "https://auth.example.com",
			MetadataURL:      "https://auth.example.com/.well-known/oauth-authorization-server",
			AuthorizationURL: "https://auth.example.com/authorize",
			TokenURL:         "https://auth.example.com/token",
			RevocationURL:    "https://auth.example.com/revoke",
			ClientID:         "agh-client",
			Scopes:           []string{"read", "write"},
		}

		result, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "full-oauth",
			Target:            TargetConfig,
			MCPServer: &aghconfig.MCPServer{
				Transport: aghconfig.MCPServerTransportHTTP,
				URL:       "https://mcp.example.com",
				Auth:      wantAuth,
			},
			MCPSecrets: MCPSecretValues{OAuthClientSecret: &clientSecret},
		})
		if err != nil {
			t.Fatalf("PutCollectionItem(full OAuth config) error = %v", err)
		}
		if got, want := result.WriteTarget, WriteTargetGlobalConfig; got != want {
			t.Fatalf("full OAuth write target = %q, want %q", got, want)
		}

		canonicalRef := "vault:mcp/global/full-oauth/oauth/client-secret"
		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionMCPServers})
		if err != nil {
			t.Fatalf("ListCollection(full OAuth config) error = %v", err)
		}
		item := findMCPItem(t, envelope.MCPServers, "full-oauth")
		if !reflect.DeepEqual(item.Auth, wantAuth) {
			t.Fatalf("full OAuth auth = %#v, want %#v", item.Auth, wantAuth)
		}
		if !item.ClientSecretConfigured {
			t.Fatal("full OAuth client secret presence = false, want true")
		}
		if got, want := secretStore.plaintext[canonicalRef], clientSecret; got != want {
			t.Fatalf("stored full OAuth secret = %q, want %q", got, want)
		}
		configPayload := readFile(t, homePaths.ConfigFile)
		if strings.Contains(configPayload, clientSecret) || !strings.Contains(configPayload, canonicalRef) {
			t.Fatalf("config must contain only the OAuth client secret ref:\n%s", configPayload)
		}
	})

	t.Run("Should replace a declared ref with the canonical ref for typed values", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		secretStore := newFakeProviderSecretStore()
		service := testService(t, homePaths, Dependencies{ProviderSecrets: secretStore})

		_, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "github",
			MCPServer: &aghconfig.MCPServer{
				Command: "npx",
				SecretEnv: map[string]string{
					"GITHUB_TOKEN": "env:GITHUB_TOKEN",
				},
			},
			MCPSecrets: MCPSecretValues{
				SecretEnv: map[string]string{"GITHUB_TOKEN": "ghp-secret"},
			},
		})
		if err != nil {
			t.Fatalf("PutCollectionItem(typed MCP secret) error = %v", err)
		}
		canonicalRef := "vault:mcp/global/github/env/GITHUB_TOKEN"
		if got, want := secretStore.plaintext[canonicalRef], "ghp-secret"; got != want {
			t.Fatalf("stored MCP secret_env = %q, want %q", got, want)
		}
		sidecarPayload := readFile(t, filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName))
		if !strings.Contains(sidecarPayload, canonicalRef) || strings.Contains(sidecarPayload, "env:GITHUB_TOKEN") {
			t.Fatalf("sidecar payload did not replace the declared ref with %q:\n%s", canonicalRef, sidecarPayload)
		}
	})

	t.Run("Should reject invalid stdio auth config without storing secrets", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		secretStore := newFakeProviderSecretStore()
		service := testService(t, homePaths, Dependencies{ProviderSecrets: secretStore})

		_, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "github",
			Target:            TargetAuto,
			MCPServer: &aghconfig.MCPServer{
				Command: "npx",
				SecretEnv: map[string]string{
					"GITHUB_TOKEN": "vault:mcp/github/env/GITHUB_TOKEN",
				},
				Auth: aghconfig.MCPAuthConfig{
					Type:             aghconfig.MCPAuthTypeOAuth2PKCE,
					AuthorizationURL: "https://example.com/oauth/authorize",
					TokenURL:         "https://example.com/oauth/token",
					ClientID:         "agh-client",
					ClientSecretRef:  "vault:mcp/github/oauth/client-secret",
				},
			},
			MCPSecrets: MCPSecretValues{
				SecretEnv: map[string]string{"GITHUB_TOKEN": "ghp-secret"},
			},
		})
		if err == nil || !strings.Contains(err.Error(), "mcp_servers[0].auth is only valid for remote MCP servers") {
			t.Fatalf("PutCollectionItem(invalid MCP stdio auth) error = %v", err)
		}
		if len(secretStore.plaintext) != 0 {
			t.Fatalf("secret store writes = %#v, want none after MCP validation failure", secretStore.plaintext)
		}
		sidecarPath := filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName)
		if _, statErr := os.Stat(sidecarPath); !os.IsNotExist(statErr) {
			t.Fatalf("sidecar created after MCP validation failure: stat error = %v", statErr)
		}
	})

	t.Run("Should reject unsafe secret write shapes before persistence", func(t *testing.T) {
		t.Parallel()

		emptyOAuthSecret := ""
		tests := []struct {
			name          string
			serverName    string
			server        aghconfig.MCPServer
			secrets       MCPSecretValues
			withoutStore  bool
			wantErrorPart string
		}{
			{
				name:          "Should require the Vault service",
				serverName:    "github",
				server:        aghconfig.MCPServer{Command: "npx"},
				secrets:       MCPSecretValues{SecretEnv: map[string]string{"TOKEN": "secret"}},
				withoutStore:  true,
				wantErrorPart: "secret store is not available",
			},
			{
				name:       "Should reject secret env for remote transport",
				serverName: "remote",
				server: aghconfig.MCPServer{
					Transport: aghconfig.MCPServerTransportHTTP,
					URL:       "https://mcp.example.com",
				},
				secrets:       MCPSecretValues{SecretEnv: map[string]string{"TOKEN": "secret"}},
				wantErrorPart: "secret_env values require stdio transport",
			},
			{
				name:          "Should reject an invalid secret env key",
				serverName:    "github",
				server:        aghconfig.MCPServer{Command: "npx"},
				secrets:       MCPSecretValues{SecretEnv: map[string]string{"NOT-VALID": "secret"}},
				wantErrorPart: "secret_env key",
			},
			{
				name:          "Should reject an empty secret env value",
				serverName:    "github",
				server:        aghconfig.MCPServer{Command: "npx"},
				secrets:       MCPSecretValues{SecretEnv: map[string]string{"TOKEN": ""}},
				wantErrorPart: "secret_env value",
			},
			{
				name:          "Should reject an invalid canonical owner",
				serverName:    "github/unsafe",
				server:        aghconfig.MCPServer{Command: "npx"},
				secrets:       MCPSecretValues{SecretEnv: map[string]string{"TOKEN": "secret"}},
				wantErrorPart: "server name cannot contain a slash",
			},
			{
				name:       "Should reject an empty OAuth client secret",
				serverName: "oauth",
				server: aghconfig.MCPServer{
					Transport: aghconfig.MCPServerTransportHTTP,
					URL:       "https://mcp.example.com",
					Auth: aghconfig.MCPAuthConfig{
						Type:      aghconfig.MCPAuthTypeOAuth2PKCE,
						IssuerURL: "https://auth.example.com",
						ClientID:  "agh-client",
					},
				},
				secrets:       MCPSecretValues{OAuthClientSecret: &emptyOAuthSecret},
				wantErrorPart: "OAuth client secret value is required",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				homePaths := testHomePaths(t)
				writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
				dependencies := Dependencies{}
				secretStore := newFakeProviderSecretStore()
				if !tc.withoutStore {
					dependencies.ProviderSecrets = secretStore
				}
				service := testService(t, homePaths, dependencies)

				_, err := service.PutCollectionItem(context.Background(), CollectionItemPutRequest{
					CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
					Name:              tc.serverName,
					MCPServer:         &tc.server,
					MCPSecrets:        tc.secrets,
				})
				if err == nil || !strings.Contains(err.Error(), tc.wantErrorPart) {
					t.Fatalf(
						"PutCollectionItem(unsafe secret shape) error = %v, want containing %q",
						err,
						tc.wantErrorPart,
					)
				}
				if len(secretStore.plaintext) != 0 {
					t.Fatalf("secret writes = %#v, want none", secretStore.plaintext)
				}
				sidecarPath := filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName)
				if _, statErr := os.Stat(sidecarPath); !os.IsNotExist(statErr) {
					t.Fatalf("MCP sidecar exists after rejected secret shape: %v", statErr)
				}
			})
		}
	})
}

func TestInstallMCPCatalogAppliesLiveConfiguration(t *testing.T) {
	t.Parallel()
	t.Run("Should apply the installed catalog configuration live", func(t *testing.T) {
		t.Parallel()

		ctx := WithMutationSource(context.Background(), "uds")
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
		applier := &fakeConfigRuntimeApplier{}
		service := testService(t, homePaths, Dependencies{
			MCPCatalog:      fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			ProviderSecrets: newFakeProviderSecretStore(),
			RuntimeApplier:  applier,
			ApplyRecords:    NewConfigApplyRecordRepository(db.DB(), nil),
		})
		if _, err := service.ActiveConfig(ctx); err != nil {
			t.Fatalf("ActiveConfig(before install) error = %v", err)
		}

		_, err = service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
			EntryID: "github",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"GITHUB_TOKEN": {Value: "ghp-secret"},
			}},
		})
		if err != nil {
			t.Fatalf("InstallMCPCatalog() error = %v", err)
		}
		if got, want := applier.calls, 1; got != want {
			t.Fatalf("runtime apply calls = %d, want %d", got, want)
		}
		records, err := service.ListApplyRecords(ctx, ApplyRecordFilter{Status: lifecycle.StatusApplied})
		if err != nil {
			t.Fatalf("ListApplyRecords() error = %v", err)
		}
		if got, want := len(records), 1; got != want {
			t.Fatalf("applied record count = %d, want %d", got, want)
		}
		if got, want := records[0].Actor, "uds"; got != want {
			t.Fatalf("apply actor = %q, want %q", got, want)
		}
		if got, want := records[0].Lifecycle, lifecycle.LiveAdd; got != want {
			t.Fatalf("apply lifecycle = %q, want %q", got, want)
		}
		if got, want := records[0].Generation, int64(1); got != want {
			t.Fatalf("active generation = %d, want %d", got, want)
		}
		active, err := service.ActiveConfig(ctx)
		if err != nil {
			t.Fatalf("ActiveConfig(after install) error = %v", err)
		}
		if len(active.MCPServers) != 1 || active.MCPServers[0].Name != "GitHub" {
			t.Fatalf("active MCP servers = %#v, want GitHub", active.MCPServers)
		}
	})
}

func TestInstallMCPCatalogSerializesConcurrentMutations(t *testing.T) {
	t.Parallel()
	t.Run("Should serialize concurrent catalog install mutations", func(t *testing.T) {
		t.Parallel()

		const installCount = 12
		ctx := WithMutationSource(context.Background(), "http")
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		secretStore := newFakeProviderSecretStore()
		applier := &fakeConfigRuntimeApplier{}
		service := testService(t, homePaths, Dependencies{
			MCPCatalog:      fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			ProviderSecrets: secretStore,
			RuntimeApplier:  applier,
		})
		if _, err := service.ActiveConfig(ctx); err != nil {
			t.Fatalf("ActiveConfig(before installs) error = %v", err)
		}

		start := make(chan struct{})
		results := make(chan MCPCatalogInstallResult, installCount)
		errorsByInstall := make(chan error, installCount)
		var installs sync.WaitGroup
		for index := range installCount {
			installs.Go(func() {
				<-start
				name := fmt.Sprintf("github-%02d", index)
				result, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
					EntryID: "github",
					Name:    name,
					Scope:   ScopeGlobal,
					Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
						"GITHUB_TOKEN": {Value: "secret-" + name},
					}},
				})
				if err != nil {
					errorsByInstall <- err
					return
				}
				results <- result
			})
		}
		close(start)
		installs.Wait()
		close(results)
		close(errorsByInstall)
		for err := range errorsByInstall {
			t.Errorf("InstallMCPCatalog() error = %v", err)
		}

		generations := make(map[int64]struct{}, installCount)
		for result := range results {
			if !result.Apply.Applied {
				t.Errorf("install %q Applied = false", result.Item.Name)
			}
			if got, want := result.Apply.Record.Lifecycle, lifecycle.LiveAdd; got != want {
				t.Errorf("install %q lifecycle = %q, want %q", result.Item.Name, got, want)
			}
			generations[result.Apply.Record.Generation] = struct{}{}
		}
		if got, want := len(generations), installCount; got != want {
			t.Fatalf("unique active generations = %d, want %d", got, want)
		}
		servers, err := aghconfig.LoadMCPServersJSONFile(filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName))
		if err != nil {
			t.Fatalf("LoadMCPServersJSONFile() error = %v", err)
		}
		if got, want := len(servers), installCount; got != want {
			t.Fatalf("persisted MCP servers = %d, want %d", got, want)
		}
		if got, want := len(secretStore.plaintext), installCount; got != want {
			t.Fatalf("persisted MCP secrets = %d, want %d", got, want)
		}
		if got, want := applier.calls, installCount; got != want {
			t.Fatalf("runtime apply calls = %d, want %d", got, want)
		}
		records, err := service.ListApplyRecords(ctx, ApplyRecordFilter{Status: lifecycle.StatusApplied})
		if err != nil {
			t.Fatalf("ListApplyRecords() error = %v", err)
		}
		if got, want := len(records), installCount; got != want {
			t.Fatalf("applied record count = %d, want %d", got, want)
		}
	})
}

func TestInstallMCPCatalogEnforcesBornValidVaultSemantics(t *testing.T) {
	t.Run("Should install a feed-locked stdio entry with scoped refs and provenance", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		secretStore := newFakeProviderSecretStore()
		installNotifier := &recordingMarketplaceInstallNotifier{}
		service := testService(t, homePaths, Dependencies{
			MCPCatalog:               fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			ProviderSecrets:          secretStore,
			MarketplaceInstallEvents: installNotifier,
		})

		result, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
			EntryID: "github",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"GITHUB_TOKEN": {Value: "ghp-secret"},
			}},
		})
		if err != nil {
			t.Fatalf("InstallMCPCatalog() error = %v", err)
		}
		if got, want := result.NextStep, MCPCatalogInstallNextStepNone; got != want {
			t.Fatalf("NextStep = %q, want %q", got, want)
		}
		if got, want := result.Item.CatalogEntry, "github"; got != want {
			t.Fatalf("CatalogEntry = %q, want %q", got, want)
		}
		if got, want := result.Item.CatalogVersion, "1.2.3"; got != want {
			t.Fatalf("CatalogVersion = %q, want %q", got, want)
		}
		canonicalRef := "vault:mcp/global/GitHub/env/GITHUB_TOKEN"
		assertMCPSecretKeys(t, result.Item, "GITHUB_TOKEN")
		if got, want := secretStore.plaintext[canonicalRef], "ghp-secret"; got != want {
			t.Fatalf("stored secret = %q, want %q", got, want)
		}
		sidecar := readFile(t, filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName))
		if strings.Contains(sidecar, "ghp-secret") || !strings.Contains(sidecar, canonicalRef) {
			t.Fatalf("sidecar must contain only the bound ref:\n%s", sidecar)
		}
		responsePayload, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("json.Marshal(install result) error = %v", err)
		}
		if strings.Contains(string(responsePayload), "ghp-secret") ||
			strings.Contains(string(responsePayload), canonicalRef) {
			t.Fatalf("install result leaked secret material or binding: %s", responsePayload)
		}
		outcomes := installNotifier.snapshot()
		outcomePayload, err := json.Marshal(outcomes)
		if err != nil {
			t.Fatalf("json.Marshal(install outcomes) error = %v", err)
		}
		if strings.Contains(string(outcomePayload), "ghp-secret") {
			t.Fatalf("marketplace install outcome leaked write-only value: %s", outcomePayload)
		}
		if len(outcomes) != 1 {
			t.Fatalf("marketplace install outcome count = %d, want 1", len(outcomes))
		}
		if outcomes[0].Kind != marketplace.KindMCP ||
			outcomes[0].EntryID != "github" ||
			outcomes[0].Outcome != marketplace.InstallOutcomeSucceeded ||
			outcomes[0].PolicyGate != marketplace.InstallPolicyGatePassed {
			t.Fatalf("marketplace install outcome = %#v", outcomes[0])
		}
	})

	t.Run("Should report an install event persistence failure after committing the server", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		notifyErr := errors.New("event store unavailable")
		service := testService(t, homePaths, Dependencies{
			MCPCatalog:               fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			ProviderSecrets:          newFakeProviderSecretStore(),
			MarketplaceInstallEvents: &recordingMarketplaceInstallNotifier{err: notifyErr},
		})

		result, err := service.InstallMCPCatalog(t.Context(), MCPCatalogInstallRequest{
			EntryID: "github",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"GITHUB_TOKEN": {Value: "ghp-secret"},
			}},
		})
		if err != nil {
			t.Fatalf("InstallMCPCatalog() error = %v, want committed result with warning", err)
		}
		if result.Item.Name != "GitHub" {
			t.Fatalf("InstallMCPCatalog() item = %#v, want committed GitHub server", result.Item)
		}
		sidecar := readFile(t, filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName))
		if !strings.Contains(sidecar, `"GitHub"`) {
			t.Fatalf("committed sidecar missing GitHub server:\n%s", sidecar)
		}
		if len(result.Warnings) != 1 ||
			result.Warnings[0].Code != diagnosticcontract.CodeMCPInstallEventPersistFailed {
			t.Fatalf("InstallMCPCatalog() warnings = %#v, want event persistence diagnostic", result.Warnings)
		}
	})

	t.Run("Should reject an unaddressable server name before catalog persistence", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{
			MCPCatalog: fakeMCPCatalog{entry: plainEnvMCPCatalogEntry()},
		})
		_, err := service.InstallMCPCatalog(t.Context(), MCPCatalogInstallRequest{
			EntryID: "plain-env",
			Name:    "team/server",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"PROJECT": {Value: "agh"},
			}},
		})
		if err == nil || !strings.Contains(err.Error(), "server name cannot contain a slash") {
			t.Fatalf("InstallMCPCatalog(unaddressable name) error = %v, want canonical-name rejection", err)
		}
		sidecarPath := filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName)
		if _, statErr := os.Stat(sidecarPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("catalog install created sidecar before name validation: %v", statErr)
		}
	})

	t.Run("Should apply required plain values and feed defaults without exposing them", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{
			MCPCatalog: fakeMCPCatalog{entry: plainEnvMCPCatalogEntry()},
		})
		result, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
			EntryID: "plain-env",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"PROJECT": {Value: "agh"},
			}},
		})
		if err != nil {
			t.Fatalf("InstallMCPCatalog(plain env) error = %v", err)
		}
		if got, want := result.Item.EnvKeys, []string{"PROJECT", "REGION"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("plain env response keys = %#v, want %#v", got, want)
		}
		sidecar := readFile(t, filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName))
		if !strings.Contains(sidecar, `"PROJECT": "agh"`) ||
			!strings.Contains(sidecar, `"REGION": "us-east-1"`) {
			t.Fatalf("plain env sidecar did not preserve supplied/default values:\n%s", sidecar)
		}
	})

	t.Run("Should accept a present shared ref and retain it on uninstall", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		secretStore := newFakeProviderSecretStore()
		sharedRef := "vault:mcp/shared/github-token"
		if _, err := secretStore.PutSecret(ctx, sharedRef, "shared", "shared-secret"); err != nil {
			t.Fatalf("PutSecret(shared) error = %v", err)
		}
		service := testService(t, homePaths, Dependencies{
			MCPCatalog:      fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			ProviderSecrets: secretStore,
		})

		result, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
			EntryID: "github",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"GITHUB_TOKEN": {VaultRef: sharedRef},
			}},
		})
		if err != nil {
			t.Fatalf("InstallMCPCatalog(shared ref) error = %v", err)
		}
		assertMCPSecretKeys(t, result.Item, "GITHUB_TOKEN")
		if _, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              result.Item.Name,
		}); err != nil {
			t.Fatalf("DeleteCollectionItem(installed MCP) error = %v", err)
		}
		metadata, err := secretStore.GetMetadata(ctx, sharedRef)
		if err != nil || !metadata.Present {
			t.Fatalf("shared ref after uninstall = %#v, %v; want retained", metadata, err)
		}
	})

	t.Run("Should garbage collect a superseded install-owned ref", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		secretStore := newFakeProviderSecretStore()
		sharedRef := "vault:mcp/shared/github-token"
		if _, err := secretStore.PutSecret(ctx, sharedRef, "shared", "shared-secret"); err != nil {
			t.Fatalf("PutSecret(shared) error = %v", err)
		}
		service := testService(t, homePaths, Dependencies{
			MCPCatalog:      fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			ProviderSecrets: secretStore,
		})

		_, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
			EntryID: "github",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"GITHUB_TOKEN": {Value: "owned-secret"},
			}},
		})
		if err != nil {
			t.Fatalf("InstallMCPCatalog(owned ref) error = %v", err)
		}
		ownedRef := "vault:mcp/global/GitHub/env/GITHUB_TOKEN"

		replacement, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
			EntryID: "github",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"GITHUB_TOKEN": {VaultRef: sharedRef},
			}},
		})
		if err != nil {
			t.Fatalf("InstallMCPCatalog(shared replacement) error = %v", err)
		}
		assertMCPSecretKeys(t, replacement.Item, "GITHUB_TOKEN")
		if _, err := secretStore.GetMetadata(ctx, ownedRef); !errors.Is(err, vault.ErrSecretNotFound) {
			t.Fatalf("GetMetadata(superseded owned ref) error = %v, want ErrSecretNotFound", err)
		}
		metadata, err := secretStore.GetMetadata(ctx, sharedRef)
		if err != nil || !metadata.Present {
			t.Fatalf("shared replacement ref = %#v, %v; want retained", metadata, err)
		}
	})

	t.Run("Should restore the previous binding when superseded secret cleanup fails", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		secretStore := newFakeProviderSecretStore()
		runtime := &recordingMCPAuthRuntime{}
		sharedRef := "vault:mcp/shared/github-token"
		if _, err := secretStore.PutSecret(ctx, sharedRef, "shared", "shared-secret"); err != nil {
			t.Fatalf("PutSecret(shared) error = %v", err)
		}
		service := testService(t, homePaths, Dependencies{
			MCPCatalog:      fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			MCPAuth:         runtime,
			ProviderSecrets: secretStore,
		})

		initial, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
			EntryID: "github",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"GITHUB_TOKEN": {Value: "owned-secret"},
			}},
		})
		if err != nil {
			t.Fatalf("InstallMCPCatalog(owned ref) error = %v", err)
		}
		ownedRef := "vault:mcp/global/GitHub/env/GITHUB_TOKEN"
		invalidationBaseline := len(runtime.invalidated)
		deleteFailure := errors.New("forced superseded secret deletion failure")
		secretStore.deleteErrors[ownedRef] = deleteFailure

		_, err = service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
			EntryID: "github",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"GITHUB_TOKEN": {VaultRef: sharedRef},
			}},
		})
		if !errors.Is(err, deleteFailure) {
			t.Fatalf("InstallMCPCatalog(failed replacement cleanup) error = %v, want deletion failure", err)
		}
		if got, want := len(runtime.invalidated), invalidationBaseline+3; got != want {
			t.Fatalf(
				"auth invalidation calls after cleanup rollback = %d, want %d (pre, post, restored)",
				got,
				want,
			)
		}
		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionMCPServers})
		if err != nil {
			t.Fatalf("ListCollection(restored binding) error = %v", err)
		}
		item := findMCPItem(t, envelope.MCPServers, initial.Item.Name)
		assertMCPSecretKeys(t, item, "GITHUB_TOKEN")
		sidecar := readFile(t, filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName))
		if !strings.Contains(sidecar, ownedRef) || strings.Contains(sidecar, sharedRef) {
			t.Fatalf("restored sidecar binding is incorrect:\n%s", sidecar)
		}
		metadata, err := secretStore.GetMetadata(ctx, ownedRef)
		if err != nil || !metadata.Present {
			t.Fatalf("restored owned ref = %#v, %v; want retained", metadata, err)
		}
		metadata, err = secretStore.GetMetadata(ctx, sharedRef)
		if err != nil || !metadata.Present {
			t.Fatalf("shared ref after failed replacement = %#v, %v; want retained", metadata, err)
		}
	})

	t.Run("Should garbage collect only an owned canonical ref", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		secretStore := newFakeProviderSecretStore()
		service := testService(t, homePaths, Dependencies{
			MCPCatalog:      fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			ProviderSecrets: secretStore,
		})
		result, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
			EntryID: "github",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"GITHUB_TOKEN": {Value: "owned-secret"},
			}},
		})
		if err != nil {
			t.Fatalf("InstallMCPCatalog(owned ref) error = %v", err)
		}
		ownedRef := "vault:mcp/global/GitHub/env/GITHUB_TOKEN"
		if _, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              result.Item.Name,
		}); err != nil {
			t.Fatalf("DeleteCollectionItem(installed MCP) error = %v", err)
		}
		if _, err := secretStore.GetMetadata(ctx, ownedRef); !errors.Is(err, vault.ErrSecretNotFound) {
			t.Fatalf("GetMetadata(owned ref) error = %v, want ErrSecretNotFound", err)
		}
	})

	t.Run("Should require Vault ownership proof before deleting canonical refs", func(t *testing.T) {
		t.Parallel()

		inspectFailure := errors.New("forced ownership inspection failure")
		deleteFailure := errors.New("forced owned secret deletion failure")
		tests := []struct {
			name          string
			mutateStore   func(*fakeProviderSecretStore, string)
			wantErrorPart string
			wantRetained  bool
		}{
			{
				name: "Should tolerate a ref already missing from Vault",
				mutateStore: func(store *fakeProviderSecretStore, ref string) {
					delete(store.metadata, ref)
					delete(store.plaintext, ref)
				},
			},
			{
				name: "Should retain a ref that Vault marks absent",
				mutateStore: func(store *fakeProviderSecretStore, ref string) {
					metadata := store.metadata[ref]
					metadata.Present = false
					store.metadata[ref] = metadata
				},
				wantRetained: true,
			},
			{
				name: "Should retain a ref with a non-owned kind",
				mutateStore: func(store *fakeProviderSecretStore, ref string) {
					metadata := store.metadata[ref]
					metadata.Kind = "shared"
					store.metadata[ref] = metadata
				},
				wantRetained: true,
			},
			{
				name: "Should surface an ownership inspection failure",
				mutateStore: func(store *fakeProviderSecretStore, ref string) {
					store.metadataErrors[ref] = inspectFailure
				},
				wantErrorPart: inspectFailure.Error(),
				wantRetained:  true,
			},
			{
				name: "Should surface an owned ref deletion failure",
				mutateStore: func(store *fakeProviderSecretStore, _ string) {
					store.deleteErr = deleteFailure
				},
				wantErrorPart: deleteFailure.Error(),
				wantRetained:  true,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				ctx := context.Background()
				homePaths := testHomePaths(t)
				writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
				secretStore := newFakeProviderSecretStore()
				service := testService(t, homePaths, Dependencies{
					MCPCatalog:      fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
					ProviderSecrets: secretStore,
				})
				result, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
					EntryID: "github",
					Scope:   ScopeGlobal,
					Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
						"GITHUB_TOKEN": {Value: "owned-secret"},
					}},
				})
				if err != nil {
					t.Fatalf("InstallMCPCatalog(ownership proof) error = %v", err)
				}
				ownedRef := "vault:mcp/global/GitHub/env/GITHUB_TOKEN"
				tc.mutateStore(secretStore, ownedRef)

				_, err = service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
					CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
					Name:              result.Item.Name,
				})
				if tc.wantErrorPart == "" && err != nil {
					t.Fatalf("DeleteCollectionItem(ownership proof) error = %v", err)
				}
				if tc.wantErrorPart != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErrorPart)) {
					t.Fatalf(
						"DeleteCollectionItem(ownership proof) error = %v, want containing %q",
						err,
						tc.wantErrorPart,
					)
				}
				_, retained := secretStore.plaintext[ownedRef]
				if retained != tc.wantRetained {
					t.Fatalf("owned ref retained = %t, want %t", retained, tc.wantRetained)
				}
				sidecar := readFile(t, filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName))
				serverRetained := strings.Contains(sidecar, `"GitHub"`)
				wantServerRetained := tc.wantErrorPart != ""
				if serverRetained != wantServerRetained {
					t.Fatalf(
						"MCP server retained after uninstall = %t, want %t:\n%s",
						serverRetained,
						wantServerRetained,
						sidecar,
					)
				}
			})
		}
	})

	t.Run("Should return authorize for OAuth without probing the remote server", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		secretStore := newFakeProviderSecretStore()
		service := testService(t, homePaths, Dependencies{
			MCPCatalog:      fakeMCPCatalog{entry: oauthMCPCatalogEntry()},
			MCPRuntime:      panicMCPRuntimeProvider{},
			ProviderSecrets: secretStore,
		})

		result, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
			EntryID: "linear",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{
				OAuthClientSecret: &MCPSecretInput{Value: "oauth-client-secret"},
			},
		})
		if err != nil {
			t.Fatalf("InstallMCPCatalog(OAuth) error = %v", err)
		}
		if got, want := result.NextStep, MCPCatalogInstallNextStepAuthorize; got != want {
			t.Fatalf("NextStep = %q, want %q", got, want)
		}
		if result.Item.RuntimeStatus != nil {
			t.Fatalf("RuntimeStatus = %#v, want no live probe result", result.Item.RuntimeStatus)
		}
		canonicalRef := "vault:mcp/global/linear/oauth/client-secret"
		if !result.Item.ClientSecretConfigured || result.Item.Auth.ClientSecretRef != "" {
			t.Fatalf(
				"OAuth client secret presence = %t with ref %q, want true without ref",
				result.Item.ClientSecretConfigured,
				result.Item.Auth.ClientSecretRef,
			)
		}
		if got := secretStore.plaintext[canonicalRef]; got != "oauth-client-secret" {
			t.Fatalf("stored OAuth client secret = %q, want typed value", got)
		}
		sidecar := readFile(t, filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName))
		if !strings.Contains(sidecar, canonicalRef) {
			t.Fatalf("OAuth sidecar missing configured client secret ref:\n%s", sidecar)
		}
	})
}

func TestInstallMCPCatalogRejectsInvalidInputsBeforeWrites(t *testing.T) {
	t.Parallel()
	invalidProjection := stdioMCPCatalogEntry()
	invalidProjection.Payload = json.RawMessage(`{`)
	nonMCPProjection := &marketplace.Entry{
		Kind:    marketplace.KindSkill,
		EntryID: "github",
		Name:    "GitHub",
		Payload: json.RawMessage(`{}`),
	}
	blankInstallName := stdioMCPCatalogEntry()
	blankInstallName.Name = ""
	catalogFailure := errors.New("forced catalog detail failure")

	tests := []struct {
		name       string
		catalog    MCPCatalog
		entryID    string
		scope      ScopeKind
		workspace  string
		values     MCPCatalogInstallValues
		wantErr    error
		wantDetail string
	}{
		{
			name:       "Should reject a missing required value",
			catalog:    fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			entryID:    "github",
			scope:      ScopeGlobal,
			wantErr:    ErrValidation,
			wantDetail: "values.env.GITHUB_TOKEN is required",
		},
		{
			name:       "Should reject a missing required plain value",
			catalog:    fakeMCPCatalog{entry: plainEnvMCPCatalogEntry()},
			entryID:    "plain-env",
			scope:      ScopeGlobal,
			wantErr:    ErrValidation,
			wantDetail: "values.env.PROJECT is required",
		},
		{
			name:    "Should reject both value modes",
			catalog: fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			entryID: "github",
			scope:   ScopeGlobal,
			values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"GITHUB_TOKEN": {Value: "secret", VaultRef: "vault:mcp/shared/token"},
			}},
			wantErr:    ErrValidation,
			wantDetail: "exactly one of value or vault_ref",
		},
		{
			name:    "Should reject neither value mode",
			catalog: fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			entryID: "github",
			scope:   ScopeGlobal,
			values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"GITHUB_TOKEN": {},
			}},
			wantErr:    ErrValidation,
			wantDetail: "exactly one of value or vault_ref",
		},
		{
			name:    "Should reject a ref outside the MCP namespace",
			catalog: fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			entryID: "github",
			scope:   ScopeGlobal,
			values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"GITHUB_TOKEN": {VaultRef: "vault:providers/github/token"},
			}},
			wantErr:    ErrValidation,
			wantDetail: "must use vault:mcp/**",
		},
		{
			name:    "Should reject a missing MCP ref",
			catalog: fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			entryID: "github",
			scope:   ScopeGlobal,
			values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"GITHUB_TOKEN": {VaultRef: "vault:mcp/shared/missing"},
			}},
			wantErr:    ErrValidation,
			wantDetail: "vault_ref is not present",
		},
		{
			name:       "Should map an unknown catalog entry to not found",
			catalog:    fakeMCPCatalog{},
			entryID:    "github",
			scope:      ScopeGlobal,
			wantErr:    ErrNotFound,
			wantDetail: "not found",
		},
		{
			name:       "Should preserve an unexpected catalog detail failure",
			catalog:    fakeMCPCatalog{detailErr: catalogFailure},
			entryID:    "github",
			scope:      ScopeGlobal,
			wantErr:    catalogFailure,
			wantDetail: "load MCP catalog entry",
		},
		{
			name:       "Should map a nil catalog detail to not found",
			catalog:    fakeMCPCatalog{returnNil: true},
			entryID:    "github",
			scope:      ScopeGlobal,
			wantErr:    ErrNotFound,
			wantDetail: "not found",
		},
		{
			name:       "Should require an entry ID",
			catalog:    fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			scope:      ScopeGlobal,
			wantErr:    ErrValidation,
			wantDetail: "entry_id is required",
		},
		{
			name:       "Should require an explicit scope",
			catalog:    fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			entryID:    "github",
			wantErr:    ErrValidation,
			wantDetail: "scope is required",
		},
		{
			name:       "Should require a workspace ID for workspace scope",
			catalog:    fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			entryID:    "github",
			scope:      ScopeWorkspace,
			wantErr:    ErrValidation,
			wantDetail: "requires workspace_id",
		},
		{
			name:       "Should reject workspace ID in global scope",
			catalog:    fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			entryID:    "github",
			scope:      ScopeGlobal,
			workspace:  "ws-1",
			wantErr:    ErrConflict,
			wantDetail: "workspace_id requires workspace scope",
		},
		{
			name:       "Should reject agent scope",
			catalog:    fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			entryID:    "github",
			scope:      ScopeAgent,
			wantErr:    ErrValidation,
			wantDetail: "does not support agent scope",
		},
		{
			name:       "Should report an unavailable catalog",
			entryID:    "github",
			scope:      ScopeGlobal,
			wantErr:    ErrUnavailable,
			wantDetail: "catalog is not configured",
		},
		{
			name:       "Should reject an invalid MCP projection",
			catalog:    fakeMCPCatalog{entry: invalidProjection},
			entryID:    "github",
			scope:      ScopeGlobal,
			wantErr:    ErrUnprocessable,
			wantDetail: "project MCP catalog entry",
		},
		{
			name:       "Should reject a catalog row that does not project MCP",
			catalog:    fakeMCPCatalog{entry: nonMCPProjection},
			entryID:    "github",
			scope:      ScopeGlobal,
			wantErr:    ErrUnprocessable,
			wantDetail: "does not project an MCP server",
		},
		{
			name:       "Should require a resolved install name",
			catalog:    fakeMCPCatalog{entry: blankInstallName},
			entryID:    "github",
			scope:      ScopeGlobal,
			wantErr:    ErrValidation,
			wantDetail: "install name is required",
		},
		{
			name:    "Should reject an undeclared field",
			catalog: fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			entryID: "github",
			scope:   ScopeGlobal,
			values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"GITHUB_TOKEN": {Value: "secret"},
				"UNKNOWN":      {Value: "value"},
			}},
			wantErr:    ErrValidation,
			wantDetail: "is not declared",
		},
		{
			name:    "Should reject a Vault ref for a plain field",
			catalog: fakeMCPCatalog{entry: plainEnvMCPCatalogEntry()},
			entryID: "plain-env",
			scope:   ScopeGlobal,
			values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"PROJECT": {VaultRef: "vault:mcp/shared/project"},
			}},
			wantErr:    ErrValidation,
			wantDetail: "must use value for a non-secret field",
		},
		{
			name:    "Should reject an OAuth secret for a non-OAuth entry",
			catalog: fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			entryID: "github",
			scope:   ScopeGlobal,
			values: MCPCatalogInstallValues{
				Env:               map[string]MCPSecretInput{"GITHUB_TOKEN": {Value: "secret"}},
				OAuthClientSecret: &MCPSecretInput{Value: "oauth-secret"},
			},
			wantErr:    ErrValidation,
			wantDetail: "only valid for a catalog entry with OAuth",
		},
		{
			name:    "Should reject ambiguous OAuth secret modes",
			catalog: fakeMCPCatalog{entry: oauthMCPCatalogEntry()},
			entryID: "linear",
			scope:   ScopeGlobal,
			values: MCPCatalogInstallValues{OAuthClientSecret: &MCPSecretInput{
				Value: "secret", VaultRef: "vault:mcp/shared/oauth",
			}},
			wantErr:    ErrValidation,
			wantDetail: "exactly one of value or vault_ref",
		},
		{
			name:    "Should reject an OAuth ref outside the MCP namespace",
			catalog: fakeMCPCatalog{entry: oauthMCPCatalogEntry()},
			entryID: "linear",
			scope:   ScopeGlobal,
			values: MCPCatalogInstallValues{OAuthClientSecret: &MCPSecretInput{
				VaultRef: "vault:providers/linear/client-secret",
			}},
			wantErr:    ErrValidation,
			wantDetail: "must use vault:mcp/**",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			homePaths := testHomePaths(t)
			writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
			secretStore := newFakeProviderSecretStore()
			service := testService(t, homePaths, Dependencies{
				MCPCatalog:      tc.catalog,
				ProviderSecrets: secretStore,
			})
			_, err := service.InstallMCPCatalog(context.Background(), MCPCatalogInstallRequest{
				EntryID:     tc.entryID,
				Scope:       tc.scope,
				WorkspaceID: tc.workspace,
				Values:      tc.values,
			})
			if !errors.Is(err, tc.wantErr) || !strings.Contains(err.Error(), tc.wantDetail) {
				t.Fatalf("InstallMCPCatalog() error = %v, want %v containing %q", err, tc.wantErr, tc.wantDetail)
			}
			if len(secretStore.plaintext) != 0 {
				t.Fatalf("secret writes = %#v, want none", secretStore.plaintext)
			}
			if _, statErr := os.Stat(filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName)); !os.IsNotExist(statErr) {
				t.Fatalf("MCP sidecar exists after validation failure: %v", statErr)
			}
		})
	}
}

func TestInstallMCPCatalogKeepsWorkspaceCanonicalRefsIsolated(t *testing.T) {
	t.Parallel()
	t.Run("Should keep canonical refs isolated across workspace identities", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		firstRoot := filepath.Join(t.TempDir(), "first")
		secondRoot := filepath.Join(t.TempDir(), "second")
		workspaceIDs := []string{"Workspace One", "workspace/two"}
		secretStore := newFakeProviderSecretStore()
		service := testService(t, homePaths, Dependencies{
			MCPCatalog:      fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			ProviderSecrets: secretStore,
			WorkspaceResolver: fakeWorkspaceResolver{resolved: map[string]workspacepkg.ResolvedWorkspace{
				workspaceIDs[0]: {Workspace: workspacepkg.Workspace{ID: workspaceIDs[0], RootDir: firstRoot}},
				workspaceIDs[1]: {Workspace: workspacepkg.Workspace{ID: workspaceIDs[1], RootDir: secondRoot}},
			}},
		})

		refs := make([]string, 0, len(workspaceIDs))
		for _, workspaceID := range workspaceIDs {
			result, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
				EntryID:     "github",
				Name:        "github",
				Scope:       ScopeWorkspace,
				WorkspaceID: workspaceID,
				Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
					"GITHUB_TOKEN": {Value: "secret-for-" + workspaceID},
				}},
			})
			if err != nil {
				t.Fatalf("InstallMCPCatalog(%q) error = %v", workspaceID, err)
			}
			assertMCPSecretKeys(t, result.Item, "GITHUB_TOKEN")
			workspaceSegment, segmentErr := vault.MCPWorkspaceSegment(workspaceID)
			if segmentErr != nil {
				t.Fatalf("MCPWorkspaceSegment(%q) error = %v", workspaceID, segmentErr)
			}
			refs = append(refs, "vault:mcp/ws/"+workspaceSegment+"/github/env/GITHUB_TOKEN")
		}
		if refs[0] == refs[1] {
			t.Fatalf("workspace refs collided: %#v", refs)
		}
		firstSegment, err := vault.MCPWorkspaceSegment(workspaceIDs[0])
		if err != nil {
			t.Fatalf("workspaceVaultSegment(%q) error = %v", workspaceIDs[0], err)
		}
		if got, want := firstSegment, "encoded-576f726b7370616365204f6e65"; got != want {
			t.Fatalf("encoded workspace segment = %q, want %q", got, want)
		}
		reservedSegment, err := vault.MCPWorkspaceSegment(firstSegment)
		if err != nil {
			t.Fatalf("workspaceVaultSegment(reserved prefix) error = %v", err)
		}
		if reservedSegment == firstSegment {
			t.Fatalf("reserved literal workspace segment collided with encoded ID: %q", reservedSegment)
		}
		for index, workspaceID := range workspaceIDs {
			segment, err := vault.MCPWorkspaceSegment(workspaceID)
			if err != nil {
				t.Fatalf("workspaceVaultSegment(%q) error = %v", workspaceID, err)
			}
			want := "vault:mcp/ws/" + segment + "/github/env/GITHUB_TOKEN"
			if refs[index] != want {
				t.Fatalf("workspace ref[%d] = %q, want %q", index, refs[index], want)
			}
		}
		firstSidecar := readFile(t, filepath.Join(firstRoot, aghconfig.DirName, aghconfig.MCPJSONName))
		secondSidecar := readFile(t, filepath.Join(secondRoot, aghconfig.DirName, aghconfig.MCPJSONName))
		if !strings.Contains(firstSidecar, refs[0]) || strings.Contains(firstSidecar, refs[1]) {
			t.Fatalf("first workspace sidecar crossed refs:\n%s", firstSidecar)
		}
		if !strings.Contains(secondSidecar, refs[1]) || strings.Contains(secondSidecar, refs[0]) {
			t.Fatalf("second workspace sidecar crossed refs:\n%s", secondSidecar)
		}
	})
}

func TestInstallMCPCatalogCompensatesPartialFailures(t *testing.T) {
	cleanupFailure := errors.New("forced cleanup failure")
	writeFailure := errors.New("forced MCP definition write failure")
	failingWriter := func(
		aghconfig.HomePaths,
		string,
		string,
		aghconfig.WriteTarget,
		aghconfig.MCPServer,
	) error {
		return writeFailure
	}
	tests := []struct {
		name             string
		deleteErr        error
		wantRefRemaining bool
	}{
		{name: "Should remove the newly created ref"},
		{
			name:             "Should surface cleanup failure with the config write failure",
			deleteErr:        cleanupFailure,
			wantRefRemaining: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			homePaths := testHomePaths(t)
			writeFile(t, homePaths.ConfigFile, strings.Join([]string{
				"[[mcp_servers]]",
				"name = \"GitHub\"",
				"command = \"npx\"",
				"",
			}, "\n"))
			secretStore := newFakeProviderSecretStore()
			secretStore.deleteErr = tc.deleteErr
			service := testService(t, homePaths, Dependencies{
				MCPCatalog:          fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
				ProviderSecrets:     secretStore,
				MCPDefinitionWriter: failingWriter,
			})

			_, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
				EntryID: "github",
				Scope:   ScopeGlobal,
				Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
					"GITHUB_TOKEN": {Value: "orphan-candidate"},
				}},
			})
			if err == nil {
				t.Fatal("InstallMCPCatalog(config write failure) error = nil")
			}
			if !errors.Is(err, writeFailure) {
				t.Fatalf("InstallMCPCatalog() error = %v, want definition write failure", err)
			}
			canonicalRef := "vault:mcp/global/GitHub/env/GITHUB_TOKEN"
			metadata, metadataErr := secretStore.GetMetadata(ctx, canonicalRef)
			if tc.wantRefRemaining {
				if metadataErr != nil || !metadata.Present {
					t.Fatalf("GetMetadata(uncompensated ref) = %#v, %v; want present", metadata, metadataErr)
				}
				if !errors.Is(err, cleanupFailure) {
					t.Fatalf("InstallMCPCatalog() error = %v, want joined cleanup failure", err)
				}
			} else if !errors.Is(metadataErr, vault.ErrSecretNotFound) {
				t.Fatalf("GetMetadata(compensated ref) error = %v, want ErrSecretNotFound", metadataErr)
			}
			if strings.Contains(err.Error(), "orphan-candidate") {
				t.Fatalf("config failure leaked write-only value: %v", err)
			}
		})
	}

	t.Run("Should restore an overwritten ref when the config write fails", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, strings.Join([]string{
			"[[mcp_servers]]",
			"name = \"GitHub\"",
			"command = \"npx\"",
			"",
		}, "\n"))
		secretStore := newFakeProviderSecretStore()
		canonicalRef := "vault:mcp/global/GitHub/env/GITHUB_TOKEN"
		if _, err := secretStore.PutSecret(ctx, canonicalRef, "shared", "original-secret"); err != nil {
			t.Fatalf("PutSecret(original) error = %v", err)
		}
		service := testService(t, homePaths, Dependencies{
			MCPCatalog:          fakeMCPCatalog{entry: stdioMCPCatalogEntry()},
			ProviderSecrets:     secretStore,
			MCPDefinitionWriter: failingWriter,
		})

		_, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
			EntryID: "github",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"GITHUB_TOKEN": {Value: "replacement-secret"},
			}},
		})
		if err == nil {
			t.Fatal("InstallMCPCatalog(overwrite config failure) error = nil")
		}
		if !errors.Is(err, writeFailure) {
			t.Fatalf("InstallMCPCatalog() error = %v, want definition write failure", err)
		}
		if got, want := secretStore.plaintext[canonicalRef], "original-secret"; got != want {
			t.Fatalf("restored secret = %q, want %q", got, want)
		}
		if got, want := secretStore.metadata[canonicalRef].Kind, "shared"; got != want {
			t.Fatalf("restored secret kind = %q, want %q", got, want)
		}
	})

	t.Run("Should restore an overwritten ref when a later Vault write fails", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		secretStore := newFakeProviderSecretStore()
		firstRef := "vault:mcp/global/MultiSecret/env/TOKEN_A"
		secondRef := "vault:mcp/global/MultiSecret/env/TOKEN_B"
		if _, err := secretStore.PutSecret(ctx, firstRef, "shared", "original-secret"); err != nil {
			t.Fatalf("PutSecret(original) error = %v", err)
		}
		storeFailure := errors.New("forced later store failure")
		secretStore.putErrors[secondRef] = storeFailure
		service := testService(t, homePaths, Dependencies{
			MCPCatalog:      fakeMCPCatalog{entry: multiSecretMCPCatalogEntry()},
			ProviderSecrets: secretStore,
		})

		_, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
			EntryID: "multi-secret",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"TOKEN_A": {Value: "replacement-secret"},
				"TOKEN_B": {Value: "second-secret"},
			}},
		})
		if !errors.Is(err, storeFailure) {
			t.Fatalf("InstallMCPCatalog(later store failure) error = %v, want forced failure", err)
		}
		if got, want := secretStore.plaintext[firstRef], "original-secret"; got != want {
			t.Fatalf("restored first secret = %q, want %q", got, want)
		}
		if got, want := secretStore.metadata[firstRef].Kind, "shared"; got != want {
			t.Fatalf("restored first secret kind = %q, want %q", got, want)
		}
		assertMCPSecretRefsAbsent(t, secretStore, secondRef)
	})

	t.Run("Should compensate a prior ref when the next metadata inspection fails", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		secretStore := newFakeProviderSecretStore()
		secondRef := "vault:mcp/global/MultiSecret/env/TOKEN_B"
		metadataFailure := errors.New("forced metadata failure")
		secretStore.metadataErrors[secondRef] = metadataFailure
		service := testService(t, homePaths, Dependencies{
			MCPCatalog:      fakeMCPCatalog{entry: multiSecretMCPCatalogEntry()},
			ProviderSecrets: secretStore,
		})

		_, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
			EntryID: "multi-secret",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"TOKEN_A": {Value: "first-secret"},
				"TOKEN_B": {Value: "second-secret"},
			}},
		})
		if !errors.Is(err, metadataFailure) {
			t.Fatalf("InstallMCPCatalog(metadata failure) error = %v, want forced metadata failure", err)
		}
		assertMCPSecretRefsAbsent(t, secretStore,
			"vault:mcp/global/MultiSecret/env/TOKEN_A",
			secondRef,
		)
		if _, statErr := os.Stat(filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName)); !os.IsNotExist(statErr) {
			t.Fatalf("MCP sidecar exists after metadata failure: %v", statErr)
		}
	})

	t.Run("Should compensate all new refs when the next Vault write fails", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		secretStore := newFakeProviderSecretStore()
		secondRef := "vault:mcp/global/MultiSecret/env/TOKEN_B"
		storeFailure := errors.New("forced store failure")
		secretStore.putErrors[secondRef] = storeFailure
		service := testService(t, homePaths, Dependencies{
			MCPCatalog:      fakeMCPCatalog{entry: multiSecretMCPCatalogEntry()},
			ProviderSecrets: secretStore,
		})

		_, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
			EntryID: "multi-secret",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"TOKEN_A": {Value: "first-secret"},
				"TOKEN_B": {Value: "second-secret"},
			}},
		})
		if !errors.Is(err, storeFailure) {
			t.Fatalf("InstallMCPCatalog(store failure) error = %v, want forced store failure", err)
		}
		assertMCPSecretRefsAbsent(t, secretStore,
			"vault:mcp/global/MultiSecret/env/TOKEN_A",
			secondRef,
		)
		if _, statErr := os.Stat(filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName)); !os.IsNotExist(statErr) {
			t.Fatalf("MCP sidecar exists after store failure: %v", statErr)
		}
	})
}

func assertMCPSecretRefsAbsent(
	t *testing.T,
	secretStore *fakeProviderSecretStore,
	refs ...string,
) {
	t.Helper()
	for _, ref := range refs {
		normalized := vault.NormalizeRef(ref)
		if metadata, ok := secretStore.metadata[normalized]; ok {
			t.Fatalf("metadata[%q] = %#v, want absent", normalized, metadata)
		}
		if plaintext, ok := secretStore.plaintext[normalized]; ok {
			t.Fatalf("plaintext[%q] = %q, want absent", normalized, plaintext)
		}
	}
}

func TestDeleteMCPServerAutoUsesHighestPrecedenceSourceInScope(t *testing.T) {
	t.Parallel()
	t.Run("Should delete the highest-precedence MCP source within each scope", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		workspaceRoot := filepath.Join(t.TempDir(), "workspace")

		writeFile(t, homePaths.ConfigFile, `
[[mcp_servers]]
name = "alpha"
command = "global-config"
`)
		writeFile(t, filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName), `{
  "mcpServers": {
    "alpha": { "command": "global-sidecar" },
    "beta": { "command": "beta-sidecar" }
  }
}`)
		writeFile(t, filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.ConfigName), `
[[mcp_servers]]
name = "alpha"
command = "workspace-config"
`)
		writeFile(t, filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.MCPJSONName), `{
  "mcpServers": {
    "alpha": { "command": "workspace-sidecar" }
  }
}`)

		service := testService(t, homePaths, Dependencies{
			WorkspaceResolver: fakeWorkspaceResolver{
				resolved: map[string]workspacepkg.ResolvedWorkspace{
					"ws-1": {
						Workspace: workspacepkg.Workspace{ID: "ws-1", RootDir: workspaceRoot},
					},
				},
			},
		})

		result, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "alpha",
			Target:            TargetAuto,
		})
		if err != nil {
			t.Fatalf("DeleteCollectionItem(global alpha sidecar) error = %v", err)
		}
		if got, want := result.WriteTarget, WriteTargetGlobalMCPSidecar; got != want {
			t.Fatalf("global alpha first delete target = %q, want %q", got, want)
		}
		sidecarPayload := readFile(t, filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName))
		if strings.Contains(sidecarPayload, `"alpha"`) {
			t.Fatalf("global sidecar alpha still present after delete:\n%s", sidecarPayload)
		}

		result, err = service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "alpha",
			Target:            TargetAuto,
		})
		if err != nil {
			t.Fatalf("DeleteCollectionItem(global alpha config) error = %v", err)
		}
		if got, want := result.WriteTarget, WriteTargetGlobalConfig; got != want {
			t.Fatalf("global alpha second delete target = %q, want %q", got, want)
		}
		configPayload := readFile(t, homePaths.ConfigFile)
		if strings.Contains(configPayload, `name = "alpha"`) {
			t.Fatalf("global config alpha still present after delete:\n%s", configPayload)
		}

		result, err = service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
			CollectionRequest: CollectionRequest{
				Collection:  CollectionMCPServers,
				Scope:       ScopeWorkspace,
				WorkspaceID: "ws-1",
			},
			Name:   "alpha",
			Target: TargetAuto,
		})
		if err != nil {
			t.Fatalf("DeleteCollectionItem(workspace alpha sidecar) error = %v", err)
		}
		if got, want := result.WriteTarget, WriteTargetWorkspaceMCPSidecar; got != want {
			t.Fatalf("workspace alpha first delete target = %q, want %q", got, want)
		}
		workspaceSidecarPayload := readFile(t, filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.MCPJSONName))
		if strings.Contains(workspaceSidecarPayload, `"alpha"`) {
			t.Fatalf("workspace sidecar alpha still present after delete:\n%s", workspaceSidecarPayload)
		}

		result, err = service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
			CollectionRequest: CollectionRequest{
				Collection:  CollectionMCPServers,
				Scope:       ScopeWorkspace,
				WorkspaceID: "ws-1",
			},
			Name:   "alpha",
			Target: TargetAuto,
		})
		if err != nil {
			t.Fatalf("DeleteCollectionItem(workspace alpha config) error = %v", err)
		}
		if got, want := result.WriteTarget, WriteTargetWorkspaceConfig; got != want {
			t.Fatalf("workspace alpha second delete target = %q, want %q", got, want)
		}
		workspaceConfigPayload := readFile(t, filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.ConfigName))
		if strings.Contains(workspaceConfigPayload, `name = "alpha"`) {
			t.Fatalf("workspace config alpha still present after delete:\n%s", workspaceConfigPayload)
		}
	})
}

func TestUpdateSectionRestartRequiredSections(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	memoryHomePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "memory-settings-home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	memoryConfig := aghconfig.DefaultWithHome(memoryHomePaths).Memory
	memoryConfig.GlobalDir = "/tmp/updated-memory"
	memoryConfig.Dream.MinHours = 12
	memoryConfig.Dream.MinSessions = 3
	memoryConfig.Dream.CheckInterval = 15 * time.Minute
	tests := []struct {
		name    string
		request SectionUpdateRequest
		want    string
	}{
		{
			name: "memory",
			request: SectionUpdateRequest{
				SectionRequest: SectionRequest{Section: SectionMemory},
				Memory:         &memoryConfig,
			},
			want: `global_dir = "/tmp/updated-memory"`,
		},
		{
			name: "skills restart required",
			request: SectionUpdateRequest{
				SectionRequest: SectionRequest{Section: SectionSkills},
				Skills: &aghconfig.SkillsConfig{
					Enabled:                 true,
					DisabledSkills:          []string{"alpha", "beta"},
					PollInterval:            45 * time.Minute,
					AllowedMarketplaceMCP:   []string{"ctx"},
					AllowedMarketplaceHooks: []string{"market"},
					Marketplace: aghconfig.MarketplaceConfig{
						Registry: "clawhub",
						BaseURL:  "https://skills-updated.example",
					},
				},
			},
			want: `base_url = "https://skills-updated.example"`,
		},
		{
			name: "automation",
			request: SectionUpdateRequest{
				SectionRequest: SectionRequest{Section: SectionAutomation},
				Automation: &AutomationSettings{
					Enabled:           true,
					Timezone:          "America/Sao_Paulo",
					MaxConcurrentJobs: 5,
					DefaultFireLimit: automationmodel.FireLimitConfig{
						Max:    9,
						Window: "1h",
					},
				},
			},
			want: `max_concurrent_jobs = 5`,
		},
		{
			name: "network",
			request: SectionUpdateRequest{
				SectionRequest: SectionRequest{Section: SectionNetwork},
			},
			want: `max_wakes = 13`,
		},
		{
			name: "observability",
			request: SectionUpdateRequest{
				SectionRequest: SectionRequest{Section: SectionObservability},
				Observability: &aghconfig.ObservabilityConfig{
					Enabled:        true,
					RetentionDays:  21,
					MaxGlobalBytes: 4096,
					Transcripts: aghconfig.ObservabilityTranscriptConfig{
						Enabled:            true,
						SegmentBytes:       1024,
						MaxBytesPerSession: 2048,
					},
				},
			},
			want: `segment_bytes = 1024`,
		},
		{
			name: "hooks extensions",
			request: SectionUpdateRequest{
				SectionRequest: SectionRequest{Section: SectionHooksExtensions},
				HooksExtensions: &aghconfig.ExtensionsConfig{
					Marketplace: aghconfig.ExtensionsMarketplaceConfig{
						Registry:        "github",
						BaseURL:         "https://extensions-updated.example",
						AllowUnverified: true,
					},
					Resources: aghconfig.ExtensionsResourcesConfig{
						AllowedKinds: []resources.ResourceKind{
							resources.ResourceKind("tool"),
							resources.ResourceKind("mcp_server"),
						},
						MaxScope: resources.ResourceScopeKindWorkspace,
						SnapshotRateLimit: aghconfig.ExtensionsResourceRateLimitConfig{
							Requests: 7,
							Window:   time.Minute,
							Queue:    3,
						},
						OperatorWriteRateLimit: aghconfig.ExtensionsResourceRateLimitConfig{
							Requests: 9,
							Window:   2 * time.Minute,
							Queue:    4,
						},
					},
				},
			},
			want: `allow_unverified = true`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			homePaths := testHomePaths(t)
			writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
			service := testService(t, homePaths, Dependencies{
				SkillsRuntime: newFakeSkillsRuntime(testSkill("alpha", false), testSkill("beta", false)),
			})
			request := tt.request
			if request.Section == SectionNetwork {
				current, err := aghconfig.LoadForHome(homePaths)
				if err != nil {
					t.Fatalf("LoadForHome(network fixture) error = %v", err)
				}
				networkConfig := current.Network
				networkConfig.Live.Defaults.MaxWakes = 13
				request.Network = &networkConfig
			}

			result, err := service.UpdateSection(ctx, request)
			if err != nil {
				t.Fatalf("UpdateSection(%s) error = %v", tt.name, err)
			}
			if got, want := result.Behavior, MutationBehaviorRestartRequired; got != want {
				t.Fatalf("behavior = %q, want %q", got, want)
			}
			if !result.RestartRequired {
				t.Fatal("restart_required = false, want true")
			}
			if got, want := result.WriteTarget, WriteTargetGlobalConfig; got != want {
				t.Fatalf("write target = %q, want %q", got, want)
			}
			payload := readFile(t, homePaths.ConfigFile)
			if !strings.Contains(payload, tt.want) {
				t.Fatalf("config payload missing %q:\n%s", tt.want, payload)
			}
			if request.Section == SectionNetwork {
				reloaded, err := aghconfig.LoadForHome(homePaths)
				if err != nil {
					t.Fatalf("LoadForHome(updated network) error = %v", err)
				}
				if got, want := reloaded.Network.Live.Defaults.MaxWakes, 13; got != want {
					t.Fatalf("reloaded Network.Live.Defaults.MaxWakes = %d, want %d", got, want)
				}
			}
		})
	}
}

func TestCollectionHelperMapsIncludeNestedFields(t *testing.T) {
	t.Parallel()

	profileValues := sandboxProfileMap(aghconfig.SandboxProfile{
		Backend:  "daytona",
		SyncMode: "mirror",
		Env:      map[string]string{"TOKEN": "value"},
		Network: aghconfig.NetworkProfile{
			AllowPublicIngress: true,
			AllowOutbound:      true,
			AllowList:          []string{"api.example"},
			DenyList:           []string{"blocked.example"},
			Required:           true,
		},
		Daytona: aghconfig.DaytonaProfile{
			APIURL:      "https://daytona.example",
			Target:      "prod",
			Image:       "agh:latest",
			Snapshot:    "snap-1",
			Class:       "large",
			AutoStop:    "15m",
			AutoArchive: "24h",
		},
	})
	if _, ok := profileValues["env"]; !ok {
		t.Fatalf("sandboxProfileMap() missing env: %#v", profileValues)
	}
	if _, ok := profileValues["network"]; !ok {
		t.Fatalf("sandboxProfileMap() missing network: %#v", profileValues)
	}
	if _, ok := profileValues["daytona"]; !ok {
		t.Fatalf("sandboxProfileMap() missing daytona: %#v", profileValues)
	}

	readOnly := true
	decl := hookspkg.HookDecl{
		Name:         "capture",
		Event:        hookspkg.HookNetworkMessagePersisted,
		Mode:         hookspkg.HookModeAsync,
		ExecutorKind: hookspkg.HookExecutorSubprocess,
		Command:      "/bin/capture",
		Args:         []string{"--json"},
		Env:          map[string]string{"TOKEN": "value"},
		Matcher: hookspkg.HookMatcher{
			ToolID:           "agh__read",
			ToolReadOnly:     &readOnly,
			MessageRole:      "assistant",
			MessageDeltaType: "text",
			NetworkMatcher: &hookspkg.NetworkMatcher{
				Channel:   "builders",
				Surface:   "thread",
				Kind:      "trace",
				Direction: "received",
				WorkState: "completed",
			},
		},
	}
	matcher := hookMatcherMap(decl)
	if got, want := matcher["tool_id"], "agh__read"; got != want {
		t.Fatalf("hookMatcherMap()[tool_id] = %#v, want %q", got, want)
	}
	if got, want := matcher["channel"], "builders"; got != want {
		t.Fatalf("hookMatcherMap()[channel] = %#v, want %q", got, want)
	}
	if got, want := matcher["work_state"], "completed"; got != want {
		t.Fatalf("hookMatcherMap()[work_state] = %#v, want %q", got, want)
	}
	executor := hookExecutorMap(decl)
	if got, want := executor["kind"], string(hookspkg.HookExecutorSubprocess); got != want {
		t.Fatalf("hookExecutorMap()[kind] = %#v, want %q", got, want)
	}
	values := hookDeclarationMap(decl)
	if _, ok := values["executor"]; !ok {
		t.Fatalf("hookDeclarationMap() missing executor: %#v", values)
	}
}

func TestHookDeclarationMapStoresCommandFieldsInExecutorBlock(t *testing.T) {
	t.Parallel()

	values := hookDeclarationMap(hookspkg.HookDecl{
		Name:    "ship",
		Event:   hookspkg.HookToolPreCall,
		Mode:    hookspkg.HookModeAsync,
		Command: "/bin/ship",
		Args:    []string{"--fast"},
		Env:     map[string]string{"TOKEN": "value"},
	})
	executor, ok := values["executor"].(map[string]any)
	if !ok {
		t.Fatalf("hookDeclarationMap() executor = %#v, want map", values["executor"])
	}
	for _, key := range []string{"command", "args", "env"} {
		if _, ok := executor[key]; !ok {
			t.Fatalf("hookDeclarationMap() executor missing %q: %#v", key, executor)
		}
	}
}

func TestHookDeclarationMapSerializesEnabledFlag(t *testing.T) {
	t.Parallel()

	base := hookspkg.HookDecl{
		Name:    "capture",
		Event:   hookspkg.HookToolPreCall,
		Mode:    hookspkg.HookModeAsync,
		Command: "/bin/capture",
	}

	t.Run("Should persist enabled=false so a disabled hook survives reload", func(t *testing.T) {
		t.Parallel()
		disabled := false
		decl := base
		decl.Enabled = &disabled
		got, ok := hookDeclarationMap(decl)["enabled"].(bool)
		if !ok {
			t.Fatal("hookDeclarationMap() dropped enabled=false; a disabled hook would revert to on")
		}
		if got {
			t.Fatal("hookDeclarationMap()[enabled] = true, want false")
		}
	})

	t.Run("Should persist enabled=true", func(t *testing.T) {
		t.Parallel()
		enabled := true
		decl := base
		decl.Enabled = &enabled
		if got, ok := hookDeclarationMap(decl)["enabled"].(bool); !ok || !got {
			t.Fatalf("hookDeclarationMap()[enabled] = %v (ok=%v), want true", got, ok)
		}
	})

	t.Run("Should omit enabled when unset so the default-on state applies", func(t *testing.T) {
		t.Parallel()
		if _, ok := hookDeclarationMap(base)["enabled"]; ok {
			t.Fatal("hookDeclarationMap() wrote enabled for a nil pointer; default-on must stay implicit")
		}
	})
}

func TestUpdateSectionNoChangesReturnsWarning(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := testHomePaths(t)
	writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
	service := testService(t, homePaths, Dependencies{})

	result, err := service.UpdateSection(ctx, SectionUpdateRequest{
		SectionRequest: SectionRequest{Section: SectionGeneral},
		General: &GeneralSettings{
			Defaults: aghconfig.DefaultsConfig{
				Agent:    "writer",
				Provider: "codex",
				Sandbox:  "dev",
			},
			Limits: aghconfig.LimitsConfig{
				MaxConcurrentAgents: 11,
			},
			Permissions:    aghconfig.PermissionsConfig{Mode: aghconfig.PermissionModeApproveReads},
			SessionTimeout: 45 * time.Minute,
			HTTP:           aghconfig.HTTPConfig{Host: "127.0.0.1", Port: 9001},
			Daemon: aghconfig.DaemonConfig{
				Socket:               "/tmp/agh.sock",
				MemoryReportInterval: aghconfig.DefaultDaemonMemoryReportInterval,
			},
			Redact: aghconfig.RedactConfig{Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("UpdateSection(no changes) error = %v", err)
	}
	if got, want := result.Behavior, MutationBehaviorAppliedNow; got != want {
		t.Fatalf("behavior = %q, want %q", got, want)
	}
	if len(result.Warnings) == 0 || result.Warnings[0] != "no changes" {
		t.Fatalf("warnings = %#v, want no changes", result.Warnings)
	}
}

func TestSectionAndCollectionValidationErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := testHomePaths(t)
	writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
	service := testService(t, homePaths, Dependencies{})

	t.Run("Should unknown section", func(t *testing.T) {
		t.Parallel()
		_, err := service.GetSection(ctx, SectionRequest{Section: SectionName("unknown")})
		if err == nil || !strings.Contains(err.Error(), `unknown section "unknown"`) {
			t.Fatalf("GetSection(unknown) error = %v", err)
		}
	})

	t.Run("Should missing section payload", func(t *testing.T) {
		t.Parallel()
		_, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionMemory},
		})
		if err == nil || !strings.Contains(err.Error(), "memory section payload is required") {
			t.Fatalf("UpdateSection(memory nil) error = %v", err)
		}
	})

	t.Run("Should empty collection name", func(t *testing.T) {
		t.Parallel()
		_, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
		})
		if err == nil || !strings.Contains(err.Error(), "collection item name is required") {
			t.Fatalf("PutCollectionItem(empty name) error = %v", err)
		}
	})

	t.Run("Should missing provider payload", func(t *testing.T) {
		t.Parallel()
		_, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "custom",
		})
		if err == nil || !strings.Contains(err.Error(), "provider payload is required") {
			t.Fatalf("PutCollectionItem(provider nil) error = %v", err)
		}
	})

	t.Run("Should missing mcp payload", func(t *testing.T) {
		t.Parallel()
		_, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "alpha",
		})
		if err == nil || !strings.Contains(err.Error(), "MCP server payload is required") {
			t.Fatalf("PutCollectionItem(mcp nil) error = %v", err)
		}
	})

	t.Run("Should unknown collection", func(t *testing.T) {
		t.Parallel()
		_, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionName("unknown")},
			Name:              "alpha",
		})
		if err == nil || !strings.Contains(err.Error(), `unknown collection "unknown"`) {
			t.Fatalf("DeleteCollectionItem(unknown) error = %v", err)
		}
	})

	t.Run("Should delete empty name", func(t *testing.T) {
		t.Parallel()
		_, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
		})
		if err == nil || !strings.Contains(err.Error(), "collection item name is required") {
			t.Fatalf("DeleteCollectionItem(empty name) error = %v", err)
		}
	})

	t.Run("Should update unknown section", func(t *testing.T) {
		t.Parallel()
		_, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionName("mystery")},
		})
		if err == nil || !strings.Contains(err.Error(), `unknown section "mystery"`) {
			t.Fatalf("UpdateSection(unknown) error = %v", err)
		}
	})
}

type fakeGeneralRuntimeProvider struct {
	status DaemonRuntimeStatus
}

func (f fakeGeneralRuntimeProvider) GeneralRuntimeStatus(context.Context) (DaemonRuntimeStatus, error) {
	return f.status, nil
}

type fakeMemoryRuntimeProvider struct {
	status MemoryHealthStatus
}

func (f fakeMemoryRuntimeProvider) MemoryHealthStatus(context.Context) (MemoryHealthStatus, error) {
	return f.status, nil
}

type fakeAutomationRuntimeProvider struct {
	status AutomationRuntimeStatus
}

func (f fakeAutomationRuntimeProvider) AutomationRuntimeStatus(context.Context) (AutomationRuntimeStatus, error) {
	return f.status, nil
}

type fakeNetworkRuntimeProvider struct {
	status NetworkRuntimeStatus
}

func (f fakeNetworkRuntimeProvider) NetworkRuntimeStatus(context.Context) (NetworkRuntimeStatus, error) {
	return f.status, nil
}

type fakeObservabilityRuntimeProvider struct {
	status ObservabilityRuntimeStatus
}

func (f fakeObservabilityRuntimeProvider) ObservabilityRuntimeStatus(
	context.Context,
) (ObservabilityRuntimeStatus, error) {
	return f.status, nil
}

type fakeExtensionStatusProvider struct {
	items []InstalledExtension
}

func (f fakeExtensionStatusProvider) InstalledExtensions(context.Context) ([]InstalledExtension, error) {
	return append([]InstalledExtension(nil), f.items...), nil
}

type fakeTransportParityProvider struct {
	status TransportParityStatus
}

func (f fakeTransportParityProvider) TransportParityStatus(context.Context) (TransportParityStatus, error) {
	return f.status, nil
}

type fakeWorkspaceResolver struct {
	resolved map[string]workspacepkg.ResolvedWorkspace
	listed   []workspacepkg.Workspace
}

func (f fakeWorkspaceResolver) Resolve(
	_ context.Context,
	idOrNameOrPath string,
) (workspacepkg.ResolvedWorkspace, error) {
	if resolved, ok := f.resolved[idOrNameOrPath]; ok {
		return resolved, nil
	}
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (f fakeWorkspaceResolver) List(context.Context) ([]workspacepkg.Workspace, error) {
	return append([]workspacepkg.Workspace(nil), f.listed...), nil
}

type fakeSkillsRuntime struct {
	skills       []*skillspkg.Skill
	enabled      map[string]bool
	agentEnabled map[string]map[string]bool
}

func newFakeSkillsRuntime(skills ...*skillspkg.Skill) *fakeSkillsRuntime {
	enabled := make(map[string]bool, len(skills))
	for _, skill := range skills {
		if skill == nil {
			continue
		}
		enabled[skill.Meta.Name] = skill.Enabled
	}
	return &fakeSkillsRuntime{
		skills:       append([]*skillspkg.Skill(nil), skills...),
		enabled:      enabled,
		agentEnabled: make(map[string]map[string]bool),
	}
}

func (f *fakeSkillsRuntime) List() []*skillspkg.Skill {
	out := make([]*skillspkg.Skill, 0, len(f.skills))
	for _, skill := range f.skills {
		if skill == nil {
			continue
		}
		cloned := *skill
		cloned.Enabled = f.enabled[skill.Meta.Name]
		out = append(out, &cloned)
	}
	return out
}

func (f *fakeSkillsRuntime) SetEnabled(name string, _ *workspacepkg.ResolvedWorkspace, enabled bool) error {
	f.enabled[name] = enabled
	return nil
}

func (f *fakeSkillsRuntime) ForAgent(
	_ context.Context,
	_ *workspacepkg.ResolvedWorkspace,
	agentName string,
) ([]*skillspkg.Skill, error) {
	key := aghconfig.NormalizeAgentName(agentName)
	out := make([]*skillspkg.Skill, 0, len(f.skills))
	for _, skill := range f.skills {
		if skill == nil {
			continue
		}
		cloned := *skill
		cloned.Enabled = f.enabled[skill.Meta.Name]
		if scoped, ok := f.agentEnabled[key]; ok {
			if enabled, ok := scoped[skill.Meta.Name]; ok {
				cloned.Enabled = enabled
			}
		}
		out = append(out, &cloned)
	}
	return out, nil
}

func (f *fakeSkillsRuntime) SetEnabledForAgent(
	name string,
	_ *workspacepkg.ResolvedWorkspace,
	agentName string,
	enabled bool,
) error {
	key := aghconfig.NormalizeAgentName(agentName)
	if _, ok := f.agentEnabled[key]; !ok {
		f.agentEnabled[key] = make(map[string]bool)
	}
	f.agentEnabled[key][name] = enabled
	return nil
}

type fakeProviderSecretStore struct {
	metadata       map[string]vault.Metadata
	plaintext      map[string]string
	metadataErrors map[string]error
	resolveErrors  map[string]error
	putErrors      map[string]error
	deleteErrors   map[string]error
	deleteErr      error
}

type fakeMCPCatalog struct {
	entry     *marketplace.Entry
	detailErr error
	returnNil bool
}

func (f fakeMCPCatalog) Detail(
	ctx context.Context,
	kind marketplace.Kind,
	entryID string,
) (*marketplace.Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.detailErr != nil {
		return nil, f.detailErr
	}
	if f.returnNil {
		return nil, nil
	}
	if f.entry == nil || kind != marketplace.KindMCP || f.entry.EntryID != strings.TrimSpace(entryID) {
		return nil, marketplace.ErrEntryNotFound
	}
	entry := *f.entry
	entry.Payload = append(json.RawMessage(nil), f.entry.Payload...)
	return &entry, nil
}

type panicMCPRuntimeProvider struct{}

func (panicMCPRuntimeProvider) MCPServerRuntimeStatus(
	context.Context,
	mcpauth.Target,
	aghconfig.MCPServer,
) (MCPServerRuntimeStatus, error) {
	panic("MCP runtime probe must not run during catalog install")
}

type recordingMarketplaceInstallNotifier struct {
	mu       sync.Mutex
	outcomes []marketplace.InstallOutcome
	err      error
}

func (n *recordingMarketplaceInstallNotifier) NotifyInstall(
	_ context.Context,
	outcome marketplace.InstallOutcome,
) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.outcomes = append(n.outcomes, outcome)
	return n.err
}

func (n *recordingMarketplaceInstallNotifier) snapshot() []marketplace.InstallOutcome {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]marketplace.InstallOutcome(nil), n.outcomes...)
}

func stdioMCPCatalogEntry() *marketplace.Entry {
	return &marketplace.Entry{
		Kind:    marketplace.KindMCP,
		EntryID: "github",
		Name:    "GitHub",
		Version: "1.2.3",
		Payload: json.RawMessage(`{
			"transport":"stdio",
			"command":"npx",
			"args":["-y","@modelcontextprotocol/server-github"],
			"env":[{"name":"GITHUB_TOKEN","required":true,"secret":true}],
			"default_scope":"global"
		}`),
	}
}

func plainEnvMCPCatalogEntry() *marketplace.Entry {
	return &marketplace.Entry{
		Kind:    marketplace.KindMCP,
		EntryID: "plain-env",
		Name:    "plain-env",
		Version: "1.0.0",
		Payload: json.RawMessage(`{
			"transport":"stdio",
			"command":"plain-env-server",
			"env":[
				{"name":"PROJECT","required":true,"secret":false},
				{"name":"REGION","required":false,"secret":false,"default":"us-east-1"}
			],
			"default_scope":"global"
		}`),
	}
}

func multiSecretMCPCatalogEntry() *marketplace.Entry {
	return &marketplace.Entry{
		Kind:    marketplace.KindMCP,
		EntryID: "multi-secret",
		Name:    "MultiSecret",
		Version: "1.0.0",
		Payload: json.RawMessage(`{
			"transport":"stdio",
			"command":"multi-secret-server",
			"env":[
				{"name":"TOKEN_A","required":true,"secret":true},
				{"name":"TOKEN_B","required":true,"secret":true}
			],
			"default_scope":"global"
		}`),
	}
}

func oauthMCPCatalogEntry() *marketplace.Entry {
	return &marketplace.Entry{
		Kind:    marketplace.KindMCP,
		EntryID: "linear",
		Name:    "linear",
		Version: "2.0.0",
		Payload: json.RawMessage(`{
			"transport":"http",
			"url":"https://mcp.linear.example/mcp",
			"oauth":{
				"authorization_url":"https://auth.linear.example/authorize",
				"token_url":"https://auth.linear.example/token",
				"client_id":"agh-desktop",
				"scopes":["read"]
			},
			"default_scope":"global"
		}`),
	}
}

func newFakeProviderSecretStore() *fakeProviderSecretStore {
	return &fakeProviderSecretStore{
		metadata:       make(map[string]vault.Metadata),
		plaintext:      make(map[string]string),
		metadataErrors: make(map[string]error),
		resolveErrors:  make(map[string]error),
		putErrors:      make(map[string]error),
		deleteErrors:   make(map[string]error),
	}
}

func (f *fakeProviderSecretStore) GetMetadata(ctx context.Context, ref string) (vault.Metadata, error) {
	if err := ctx.Err(); err != nil {
		return vault.Metadata{}, err
	}
	normalized := vault.NormalizeRef(ref)
	if err := f.metadataErrors[normalized]; err != nil {
		return vault.Metadata{}, err
	}
	metadata, ok := f.metadata[normalized]
	if !ok {
		return vault.Metadata{}, vault.ErrSecretNotFound
	}
	return metadata, nil
}

func (f *fakeProviderSecretStore) ResolveRef(ctx context.Context, ref string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	normalized := vault.NormalizeRef(ref)
	if err := f.resolveErrors[normalized]; err != nil {
		return "", err
	}
	metadata, ok := f.metadata[normalized]
	if !ok || !metadata.Present {
		return "", vault.ErrSecretNotFound
	}
	plaintext, ok := f.plaintext[normalized]
	if !ok {
		return "", vault.ErrSecretNotFound
	}
	return plaintext, nil
}

func (f *fakeProviderSecretStore) PutSecret(
	ctx context.Context,
	ref string,
	kind string,
	plaintext string,
) (vault.Metadata, error) {
	if err := ctx.Err(); err != nil {
		return vault.Metadata{}, err
	}
	normalized := vault.NormalizeRef(ref)
	if err := f.putErrors[normalized]; err != nil {
		return vault.Metadata{}, err
	}
	metadata := vault.Metadata{
		Ref:     normalized,
		Kind:    strings.TrimSpace(kind),
		Present: true,
	}
	f.metadata[normalized] = metadata
	f.plaintext[normalized] = plaintext
	return metadata, nil
}

func (f *fakeProviderSecretStore) DeleteSecret(ctx context.Context, ref string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.deleteErr != nil {
		return f.deleteErr
	}
	normalized := vault.NormalizeRef(ref)
	if err := f.deleteErrors[normalized]; err != nil {
		return err
	}
	if _, ok := f.metadata[normalized]; !ok {
		return vault.ErrSecretNotFound
	}
	delete(f.metadata, normalized)
	delete(f.plaintext, normalized)
	return nil
}

type recordingEventSummaryStore struct {
	mu        sync.Mutex
	summaries []store.EventSummary
}

func (r *recordingEventSummaryStore) WriteEventSummary(_ context.Context, summary store.EventSummary) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	summary.Content = append([]byte(nil), summary.Content...)
	r.summaries = append(r.summaries, summary)
	return nil
}

func (r *recordingEventSummaryStore) ListEventSummaries(
	_ context.Context,
	_ store.EventSummaryQuery,
) ([]store.EventSummary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cloned := make([]store.EventSummary, 0, len(r.summaries))
	for _, summary := range r.summaries {
		next := summary
		next.Content = append([]byte(nil), summary.Content...)
		cloned = append(cloned, next)
	}
	return cloned, nil
}

func TestSettingsMutationsEmitEventSummaries(t *testing.T) {
	t.Parallel()

	t.Run("Should emit settings changed for section updates", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())

		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}

		eventStore := &recordingEventSummaryStore{}
		service := testService(t, homePaths, Dependencies{
			EventSummaries: eventStore,
		})

		_, err = service.UpdateSection(WithMutationSource(context.Background(), "http"), SectionUpdateRequest{
			SectionRequest: SectionRequest{
				Section: SectionGeneral,
				Scope:   ScopeGlobal,
			},
			General: &GeneralSettings{
				Defaults: cfg.Defaults,
				Limits: aghconfig.LimitsConfig{
					MaxConcurrentAgents: cfg.Limits.MaxConcurrentAgents + 1,
				},
				Permissions:    cfg.Permissions,
				SessionTimeout: cfg.Session.Limits.Timeout,
				HTTP:           cfg.HTTP,
				Daemon:         cfg.Daemon,
			},
		})
		if err != nil {
			t.Fatalf("UpdateSection() error = %v", err)
		}

		summaries, err := eventStore.ListEventSummaries(context.Background(), store.EventSummaryQuery{})
		if err != nil {
			t.Fatalf("ListEventSummaries() error = %v", err)
		}
		if got, want := len(summaries), 1; got != want {
			t.Fatalf("len(summaries) = %d, want %d", got, want)
		}
		if got, want := summaries[0].Type, "settings.changed"; got != want {
			t.Fatalf("summaries[0].Type = %q, want %q", got, want)
		}

		var content map[string]string
		if err := json.Unmarshal(summaries[0].Content, &content); err != nil {
			t.Fatalf("Unmarshal(content) error = %v", err)
		}
		if got, want := content["section"], string(SectionGeneral); got != want {
			t.Fatalf("content.section = %q, want %q", got, want)
		}
		if got, want := content["source"], "http"; got != want {
			t.Fatalf("content.source = %q, want %q", got, want)
		}
		if got, want := content["operation"], "patch"; got != want {
			t.Fatalf("content.operation = %q, want %q", got, want)
		}
	})
}

func testService(t *testing.T, homePaths aghconfig.HomePaths, deps Dependencies) Service {
	t.Helper()

	if deps.MCPCatalog != nil && deps.ApplyRecords == nil {
		ctx := context.Background()
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(ctx); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
		deps.ApplyRecords = NewConfigApplyRecordRepository(db.DB(), nil)
	}
	if deps.CommandLookPath == nil {
		deps.CommandLookPath = func(string) (string, error) { return "/bin/tool", nil }
	}
	if deps.LookupEnv == nil {
		deps.LookupEnv = func(key string) (string, bool) {
			if key == "OPENAI_API_KEY" {
				return "token", true
			}
			return "", false
		}
	}

	service, err := NewService(homePaths, deps)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

type settingsModelCatalogStub struct {
	mu     sync.Mutex
	models map[string][]modelcatalog.Model
	errs   map[string]error
	opts   []modelcatalog.ListOptions
}

func (s *settingsModelCatalogStub) ListModels(
	_ context.Context,
	opts modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opts = append(s.opts, opts)
	if err := s.errs[opts.ProviderID]; err != nil {
		return nil, err
	}
	return append([]modelcatalog.Model(nil), s.models[opts.ProviderID]...), nil
}

func (s *settingsModelCatalogStub) Refresh(
	context.Context,
	modelcatalog.RefreshOptions,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

func (s *settingsModelCatalogStub) ListSourceStatus(
	context.Context,
	string,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

func (s *settingsModelCatalogStub) sawCuratedView(providerID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, opts := range s.opts {
		if opts.ProviderID == providerID && opts.View == modelcatalog.CatalogViewCurated {
			return true
		}
	}
	return false
}

func requireConfiguredProviderModel(
	t *testing.T,
	models []aghconfig.ProviderModelConfig,
	modelID string,
) aghconfig.ProviderModelConfig {
	t.Helper()
	for _, model := range models {
		if model.ID == modelID {
			return model
		}
	}
	t.Fatalf("configured provider model %q not found in %#v", modelID, models)
	return aghconfig.ProviderModelConfig{}
}

func testHomePaths(t *testing.T) aghconfig.HomePaths {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	return homePaths
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimLeft(contents, "\n")), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return string(payload)
}

func testWindowManagerConfig() aghconfig.WindowManagerConfig {
	return aghconfig.WindowManagerConfig{
		NewWindowPolicy:     aghconfig.WindowNewPolicyBesideFocus,
		SmallViewportPolicy: aghconfig.WindowSmallViewportReject,
		FocusPolicy:         aghconfig.WindowFocusDirectional,
		FocusWrap:           true,
		FocusFollowsPointer: true,
		RaiseOnFocus:        false,
		DragAwayPolicy:      aghconfig.WindowDragAwayGroup,
		GroupMoveModifier:   "control",
		SwapModifier:        "meta",
		HistoryLimit:        77,
		DesktopTransition:   aghconfig.WindowDesktopTransitionCrossfade,
		Gaps: aghconfig.WindowManagerGapsConfig{
			Inner:  12,
			Top:    18,
			Right:  14,
			Bottom: 16,
			Left:   20,
		},
		Snap: aghconfig.WindowManagerSnapConfig{
			EdgeBand:     40,
			CornerReach:  180,
			ExitSlack:    20,
			RepeatRatios: []float64{0.4, 0.7, 0.3},
		},
		Bindings: aghconfig.WindowManagerBindingConfig{
			TopCenter:    "none",
			BottomCenter: "zoom",
		},
		Shortcuts: map[string]string{
			"desktop.switch.next": "Meta+ArrowRight",
			"window.focus.left":   "Alt+ArrowLeft",
		},
	}
}

func baseSettingsConfig() string {
	return `
[defaults]
agent = "writer"
provider = "codex"
sandbox = "dev"

[limits]
max_concurrent_agents = 11

[session.limits]
timeout = "45m"

[permissions]
mode = "approve-reads"

[http]
host = "127.0.0.1"
port = 9001

[daemon]
socket = "/tmp/agh.sock"

[memory]
enabled = true
global_dir = "/tmp/memory"

[memory.dream]
min_hours = 12
min_sessions = 2
check_interval = "15m"

[skills]
enabled = true
disabled_skills = ["alpha", "beta"]
poll_interval = "30m"
allowed_marketplace_mcp = ["ctx"]
allowed_marketplace_hooks = ["market"]

[skills.marketplace]
registry = "clawhub"
base_url = "https://skills.example"

[automation]
enabled = true
timezone = "UTC"
max_concurrent_jobs = 3

[automation.default_fire_limit]
max = 9
window = "1h"

[sandboxes.dev]
backend = "local"

[network]
enabled = true
max_replay_age = 60

[network.live.defaults]
max_wakes = 12
max_wake_wall_time = "5m"
max_total_wall_time = "30m"
max_input_tokens = 200000
max_output_tokens = 50000
max_wake_depth = 3
coalesce_window = "500ms"

[network.live.limits]
max_wakes = 64
max_wake_wall_time = "15m"
max_total_wall_time = "2h"
max_input_tokens = 1000000
max_output_tokens = 200000
max_wake_depth = 5
min_coalesce_window = "100ms"
max_coalesce_window = "5s"

[observability]
enabled = true
retention_days = 14
max_global_bytes = 2048

[observability.transcripts]
enabled = true
segment_bytes = 512
max_bytes_per_session = 1024

[extensions.marketplace]
registry = "github"
base_url = "https://ext.example"

[extensions.resources]
allowed_kinds = ["tool", "mcp_server"]
max_scope = "workspace"

[extensions.resources.snapshot_rate_limit]
requests = 5
window = "1m"
queue = 2

[extensions.resources.operator_write_rate_limit]
requests = 7
window = "2m"
queue = 3

[[hooks.declarations]]
name = "audit"
event = "tool.pre_call"
mode = "sync"
command = "/bin/echo"
`
}

func mustFindProviderItem(t *testing.T, items []ProviderItem, name string) ProviderItem {
	t.Helper()
	for idx := range items {
		item := &items[idx]
		if item.Name == name {
			return *item
		}
	}
	t.Fatalf("Provider item %q not found in %#v", name, items)
	return ProviderItem{}
}

func findSandboxItem(t *testing.T, items []SandboxItem, name string) SandboxItem {
	t.Helper()
	for _, item := range items {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("Sandbox item %q not found in %#v", name, items)
	return SandboxItem{}
}

func findMCPItem(t *testing.T, items []MCPServerItem, name string) MCPServerItem {
	t.Helper()
	for _, item := range items {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("MCP item %q not found in %#v", name, items)
	return MCPServerItem{}
}

func assertMCPSecretKeys(t *testing.T, item MCPServerItem, expected ...string) {
	t.Helper()
	if !reflect.DeepEqual(item.SecretEnvKeys, expected) {
		t.Fatalf("MCP secret env keys = %#v, want %#v", item.SecretEnvKeys, expected)
	}
	if item.Auth.ClientSecretRef != "" {
		t.Fatalf("MCP item exposed OAuth client secret ref %q", item.Auth.ClientSecretRef)
	}
}

func equalWriteTargets(left []WriteTargetKind, right []WriteTargetKind) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func testSkill(name string, enabled bool) *skillspkg.Skill {
	return &skillspkg.Skill{
		Meta:    skillspkg.SkillMeta{Name: name},
		Enabled: enabled,
	}
}
