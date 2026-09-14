package settings

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	automationmodel "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/cmdpalette"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/config/lifecycle"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/resources"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/vault"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestGetSectionBuildsSupportedSections(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := testHomePaths(t)
	writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+"\n[roles.dream]\nenabled = true\n")

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
				if got, want := envelope.General.Settings.Limits.MaxConcurrentAgents, 11; got != want {
					t.Fatalf("General max concurrent agents = %d, want %d", got, want)
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
			name:  SectionPersona,
			label: "Should build the persona section",
			assert: func(t *testing.T, envelope SectionEnvelope) {
				t.Helper()
				if envelope.Persona == nil {
					t.Fatal("Persona section = nil")
				}
				if got, want := envelope.Persona.Config.Agent, "writer"; got != want {
					t.Fatalf("Persona agent = %q, want %q", got, want)
				}
				if !slices.Equal(envelope.AvailableScopes, []ScopeKind{ScopeUser, ScopeProfile, ScopeWorkspace}) {
					t.Fatalf("Persona scopes = %q, want user, profile, and workspace", envelope.AvailableScopes)
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
					compozyconfig.RoleFallback{Provider: "mutated", Model: "mutated"},
				)
				reloaded, err := service.GetSection(ctx, SectionRequest{Section: SectionRoles})
				if err != nil {
					t.Fatalf("GetSection(roles after mutation) error = %v", err)
				}
				if slices.ContainsFunc(reloaded.Roles.Config.AutoTitle.FallbackChain, func(
					fallback compozyconfig.RoleFallback,
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

				if got, want := envelope.Skills.DiscoveredCount, 3; got != want {
					t.Fatalf("Skills discovered count = %d, want %d", got, want)
				}
				if got, want := envelope.Skills.DisabledCount, 2; got != want {
					t.Fatalf("Skills disabled count = %d, want %d", got, want)
				}
				if got, want := envelope.Skills.Links, []OperationalLink{{
					Label: "skills",
					Path:  "/marketplace/skills",
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
			name:  SectionGateway,
			label: "Should build the gateway section",
			assert: func(t *testing.T, envelope SectionEnvelope) {
				t.Helper()
				if envelope.Gateway == nil {
					t.Fatal("Gateway section = nil")
				}
				if envelope.Scope != ScopeUser {
					t.Fatalf("Gateway scope = %q, want %q", envelope.Scope, ScopeUser)
				}
				if envelope.Gateway.Config.Enabled {
					t.Fatal("Gateway enabled = true, want secure disabled default")
				}
				if envelope.Gateway.Config.Pairing.MaxPending != 8 {
					t.Fatalf("Gateway max pending = %d, want 8", envelope.Gateway.Config.Pairing.MaxPending)
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
			name:  SectionCmdPalette,
			label: "Should build the command-palette section",
			assert: func(t *testing.T, envelope SectionEnvelope) {
				t.Helper()
				if envelope.CmdPalette == nil {
					t.Fatal("CmdPalette section = nil")
				}
				if !envelope.CmdPalette.Personalization {
					t.Fatal("CmdPalette personalization = false, want true")
				}
				if !slices.Equal(envelope.AvailableScopes, []ScopeKind{ScopeUser, ScopeProfile, ScopeWorkspace}) {
					t.Fatalf("CmdPalette scopes = %q, want user, profile, and workspace", envelope.AvailableScopes)
				}
			},
		},
		{
			name:  SectionAttention,
			label: "Should build the attention section",
			assert: func(t *testing.T, envelope SectionEnvelope) {
				t.Helper()
				if envelope.Attention == nil {
					t.Fatal("Attention section = nil")
				}
				if !envelope.Attention.Config.Toasts || !envelope.Attention.Config.Sound {
					t.Fatalf("Attention defaults = %#v, want toasts and sound enabled", envelope.Attention.Config)
				}
				if envelope.Attention.Config.System {
					t.Fatal("Attention system = true, want false")
				}
			},
		},
		{
			name:  SectionShell,
			label: "Should build the shell section",
			assert: func(t *testing.T, envelope SectionEnvelope) {
				t.Helper()
				if envelope.Shell == nil {
					t.Fatal("Shell section = nil")
				}
				if got, want := envelope.Shell.Config.Sessions.Sort, compozyconfig.ShellSessionSortLastActivity; got != want {
					t.Fatalf("Shell session sort = %q, want %q", got, want)
				}
				if got, want := envelope.Shell.Config.Sessions.Scope, compozyconfig.ShellSessionScopeWorkspace; got != want {
					t.Fatalf("Shell session scope = %q, want %q", got, want)
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

	t.Run("Should diagnose dead workspace shortcut IDs without failing the section [IT-013 GET]", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		writeFile(
			t,
			filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.ConfigName),
			"[window_manager.shortcuts]\n\"ext.removed.capture\" = [\"meta+alt+KeyQ\"]\n",
		)
		workspaceService := testService(t, homePaths, Dependencies{
			WorkspaceResolver: fakeWorkspaceResolver{resolved: map[string]workspacepkg.ResolvedWorkspace{
				"ws-1": {
					Workspace:   workspacepkg.Workspace{ID: "ws-1", RootDir: workspaceRoot},
					WorkspaceID: "ws-1",
				},
			}},
			CmdPalette: fakeCmdPaletteCatalog{catalog: cmdpalette.Catalog{
				Commands: []cmdpalette.ResolvedCommand{{
					Descriptor: cmdpalette.Descriptor{
						ID: "session.new", Title: "New session", Section: "Sessions",
						Source: cmdpalette.Source{Kind: cmdpalette.SourceKindCore},
					},
					Bindings: []string{"meta+KeyN"},
				}},
			}},
		})

		envelope, err := workspaceService.GetSection(ctx, SectionRequest{
			Section: SectionWindowManager, Scope: ScopeWorkspace, WorkspaceID: "ws-1",
		})
		if err != nil {
			t.Fatalf("GetSection(workspace window-manager) error = %v", err)
		}
		if envelope.WindowManager == nil || len(envelope.WindowManager.Diagnostics) != 1 ||
			envelope.WindowManager.Diagnostics[0].CommandID != "ext.removed.capture" {
			t.Fatalf("window-manager diagnostics = %#v, want dead extension command", envelope.WindowManager)
		}
	})
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
	writeFile(t, filepath.Join(homePaths.HomeDir, compozyconfig.MCPJSONName), `{
  "mcpServers": {
    "alpha": {
      "command": "global-sidecar",
      "catalog_entry": "alpha-catalog",
      "catalog_version": "1.2.3"
    },
    "beta": { "command": "beta-sidecar" }
  }
}`)
	writeFile(t, filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.ConfigName), `
[[mcp_servers]]
name = "alpha"
command = "workspace-config"
`)
	writeFile(t, filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.MCPJSONName), `{
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
		MCPServer: &compozyconfig.MCPServer{
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
		MCPServer: &compozyconfig.MCPServer{
			Command: "beta-command",
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(new beta) error = %v", err)
	}
	if got, want := result.WriteTarget, WriteTargetGlobalMCPSidecar; got != want {
		t.Fatalf("new beta write target = %q, want %q", got, want)
	}
	sidecarPayload := readFile(t, filepath.Join(homePaths.HomeDir, compozyconfig.MCPJSONName))
	if !strings.Contains(sidecarPayload, `"beta"`) || !strings.Contains(sidecarPayload, `"beta-command"`) {
		t.Fatalf("sidecar payload missing beta server:\n%s", sidecarPayload)
	}
}

func TestProfileScopedSettingsShareCanonicalConfigAndSidecarTargets(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve profile-layer provenance across settings surfaces", func(t *testing.T) {
		t.Parallel()
		testProfileScopedSettingsShareCanonicalConfigAndSidecarTargets(t)
	})
}

func testProfileScopedSettingsShareCanonicalConfigAndSidecarTargets(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	homePaths := testHomePaths(t)
	writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
	secretStore := newFakeProviderSecretStore()
	authRuntime := &recordingMCPAuthRuntime{}
	service := testService(t, homePaths, Dependencies{ProviderSecrets: secretStore, MCPAuth: authRuntime})
	profileRequest := SectionRequest{Scope: ScopeProfile, ProfileName: "marketing"}

	persona := compozyconfig.DefaultsConfig{Agent: "editor", Provider: "codex", Sandbox: "dev"}
	personaResult, err := service.UpdateSection(ctx, SectionUpdateRequest{
		SectionRequest: withSettingsSection(profileRequest, SectionPersona),
		Persona:        &persona,
	})
	if err != nil {
		t.Fatalf("UpdateSection(persona profile) error = %v", err)
	}
	if personaResult.WriteTarget != WriteTargetProfileConfig || personaResult.ProfileName != "marketing" {
		t.Fatalf("persona mutation = %#v, want marketing profile config", personaResult)
	}

	aliases := map[string]string{"review": "agent:editor"}
	paletteResult, err := service.UpdateSection(ctx, SectionUpdateRequest{
		SectionRequest: withSettingsSection(profileRequest, SectionCmdPalette),
		CmdPalette: &CmdPaletteUpdate{
			Personalization: new(false),
			Aliases:         &aliases,
		},
	})
	if err != nil {
		t.Fatalf("UpdateSection(cmd palette profile) error = %v", err)
	}
	if paletteResult.WriteTarget != WriteTargetProfileConfig {
		t.Fatalf("cmd palette write target = %q, want profile config", paletteResult.WriteTarget)
	}

	hookResult, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{
			Collection: CollectionHooks, Scope: ScopeProfile, ProfileName: "marketing",
		},
		Name: "ship",
		Hook: &hookspkg.HookDecl{
			Event: hookspkg.HookToolPreCall, Mode: hookspkg.HookModeAsync, Command: "/bin/ship",
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(hook profile) error = %v", err)
	}
	if hookResult.WriteTarget != WriteTargetProfileConfig {
		t.Fatalf("hook write target = %q, want profile config", hookResult.WriteTarget)
	}

	mcpResult, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{
			Collection: CollectionMCPServers, Scope: ScopeProfile, ProfileName: "marketing",
		},
		Name: "linear", Target: TargetAuto,
		MCPServer: &compozyconfig.MCPServer{Command: "linear-mcp"},
		MCPSecrets: MCPSecretValues{SecretEnv: map[string]string{
			"LINEAR_TOKEN": "profile-secret",
		}},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(MCP profile) error = %v", err)
	}
	if mcpResult.WriteTarget != WriteTargetProfileMCPSidecar || mcpResult.ProfileName != "marketing" {
		t.Fatalf("MCP mutation = %#v, want marketing profile sidecar", mcpResult)
	}
	if got := secretStore.plaintext["vault:mcp/profile/marketing/linear/env/LINEAR_TOKEN"]; got != "profile-secret" {
		t.Fatalf("profile MCP secret = %q, want profile-secret", got)
	}
	if len(authRuntime.operations) != 2 || authRuntime.operations[0].target.Scope != mcpauth.ScopeProfile ||
		authRuntime.operations[0].target.WorkspaceID != "marketing" ||
		authRuntime.operations[1].target != authRuntime.operations[0].target {
		t.Fatalf("profile MCP auth lifecycle = %#v, want exact marketing target", authRuntime.operations)
	}

	loaded, err := compozyconfig.LoadForHome(homePaths, compozyconfig.WithProfile("marketing"))
	if err != nil {
		t.Fatalf("LoadForHome(profile) error = %v", err)
	}
	if loaded.Defaults != persona || loaded.CmdPalette.Personalization ||
		loaded.CmdPalette.Aliases["review"] != "agent:editor" {
		t.Fatalf("profile effective config = %#v, want persona and palette overrides", loaded)
	}
	if !slices.ContainsFunc(
		loaded.Hooks.Declarations,
		func(decl hookspkg.HookDecl) bool { return decl.Name == "ship" },
	) {
		t.Fatal("profile effective config is missing the profile hook")
	}
	if !slices.ContainsFunc(
		loaded.MCPServers,
		func(server compozyconfig.MCPServer) bool { return server.Name == "linear" },
	) {
		t.Fatal("profile effective config is missing the profile MCP server")
	}
	if _, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{
			Collection: CollectionSandboxes, Scope: ScopeProfile, ProfileName: "marketing",
		},
		Name: "forbidden", Sandbox: &compozyconfig.SandboxProfile{Backend: "local"},
	}); err == nil || !errors.Is(err, ErrConflict) {
		t.Fatalf("PutCollectionItem(sandbox profile) error = %v, want scope conflict", err)
	}
}

// Invariant: a profile selector is legal only with profile scope, and every
// collection read/write path rejects the mismatch before loading or mutating.
// Owner: settings collection request validation.
// Canonical suite: settings service tests.
func TestCollectionProfileSelectorMustMatchScope(t *testing.T) {
	t.Parallel()
	t.Run("Should reject a profile selector on user collection operations", func(t *testing.T) {
		t.Parallel()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{})
		ctx := context.Background()
		if _, err := service.ListCollection(ctx, CollectionRequest{
			Collection: CollectionHooks, Scope: ScopeUser, ProfileName: "marketing",
		}); err == nil || !errors.Is(err, ErrConflict) {
			t.Fatalf("ListCollection(profile selector in user scope) error = %v, want ErrConflict", err)
		}
		if _, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{
				Collection: CollectionHooks, Scope: ScopeUser, ProfileName: "marketing",
			},
			Name: "ship", Hook: &hookspkg.HookDecl{Event: hookspkg.HookToolPreCall, Command: "/bin/ship"},
		}); err == nil || !errors.Is(err, ErrConflict) {
			t.Fatalf("PutCollectionItem(profile selector in user scope) error = %v, want ErrConflict", err)
		}
		if _, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
			CollectionRequest: CollectionRequest{
				Collection: CollectionHooks, Scope: ScopeUser, ProfileName: "marketing",
			},
			Name: "ship",
		}); err == nil || !errors.Is(err, ErrConflict) {
			t.Fatalf("DeleteCollectionItem(profile selector in user scope) error = %v, want ErrConflict", err)
		}
	})
}

func withSettingsSection(request SectionRequest, section SectionName) SectionRequest {
	request.Section = section
	return request
}

// not parallel: exercises the process catalog-source environment.
// Invariant: unrelated catalog edits do not persist the runtime-only source override.
// Owner: settings section persistence; canonical suite: service_test.go.
func TestUpdateSectionMarketplaceRuntimeSource(t *testing.T) {
	for _, tc := range []struct {
		name        string
		storedURL   string
		explicitURL string
	}{
		{name: "Should preserve an absent source during TTL edits"},
		{name: "Should preserve the configured source during TTL edits", storedURL: "https://catalog.example.test"},
		{name: "Should persist an explicit source edit", storedURL: "https://catalog.example.test", explicitURL: "https://new.example.test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(compozyconfig.MarketplaceCatalogBaseURLEnv, "file:///tmp/runtime-only-catalog")
			homePaths := testHomePaths(t)
			contents := baseSettingsConfig()
			if tc.storedURL != "" {
				contents += "\n[marketplace.catalog]\nbase_url = \"" + tc.storedURL + "\"\n"
			}
			writeFile(t, homePaths.ConfigFile, contents)
			service := testService(t, homePaths, Dependencies{})
			cfg, err := compozyconfig.LoadForHome(homePaths)
			if err != nil {
				t.Fatal(err)
			}
			desired := cfg.Marketplace.Catalog
			desired.TTL = "37m"
			desired.Timeout = "17s"
			if tc.explicitURL != "" {
				desired.BaseURL = tc.explicitURL
			}
			if _, err := service.UpdateSection(t.Context(), SectionUpdateRequest{
				SectionRequest: SectionRequest{Section: SectionMarketplace}, Marketplace: &desired,
			}); err != nil {
				t.Fatal(err)
			}
			t.Setenv(compozyconfig.MarketplaceCatalogBaseURLEnv, "")
			stored, err := compozyconfig.LoadForHome(homePaths)
			if err != nil {
				t.Fatal(err)
			}
			wantURL := tc.storedURL
			if tc.explicitURL != "" {
				wantURL = tc.explicitURL
			}
			if wantURL == "" {
				wantURL = compozyconfig.DefaultMarketplaceCatalogBaseURL
			}
			if stored.Marketplace.Catalog.BaseURL != wantURL || stored.Marketplace.Catalog.TTL != desired.TTL ||
				stored.Marketplace.Catalog.Timeout != desired.Timeout {
				t.Fatalf("persisted catalog = %#v, want source %q with updated TTL and timeout", stored.Marketplace.Catalog, wantURL)
			}
		})
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
			Limits: compozyconfig.LimitsConfig{
				MaxConcurrentAgents: 11,
			},
			Permissions:    compozyconfig.PermissionsConfig{Mode: compozyconfig.PermissionModeApproveReads},
			SessionTimeout: 45 * time.Minute,
			HTTP:           compozyconfig.HTTPConfig{Host: "127.0.0.1", Port: 9001},
			Daemon: compozyconfig.DaemonConfig{
				Socket:               "/tmp/compozy.sock",
				MemoryReportInterval: compozyconfig.DefaultDaemonMemoryReportInterval,
			},
			Redact: compozyconfig.RedactConfig{Enabled: false},
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

// TestUpdateSectionWindowManager verifies scoped persistence, shortcut preservation, and runtime application.
func TestUpdateSectionWindowManager(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve current shortcuts when saving stale behavior with zero gaps", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{})
		stale := testWindowManagerConfig()
		shortcuts := map[string]windowmanager.ShortcutBinding{"window.close": {"meta+shift+KeyW"}}
		globals := map[string]string{windowmanager.DefaultGlobalSummonCommandID: "meta+shift+Space"}
		aliases := map[string]string{"session.new": "start"}
		if _, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest:               SectionRequest{Section: SectionWindowManager},
			WindowManagerShortcuts:       &shortcuts,
			WindowManagerGlobalShortcuts: &globals,
			WindowManagerAliases:         &aliases,
		}); err != nil {
			t.Fatalf("update shortcuts: %v", err)
		}
		stale.Gaps = compozyconfig.WindowManagerGapsConfig{}
		for range 2 {
			if _, err := service.UpdateSection(ctx, SectionUpdateRequest{
				SectionRequest:                 SectionRequest{Section: SectionWindowManager},
				WindowManager:                  &stale,
				WindowManagerPreserveShortcuts: true,
			}); err != nil {
				t.Fatalf("save behavior: %v", err)
			}
		}
		loaded, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("reload config: %v", err)
		}
		if loaded.WindowManager.Gaps != (compozyconfig.WindowManagerGapsConfig{}) {
			t.Fatalf("gaps = %#v, want all zeros", loaded.WindowManager.Gaps)
		}
		if !reflect.DeepEqual(loaded.WindowManager.Shortcuts, shortcuts) ||
			!reflect.DeepEqual(loaded.WindowManager.GlobalShortcuts, globals) ||
			!reflect.DeepEqual(loaded.CmdPalette.Aliases, aliases) {
			t.Fatalf(
				"behavior save replaced shortcut state: %#v, aliases %#v",
				loaded.WindowManager,
				loaded.CmdPalette.Aliases,
			)
		}
	})

	t.Run("Should round-trip the complete validated global config", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{})
		desired := testWindowManagerConfig()
		desired.Gaps.Inner = 0
		desired.GlobalShortcuts = map[string]string{windowmanager.DefaultGlobalSummonCommandID: "meta+shift+Space"}

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

		loaded, err := compozyconfig.LoadForHome(homePaths)
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

		validationError, validationErrorMatched := errors.AsType[compozyconfig.ValidationError](err)
		if !validationErrorMatched {
			t.Fatalf("UpdateSection(invalid window-manager) error = %T %v, want config.ValidationError", err, err)
		}
		if got, want := validationError.Path, "window_manager.history_limit"; got != want {
			t.Fatalf("UpdateSection(invalid window-manager) validation path = %q, want %q", got, want)
		}
		if after := readFile(t, homePaths.ConfigFile); after != before {
			t.Fatalf("config changed after validation failure\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})

	t.Run("Should name the shortcut owner and transfer the chord only with overwrite", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{})
		shortcuts := map[string]windowmanager.ShortcutBinding{
			"palette.open": {"meta+KeyN"},
		}
		request := SectionUpdateRequest{
			SectionRequest:         SectionRequest{Section: SectionWindowManager},
			WindowManagerShortcuts: &shortcuts,
		}

		_, err := service.UpdateSection(ctx, request)
		var conflict *windowmanager.ShortcutConflictError
		if !errors.As(err, &conflict) {
			t.Fatalf("UpdateSection(conflict) error = %T %v, want ShortcutConflictError", err, err)
		}
		if conflict.Owner != "session.new" || conflict.Chord != "meta+KeyN" {
			t.Fatalf("shortcut conflict = %#v, want session.new owner of meta+KeyN", conflict)
		}

		request.Overwrite = true
		if _, err := service.UpdateSection(ctx, request); err != nil {
			t.Fatalf("UpdateSection(overwrite) error = %v", err)
		}
		loaded, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(overwrite) error = %v", err)
		}
		if got := loaded.WindowManager.Shortcuts["session.new"]; len(got) != 0 {
			t.Fatalf("session.new shortcuts = %q, want unbound", got)
		}
		got := loaded.WindowManager.Shortcuts["palette.open"]
		want := windowmanager.ShortcutBinding{"meta+KeyN"}
		if !slices.Equal(got, want) {
			t.Fatalf("palette.open shortcuts = %q, want %q", got, want)
		}
	})

	t.Run("Should transfer a desktop-global chord only through an atomic overwrite", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{})
		desired := map[string]string{
			windowmanager.DefaultGlobalSummonCommandID: windowmanager.DefaultGlobalSummonChord,
			"session.new": windowmanager.DefaultGlobalSummonChord,
		}
		request := SectionUpdateRequest{
			SectionRequest:               SectionRequest{Section: SectionWindowManager},
			WindowManagerGlobalShortcuts: &desired,
		}

		_, err := service.UpdateSection(ctx, request)
		var conflict *windowmanager.ShortcutConflictError
		if !errors.As(err, &conflict) {
			t.Fatalf("UpdateSection(global conflict) error = %T %v, want ShortcutConflictError", err, err)
		}
		if conflict.Owner != windowmanager.DefaultGlobalSummonCommandID ||
			conflict.Chord != windowmanager.DefaultGlobalSummonChord {
			t.Fatalf("global shortcut conflict = %#v", conflict)
		}
		beforeOverwrite, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(before overwrite) error = %v", err)
		}
		if beforeOverwrite.WindowManager.GlobalShortcuts[windowmanager.DefaultGlobalSummonCommandID] !=
			windowmanager.DefaultGlobalSummonChord {
			t.Fatalf("failed mutation changed global shortcuts = %#v", beforeOverwrite.WindowManager.GlobalShortcuts)
		}

		request.Overwrite = true
		if _, err := service.UpdateSection(ctx, request); err != nil {
			t.Fatalf("UpdateSection(global overwrite) error = %v", err)
		}
		loaded, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(global overwrite) error = %v", err)
		}
		if _, exists := loaded.WindowManager.GlobalShortcuts[windowmanager.DefaultGlobalSummonCommandID]; exists {
			t.Fatal("global summon still owns chord after overwrite")
		}
		if loaded.WindowManager.GlobalShortcuts["session.new"] != windowmanager.DefaultGlobalSummonChord {
			t.Fatalf("global shortcuts = %#v, want session.new owner", loaded.WindowManager.GlobalShortcuts)
		}
	})

	t.Run("Should validate aliases and transfer ownership atomically", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{})
		initialAliases := map[string]string{"session.new": "new", "palette.open": "open"}
		if _, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest:       SectionRequest{Section: SectionWindowManager},
			WindowManagerAliases: &initialAliases,
		}); err != nil {
			t.Fatalf("UpdateSection(initial aliases) error = %v", err)
		}

		conflictingAliases := map[string]string{"session.new": "new", "palette.open": "new"}
		request := SectionUpdateRequest{
			SectionRequest:       SectionRequest{Section: SectionWindowManager},
			WindowManagerAliases: &conflictingAliases,
		}
		_, err := service.UpdateSection(ctx, request)
		var conflict *AliasConflictError
		if !errors.As(err, &conflict) {
			t.Fatalf("UpdateSection(alias conflict) error = %T %v, want AliasConflictError", err, err)
		}
		if conflict.Owner != "session.new" || conflict.Alias != "new" {
			t.Fatalf("alias conflict = %#v, want session.new owner of new", conflict)
		}

		request.Overwrite = true
		if _, err := service.UpdateSection(ctx, request); err != nil {
			t.Fatalf("UpdateSection(alias overwrite) error = %v", err)
		}
		loaded, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(alias overwrite) error = %v", err)
		}
		if _, exists := loaded.CmdPalette.Aliases["session.new"]; exists {
			t.Fatal("session.new alias still exists after overwrite")
		}
		if got, want := loaded.CmdPalette.Aliases["palette.open"], "new"; got != want {
			t.Fatalf("palette.open alias = %q, want %q", got, want)
		}

		invalidAliases := map[string]string{"palette.open": "my alias"}
		_, err = service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest:       SectionRequest{Section: SectionWindowManager},
			WindowManagerAliases: &invalidAliases,
		})
		if _, ok := errors.AsType[*InvalidAliasError](err); !ok {
			t.Fatalf("UpdateSection(invalid alias) error = %T %v, want InvalidAliasError", err, err)
		}
	})

	t.Run("Should accept a workspace extension ID from the command catalog", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		workspaceRoot := t.TempDir()
		paletteEvents := &recordingCmdPaletteCatalog{catalog: cmdpalette.Catalog{Commands: []cmdpalette.ResolvedCommand{
			{Descriptor: cmdpalette.Descriptor{
				ID: "session.new", Source: cmdpalette.Source{Kind: cmdpalette.SourceKindCore},
			}},
			{Descriptor: cmdpalette.Descriptor{
				ID: "ext.notes.capture",
				Source: cmdpalette.Source{
					Kind: cmdpalette.SourceKindExtension, Extension: "notes",
				},
			}},
		}}}
		service := testService(t, homePaths, Dependencies{
			WorkspaceResolver: fakeWorkspaceResolver{resolved: map[string]workspacepkg.ResolvedWorkspace{
				"ws-1": {Workspace: workspacepkg.Workspace{ID: "ws-1", RootDir: workspaceRoot}},
			}},
			CmdPalette: paletteEvents,
		})
		shortcuts := map[string]windowmanager.ShortcutBinding{
			"ext.notes.capture": {"alt+shift+KeyN"},
		}
		aliases := map[string]string{"ext.notes.capture": "cap"}

		result, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest: SectionRequest{
				Section: SectionWindowManager, Scope: ScopeWorkspace, WorkspaceID: "ws-1",
			},
			WindowManagerShortcuts: &shortcuts,
			WindowManagerAliases:   &aliases,
		})
		if err != nil {
			t.Fatalf("UpdateSection(extension shortcut) error = %v", err)
		}
		if result.Scope != ScopeWorkspace || result.WriteTarget != WriteTargetWorkspaceConfig {
			t.Fatalf("extension shortcut result = %#v, want workspace config", result)
		}
		loaded, err := compozyconfig.LoadForHome(homePaths, compozyconfig.WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			t.Fatalf("LoadForHome(workspace shortcut) error = %v", err)
		}
		got := loaded.WindowManager.Shortcuts["ext.notes.capture"]
		want := windowmanager.ShortcutBinding{"alt+shift+KeyN"}
		if !slices.Equal(got, want) {
			t.Fatalf("extension shortcuts = %q, want %q", got, want)
		}
		if got, want := loaded.CmdPalette.Aliases["ext.notes.capture"], "cap"; got != want {
			t.Fatalf("extension alias = %q, want %q", got, want)
		}
		requests := paletteEvents.recordedRequests()
		if len(requests) != 1 {
			t.Fatalf("workspace binding catalog requests = %#v, want one", requests)
		}
		if got, want := requests[0].ProfileLens, cmdpalette.ScopedProfileLens(
			cmdpalette.DefaultProfileLensID,
			"default",
		); got != want ||
			requests[0].WorkspaceID != "ws-1" ||
			requests[0].ClientID != "" {
			t.Fatalf("workspace binding catalog request = %#v, want default profile, ws-1, blank client", requests[0])
		}
		events := paletteEvents.recorded()
		if len(events) != 2 || events[0].Name != cmdpalette.EventBindingChanged ||
			events[1].Name != cmdpalette.EventAliasChanged {
			t.Fatalf("command palette settings events = %#v, want binding then alias", events)
		}
		for index, event := range events {
			if event.ProfileLens != cmdpalette.AggregateProfileLens() {
				t.Fatalf("settings event[%d] profile lens = %#v, want aggregate", index, event.ProfileLens)
			}
		}
		for index, event := range events {
			if event.WorkspaceID != "ws-1" || event.CommandID != "ext.notes.capture" {
				t.Fatalf("settings event[%d] correlation = %#v", index, event)
			}
		}
	})

	t.Run("Should emit global binding changes once per registered workspace [IT-033]", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		paletteEvents := &recordingCmdPaletteCatalog{}
		service := testService(t, homePaths, Dependencies{
			WorkspaceResolver: fakeWorkspaceResolver{listed: []workspacepkg.Workspace{
				{ID: "ws-b"}, {ID: "ws-a"}, {ID: "ws-b"},
			}},
			CmdPalette: paletteEvents,
		})
		shortcuts := map[string]windowmanager.ShortcutBinding{
			"palette.open": {"meta+alt+KeyP"},
		}
		if _, err := service.UpdateSection(t.Context(), SectionUpdateRequest{
			SectionRequest:         SectionRequest{Section: SectionWindowManager},
			WindowManagerShortcuts: &shortcuts,
		}); err != nil {
			t.Fatalf("UpdateSection(global shortcut) error = %v", err)
		}
		events := paletteEvents.recorded()
		if len(events) != 2 {
			t.Fatalf("global binding events = %#v, want one per unique workspace", events)
		}
		requests := paletteEvents.recordedRequests()
		if len(requests) != 0 {
			t.Fatalf("global binding catalog requests = %#v, want none", requests)
		}
		if events[0].WorkspaceID != "ws-a" || events[1].WorkspaceID != "ws-b" {
			t.Fatalf("global binding workspaces = %#v, want sorted ws-a / ws-b", events)
		}
		for index, event := range events {
			if event.Name != cmdpalette.EventBindingChanged || event.CommandID != "palette.open" ||
				event.ProfileLens != cmdpalette.AggregateProfileLens() {
				t.Fatalf("global binding event[%d] = %#v", index, event)
			}
		}
	})
}

func TestUpdateSectionCmdPalette(t *testing.T) {
	t.Parallel()

	t.Run("Should persist both command palette controls as live scalar mutations [IT-015]", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{})

		result, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionCmdPalette},
			CmdPalette: &CmdPaletteUpdate{
				FallbackAgentEnabled: new(false),
				Personalization:      new(false),
			},
		})
		if err != nil {
			t.Fatalf("UpdateSection(cmd-palette) error = %v", err)
		}
		if result.Lifecycle != lifecycle.Live || !result.Applied || result.RestartRequired {
			t.Fatalf("UpdateSection(cmd-palette) = %#v, want live applied mutation", result)
		}
		loaded, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(cmd-palette) error = %v", err)
		}
		if loaded.CmdPalette.Personalization {
			t.Fatal("CmdPalette personalization = true, want false")
		}
		if len(loaded.CmdPalette.FallbackTargets) != 0 {
			t.Fatalf("CmdPalette fallback targets = %q, want disabled", loaded.CmdPalette.FallbackTargets)
		}
	})

	t.Run("Should preserve an omitted command palette control", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{})

		result, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionCmdPalette},
			CmdPalette:     &CmdPaletteUpdate{FallbackAgentEnabled: new(false)},
		})
		if err != nil {
			t.Fatalf("UpdateSection(cmd-palette fallback) error = %v", err)
		}
		if result.Lifecycle != lifecycle.Live || !result.Applied || result.RestartRequired {
			t.Fatalf("UpdateSection(cmd-palette fallback) = %#v, want live applied mutation", result)
		}
		loaded, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(cmd-palette fallback) error = %v", err)
		}
		if len(loaded.CmdPalette.FallbackTargets) != 0 || !loaded.CmdPalette.Personalization {
			t.Fatalf("CmdPalette = %#v, want fallback off and personalization preserved", loaded.CmdPalette)
		}
	})
}

func TestUpdateSectionAttention(t *testing.T) {
	t.Parallel()

	t.Run("Should round-trip the complete validated global config", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{})
		desired := AttentionSettings{
			Toasts:          false,
			Sound:           false,
			System:          true,
			MutedWorkspaces: []string{"ws_0123456789abcdef"},
		}

		result, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest:                 SectionRequest{Section: SectionAttention},
			Attention:                      &desired,
			ReplaceAttentionWorkspaceMutes: true,
		})
		if err != nil {
			t.Fatalf("UpdateSection(attention) error = %v", err)
		}
		if got, want := result.Behavior, MutationBehaviorAppliedNow; got != want {
			t.Fatalf("attention behavior = %q, want %q", got, want)
		}
		if !result.Applied || result.RestartRequired {
			t.Fatalf("attention result = %#v, want live applied mutation", result)
		}
		if got, want := result.WriteTarget, WriteTargetGlobalConfig; got != want {
			t.Fatalf("attention write target = %q, want %q", got, want)
		}

		loaded, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(updated attention) error = %v", err)
		}
		wantConfig := compozyconfig.AttentionConfig{Toasts: false, Sound: false, System: true}
		if !reflect.DeepEqual(loaded.Attention, wantConfig) {
			t.Fatalf("loaded Attention = %#v, want %#v", loaded.Attention, wantConfig)
		}
		envelope, err := service.GetSection(ctx, SectionRequest{Section: SectionAttention})
		if err != nil {
			t.Fatalf("GetSection(attention) error = %v", err)
		}
		if envelope.Attention == nil || !reflect.DeepEqual(envelope.Attention.Config, desired) {
			t.Fatalf("attention section = %#v, want %#v", envelope.Attention, desired)
		}
	})

	t.Run("Should leave config unchanged when validation fails", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		before := readFile(t, homePaths.ConfigFile)
		service := testService(t, homePaths, Dependencies{})
		invalid := AttentionSettings{MutedWorkspaces: []string{"not-a-workspace"}}

		_, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest:                 SectionRequest{Section: SectionAttention},
			Attention:                      &invalid,
			ReplaceAttentionWorkspaceMutes: true,
		})
		if err == nil {
			t.Fatal("UpdateSection(invalid attention) error = nil, want validation error")
		}

		validationError, matched := errors.AsType[compozyconfig.ValidationError](err)
		if !matched {
			t.Fatalf("UpdateSection(invalid attention) error = %T %v, want config.ValidationError", err, err)
		}
		if got, want := validationError.Path, "muted_workspaces"; got != want {
			t.Fatalf("UpdateSection(invalid attention) validation path = %q, want %q", got, want)
		}
		if after := readFile(t, homePaths.ConfigFile); after != before {
			t.Fatalf("config changed after validation failure\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})

	t.Run("Should restore delivery config when mute persistence fails", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		mutes := &settingsTestAttentionMuteStore{
			byProfile:  map[string][]string{store.DefaultProfileID: {"ws_0123456789abcdef"}},
			replaceErr: errors.New("mute store unavailable"),
		}
		service := testService(t, homePaths, Dependencies{AttentionWorkspaceMutes: mutes})
		desired := AttentionSettings{
			Toasts:          false,
			Sound:           false,
			System:          true,
			MutedWorkspaces: []string{"ws_abcdef0123456789"},
		}

		_, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest:                 SectionRequest{Section: SectionAttention},
			Attention:                      &desired,
			ReplaceAttentionWorkspaceMutes: true,
		})
		if err == nil || !strings.Contains(err.Error(), "mute store unavailable") {
			t.Fatalf("UpdateSection(attention) error = %v, want mute persistence failure", err)
		}

		loaded, loadErr := compozyconfig.LoadForHome(homePaths)
		if loadErr != nil {
			t.Fatalf("LoadForHome(after rollback) error = %v", loadErr)
		}
		wantConfig := compozyconfig.AttentionConfig{Toasts: true, Sound: true, System: false}
		if !reflect.DeepEqual(loaded.Attention, wantConfig) {
			t.Fatalf("loaded Attention = %#v, want restored %#v", loaded.Attention, wantConfig)
		}
		stored, listErr := mutes.ListAttentionWorkspaceMutes(ctx, store.DefaultProfileID)
		if listErr != nil {
			t.Fatalf("ListAttentionWorkspaceMutes(after failure) error = %v", listErr)
		}
		if !reflect.DeepEqual(stored, []string{"ws_0123456789abcdef"}) {
			t.Fatalf("stored mutes = %#v, want original mute", stored)
		}
	})
}

func TestUpdateSectionShell(t *testing.T) {
	t.Parallel()

	t.Run("Should round-trip the complete validated global shell config", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		service := testService(t, homePaths, Dependencies{})
		desired := compozyconfig.ShellConfig{Sessions: compozyconfig.ShellSessionsConfig{
			Sort:  compozyconfig.ShellSessionSortAttention,
			Scope: compozyconfig.ShellSessionScopeAllWorkspaces,
		}}

		result, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionShell},
			Shell:          &desired,
		})
		if err != nil {
			t.Fatalf("UpdateSection(shell) error = %v", err)
		}
		if result.Behavior != MutationBehaviorAppliedNow || !result.Applied || result.RestartRequired {
			t.Fatalf("shell result = %#v, want live applied mutation", result)
		}

		loaded, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(updated shell) error = %v", err)
		}
		if !reflect.DeepEqual(loaded.Shell, desired) {
			t.Fatalf("loaded Shell = %#v, want %#v", loaded.Shell, desired)
		}
	})

	t.Run("Should leave config unchanged when a shell preference is invalid", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		before := readFile(t, homePaths.ConfigFile)
		service := testService(t, homePaths, Dependencies{})
		invalid := compozyconfig.ShellConfig{Sessions: compozyconfig.ShellSessionsConfig{
			Sort: "priority", Scope: compozyconfig.ShellSessionScopeWorkspace,
		}}

		_, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionShell},
			Shell:          &invalid,
		})
		validationErr, matched := errors.AsType[compozyconfig.ValidationError](err)
		if !matched {
			t.Fatalf("UpdateSection(invalid shell) error = %T %v, want config.ValidationError", err, err)
		}
		if got, want := validationErr.Path, "shell.sessions.sort"; got != want {
			t.Fatalf("shell validation path = %q, want %q", got, want)
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
		Skills: &compozyconfig.SkillsConfig{
			Enabled:                 true,
			DisabledSkills:          []string{"beta"},
			PollInterval:            30 * time.Minute,
			AllowedMarketplaceHooks: []string{"market"},
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
			Skills: &compozyconfig.SkillsConfig{
				Enabled:                 true,
				DisabledSkills:          []string{"beta"},
				PollInterval:            30 * time.Minute,
				AllowedMarketplaceHooks: []string{"market"},
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

func TestUpdateSectionSkillSourceScopesAndConcurrentWrites(t *testing.T) {
	t.Parallel()

	t.Run("Should reject invalid source policy at the settings domain boundary", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		before := readFile(t, homePaths.ConfigFile)
		service := testService(t, homePaths, Dependencies{})
		loaded, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		loaded.Skills.Sources = []string{"agnets"}

		_, err = service.UpdateSection(context.Background(), SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionSkills, Scope: ScopeUser},
			Skills:         &loaded.Skills,
		})
		var validation *compozyconfig.SkillSourceValidationError
		if !errors.Is(err, ErrValidation) || !errors.As(err, &validation) ||
			validation.Code != "unknown_skill_source" || validation.Suggestion != "agents" {
			t.Fatalf("UpdateSection(invalid sources) error = %#v, want portable source validation", err)
		}
		if after := readFile(t, homePaths.ConfigFile); after != before {
			t.Fatalf("config changed after invalid source update\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})

	t.Run("Should reserve source policy at agent scope while leaving disabled skills mutable", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		agentPath := filepath.Join(homePaths.AgentsDir, "coder", compozyconfig.AgentDefinitionFileName)
		writeFile(t, agentPath, `---
name: coder
provider: codex
skills:
  disabled:
    - alpha
---

Test agent.
`)
		runtime := newFakeSkillsRuntime(testSkill("alpha", false), testSkill("beta", false))
		service := testService(t, homePaths, Dependencies{SkillsRuntime: runtime})
		loaded, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		sourceChange := loaded.Skills
		sourceChange.Sources = []string{"claude"}
		_, err = service.UpdateSection(context.Background(), SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionSkills, Scope: ScopeAgent, AgentName: "coder"},
			Skills:         &sourceChange,
		})
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "only supports skills.disabled_skills") ||
			!strings.Contains(err.Error(), "skills.sources") {
			t.Fatalf("agent source update error = %v, want scope-policy rejection", err)
		}

		disabledChange := loaded.Skills
		disabledChange.DisabledSkills = []string{"beta"}
		result, err := service.UpdateSection(context.Background(), SectionUpdateRequest{
			SectionRequest: SectionRequest{Section: SectionSkills, Scope: ScopeAgent, AgentName: "coder"},
			Skills:         &disabledChange,
		})
		if err != nil {
			t.Fatalf("agent disabled_skills update error = %v", err)
		}
		if result.Scope != ScopeAgent || !result.Applied || runtime.enabled["beta"] {
			t.Fatalf("agent disabled_skills result = %#v, runtime=%#v", result, runtime.enabled)
		}
	})

	t.Run(
		"Should serialize user and workspace source writes without losing either overlay [IT-011]",
		func(t *testing.T) {
			homePaths := testHomePaths(t)
			writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
			workspaceRoot := t.TempDir()
			resolver := fakeWorkspaceResolver{resolved: map[string]workspacepkg.ResolvedWorkspace{
				"ws-alpha": {
					Workspace:   workspacepkg.Workspace{ID: "ws-alpha", RootDir: workspaceRoot},
					WorkspaceID: "ws-alpha",
				},
			}}
			service := testService(t, homePaths, Dependencies{WorkspaceResolver: resolver})
			loaded, err := compozyconfig.LoadForHome(homePaths)
			if err != nil {
				t.Fatalf("LoadForHome() error = %v", err)
			}
			userSkills := loaded.Skills
			userSkills.Sources = []string{"claude"}

			requests := []SectionUpdateRequest{
				{
					SectionRequest: SectionRequest{Section: SectionSkills, Scope: ScopeUser},
					Skills:         &userSkills,
				},
				{
					SectionRequest: SectionRequest{
						Section:     SectionSkills,
						Scope:       ScopeWorkspace,
						WorkspaceID: "ws-alpha",
					},
					SkillSourcesOverride: &SkillSourcesOverride{
						Sources: OptionalStringList{Present: true, Value: []string{"agents"}},
					},
				},
			}
			var wait sync.WaitGroup
			errorsByRequest := make([]error, len(requests))
			for index := range requests {
				wait.Add(1)
				go func(requestIndex int) {
					defer wait.Done()
					_, errorsByRequest[requestIndex] = service.UpdateSection(
						context.Background(),
						requests[requestIndex],
					)
				}(index)
			}
			wait.Wait()
			for index, updateErr := range errorsByRequest {
				if updateErr != nil {
					t.Fatalf("UpdateSection(request %d) error = %v", index, updateErr)
				}
			}

			global, err := compozyconfig.LoadForHome(homePaths)
			if err != nil {
				t.Fatalf("LoadForHome(global after writes) error = %v", err)
			}
			if !slices.Equal(global.Skills.Sources, []string{"claude"}) {
				t.Fatalf("global sources = %q, want claude", global.Skills.Sources)
			}
			workspace, err := compozyconfig.LoadForHome(homePaths, compozyconfig.WithWorkspaceRoot(workspaceRoot))
			if err != nil {
				t.Fatalf("LoadForHome(workspace after writes) error = %v", err)
			}
			if !slices.Equal(workspace.Skills.Sources, []string{"agents"}) {
				t.Fatalf("workspace sources = %q, want override agents", workspace.Skills.Sources)
			}
			if !slices.Equal(workspace.Skills.CustomSources, global.Skills.CustomSources) {
				t.Fatalf(
					"workspace custom_sources = %q, want inherited %q",
					workspace.Skills.CustomSources,
					global.Skills.CustomSources,
				)
			}
		},
	)
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
	workspaceRoot := filepath.Join(t.TempDir(), "hook-workspace")
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
	writeFile(t, filepath.Join(homePaths.ProfilesDir, "marketing", compozyconfig.ConfigName), `
[[hooks.declarations]]
name = "ship"
event = "session.post_create"
command = "/bin/profile-ship"
`)
	writeFile(t, filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.ConfigName), `
[[hooks.declarations]]
name = "ship"
event = "session.post_create"
command = "/bin/workspace-ship"
`)
	writeFile(t, filepath.Join(
		workspaceRoot,
		compozyconfig.DirName,
		compozyconfig.ProfilesDirName,
		"marketing",
		compozyconfig.ConfigName,
	), `
[[hooks.declarations]]
name = "ship"
event = "session.post_create"
command = "/bin/workspace-profile-ship"
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
			resolved: map[string]workspacepkg.ResolvedWorkspace{
				"ws-hooks": {
					Workspace: workspacepkg.Workspace{ID: "ws-hooks", RootDir: workspaceRoot},
				},
			},
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

	profileHooks, err := service.ListCollection(ctx, CollectionRequest{
		Collection: CollectionHooks,
		Scope:      ScopeProfile, WorkspaceID: "ws-hooks", ProfileName: "marketing",
	})
	if err != nil {
		t.Fatalf("ListCollection(profile hooks) error = %v", err)
	}
	profileShip := findHookItem(t, profileHooks.Hooks, "ship")
	if got, want := profileShip.SourceMetadata.EffectiveSource.Kind, SourceKindWorkspaceProfileConfig; got != want {
		t.Fatalf("profile hook effective source = %q, want %q", got, want)
	}
	if got, want := profileShip.Declaration.Command, "/bin/workspace-profile-ship"; got != want {
		t.Fatalf("profile hook command = %q, want %q", got, want)
	}
	if got, want := len(profileShip.SourceMetadata.ShadowedSources), 3; got != want {
		t.Fatalf("profile hook shadowed sources = %d, want %d", got, want)
	}
	if got, want := profileShip.SourceMetadata.AvailableTargets, []WriteTargetKind{
		WriteTargetGlobalConfig,
		WriteTargetProfileConfig,
		WriteTargetWorkspaceConfig,
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("profile hook available targets = %#v, want %#v", got, want)
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
			Models: compozyconfig.ProviderModelsConfig{
				Default: "custom-model",
				Curated: []compozyconfig.ProviderModelConfig{
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
			CredentialSlots: []compozyconfig.ProviderCredentialSlot{
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
			Models: compozyconfig.ProviderModelsConfig{
				Curated: []compozyconfig.ProviderModelConfig{},
			},
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(explicit empty curated) error = %v", err)
	}
	if got, want := emptyCuratedResult.WriteTarget, WriteTargetGlobalConfig; got != want {
		t.Fatalf("empty curated write target = %q, want %q", got, want)
	}
	loadedConfig, err := compozyconfig.LoadForHome(homePaths)
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
			Models: compozyconfig.ProviderModelsConfig{
				Curated: []compozyconfig.ProviderModelConfig{
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
	loadedConfig, err = compozyconfig.LoadForHome(homePaths)
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
			Models: compozyconfig.ProviderModelsConfig{
				Curated: []compozyconfig.ProviderModelConfig{
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
	loadedConfig, err = compozyconfig.LoadForHome(homePaths)
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
		Sandbox: &compozyconfig.SandboxProfile{
			Backend:     "local",
			SyncMode:    "session-bidirectional",
			Persistence: "transient",
			RuntimeRoot: "/tmp/staging",
			Env: map[string]string{
				"QA_VISIBLE": "yes",
			},
			Network: compozyconfig.NetworkProfile{
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
		Sandbox: &compozyconfig.SandboxProfile{
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

	t.Run("Should read the persisted catalog without starting an implicit refresh", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		catalog := &settingsModelCatalogStub{}
		service := testService(t, homePaths, Dependencies{ModelCatalog: catalog})

		if _, err := service.ListCollection(context.Background(), CollectionRequest{
			Collection: CollectionProviders,
		}); err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}

		catalog.mu.Lock()
		defer catalog.mu.Unlock()
		if len(catalog.opts) == 0 {
			t.Fatal("model catalog reads = 0, want one read per provider")
		}
		for _, opts := range catalog.opts {
			if !opts.SkipRefreshIfEmpty {
				t.Fatalf("model catalog read for provider %q allows implicit refresh", opts.ProviderID)
			}
		}
	})

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
		if got, want := custom.Settings.Models.Reasoning.Apply, compozyconfig.ReasoningApplyACPOption; got != want {
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
		cfg, err := compozyconfig.LoadForHome(homePaths)
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
		cfg, err = compozyconfig.LoadForHome(homePaths)
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

	t.Run("Should accept a default effort advertised by a launch binding", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		xhigh := modelcatalog.ReasoningEffortXHigh
		catalog := &settingsModelCatalogStub{models: map[string][]modelcatalog.Model{
			"cursor": {
				{
					ProviderID:  "cursor",
					ModelID:     "grok-4.6",
					DisplayName: "Cursor Grok 4.6",
					TransportBindings: []modelcatalog.ModelTransportBinding{
						{TransportModelID: "cursor-grok-4.6-extra-high", ReasoningEffort: &xhigh},
					},
					Curated: true,
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
		cursor := mustFindProviderItem(t, envelope.Providers, "cursor")
		settings := cursor.Settings
		settings.ModelsSet = true
		settings.Models.Default = "grok-4.6"

		if _, err := service.PutCollectionItem(context.Background(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "cursor",
			Provider:          &settings,
			ProviderModelCuration: &ProviderModelCurationRequest{
				ModelID:                "grok-4.6",
				DefaultReasoningEffort: &xhigh,
			},
		}); err != nil {
			t.Fatalf("PutCollectionItem(Cursor xhigh default) error = %v", err)
		}

		cfg, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after Cursor default write) error = %v", err)
		}
		curated := requireConfiguredProviderModel(t, cfg.Providers["cursor"].Models.Curated, "grok-4.6")
		if got, want := curated.DefaultReasoningEffort, string(modelcatalog.ReasoningEffortXHigh); got != want {
			t.Fatalf("Cursor default reasoning effort = %q, want %q", got, want)
		}
		if curated.ReasoningEfforts != nil {
			t.Fatalf("Cursor config froze catalog reasoning efforts: %#v", curated.ReasoningEfforts)
		}
		if contents := readFile(t, homePaths.ConfigFile); strings.Contains(contents, "cursor-grok-4.6-extra-high") {
			t.Fatalf("Cursor private transport binding leaked into config:\n%s", contents)
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
		beforeConfig, err := compozyconfig.LoadForHome(homePaths)
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

		afterConfig, err := compozyconfig.LoadForHome(homePaths)
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

		cfg, err := compozyconfig.LoadForHome(homePaths)
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
			compozyconfig.ProviderModelConfig{ID: "excluded"},
		)
		if _, err := service.PutCollectionItem(context.Background(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "custom",
			Provider:          &settings,
		}); err != nil {
			t.Fatalf("PutCollectionItem(add excluded model) error = %v", err)
		}

		cfg, err := compozyconfig.LoadForHome(homePaths)
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
				Models: compozyconfig.ProviderModelsConfig{
					Curated: []compozyconfig.ProviderModelConfig{{ID: "missing-model"}},
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
			Models: compozyconfig.ProviderModelsConfig{
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
		cfg, err := compozyconfig.LoadForHome(homePaths)
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
			Models: compozyconfig.ProviderModelsConfig{
				Default: "raw-model",
				Curated: []compozyconfig.ProviderModelConfig{},
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
			Models: compozyconfig.ProviderModelsConfig{
				Default: "raw-model",
				Curated: []compozyconfig.ProviderModelConfig{},
			},
		}
		before := readFile(t, homePaths.ConfigFile)
		_, err := service.PutCollectionItem(context.Background(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "custom",
			Provider:          &settings,
		})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("PutCollectionItem(explicit clear without catalog) error = %v, want ErrValidation", err)
		}
		if errors.Is(err, modelcatalog.ErrAllSourcesFailed) {
			t.Fatalf("PutCollectionItem(explicit clear without catalog) error = %v, want no source failure", err)
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

	t.Run("Should persist steer overrides and preserve them when an older client omits the field", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		home := testHomePaths(t)
		writeFile(t, home.ConfigFile, baseSettingsConfig())
		service := testService(t, home, Dependencies{})
		put := func(settings ProviderSettings) {
			t.Helper()
			if _, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
				CollectionRequest: CollectionRequest{
					Collection: CollectionProviders,
				},
				Name:     "codex",
				Provider: &settings,
			}); err != nil {
				t.Fatal(err)
			}
		}
		put(ProviderSettings{SteerCapability: compozyconfig.SteerCapabilityConcurrentPrompt})
		put(ProviderSettings{DisplayName: "Updated without capability"})
		cfg, err := compozyconfig.LoadForHome(home)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Providers["codex"].SteerCapability != compozyconfig.SteerCapabilityConcurrentPrompt {
			t.Fatalf("override lost: %#v", cfg.Providers["codex"])
		}
		put(ProviderSettings{SteerCapability: compozyconfig.SteerCapabilityNone})
		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionProviders})
		if err != nil {
			t.Fatal(err)
		}
		if got := mustFindProviderItem(
			t,
			envelope.Providers,
			"codex",
		).Settings.SteerCapability; got != compozyconfig.SteerCapabilityNone {
			t.Fatalf("read capability=%q", got)
		}
	})

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
				Harness:         compozyconfig.ProviderHarnessACP,
				RuntimeProvider: "codex",
				AuthMode:        compozyconfig.ProviderAuthModeNativeCLI,
				EnvPolicy:       compozyconfig.ProviderEnvPolicyFiltered,
				HomePolicy:      compozyconfig.ProviderHomePolicyOperator,
				AuthLoginCmd:    "codex login",
				Models: compozyconfig.ProviderModelsConfig{
					Default: "gpt-5.5",
					Curated: []compozyconfig.ProviderModelConfig{
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

	t.Run("Should preserve the write-only login command when a read projection is written back", func(t *testing.T) {
		t.Parallel()

		const rawLoginCommand = "codex login --tenant corp --token raw-login-secret"
		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+`

[providers.codex]
display_name = "Codex Enterprise"
auth_login_command = "codex login --tenant corp --token raw-login-secret"
`)
		service := testService(t, homePaths, Dependencies{})
		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionProviders})
		if err != nil {
			t.Fatalf("ListCollection(providers) error = %v", err)
		}
		codex := mustFindProviderItem(t, envelope.Providers, "codex")
		if codex.Settings.AuthLoginCmd != "" || codex.Settings.AuthLoginCmdSet {
			t.Fatalf("public provider settings retained write-only login command: %#v", codex.Settings)
		}

		settings := codex.Settings
		settings.DisplayName = "Codex Enterprise Updated"
		if _, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionProviders},
			Name:              "codex",
			Provider:          &settings,
		}); err != nil {
			t.Fatalf("PutCollectionItem(read projection) error = %v", err)
		}

		cfg, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after read projection write) error = %v", err)
		}
		if got := cfg.Providers["codex"].AuthLoginCmd; got != rawLoginCommand {
			t.Fatalf("stored write-only login command = %q, want preserved value", got)
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
					AuthMode: compozyconfig.ProviderAuthModeNone,
					CredentialSlots: []compozyconfig.ProviderCredentialSlot{{
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
			MCPServer: &compozyconfig.MCPServer{
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
		if got, want := secretStore.plaintext["vault:mcp/user/github/env/GITHUB_TOKEN"], "ghp-secret"; got != want {
			t.Fatalf("stored MCP secret_env = %q, want %q", got, want)
		}
		sidecarPayload := readFile(t, filepath.Join(homePaths.HomeDir, compozyconfig.MCPJSONName))
		if !strings.Contains(sidecarPayload, "vault:mcp/user/github/env/GITHUB_TOKEN") {
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
			MCPServer: &compozyconfig.MCPServer{
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
		if got, want := secretStore.plaintext["vault:mcp/user/github/env/GITHUB_TOKEN"], "ghp-secret"; got != want {
			t.Fatalf("preserved MCP secret plaintext = %q, want %q", got, want)
		}
		if _, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "github",
		}); err != nil {
			t.Fatalf("DeleteCollectionItem(non-catalog MCP) error = %v", err)
		}
		deletedRef := "vault:mcp/user/github/env/GITHUB_TOKEN"
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
				MCPServer:         &compozyconfig.MCPServer{Command: "old-command"},
				MCPSecrets: MCPSecretValues{
					SecretEnv: map[string]string{"GITHUB_TOKEN": "old-secret"},
				},
			}
			if _, err := initialService.PutCollectionItem(t.Context(), request); err != nil {
				t.Fatalf("PutCollectionItem(initial) error = %v", err)
			}

			invalidateErr := errors.New("auth invalidation unavailable")
			runtime := &recordingMCPAuthRuntime{deleteStateErrs: map[int]error{1: invalidateErr}}
			service := testService(t, homePaths, Dependencies{
				MCPAuth:         runtime,
				ProviderSecrets: secretStore,
			})
			request.MCPServer = &compozyconfig.MCPServer{Command: "new-command"}
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
			ref := "vault:mcp/user/github/env/GITHUB_TOKEN"
			if got := secretStore.plaintext[ref]; got != "old-secret" {
				t.Fatalf("restored secret = %q, want old-secret", got)
			}
			if got, want := len(runtime.operations), 3; got != want {
				t.Fatalf(
					"auth lifecycle calls = %d, want invalidate, failed state deletion, and rollback invalidate",
					got,
				)
			}
			wantKinds := []string{
				recordingMCPAuthInvalidate,
				recordingMCPAuthDeleteState,
				recordingMCPAuthInvalidate,
			}
			for index, operation := range runtime.operations {
				if operation.kind != wantKinds[index] {
					t.Fatalf("auth lifecycle[%d] = %#v, want kind %q", index, operation, wantKinds[index])
				}
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
				MCPServer:         &compozyconfig.MCPServer{Command: "old-command"},
				MCPSecrets: MCPSecretValues{
					SecretEnv: map[string]string{"GITHUB_TOKEN": "old-secret"},
				},
			}
			if _, err := initialService.PutCollectionItem(t.Context(), request); err != nil {
				t.Fatalf("PutCollectionItem(initial) error = %v", err)
			}

			invalidateErr := errors.New("auth invalidation unavailable")
			rollbackErr := errors.New("definition rollback unavailable")
			runtime := &recordingMCPAuthRuntime{deleteStateErrs: map[int]error{1: invalidateErr}}
			writeCalls := 0
			writer := func(
				home compozyconfig.HomePaths,
				root string,
				name string,
				target compozyconfig.WriteTarget,
				server compozyconfig.MCPServer,
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
			request.MCPServer = &compozyconfig.MCPServer{Command: "new-command"}
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
			ref := "vault:mcp/user/github/env/GITHUB_TOKEN"
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
		canonicalRef := "vault:mcp/user/shared/env/TOKEN"
		if _, err := service.PutCollectionItem(t.Context(), CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "shared",
			Target:            TargetConfig,
			MCPServer:         &compozyconfig.MCPServer{Command: "config-command"},
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
			MCPServer: &compozyconfig.MCPServer{
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
			MCPServer: &compozyconfig.MCPServer{
				Command: "old-command",
				Env:     map[string]string{"PROJECT": "compozy"},
			},
		}); err != nil {
			t.Fatalf("PutCollectionItem(initial plain env) error = %v", err)
		}

		if _, err := service.PutCollectionItem(t.Context(), CollectionItemPutRequest{
			CollectionRequest:  CollectionRequest{Collection: CollectionMCPServers},
			Name:               "plain-env",
			Target:             TargetSidecar,
			MCPServer:          &compozyconfig.MCPServer{Command: "new-command"},
			MCPEnvPreservation: []string{"PROJECT"},
		}); err != nil {
			t.Fatalf("PutCollectionItem(preserve plain env) error = %v", err)
		}
		servers, err := compozyconfig.LoadMCPServersJSONFile(
			filepath.Join(homePaths.HomeDir, compozyconfig.MCPJSONName),
		)
		if err != nil {
			t.Fatalf("LoadMCPServersJSONFile() error = %v", err)
		}
		var persisted compozyconfig.MCPServer
		for _, candidate := range servers {
			if candidate.Name == "plain-env" {
				persisted = candidate
				break
			}
		}
		if got, want := persisted.Env["PROJECT"], "compozy"; got != want {
			t.Fatalf("preserved PROJECT = %q, want %q", got, want)
		}

		_, err = service.PutCollectionItem(t.Context(), CollectionItemPutRequest{
			CollectionRequest:  CollectionRequest{Collection: CollectionMCPServers},
			Name:               "plain-env",
			Target:             TargetConfig,
			MCPServer:          &compozyconfig.MCPServer{Command: "config-command"},
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
			MCPServer: &compozyconfig.MCPServer{
				Transport: compozyconfig.MCPServerTransportHTTP,
				URL:       "https://mcp.linear.app/mcp",
				Auth: compozyconfig.MCPAuthConfig{
					Registration: compozyconfig.MCPAuthRegistrationPreRegistered,
					IssuerURL:    "https://linear.app",
					ClientID:     "compozy-client",
				},
			},
			MCPSecrets: MCPSecretValues{OAuthClientSecret: &clientSecret},
		})
		if err != nil {
			t.Fatalf("PutCollectionItem(MCP OAuth secret) error = %v", err)
		}
		if got, want := secretStore.plaintext["vault:mcp/user/linear/oauth/client-secret"], clientSecret; got != want {
			t.Fatalf("stored MCP OAuth secret = %q, want %q", got, want)
		}
		sidecarPayload := readFile(t, filepath.Join(homePaths.HomeDir, compozyconfig.MCPJSONName))
		if !strings.Contains(sidecarPayload, "vault:mcp/user/linear/oauth/client-secret") {
			t.Fatalf("sidecar payload missing client secret ref:\n%s", sidecarPayload)
		}
		if strings.Contains(sidecarPayload, clientSecret) {
			t.Fatalf("sidecar payload leaked OAuth client secret:\n%s", sidecarPayload)
		}
		if _, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "linear",
			Target:            TargetAuto,
			MCPServer: &compozyconfig.MCPServer{
				Transport: compozyconfig.MCPServerTransportHTTP,
				URL:       "https://mcp.linear.app/mcp-v2",
				Auth: compozyconfig.MCPAuthConfig{
					Registration: compozyconfig.MCPAuthRegistrationPreRegistered,
					IssuerURL:    "https://linear.app",
					ClientID:     "compozy-client",
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
		if got, want := item.URL, "https://mcp.linear.app/mcp-v2"; got != want {
			t.Fatalf("preserved MCP URL = %q, want %q", got, want)
		}
		if !item.ClientSecretConfigured || item.Auth.ClientSecretRef != "" {
			t.Fatalf(
				"preserved MCP OAuth presence = %t with ref %q, want true without ref",
				item.ClientSecretConfigured,
				item.Auth.ClientSecretRef,
			)
		}
		if got := secretStore.plaintext["vault:mcp/user/linear/oauth/client-secret"]; got != clientSecret {
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
			MCPServer:         &compozyconfig.MCPServer{Command: "npx"},
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
		wantAuth := compozyconfig.MCPAuthConfig{
			Registration: compozyconfig.MCPAuthRegistrationPreRegistered,
			IssuerURL:    "https://auth.example.com",
			ClientID:     "compozy-client",
			Scopes:       []string{"read", "write"},
		}

		result, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			CollectionRequest: CollectionRequest{Collection: CollectionMCPServers},
			Name:              "full-oauth",
			Target:            TargetConfig,
			MCPServer: &compozyconfig.MCPServer{
				Transport: compozyconfig.MCPServerTransportHTTP,
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

		canonicalRef := "vault:mcp/user/full-oauth/oauth/client-secret"
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
			MCPServer: &compozyconfig.MCPServer{
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
		canonicalRef := "vault:mcp/user/github/env/GITHUB_TOKEN"
		if got, want := secretStore.plaintext[canonicalRef], "ghp-secret"; got != want {
			t.Fatalf("stored MCP secret_env = %q, want %q", got, want)
		}
		sidecarPayload := readFile(t, filepath.Join(homePaths.HomeDir, compozyconfig.MCPJSONName))
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
			MCPServer: &compozyconfig.MCPServer{
				Command: "npx",
				SecretEnv: map[string]string{
					"GITHUB_TOKEN": "vault:mcp/github/env/GITHUB_TOKEN",
				},
				Auth: compozyconfig.MCPAuthConfig{
					Registration:    compozyconfig.MCPAuthRegistrationPreRegistered,
					IssuerURL:       "https://example.com",
					ClientID:        "compozy-client",
					ClientSecretRef: "vault:mcp/github/oauth/client-secret",
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
		sidecarPath := filepath.Join(homePaths.HomeDir, compozyconfig.MCPJSONName)
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
			server        compozyconfig.MCPServer
			secrets       MCPSecretValues
			withoutStore  bool
			wantErrorPart string
		}{
			{
				name:          "Should require the Vault service",
				serverName:    "github",
				server:        compozyconfig.MCPServer{Command: "npx"},
				secrets:       MCPSecretValues{SecretEnv: map[string]string{"TOKEN": "secret"}},
				withoutStore:  true,
				wantErrorPart: "secret store is not available",
			},
			{
				name:       "Should reject secret env for remote transport",
				serverName: "remote",
				server: compozyconfig.MCPServer{
					Transport: compozyconfig.MCPServerTransportHTTP,
					URL:       "https://mcp.example.com",
				},
				secrets:       MCPSecretValues{SecretEnv: map[string]string{"TOKEN": "secret"}},
				wantErrorPart: "secret_env values require stdio transport",
			},
			{
				name:          "Should reject an invalid secret env key",
				serverName:    "github",
				server:        compozyconfig.MCPServer{Command: "npx"},
				secrets:       MCPSecretValues{SecretEnv: map[string]string{"NOT-VALID": "secret"}},
				wantErrorPart: "secret_env key",
			},
			{
				name:          "Should reject an empty secret env value",
				serverName:    "github",
				server:        compozyconfig.MCPServer{Command: "npx"},
				secrets:       MCPSecretValues{SecretEnv: map[string]string{"TOKEN": ""}},
				wantErrorPart: "secret_env value",
			},
			{
				name:          "Should reject an invalid canonical owner",
				serverName:    "github/unsafe",
				server:        compozyconfig.MCPServer{Command: "npx"},
				secrets:       MCPSecretValues{SecretEnv: map[string]string{"TOKEN": "secret"}},
				wantErrorPart: "server name cannot contain a slash",
			},
			{
				name:       "Should reject an empty OAuth client secret",
				serverName: "oauth",
				server: compozyconfig.MCPServer{
					Transport: compozyconfig.MCPServerTransportHTTP,
					URL:       "https://mcp.example.com",
					Auth: compozyconfig.MCPAuthConfig{
						Registration: compozyconfig.MCPAuthRegistrationPreRegistered,
						IssuerURL:    "https://auth.example.com",
						ClientID:     "compozy-client",
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
				sidecarPath := filepath.Join(homePaths.HomeDir, compozyconfig.MCPJSONName)
				if _, statErr := os.Stat(sidecarPath); !os.IsNotExist(statErr) {
					t.Fatalf("MCP sidecar exists after rejected secret shape: %v", statErr)
				}
			})
		}
	})
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
		writeFile(t, filepath.Join(homePaths.HomeDir, compozyconfig.MCPJSONName), `{
  "mcpServers": {
    "alpha": { "command": "global-sidecar" },
    "beta": { "command": "beta-sidecar" }
  }
}`)
		writeFile(t, filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.ConfigName), `
[[mcp_servers]]
name = "alpha"
command = "workspace-config"
`)
		writeFile(t, filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.MCPJSONName), `{
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
		sidecarPayload := readFile(t, filepath.Join(homePaths.HomeDir, compozyconfig.MCPJSONName))
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
		workspaceSidecarPayload := readFile(
			t,
			filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.MCPJSONName),
		)
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
		workspaceConfigPayload := readFile(
			t,
			filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.ConfigName),
		)
		if strings.Contains(workspaceConfigPayload, `name = "alpha"`) {
			t.Fatalf("workspace config alpha still present after delete:\n%s", workspaceConfigPayload)
		}
	})
}

func TestUpdateSectionRestartRequiredSections(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	memoryHomePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "memory-settings-home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	memoryConfig := compozyconfig.DefaultWithHome(memoryHomePaths).Memory
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
				Skills: &compozyconfig.SkillsConfig{
					Enabled:                 true,
					DisabledSkills:          []string{"alpha", "beta"},
					PollInterval:            45 * time.Minute,
					AllowedMarketplaceHooks: []string{"market"},
				},
			},
			want: `poll_interval = "45m0s"`,
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
				Observability: &compozyconfig.ObservabilityConfig{
					Enabled:        true,
					RetentionDays:  21,
					MaxGlobalBytes: 4096,
					Transcripts: compozyconfig.ObservabilityTranscriptConfig{
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
				HooksExtensions: &compozyconfig.ExtensionsConfig{
					Trust: compozyconfig.ExtensionsTrustConfig{AllowUnverified: true},
					Sources: compozyconfig.ExtensionsSourcesConfig{
						GitHub: compozyconfig.ExtensionsGitHubSourceConfig{
							Enabled: true,
							BaseURL: "https://extensions-updated.example",
						},
						Git: compozyconfig.ExtensionsGitSourceConfig{Enabled: true},
					},
					Dev: compozyconfig.ExtensionsDevConfig{WatchInterval: 2 * time.Second},
					Resources: compozyconfig.ExtensionsResourcesConfig{
						AllowedKinds: []resources.ResourceKind{
							resources.ResourceKind("tool"),
							resources.ResourceKind("mcp_server"),
						},
						MaxScope: resources.ResourceScopeKindWorkspace,
						SnapshotRateLimit: compozyconfig.ExtensionsResourceRateLimitConfig{
							Requests: 7,
							Window:   time.Minute,
							Queue:    3,
						},
						OperatorWriteRateLimit: compozyconfig.ExtensionsResourceRateLimitConfig{
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
				current, err := compozyconfig.LoadForHome(homePaths)
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
				reloaded, err := compozyconfig.LoadForHome(homePaths)
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

	profileValues := sandboxProfileMap(compozyconfig.SandboxProfile{
		Backend:  "daytona",
		SyncMode: "mirror",
		Env:      map[string]string{"TOKEN": "value"},
		Network: compozyconfig.NetworkProfile{
			AllowPublicIngress: true,
			AllowOutbound:      true,
			AllowList:          []string{"api.example"},
			DenyList:           []string{"blocked.example"},
			Required:           true,
		},
		Daytona: compozyconfig.DaytonaProfile{
			APIURL:      "https://daytona.example",
			Target:      "prod",
			Image:       "compozy:latest",
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
			ToolID:           "compozy__read",
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
	if got, want := matcher["tool_id"], "compozy__read"; got != want {
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
			Limits: compozyconfig.LimitsConfig{
				MaxConcurrentAgents: 11,
			},
			Permissions:    compozyconfig.PermissionsConfig{Mode: compozyconfig.PermissionModeApproveReads},
			SessionTimeout: 45 * time.Minute,
			HTTP:           compozyconfig.HTTPConfig{Host: "127.0.0.1", Port: 9001},
			Daemon: compozyconfig.DaemonConfig{
				Socket:               "/tmp/compozy.sock",
				MemoryReportInterval: compozyconfig.DefaultDaemonMemoryReportInterval,
			},
			Redact: compozyconfig.RedactConfig{Enabled: true},
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

type fakeCmdPaletteCatalog struct {
	catalog cmdpalette.Catalog
}

type recordingCmdPaletteCatalog struct {
	mu       sync.Mutex
	catalog  cmdpalette.Catalog
	requests []cmdpalette.CatalogRequest
	events   []cmdpalette.Event
}

func (r *recordingCmdPaletteCatalog) Catalog(
	_ context.Context,
	request cmdpalette.CatalogRequest,
) (cmdpalette.Catalog, error) {
	r.mu.Lock()
	r.requests = append(r.requests, request)
	r.mu.Unlock()
	return r.catalog, nil
}

func (r *recordingCmdPaletteCatalog) recordedRequests() []cmdpalette.CatalogRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]cmdpalette.CatalogRequest(nil), r.requests...)
}

func (r *recordingCmdPaletteCatalog) NotifyBindingChanged(
	_ context.Context,
	profileLens cmdpalette.ProfileLens,
	workspaceID cmdpalette.WorkspaceID,
	commandID cmdpalette.CommandID,
) {
	r.record(cmdpalette.Event{
		Name: cmdpalette.EventBindingChanged, ProfileLens: profileLens,
		WorkspaceID: workspaceID, CommandID: commandID,
	})
}

func (r *recordingCmdPaletteCatalog) NotifyAliasChanged(
	_ context.Context,
	profileLens cmdpalette.ProfileLens,
	workspaceID cmdpalette.WorkspaceID,
	commandID cmdpalette.CommandID,
) {
	r.record(cmdpalette.Event{
		Name: cmdpalette.EventAliasChanged, ProfileLens: profileLens,
		WorkspaceID: workspaceID, CommandID: commandID,
	})
}

func (r *recordingCmdPaletteCatalog) record(event cmdpalette.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recordingCmdPaletteCatalog) recorded() []cmdpalette.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]cmdpalette.Event(nil), r.events...)
}

func (f fakeCmdPaletteCatalog) Catalog(
	context.Context,
	cmdpalette.CatalogRequest,
) (cmdpalette.Catalog, error) {
	return f.catalog, nil
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
	key := compozyconfig.NormalizeAgentName(agentName)
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
	key := compozyconfig.NormalizeAgentName(agentName)
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
	summary.SetContent(summary.ContentValue())
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
		next.SetContent(summary.ContentValue())
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

		cfg, err := compozyconfig.LoadForHome(homePaths)
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
				Scope:   ScopeUser,
			},
			General: &GeneralSettings{
				Limits: compozyconfig.LimitsConfig{
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

		summaries, err := eventStore.ListEventSummaries(
			context.Background(),
			store.EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true}},
		)
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
		if err := json.Unmarshal(summaries[0].ContentValue(), &content); err != nil {
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

func testService(t *testing.T, homePaths compozyconfig.HomePaths, deps Dependencies) Service {
	t.Helper()
	if deps.ProfileResolver == nil {
		deps.ProfileResolver = settingsTestProfileResolver{}
	}
	if deps.AttentionWorkspaceMutes == nil {
		deps.AttentionWorkspaceMutes = &settingsTestAttentionMuteStore{byProfile: make(map[string][]string)}
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

type settingsTestProfileResolver struct{}

func (settingsTestProfileResolver) AvailableProfileID(_ context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == compozyconfig.DefaultProfileDirName {
		return store.DefaultProfileID, nil
	}
	if name == "" {
		return "", errors.New("test profile name is required")
	}
	return "profile-" + name, nil
}

type settingsTestAttentionMuteStore struct {
	mu         sync.Mutex
	byProfile  map[string][]string
	replaceErr error
}

func (s *settingsTestAttentionMuteStore) ListAttentionWorkspaceMutes(
	_ context.Context,
	profileID string,
) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.byProfile[profileID]...), nil
}

func (s *settingsTestAttentionMuteStore) ReplaceAttentionWorkspaceMutes(
	_ context.Context,
	profileID string,
	workspaceIDs []string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.replaceErr != nil {
		return s.replaceErr
	}
	s.byProfile[profileID] = append([]string(nil), workspaceIDs...)
	return nil
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
	modelcatalog.StatusOptions,
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
	models []compozyconfig.ProviderModelConfig,
	modelID string,
) compozyconfig.ProviderModelConfig {
	t.Helper()
	for _, model := range models {
		if model.ID == modelID {
			return model
		}
	}
	t.Fatalf("configured provider model %q not found in %#v", modelID, models)
	return compozyconfig.ProviderModelConfig{}
}

func testHomePaths(t *testing.T) compozyconfig.HomePaths {
	t.Helper()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
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

func testWindowManagerConfig() compozyconfig.WindowManagerConfig {
	return compozyconfig.WindowManagerConfig{
		NewWindowPolicy:     compozyconfig.WindowNewPolicyBesideFocus,
		SmallViewportPolicy: compozyconfig.WindowSmallViewportReject,
		FocusPolicy:         compozyconfig.WindowFocusDirectional,
		FocusWrap:           true,
		FocusFollowsPointer: true,
		RaiseOnFocus:        false,
		DragAwayPolicy:      compozyconfig.WindowDragAwayGroup,
		GroupMoveModifier:   "control",
		SwapModifier:        "meta",
		HistoryLimit:        77,
		NavStackLimit:       66,
		ClosedEntryLimit:    18,
		DesktopTransition:   compozyconfig.WindowDesktopTransitionCrossfade,
		Gaps: compozyconfig.WindowManagerGapsConfig{
			Inner:  12,
			Top:    18,
			Right:  14,
			Bottom: 16,
			Left:   20,
		},
		Snap: compozyconfig.WindowManagerSnapConfig{
			EdgeBand:     40,
			CornerReach:  180,
			ExitSlack:    20,
			RepeatRatios: []float64{0.4, 0.7, 0.3},
		},
		Bindings: compozyconfig.WindowManagerBindingConfig{
			TopCenter:    "none",
			BottomCenter: "zoom",
		},
		Shortcuts: map[string]windowmanager.ShortcutBinding{
			"desktop.switch.next": {"Meta+ArrowRight"},
			"window.focus.left":   {"Alt+ArrowLeft"},
		},
		GlobalShortcuts: map[string]string{
			windowmanager.DefaultGlobalSummonCommandID: windowmanager.DefaultGlobalSummonChord,
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
socket = "/tmp/compozy.sock"

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
allowed_marketplace_hooks = ["market"]

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

[extensions.trust]
allow_unverified = true

[extensions.sources.github]
enabled = true
base_url = "https://ext.example"

[extensions.sources.git]
enabled = true

[extensions.dev]
watch_interval = "2s"

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
	for index := range items {
		if items[index].Name == name {
			return items[index]
		}
	}
	t.Fatalf("Sandbox item %q not found in %#v", name, items)
	return SandboxItem{}
}

func findHookItem(t *testing.T, items []HookItem, name string) HookItem {
	t.Helper()
	for index := range items {
		if items[index].Name == name {
			return items[index]
		}
	}
	t.Fatalf("Hook item %q not found in %#v", name, items)
	return HookItem{}
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
