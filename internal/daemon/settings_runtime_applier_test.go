package daemon

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	diagcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/resources"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/windowmanager"
)

func TestDaemonSettingsRuntimeApplier(t *testing.T) {
	t.Run("Should roll back staged skill resources when generation commit fails", func(t *testing.T) {
		t.Parallel()

		registry := skillspkg.NewRegistry(skillspkg.RegistryConfig{})
		if err := registry.ApplyResourceRecords(t.Context(), 1, []resources.Record[skillspkg.SkillResourceSpec]{
			{
				Kind:  skillspkg.SkillResourceKind,
				ID:    "initial",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
				Spec: skillspkg.SkillResourceSpec{
					Name:        "initial",
					Description: "Initial",
					Source:      "user",
					Enabled:     true,
				},
			},
		}); err != nil {
			t.Fatalf("ApplyResourceRecords(initial) error = %v", err)
		}
		previous := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{})
		next := previous
		next.Skills.Sources = []string{compozyconfig.SkillSourceClaude}
		publisher := &stagedSkillPublisherStub{}
		failures := daemonSettingsRuntimeApplier{
			daemon: &Daemon{},
			state:  &bootState{cfg: previous, skillsRegistry: registry, agentSkillResources: publisher},
		}.ApplyActiveConfig(t.Context(), &next)
		if len(failures) != 1 || failures[0].Subsystem != "skill_sources" {
			t.Fatalf("ApplyActiveConfig() failures = %#v, want skill_sources failure", failures)
		}
		if publisher.stagedCalls != 1 || publisher.rollbackCalls != 1 {
			t.Fatalf(
				"publisher staged/rollback calls = %d/%d, want 1/1",
				publisher.stagedCalls,
				publisher.rollbackCalls,
			)
		}
		if registry.ConfigGeneration() != 0 {
			t.Fatalf("registry.ConfigGeneration() = %d, want previous generation 0", registry.ConfigGeneration())
		}
		if skill, ok := registry.Get("initial"); !ok || skill.Meta.Description != "Initial" {
			t.Fatalf("registry.Get(initial) = (%#v, %t), want previous catalog", skill, ok)
		}
	})

	t.Run("Should apply attention config before publishing active config", func(t *testing.T) {
		t.Parallel()

		previous := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{})
		next := previous
		next.Attention = compozyconfig.AttentionConfig{
			Toasts: false,
			Sound:  false,
			System: true,
		}
		sessions := &attentionConfigSessionManager{fakeSessionManager: &fakeSessionManager{}}
		daemonInstance := &Daemon{config: previous}
		failures := daemonSettingsRuntimeApplier{
			daemon: daemonInstance,
			state:  &bootState{cfg: previous, sessions: sessions},
		}.ApplyActiveConfig(t.Context(), &next)
		if len(failures) != 0 {
			t.Fatalf("ApplyActiveConfig() failures = %#v, want none", failures)
		}
		if len(sessions.configs) != 1 || !attentionConfigsEqual(sessions.configs[0], next.Attention) {
			t.Fatalf("SetAttentionConfig() configs = %#v, want candidate", sessions.configs)
		}
		if !attentionConfigsEqual(daemonInstance.config.Attention, next.Attention) {
			t.Fatalf("published attention = %#v, want %#v", daemonInstance.config.Attention, next.Attention)
		}
	})

	t.Run("Should keep previous config when attention sync fails", func(t *testing.T) {
		t.Parallel()

		previous := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{})
		next := previous
		next.Attention.System = true
		sessions := &attentionConfigSessionManager{
			fakeSessionManager: &fakeSessionManager{},
			err:                errors.New("attention sync boom"),
		}
		daemonInstance := &Daemon{config: previous}
		failures := daemonSettingsRuntimeApplier{
			daemon: daemonInstance,
			state:  &bootState{cfg: previous, sessions: sessions},
		}.ApplyActiveConfig(t.Context(), &next)
		if len(failures) != 1 || failures[0].Subsystem != "attention" {
			t.Fatalf("ApplyActiveConfig() failures = %#v, want attention failure", failures)
		}
		if daemonInstance.config.Attention.System {
			t.Fatal("published attention.system = true, want previous false config")
		}
	})

	t.Run("Should apply the gateway ceiling before publishing active config", func(t *testing.T) {
		t.Parallel()

		previous := compozyconfig.Config{Gateway: compozyconfig.GatewayConfig{Enabled: true}}
		next := previous
		next.Gateway.Enabled = false
		policy := &recordingGatewayPolicy{}
		daemonInstance := &Daemon{config: previous}
		failures := daemonSettingsRuntimeApplier{
			daemon: daemonInstance,
			state:  &bootState{cfg: previous, gateway: policy},
		}.ApplyActiveConfig(t.Context(), &next)
		if len(failures) != 0 {
			t.Fatalf("ApplyActiveConfig() failures = %#v, want none", failures)
		}
		if len(policy.enabledCalls) != 1 || policy.enabledCalls[0] {
			t.Fatalf("gateway SetEnabled calls = %#v, want [false]", policy.enabledCalls)
		}
		if daemonInstance.config.Gateway.Enabled {
			t.Fatal("published gateway ceiling = true, want false")
		}
	})

	t.Run("Should keep previous config when the gateway ceiling sync fails", func(t *testing.T) {
		t.Parallel()

		previous := compozyconfig.Config{
			Gateway: compozyconfig.GatewayConfig{Enabled: true},
			Network: compozyconfig.NetworkConfig{Enabled: true},
		}
		next := previous
		next.Gateway.Enabled = false
		next.Network.Enabled = false
		policy := &recordingGatewayPolicy{setEnabledErr: errors.New("gateway sync boom")}
		availability := &recordingNetworkAvailabilityStore{}
		daemonInstance := &Daemon{config: previous}
		failures := daemonSettingsRuntimeApplier{
			daemon:              daemonInstance,
			state:               &bootState{cfg: previous, gateway: policy},
			networkAvailability: availability,
		}.ApplyActiveConfig(t.Context(), &next)
		if len(failures) != 1 || failures[0].Subsystem != "gateway_ceiling" {
			t.Fatalf("ApplyActiveConfig() failures = %#v, want gateway ceiling failure", failures)
		}
		if len(policy.enabledCalls) != 2 || policy.enabledCalls[0] || !policy.enabledCalls[1] {
			t.Fatalf("gateway SetEnabled calls = %#v, want [false true]", policy.enabledCalls)
		}
		if len(availability.enabled) != 2 || availability.enabled[0] || !availability.enabled[1] {
			t.Fatalf("availability writes = %#v, want [false true]", availability.enabled)
		}
		if len(availability.updatedBy) != 2 ||
			availability.updatedBy[0] != "config.apply" ||
			availability.updatedBy[1] != "config.rollback" {
			t.Fatalf("availability actors = %#v, want [config.apply config.rollback]", availability.updatedBy)
		}
		if !daemonInstance.config.Gateway.Enabled {
			t.Fatal("published gateway ceiling = false, want previous true config")
		}
		if !daemonInstance.config.Network.Enabled {
			t.Fatal("published network availability = false, want previous true config")
		}
	})

	t.Run("Should reconfigure active gateway dependencies before publishing config", func(t *testing.T) {
		t.Parallel()

		previous := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{})
		next := previous
		next.Gateway.Auth.RateLimit.MaxFails = 1
		next.Gateway.Auth.RateLimit.Window = 2 * time.Minute
		next.Gateway.Verify.Timeout = 250 * time.Millisecond
		next.Gateway.Verify.PublicDNSResolver = "8.8.8.8:853"
		limiter := gateway.NewAuthFailureLimiter(
			previous.Gateway.Auth.RateLimit.MaxFails,
			previous.Gateway.Auth.RateLimit.Window,
			nil,
		)
		verifier, err := gateway.NewEndpointVerifier(
			previous.Gateway.Verify.Timeout,
			previous.Gateway.Verify.PublicDNSResolver,
		)
		if err != nil {
			t.Fatalf("NewEndpointVerifier() error = %v", err)
		}
		state := &bootState{cfg: previous, gatewayVerifier: verifier}
		state.deps.Config = previous
		state.deps.GatewayAuthLimiter = limiter
		failures := daemonSettingsRuntimeApplier{
			daemon: &Daemon{config: previous},
			state:  state,
		}.ApplyActiveConfig(t.Context(), &next)
		if len(failures) != 0 {
			t.Fatalf("ApplyActiveConfig() failures = %#v, want none", failures)
		}
		if got := verifier.Timeout(); got != next.Gateway.Verify.Timeout {
			t.Fatalf("gateway verifier timeout = %s, want %s", got, next.Gateway.Verify.Timeout)
		}
		if !limiter.Allow("198.51.100.12", false) {
			t.Fatal("gateway limiter first attempt = false")
		}
		if limiter.Allow("198.51.100.12", false) {
			t.Fatal("gateway limiter second attempt = true, want live limit")
		}
		if got := state.deps.Config.Gateway.Verify.Timeout; got != next.Gateway.Verify.Timeout {
			t.Fatalf("runtime dependency config timeout = %s, want %s", got, next.Gateway.Verify.Timeout)
		}
	})

	t.Run("Should invalidate only the owning provider prestart cache after active config apply", func(t *testing.T) {
		t.Parallel()

		ownerCalls := 0
		otherCalls := 0
		ownerStarter := providers.NewPreStarter()
		otherStarter := providers.NewPreStarter()
		provider := compozyconfig.ProviderConfig{
			Command:  "config-apply-cache acp",
			AuthMode: compozyconfig.ProviderAuthModeNativeCLI,
		}
		ownerEnv := &providers.ProbeEnv{
			ProviderName: "config-apply-cache",
			PreStartScope: providers.PreStartScope{
				WorkspaceID:    "workspace-owner",
				ProfileID:      "profile-owner",
				HomeIdentity:   "/provider-home-owner",
				SandboxID:      "sandbox-owner",
				SandboxBackend: "local",
				SandboxProfile: "local",
			},
			LookPath: func(string) (string, error) {
				ownerCalls++
				return "", exec.ErrNotFound
			},
		}
		otherEnv := &providers.ProbeEnv{
			ProviderName: "config-apply-cache",
			PreStartScope: providers.PreStartScope{
				WorkspaceID:    "workspace-other",
				ProfileID:      "profile-other",
				HomeIdentity:   "/provider-home-other",
				SandboxID:      "sandbox-other",
				SandboxBackend: "local",
				SandboxProfile: "local",
			},
			LookPath: func(string) (string, error) {
				otherCalls++
				return "", exec.ErrNotFound
			},
		}
		assertMissingCLIReport(t, "first", ownerStarter.PreStart(t.Context(), provider, ownerEnv))
		assertMissingCLIReport(t, "cached", ownerStarter.PreStart(t.Context(), provider, ownerEnv))
		assertMissingCLIReport(t, "other first", otherStarter.PreStart(t.Context(), provider, otherEnv))
		assertMissingCLIReport(t, "other cached", otherStarter.PreStart(t.Context(), provider, otherEnv))
		if ownerCalls != 1 || otherCalls != 1 {
			t.Fatalf("PreStart LookPath calls before apply = %d/%d, want 1/1", ownerCalls, otherCalls)
		}

		cfg := compozyconfig.Config{}
		failures := daemonSettingsRuntimeApplier{
			daemon: &Daemon{providerPreStarter: ownerStarter},
			state:  &bootState{cfg: cfg},
		}.ApplyActiveConfig(t.Context(), &cfg)
		if len(failures) != 0 {
			t.Fatalf("ApplyActiveConfig() failures = %#v, want none", failures)
		}

		assertMissingCLIReport(t, "owner after apply", ownerStarter.PreStart(t.Context(), provider, ownerEnv))
		assertMissingCLIReport(t, "other after apply", otherStarter.PreStart(t.Context(), provider, otherEnv))
		if ownerCalls != 2 || otherCalls != 1 {
			t.Fatalf("PreStart LookPath calls after apply = %d/%d, want 2/1", ownerCalls, otherCalls)
		}
	})

	t.Run("Should rollback MCP after runtime apply failure", func(t *testing.T) {
		t.Parallel()

		previous := compozyconfig.Config{
			Network: compozyconfig.DefaultNetworkConfig(),
			Providers: map[string]compozyconfig.ProviderConfig{
				"codex": {Command: "codex acp", AuthMode: compozyconfig.ProviderAuthModeNativeCLI},
			},
		}
		next := previous
		next.Network.Enabled = false
		next.Providers = map[string]compozyconfig.ProviderConfig{
			"codex": {Command: "codex acp --next", AuthMode: compozyconfig.ProviderAuthModeNativeCLI},
		}
		publisher := &recordingToolMCPPublisher{errors: []error{errors.New("mcp sync boom"), nil}}
		daemonInstance := &Daemon{config: previous}
		availability := &recordingNetworkAvailabilityStore{}
		failures := daemonSettingsRuntimeApplier{
			daemon:              daemonInstance,
			networkAvailability: availability,
			state: &bootState{
				cfg:              previous,
				toolMCPResources: publisher,
			},
		}.ApplyActiveConfig(t.Context(), &next)
		if len(publisher.configs) != 2 {
			t.Fatalf("MCP SyncConfig calls = %d, want 2 (apply + rollback)", len(publisher.configs))
		}
		if got := publisher.configs[0].Providers["codex"].Command; got != "codex acp --next" {
			t.Fatalf("MCP apply config command = %q, want candidate", got)
		}
		if got := publisher.configs[1].Providers["codex"].Command; got != "codex acp" {
			t.Fatalf("MCP rollback config command = %q, want previous", got)
		}
		if len(failures) != 1 {
			t.Fatalf("ApplyActiveConfig() failures = %#v, want one mcp failure", failures)
		}
		if failures[0].Subsystem != "mcp" {
			t.Fatalf("failure subsystem = %q, want mcp", failures[0].Subsystem)
		}
		if got := daemonInstance.config.Providers["codex"].Command; got != "codex acp" {
			t.Fatalf("restored daemon config command = %q, want previous", got)
		}
		if len(availability.enabled) != 0 || len(availability.updatedBy) != 0 {
			t.Fatalf(
				"availability writes/actors = %#v/%#v, want none before dependency success",
				availability.enabled,
				availability.updatedBy,
			)
		}
	})

	t.Run("Should hot-apply and rollback window-manager defaults with the active config", func(t *testing.T) {
		t.Parallel()
		previous := compozyconfig.Config{
			Network:       compozyconfig.DefaultNetworkConfig(),
			WindowManager: compozyconfig.DefaultWindowManagerConfig(),
		}
		next := previous
		next.WindowManager.HistoryLimit = 1
		fixture := newDaemonWindowManagerFixture(t)
		if err := fixture.registry.UpdateDefaults(windowManagerDefaults(previous.WindowManager)); err != nil {
			t.Fatalf("windowManagerRegistry.UpdateDefaults() error = %v", err)
		}
		manager := fixture.manager
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		state := &bootState{
			cfg: previous, windowManagerBootState: windowManagerBootState{windowManagers: fixture.registry},
			toolMCPResources: &recordingToolMCPPublisher{errors: []error{errors.New("sync boom"), nil}},
		}
		failures := daemonSettingsRuntimeApplier{daemon: &Daemon{}, state: state}.
			ApplyActiveConfig(t.Context(), &next)
		if len(failures) != 1 || failures[0].Subsystem != "mcp" {
			t.Fatalf("ApplyActiveConfig(failed) failures = %#v", failures)
		}
		first, err := manager.Execute(t.Context(), windowmanager.CommandRequest{
			WorkspaceID: workspaceID,
			Payload: windowmanager.CreateDesktopCommand{
				DesktopID: "desktop-two", Name: "Two",
			},
		})
		if err != nil {
			t.Fatalf("Execute(first) error = %v", err)
		}
		second, err := manager.Execute(t.Context(), windowmanager.CommandRequest{
			WorkspaceID: workspaceID, ExpectedRevision: first.Snapshot.Revision,
			Payload: windowmanager.CreateDesktopCommand{
				DesktopID: "desktop-three", Name: "Three",
			},
		})
		if err != nil {
			t.Fatalf("Execute(after rollback) error = %v", err)
		}
		if len(second.Snapshot.History.Undo) != 2 {
			t.Fatalf("Execute(after rollback) history = %d, want 2", len(second.Snapshot.History.Undo))
		}

		state.toolMCPResources = nil
		failures = daemonSettingsRuntimeApplier{daemon: &Daemon{}, state: state}.
			ApplyActiveConfig(t.Context(), &next)
		if len(failures) != 0 {
			t.Fatalf("ApplyActiveConfig(success) failures = %#v", failures)
		}
		third, err := manager.Execute(t.Context(), windowmanager.CommandRequest{
			WorkspaceID: workspaceID, ExpectedRevision: second.Snapshot.Revision,
			Payload: windowmanager.CreateDesktopCommand{
				DesktopID: "desktop-four", Name: "Four",
			},
		})
		if err != nil {
			t.Fatalf("Execute(after hot apply) error = %v", err)
		}
		if len(third.Snapshot.History.Undo) != 1 {
			t.Fatalf("Execute(after hot apply) history = %d, want 1", len(third.Snapshot.History.Undo))
		}
	})

	t.Run("Should persist a network availability transition before advancing config", func(t *testing.T) {
		t.Parallel()

		previous := compozyconfig.Config{Network: compozyconfig.DefaultNetworkConfig()}
		next := previous
		next.Network.Enabled = false
		daemonInstance := &Daemon{config: previous}
		var enabledAtWrite bool
		availability := &recordingNetworkAvailabilityStore{
			beforeWrite: func(bool) {
				enabledAtWrite = daemonInstance.config.Network.Enabled
			},
		}
		failures := daemonSettingsRuntimeApplier{
			daemon:              daemonInstance,
			state:               &bootState{cfg: previous},
			networkAvailability: availability,
		}.ApplyActiveConfig(t.Context(), &next)
		if len(failures) != 0 {
			t.Fatalf("ApplyActiveConfig() failures = %#v, want none", failures)
		}
		if len(availability.enabled) != 1 || availability.enabled[0] {
			t.Fatalf("availability writes = %#v, want one disabled write", availability.enabled)
		}
		if availability.updatedBy[0] != "config.apply" {
			t.Fatalf("availability actor = %q, want config.apply", availability.updatedBy[0])
		}
		if !enabledAtWrite {
			t.Fatal("daemon network enabled at availability write = false, want previous config still published")
		}
		if daemonInstance.config.Network.Enabled {
			t.Fatal("daemon network enabled = true, want applied false")
		}
	})

	t.Run("Should keep previous config when availability persistence fails", func(t *testing.T) {
		t.Parallel()

		previous := compozyconfig.Config{Network: compozyconfig.DefaultNetworkConfig()}
		next := previous
		next.Network.Enabled = false
		daemonInstance := &Daemon{config: previous}
		failures := daemonSettingsRuntimeApplier{
			daemon: daemonInstance,
			state:  &bootState{cfg: previous},
			networkAvailability: &recordingNetworkAvailabilityStore{
				err: errors.New("availability write boom"),
			},
		}.ApplyActiveConfig(t.Context(), &next)
		if len(failures) != 1 || failures[0].Subsystem != "network_availability" {
			t.Fatalf("ApplyActiveConfig() failures = %#v, want network availability failure", failures)
		}
		if !daemonInstance.config.Network.Enabled {
			t.Fatal("daemon network enabled = false, want previous config retained")
		}
	})

	t.Run("Should rollback applied dependencies when availability persistence fails", func(t *testing.T) {
		t.Parallel()

		previous := compozyconfig.Config{Network: compozyconfig.DefaultNetworkConfig()}
		next := previous
		next.Network.Enabled = false
		daemonInstance := &Daemon{config: previous}
		syncCalls := 0
		failures := daemonSettingsRuntimeApplier{
			daemon: daemonInstance,
			state: &bootState{
				cfg: previous,
				toolMCPResources: toolMCPPublisherFunc(func(context.Context) error {
					syncCalls++
					return nil
				}),
			},
			networkAvailability: &recordingNetworkAvailabilityStore{
				err: errors.New("availability write boom"),
			},
		}.ApplyActiveConfig(t.Context(), &next)
		if len(failures) != 1 || failures[0].Subsystem != "network_availability" {
			t.Fatalf("ApplyActiveConfig() failures = %#v, want network availability failure", failures)
		}
		if syncCalls != 2 {
			t.Fatalf("MCP Sync calls = %d, want apply plus rollback", syncCalls)
		}
		if !daemonInstance.config.Network.Enabled {
			t.Fatal("daemon network enabled = false, want previous config retained")
		}
	})

	t.Run("Should apply and rollback extension side-load policy live", func(t *testing.T) {
		t.Parallel()

		nativeDeps, registry, _, _ := newNativeExtensionToolDeps(t)
		extensionService, ok := newDaemonExtensionService(&daemonExtensionServiceDeps{
			Registry:  registry,
			HomePaths: nativeDeps.HomePaths,
		},
		).(*daemonExtensionService)
		if !ok {
			t.Fatal("newDaemonExtensionService() did not return daemon service")
		}
		previous := compozyconfig.Config{}
		next := previous
		next.Extensions.Trust.AllowUnverified = true
		syncCalls := 0
		state := &bootState{
			cfg:  previous,
			deps: RuntimeDeps{Extensions: extensionService},
			toolMCPResources: toolMCPPublisherFunc(func(context.Context) error {
				syncCalls++
				if syncCalls == 1 && !extensionService.marketplaceConfig().Trust.AllowUnverified {
					t.Error("extension side-load policy = false during active apply, want true")
				}
				if syncCalls == 2 && extensionService.marketplaceConfig().Trust.AllowUnverified {
					t.Error("extension side-load policy = true during rollback, want false")
				}
				return errors.New("mcp sync boom")
			}),
		}
		failures := daemonSettingsRuntimeApplier{daemon: &Daemon{}, state: state}.ApplyActiveConfig(t.Context(), &next)
		if syncCalls != 2 {
			t.Fatalf("MCP Sync calls = %d, want 2 (apply + rollback)", syncCalls)
		}
		if len(failures) != 2 {
			t.Fatalf("ApplyActiveConfig() failures = %#v, want mcp plus rollback", failures)
		}
		if failures[0].Subsystem != "mcp" || failures[1].Subsystem != "mcp_rollback" {
			t.Fatalf("ApplyActiveConfig() subsystems = %#v, want mcp then mcp_rollback", failures)
		}
		if extensionService.marketplaceConfig().Trust.AllowUnverified {
			t.Fatal("extension side-load policy = true after rollback, want false")
		}
	})

	t.Run("Should record mcp_rollback when MCP rollback sync fails", func(t *testing.T) {
		t.Parallel()

		previous := compozyconfig.Config{}
		next := compozyconfig.Config{
			Providers: map[string]compozyconfig.ProviderConfig{
				"codex": {Command: "codex acp", AuthMode: compozyconfig.ProviderAuthModeNativeCLI},
			},
		}
		syncCalls := 0
		failures := daemonSettingsRuntimeApplier{
			daemon: &Daemon{},
			state: &bootState{
				cfg: previous,
				toolMCPResources: toolMCPPublisherFunc(func(context.Context) error {
					syncCalls++
					return errors.New("mcp sync boom")
				}),
			},
		}.ApplyActiveConfig(t.Context(), &next)
		if syncCalls != 2 {
			t.Fatalf("MCP Sync calls = %d, want 2 (apply + rollback)", syncCalls)
		}
		if len(failures) != 2 {
			t.Fatalf("ApplyActiveConfig() failures = %#v, want mcp + mcp_rollback", failures)
		}
		if failures[0].Subsystem != "mcp" {
			t.Fatalf("first failure subsystem = %q, want mcp", failures[0].Subsystem)
		}
		if failures[1].Subsystem != "mcp_rollback" {
			t.Fatalf("second failure subsystem = %q, want mcp_rollback", failures[1].Subsystem)
		}
	})

	t.Run("Should restore marketplace sources when another live dependency fails", func(t *testing.T) {
		t.Parallel()

		firstServer := newMarketplaceFeedServer(t, "rollback-first")
		secondServer := newMarketplaceFeedServer(t, "rollback-second")
		homePaths := testHomePaths(t)
		previous := compozyconfig.DefaultWithHome(homePaths)
		previous.Marketplace.Catalog.BaseURL = firstServer.URL
		previous.Marketplace.Catalog.Timeout = "1s"
		next := previous
		next.Marketplace.Catalog.BaseURL = secondServer.URL

		registry, err := openDaemonTestGlobalDBAtPath(
			t.Context(),
			filepath.Join(t.TempDir(), store.GlobalDatabaseName),
		)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := registry.Close(context.Background()); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
		marketplaceStore, err := marketplace.NewSQLiteStore(registry)
		if err != nil {
			t.Fatalf("NewSQLiteStore() error = %v", err)
		}
		runtime, err := newMarketplaceRuntime(marketplaceStore, nil, previous.Marketplace.Catalog, nil)
		if err != nil {
			t.Fatalf("newMarketplaceRuntime() error = %v", err)
		}
		if _, err := runtime.Refresh(t.Context(), marketplace.KindSkill); err != nil {
			t.Fatalf("Refresh(seed) error = %v", err)
		}

		syncCalls := 0
		failures := daemonSettingsRuntimeApplier{
			daemon: &Daemon{},
			state: &bootState{
				cfg:         previous,
				marketplace: runtime,
				toolMCPResources: toolMCPPublisherFunc(func(ctx context.Context) error {
					syncCalls++
					if syncCalls == 1 {
						if _, err := runtime.Refresh(ctx, marketplace.KindSkill); err != nil {
							return errors.Join(errors.New("verify active marketplace source"), err)
						}
						assertMarketplaceRuntimeEntry(t, runtime, "rollback-second")
						return errors.New("mcp sync boom")
					}
					return nil
				}),
			},
		}.ApplyActiveConfig(t.Context(), &next)
		if len(failures) != 1 || failures[0].Subsystem != "mcp" {
			t.Fatalf("ApplyActiveConfig() failures = %#v, want one mcp failure", failures)
		}
		if _, err := runtime.Refresh(t.Context(), marketplace.KindSkill); err != nil {
			t.Fatalf("Refresh(after rollback) error = %v", err)
		}
		assertMarketplaceRuntimeEntry(t, runtime, "rollback-first")
	})
}

func TestModelCatalogConfigChanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*compozyconfig.Config)
		want   bool
	}{
		{
			name: "Should ignore an MCP-only update",
			mutate: func(cfg *compozyconfig.Config) {
				cfg.MCPServers = append(cfg.MCPServers, compozyconfig.MCPServer{Name: "added"})
			},
		},
		{
			name: "Should detect a provider update",
			mutate: func(cfg *compozyconfig.Config) {
				cfg.Providers = compozyconfig.CloneProviderConfigs(cfg.Providers)
				cfg.Providers["codex"] = compozyconfig.ProviderConfig{Command: "codex acp --next"}
			},
			want: true,
		},
		{
			name: "Should detect a model catalog update",
			mutate: func(cfg *compozyconfig.Config) {
				cfg.ModelCatalog.Sources.ModelsDev.Timeout = "3s"
			},
			want: true,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			previous := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{})
			next := previous
			testCase.mutate(&next)
			if got := modelCatalogConfigChanged(&previous, &next); got != testCase.want {
				t.Fatalf("modelCatalogConfigChanged() = %t, want %t", got, testCase.want)
			}
		})
	}
}

type stagedSkillPublisherStub struct {
	stagedCalls   int
	rollbackCalls int
}

func (s *stagedSkillPublisherStub) Sync(context.Context) error { return nil }

func (s *stagedSkillPublisherStub) SyncSkills(context.Context) error { return nil }

func (s *stagedSkillPublisherStub) SyncSkillsStaged(context.Context) (func(context.Context) error, error) {
	s.stagedCalls++
	return func(context.Context) error {
		s.rollbackCalls++
		return nil
	}, nil
}

type attentionConfigSessionManager struct {
	*fakeSessionManager
	configs []compozyconfig.AttentionConfig
	err     error
}

func (m *attentionConfigSessionManager) SetAttentionConfig(cfg compozyconfig.AttentionConfig) error {
	m.configs = append(m.configs, compozyconfig.AttentionConfig{
		Toasts: cfg.Toasts, Sound: cfg.Sound, System: cfg.System,
	})
	return m.err
}

func TestDaemonAppliesIsolatedProviderPreStarterDefaults(t *testing.T) {
	t.Parallel()
	t.Run("Should allocate one pre-starter per daemon", testDaemonAppliesIsolatedProviderPreStarterDefaults)
}

func testDaemonAppliesIsolatedProviderPreStarterDefaults(t *testing.T) {
	t.Helper()

	first := &Daemon{}
	second := &Daemon{}
	first.applyCoreDefaults()
	second.applyCoreDefaults()
	if first.providerPreStarter == nil || second.providerPreStarter == nil {
		t.Fatal("Daemon provider pre-starter = nil, want a daemon-owned instance")
	}
	if first.providerPreStarter == second.providerPreStarter {
		t.Fatal("Daemons share a provider pre-starter, want isolated cache ownership")
	}
}

type recordingToolMCPPublisher struct {
	configs []compozyconfig.Config
	errors  []error
}

func (p *recordingToolMCPPublisher) Sync(ctx context.Context) error {
	return p.SyncConfig(ctx, nil)
}

func (p *recordingToolMCPPublisher) SyncConfig(_ context.Context, cfg *compozyconfig.Config) error {
	if cfg == nil {
		p.configs = append(p.configs, compozyconfig.Config{})
	} else {
		p.configs = append(p.configs, *cfg)
	}
	index := len(p.configs) - 1
	if index < len(p.errors) {
		return p.errors[index]
	}
	return nil
}

type recordingNetworkAvailabilityStore struct {
	enabled     []bool
	updatedBy   []string
	beforeWrite func(bool)
	err         error
}

type recordingGatewayPolicy struct {
	enabledCalls  []bool
	setEnabledErr error
}

func (p *recordingGatewayPolicy) Plan(context.Context) (gateway.ExposurePlan, error) {
	return gateway.ExposurePlan{}, nil
}

func (p *recordingGatewayPolicy) Transition(
	context.Context,
	gateway.TransitionRequest,
) (gateway.Status, error) {
	return gateway.Status{}, nil
}

func (p *recordingGatewayPolicy) Reconcile(context.Context) (gateway.Status, error) {
	return gateway.Status{}, nil
}

func (p *recordingGatewayPolicy) SetEnabled(_ context.Context, enabled bool) error {
	p.enabledCalls = append(p.enabledCalls, enabled)
	err := p.setEnabledErr
	p.setEnabledErr = nil
	return err
}

func (p *recordingGatewayPolicy) Status(context.Context) (gateway.Status, error) {
	return gateway.Status{}, nil
}

func (p *recordingGatewayPolicy) Acquire(gateway.Tier, gateway.Surface) (func(), error) {
	return func() {}, nil
}

func (p *recordingGatewayPolicy) Close(context.Context) error { return nil }

func (s *recordingNetworkAvailabilityStore) GetNetworkAvailability(
	context.Context,
) (store.NetworkAvailability, error) {
	return store.NetworkAvailability{}, nil
}

func (s *recordingNetworkAvailabilityStore) SetNetworkAvailability(
	_ context.Context,
	enabled bool,
	updatedBy string,
) (store.NetworkAvailability, error) {
	if s.beforeWrite != nil {
		s.beforeWrite(enabled)
	}
	s.enabled = append(s.enabled, enabled)
	s.updatedBy = append(s.updatedBy, updatedBy)
	if s.err != nil {
		return store.NetworkAvailability{}, s.err
	}
	return store.NetworkAvailability{
		Enabled:   enabled,
		Epoch:     int64(len(s.enabled)),
		UpdatedAt: time.Now().UTC(),
		UpdatedBy: updatedBy,
	}, nil
}

func assertMissingCLIReport(t *testing.T, label string, report providers.PreStartReport) {
	t.Helper()

	if report.Item == nil {
		t.Fatalf("PreStart(%s).Item = nil, want diagnostic", label)
	}
	if report.Item.Code != diagcontract.CodeProviderCLIMissing {
		t.Fatalf("PreStart(%s).Code = %q, want %q", label, report.Item.Code, diagcontract.CodeProviderCLIMissing)
	}
}
