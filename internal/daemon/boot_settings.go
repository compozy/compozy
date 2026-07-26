package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	settingspkg "github.com/compozy/agh/internal/settings"
)

func (d *Daemon) bootSettings(ctx context.Context, state *bootState) error {
	if state == nil {
		return errors.New("daemon: boot settings state is required")
	}

	surface, err := newSettingsRuntimeSurface(d, state)
	if err != nil {
		return fmt.Errorf("daemon: create settings runtime surface: %w", err)
	}
	var applyRecords settingspkg.ApplyRecordStore
	if dbSource, ok := state.registry.(settingspkg.ApplyRecordDBSource); ok {
		applyRecords = settingspkg.NewConfigApplyRecordRepository(dbSource.DB(), time.Now)
	}
	networkAvailability := networkAvailabilityStoreDependency(state.registry)
	service, err := settingspkg.NewService(d.homePaths, settingspkg.Dependencies{
		WorkspaceResolver:        state.workspaceResolver,
		GeneralRuntime:           surface,
		MemoryRuntime:            surface,
		SkillsRuntime:            state.skillsRegistry,
		AutomationRuntime:        surface,
		NetworkRuntime:           surface,
		ObservabilityRuntime:     surface,
		Extensions:               surface,
		TransportParity:          surface,
		MCPAuth:                  surface,
		MCPRuntime:               surface,
		MCPCatalog:               settingsMarketplaceCatalogDependency(state.marketplace),
		MarketplaceInstallEvents: state.marketplaceNotifier,
		ModelCatalog:             state.modelCatalog,
		RuntimeApplier: daemonSettingsRuntimeApplier{
			daemon:              d,
			state:               state,
			networkAvailability: networkAvailability,
			networkWakeRunner:   state.networkWakeRunner,
		},
		ProviderSecrets:            settingsProviderVaultDependency(state.providerVault),
		EventSummaries:             state.registry,
		ApplyRecords:               applyRecords,
		RestartActionAvailable:     true,
		ConsolidateActionAvailable: state.dreamRuntime != nil && state.dreamRuntime.Enabled(),
		LogTailAvailable:           strings.TrimSpace(d.homePaths.LogFile) != "",
	})
	if err != nil {
		return fmt.Errorf("daemon: create settings service: %w", err)
	}
	if applyRecords != nil {
		if _, err := service.Reload(settingspkg.WithMutationSource(ctx, "boot")); err != nil {
			return fmt.Errorf("daemon: reconcile desired config with active generation: %w", err)
		}
	}

	updateManager, err := newSettingsUpdateManager(d)
	if err != nil {
		return fmt.Errorf("daemon: create settings update manager: %w", err)
	}
	updateManager.PrimeInstallDetection(ctx)

	state.deps.Settings = service
	state.deps.SettingsRestart = settingsRestartController{daemon: d}
	state.deps.SettingsUpdate = settingsUpdateController{manager: updateManager}
	return nil
}
