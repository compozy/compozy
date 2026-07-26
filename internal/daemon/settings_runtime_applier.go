package daemon

import (
	"context"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/providers"
	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/compozy/agh/internal/store"
)

type daemonSettingsRuntimeApplier struct {
	daemon              *Daemon
	state               *bootState
	networkAvailability store.NetworkAvailabilityStore
	networkWakeRunner   interface {
		SetEnabled(context.Context, bool) error
	}
}

func (a daemonSettingsRuntimeApplier) ApplyActiveConfig(
	ctx context.Context,
	snap *aghconfig.Config,
) []settingspkg.ApplyFailure {
	if a.daemon == nil || a.state == nil || snap == nil {
		return nil
	}
	next := *snap

	a.daemon.mu.Lock()
	previous := a.state.cfg
	a.daemon.mu.Unlock()

	failures := a.applyRuntimeDependencies(ctx, &next)
	if len(failures) > 0 {
		return a.rollbackRuntimeDependencies(ctx, &previous, failures)
	}

	availabilityChanged := a.networkAvailability != nil && previous.Network.Enabled != next.Network.Enabled
	if availabilityChanged {
		if failure := a.persistNetworkAvailability(
			ctx,
			next.Network.Enabled,
			"config.apply",
			"network_availability",
			"Network availability sync failed",
		); failure != nil {
			return a.rollbackRuntimeDependencies(ctx, &previous, []settingspkg.ApplyFailure{*failure})
		}
		if a.networkWakeRunner != nil {
			if err := a.networkWakeRunner.SetEnabled(ctx, next.Network.Enabled); err != nil {
				failures := []settingspkg.ApplyFailure{configApplyFailure(
					"network_wake_runner",
					diagnosticcontract.CategoryConfig,
					"Network wake runner sync failed",
					err,
				)}
				if rollbackErr := a.networkWakeRunner.SetEnabled(ctx, previous.Network.Enabled); rollbackErr != nil {
					failures = append(failures, configApplyFailure(
						"network_wake_runner_rollback",
						diagnosticcontract.CategoryConfig,
						"Network wake runner rollback failed",
						rollbackErr,
					))
				}
				if failure := a.persistNetworkAvailability(
					ctx,
					previous.Network.Enabled,
					"config.rollback",
					"network_availability_rollback",
					"Network availability rollback failed",
				); failure != nil {
					failures = append(failures, *failure)
				}
				return a.rollbackRuntimeDependencies(ctx, &previous, failures)
			}
		}
		if networkRuntime, ok := a.state.network.(interface{ SetEnabled(bool) }); ok {
			networkRuntime.SetEnabled(next.Network.Enabled)
		}
	}

	a.daemon.mu.Lock()
	a.state.cfg = next
	a.daemon.config = next
	a.daemon.mu.Unlock()
	// Drop cached workspace overlays so role and status resolution sees the applied global config.
	if a.state.workspaceResolver != nil {
		a.state.workspaceResolver.InvalidateAll()
	}

	providers.InvalidatePreStartCache()
	return nil
}

func (a daemonSettingsRuntimeApplier) rollbackRuntimeDependencies(
	ctx context.Context,
	previous *aghconfig.Config,
	failures []settingspkg.ApplyFailure,
) []settingspkg.ApplyFailure {
	if a.state.windowManager != nil {
		if err := a.state.windowManager.UpdateDefaults(windowManagerDefaults(previous.WindowManager)); err != nil {
			failures = append(failures, configApplyFailure(
				"window_manager_rollback",
				diagnosticcontract.CategoryConfig,
				"Window manager rollback failed",
				err,
			))
		}
	}
	a.reconcileExtensionMarketplace(previous)
	if a.state.modelCatalog != nil {
		if err := a.state.modelCatalog.ReconcileConfig(ctx, previous); err != nil {
			failures = append(failures, configApplyFailure(
				"model_catalog_rollback",
				diagnosticcontract.CategoryConfig,
				"Model catalog rollback failed",
				err,
			))
		}
	}
	if a.state.marketplace != nil {
		if err := a.state.marketplace.ReconcileConfig(ctx, previous); err != nil {
			failures = append(failures, configApplyFailure(
				"marketplace_rollback",
				diagnosticcontract.CategoryConfig,
				"Marketplace rollback failed",
				err,
			))
		}
	}
	if a.state.toolMCPResources != nil {
		if err := a.state.toolMCPResources.SyncConfig(ctx, previous); err != nil {
			failures = append(failures, configApplyFailure(
				"mcp_rollback",
				diagnosticcontract.CategoryMCP,
				"MCP runtime rollback failed",
				err,
			))
		}
	}
	return failures
}

func (a daemonSettingsRuntimeApplier) applyRuntimeDependencies(
	ctx context.Context,
	next *aghconfig.Config,
) []settingspkg.ApplyFailure {
	var failures []settingspkg.ApplyFailure
	if a.state.windowManager != nil {
		if err := a.state.windowManager.UpdateDefaults(windowManagerDefaults(next.WindowManager)); err != nil {
			failures = append(failures, configApplyFailure(
				"window_manager",
				diagnosticcontract.CategoryConfig,
				"Window manager sync failed",
				err,
			))
		}
	}
	a.reconcileExtensionMarketplace(next)
	if a.state.modelCatalog != nil {
		if err := a.state.modelCatalog.ReconcileConfig(ctx, next); err != nil {
			failures = append(failures, configApplyFailure(
				"model_catalog",
				diagnosticcontract.CategoryConfig,
				"Model catalog sync failed",
				err,
			))
		}
	}
	if a.state.marketplace != nil {
		if err := a.state.marketplace.ReconcileConfig(ctx, next); err != nil {
			failures = append(failures, configApplyFailure(
				"marketplace",
				diagnosticcontract.CategoryConfig,
				"Marketplace sync failed",
				err,
			))
		}
	}
	if a.state.toolMCPResources != nil {
		if err := a.state.toolMCPResources.SyncConfig(ctx, next); err != nil {
			failures = append(failures, configApplyFailure(
				"mcp",
				diagnosticcontract.CategoryMCP,
				"MCP runtime sync failed",
				err,
			))
		}
	}
	return failures
}

func (a daemonSettingsRuntimeApplier) reconcileExtensionMarketplace(cfg *aghconfig.Config) {
	if a.state == nil || cfg == nil {
		return
	}
	service, ok := a.state.deps.Extensions.(*daemonExtensionService)
	if !ok || service == nil {
		return
	}
	service.reconcileMarketplaceConfig(cfg.Extensions.Marketplace)
}

func (a daemonSettingsRuntimeApplier) persistNetworkAvailability(
	ctx context.Context,
	enabled bool,
	updatedBy string,
	subsystem string,
	summary string,
) *settingspkg.ApplyFailure {
	if _, err := a.networkAvailability.SetNetworkAvailability(ctx, enabled, updatedBy); err != nil {
		failure := configApplyFailure(
			subsystem,
			diagnosticcontract.CategoryConfig,
			summary,
			err,
		)
		return &failure
	}
	return nil
}

func configApplyFailure(
	subsystem string,
	category string,
	summary string,
	err error,
) settingspkg.ApplyFailure {
	return settingspkg.ApplyFailure{
		Subsystem: subsystem,
		Diagnostic: diagnostics.NewItem(
			"config.apply."+subsystem+"_sync_failed",
			diagnosticcontract.CodeConfigPartialFailure,
			category,
			summary,
			diagnostics.RedactAndBound(err.Error(), 1024),
			diagnosticcontract.SeverityError,
			diagnosticcontract.FreshnessLive,
			diagnostics.WithSuggestedCommand("agh config reload"),
		),
	}
}
