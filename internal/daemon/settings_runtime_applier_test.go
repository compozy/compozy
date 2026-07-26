package daemon

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	diagcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/marketplace"
	"github.com/compozy/agh/internal/providers"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/windowmanager"
)

func TestDaemonSettingsRuntimeApplier(t *testing.T) {
	// not parallel: this test mutates the provider pre-start process-wide cache.
	t.Run("Should invalidate provider prestart cache after active config apply", func(t *testing.T) {
		providers.InvalidatePreStartCache()
		t.Cleanup(providers.InvalidatePreStartCache)

		calls := 0
		env := &providers.ProbeEnv{
			ProviderName: "config-apply-cache",
			LookPath: func(string) (string, error) {
				calls++
				return "", exec.ErrNotFound
			},
		}
		provider := aghconfig.ProviderConfig{
			Command:  "config-apply-cache acp",
			AuthMode: aghconfig.ProviderAuthModeNativeCLI,
		}
		assertMissingCLIReport(t, "first", providers.PreStart(t.Context(), provider, env))
		assertMissingCLIReport(t, "cached", providers.PreStart(t.Context(), provider, env))
		if calls != 1 {
			t.Fatalf("PreStart LookPath calls before apply = %d, want 1", calls)
		}

		cfg := aghconfig.Config{}
		failures := daemonSettingsRuntimeApplier{
			daemon: &Daemon{},
			state:  &bootState{cfg: cfg},
		}.ApplyActiveConfig(t.Context(), &cfg)
		if len(failures) != 0 {
			t.Fatalf("ApplyActiveConfig() failures = %#v, want none", failures)
		}

		assertMissingCLIReport(t, "after apply", providers.PreStart(t.Context(), provider, env))
		if calls != 2 {
			t.Fatalf("PreStart LookPath calls after apply = %d, want 2", calls)
		}
	})

	t.Run("Should rollback MCP after runtime apply failure", func(t *testing.T) {
		t.Parallel()

		previous := aghconfig.Config{
			Network: aghconfig.DefaultNetworkConfig(),
			Providers: map[string]aghconfig.ProviderConfig{
				"codex": {Command: "codex acp", AuthMode: aghconfig.ProviderAuthModeNativeCLI},
			},
		}
		next := previous
		next.Network.Enabled = false
		next.Providers = map[string]aghconfig.ProviderConfig{
			"codex": {Command: "codex acp --next", AuthMode: aghconfig.ProviderAuthModeNativeCLI},
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
		previous := aghconfig.Config{
			Network:       aghconfig.DefaultNetworkConfig(),
			WindowManager: aghconfig.DefaultWindowManagerConfig(),
		}
		next := previous
		next.WindowManager.HistoryLimit = 1
		manager, err := windowmanager.NewService(
			windowmanager.NewMemoryRepository(),
			windowmanager.NewMemoryWorkspaceResolver("workspace-a"),
			nil,
			windowManagerDefaults(previous.WindowManager),
		)
		if err != nil {
			t.Fatalf("windowmanager.NewService() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := manager.Close(); closeErr != nil {
				t.Errorf("Manager.Close() error = %v", closeErr)
			}
		})
		state := &bootState{
			cfg: previous, windowManagerBootState: windowManagerBootState{windowManager: manager},
			toolMCPResources: &recordingToolMCPPublisher{errors: []error{errors.New("sync boom"), nil}},
		}
		failures := daemonSettingsRuntimeApplier{daemon: &Daemon{}, state: state}.
			ApplyActiveConfig(t.Context(), &next)
		if len(failures) != 1 || failures[0].Subsystem != "mcp" {
			t.Fatalf("ApplyActiveConfig(failed) failures = %#v", failures)
		}
		first, err := manager.Execute(t.Context(), windowmanager.CommandRequest{
			WorkspaceID: "workspace-a",
			Payload: windowmanager.CreateDesktopCommand{
				DesktopID: "desktop-two", Name: "Two",
			},
		})
		if err != nil {
			t.Fatalf("Execute(first) error = %v", err)
		}
		second, err := manager.Execute(t.Context(), windowmanager.CommandRequest{
			WorkspaceID: "workspace-a", ExpectedRevision: first.Snapshot.Revision,
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
			WorkspaceID: "workspace-a", ExpectedRevision: second.Snapshot.Revision,
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

		previous := aghconfig.Config{Network: aghconfig.DefaultNetworkConfig()}
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

		previous := aghconfig.Config{Network: aghconfig.DefaultNetworkConfig()}
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

		previous := aghconfig.Config{Network: aghconfig.DefaultNetworkConfig()}
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
		extensionService, ok := newDaemonExtensionService(
			registry,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nativeDeps.HomePaths,
			nil,
			nil,
		).(*daemonExtensionService)
		if !ok {
			t.Fatal("newDaemonExtensionService() did not return daemon service")
		}
		previous := aghconfig.Config{}
		next := previous
		next.Extensions.Marketplace.AllowUnverified = true
		syncCalls := 0
		state := &bootState{
			cfg:  previous,
			deps: RuntimeDeps{Extensions: extensionService},
			toolMCPResources: toolMCPPublisherFunc(func(context.Context) error {
				syncCalls++
				if syncCalls == 1 && !extensionService.marketplaceConfig().AllowUnverified {
					t.Error("extension side-load policy = false during active apply, want true")
				}
				if syncCalls == 2 && extensionService.marketplaceConfig().AllowUnverified {
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
		if extensionService.marketplaceConfig().AllowUnverified {
			t.Fatal("extension side-load policy = true after rollback, want false")
		}
	})

	t.Run("Should record mcp_rollback when MCP rollback sync fails", func(t *testing.T) {
		t.Parallel()

		previous := aghconfig.Config{}
		next := aghconfig.Config{
			Providers: map[string]aghconfig.ProviderConfig{
				"codex": {Command: "codex acp", AuthMode: aghconfig.ProviderAuthModeNativeCLI},
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
		previous := aghconfig.DefaultWithHome(homePaths)
		previous.Marketplace.Catalog.BaseURL = firstServer.URL
		previous.Marketplace.Catalog.Timeout = "1s"
		next := previous
		next.Marketplace.Catalog.BaseURL = secondServer.URL

		registry, err := globaldb.OpenGlobalDB(
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

type recordingToolMCPPublisher struct {
	configs []aghconfig.Config
	errors  []error
}

func (p *recordingToolMCPPublisher) Sync(ctx context.Context) error {
	return p.SyncConfig(ctx, nil)
}

func (p *recordingToolMCPPublisher) SyncConfig(_ context.Context, cfg *aghconfig.Config) error {
	if cfg == nil {
		p.configs = append(p.configs, aghconfig.Config{})
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
