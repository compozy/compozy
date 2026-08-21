package daemon

import (
	"context"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/deadentity"
	"github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
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
	snap *compozyconfig.Config,
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
	nextLoopTargetHealth, failure := a.prepareLoopTargetHealthConfigChange(&previous, &next)
	if failure != nil {
		return a.rollbackRuntimeDependencies(ctx, &previous, []settingspkg.ApplyFailure{*failure})
	}
	if failures := a.applyNetworkAvailabilityChange(ctx, &previous, &next); len(failures) > 0 {
		return a.rollbackRuntimeDependencies(ctx, &previous, failures)
	}
	if failures := a.applyGatewayTuningChange(&previous, &next); len(failures) > 0 {
		failures = a.rollbackRuntimeDependencies(ctx, &previous, failures)
		return a.rollbackGatewayAndNetwork(ctx, &previous, &next, failures)
	}
	if failures := a.applyGatewayCeilingChange(ctx, &previous, &next); len(failures) > 0 {
		failures = a.rollbackRuntimeDependencies(ctx, &previous, failures)
		return a.rollbackGatewayAndNetwork(ctx, &previous, &next, failures)
	}
	if failure := a.applyAttentionConfigChange(&previous, &next); failure != nil {
		failures := a.rollbackRuntimeDependencies(ctx, &previous, []settingspkg.ApplyFailure{*failure})
		return a.rollbackGatewayAndNetwork(ctx, &previous, &next, failures)
	}

	a.daemon.mu.Lock()
	if nextLoopTargetHealth != nil {
		a.state.loopTargetHealth.Swap(nextLoopTargetHealth)
	}
	a.state.deps.Config = next
	a.state.cfg = next
	a.daemon.config = next
	preStarter := a.daemon.providerPreStarter
	worktrees := a.state.worktrees
	a.daemon.mu.Unlock()
	if a.state.agentProbeConfig != nil {
		a.state.agentProbeConfig.Update(&next)
	}
	// Drop cached workspace overlays so role and status resolution sees the applied global config.
	if a.state.workspaceResolver != nil {
		a.state.workspaceResolver.InvalidateAll()
	}
	if worktrees != nil {
		worktrees.Reconfigure(next.Worktrees, next.Worktrees.ResolveRoot(a.daemon.homePaths))
	}

	if preStarter != nil {
		preStarter.Clear()
	}
	if a.state.toolProjectionEpoch != nil {
		a.state.toolProjectionEpoch.Advance()
	}
	return nil
}

func (a daemonSettingsRuntimeApplier) applyGatewayTuningChange(
	previous *compozyconfig.Config,
	next *compozyconfig.Config,
) []settingspkg.ApplyFailure {
	if !gatewayTuningChanged(previous.Gateway, next.Gateway) {
		return nil
	}
	var failures []settingspkg.ApplyFailure
	if service := a.state.deps.Gateway; service != nil {
		if err := service.ReconfigureDeviceLimits(
			next.Gateway.Pairing.MaxPending,
			next.Gateway.Pairing.TTL,
			next.Gateway.StreamTicket.TTL,
		); err != nil {
			failures = append(failures, configApplyFailure(
				"gateway_device_limits",
				diagnosticcontract.CategoryConfig,
				"Gateway device limit sync failed",
				err,
			))
		}
	}
	if limiter := a.state.deps.GatewayAuthLimiter; limiter != nil {
		if err := limiter.Reconfigure(
			next.Gateway.Auth.RateLimit.MaxFails,
			next.Gateway.Auth.RateLimit.Window,
		); err != nil {
			failures = append(failures, configApplyFailure(
				"gateway_auth_limits",
				diagnosticcontract.CategoryConfig,
				"Gateway authentication limit sync failed",
				err,
			))
		}
	}
	if verifier := a.state.gatewayVerifier; verifier != nil && previous.Gateway.Verify != next.Gateway.Verify {
		if err := verifier.Reconfigure(
			next.Gateway.Verify.Timeout,
			next.Gateway.Verify.PublicDNSResolver,
		); err != nil {
			failures = append(failures, configApplyFailure(
				"gateway_verify",
				diagnosticcontract.CategoryConfig,
				"Gateway verification config sync failed",
				err,
			))
		}
	}
	return failures
}

func gatewayTuningChanged(previous compozyconfig.GatewayConfig, next compozyconfig.GatewayConfig) bool {
	return previous.Pairing != next.Pairing ||
		previous.StreamTicket != next.StreamTicket ||
		previous.Auth != next.Auth ||
		previous.Verify != next.Verify
}

func (a daemonSettingsRuntimeApplier) rollbackGatewayAndNetwork(
	ctx context.Context,
	previous *compozyconfig.Config,
	next *compozyconfig.Config,
	failures []settingspkg.ApplyFailure,
) []settingspkg.ApplyFailure {
	failures = a.rollbackGatewayTuning(previous, next, failures)
	failures = a.rollbackGatewayCeiling(ctx, previous, next, failures)
	return a.rollbackNetworkAvailability(ctx, previous, next, failures)
}

func (a daemonSettingsRuntimeApplier) rollbackGatewayTuning(
	previous *compozyconfig.Config,
	next *compozyconfig.Config,
	failures []settingspkg.ApplyFailure,
) []settingspkg.ApplyFailure {
	if !gatewayTuningChanged(previous.Gateway, next.Gateway) {
		return failures
	}
	if service := a.state.deps.Gateway; service != nil {
		if err := service.ReconfigureDeviceLimits(
			previous.Gateway.Pairing.MaxPending,
			previous.Gateway.Pairing.TTL,
			previous.Gateway.StreamTicket.TTL,
		); err != nil {
			failures = append(failures, configApplyFailure(
				"gateway_device_limits_rollback",
				diagnosticcontract.CategoryConfig,
				"Gateway device limit rollback failed",
				err,
			))
		}
	}
	if limiter := a.state.deps.GatewayAuthLimiter; limiter != nil {
		if err := limiter.Reconfigure(
			previous.Gateway.Auth.RateLimit.MaxFails,
			previous.Gateway.Auth.RateLimit.Window,
		); err != nil {
			failures = append(failures, configApplyFailure(
				"gateway_auth_limits_rollback",
				diagnosticcontract.CategoryConfig,
				"Gateway authentication limit rollback failed",
				err,
			))
		}
	}
	if verifier := a.state.gatewayVerifier; verifier != nil {
		if err := verifier.Reconfigure(
			previous.Gateway.Verify.Timeout,
			previous.Gateway.Verify.PublicDNSResolver,
		); err != nil {
			failures = append(failures, configApplyFailure(
				"gateway_verify_rollback",
				diagnosticcontract.CategoryConfig,
				"Gateway verification config rollback failed",
				err,
			))
		}
	}
	return failures
}

func (a daemonSettingsRuntimeApplier) rollbackGatewayCeiling(
	ctx context.Context,
	previous *compozyconfig.Config,
	next *compozyconfig.Config,
	failures []settingspkg.ApplyFailure,
) []settingspkg.ApplyFailure {
	if a.state.gateway != nil && previous.Gateway.Enabled != next.Gateway.Enabled {
		if err := a.state.gateway.SetEnabled(ctx, previous.Gateway.Enabled); err != nil {
			failures = append(failures, configApplyFailure(
				"gateway_ceiling_rollback",
				diagnosticcontract.CategoryConfig,
				"Gateway ceiling rollback failed",
				err,
			))
		}
	}
	return failures
}

func (a daemonSettingsRuntimeApplier) rollbackNetworkAvailability(
	ctx context.Context,
	previous *compozyconfig.Config,
	next *compozyconfig.Config,
	failures []settingspkg.ApplyFailure,
) []settingspkg.ApplyFailure {
	if a.networkAvailability != nil && previous.Network.Enabled != next.Network.Enabled {
		if failure := a.persistNetworkAvailability(
			ctx,
			previous.Network.Enabled,
			"config.rollback",
			"network_availability_rollback",
			"Network availability rollback failed",
		); failure != nil {
			failures = append(failures, *failure)
		}
		if a.networkWakeRunner != nil {
			if err := a.networkWakeRunner.SetEnabled(ctx, previous.Network.Enabled); err != nil {
				failures = append(failures, configApplyFailure(
					"network_wake_runner_rollback",
					diagnosticcontract.CategoryConfig,
					"Network wake runner rollback failed",
					err,
				))
			}
		}
		if networkRuntime, ok := a.state.network.(interface{ SetEnabled(bool) }); ok {
			networkRuntime.SetEnabled(previous.Network.Enabled)
		}
	}
	return failures
}

func (a daemonSettingsRuntimeApplier) applyGatewayCeilingChange(
	ctx context.Context,
	previous *compozyconfig.Config,
	next *compozyconfig.Config,
) []settingspkg.ApplyFailure {
	if a.state.gateway == nil || previous.Gateway.Enabled == next.Gateway.Enabled {
		return nil
	}
	if err := a.state.gateway.SetEnabled(ctx, next.Gateway.Enabled); err != nil {
		return []settingspkg.ApplyFailure{configApplyFailure(
			"gateway_ceiling",
			diagnosticcontract.CategoryConfig,
			"Gateway ceiling sync failed",
			err,
		)}
	}
	return nil
}

func (a daemonSettingsRuntimeApplier) applyNetworkAvailabilityChange(
	ctx context.Context,
	previous *compozyconfig.Config,
	next *compozyconfig.Config,
) []settingspkg.ApplyFailure {
	if a.networkAvailability == nil || previous.Network.Enabled == next.Network.Enabled {
		return nil
	}
	if failure := a.persistNetworkAvailability(
		ctx,
		next.Network.Enabled,
		"config.apply",
		"network_availability",
		"Network availability sync failed",
	); failure != nil {
		return []settingspkg.ApplyFailure{*failure}
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
			return failures
		}
	}
	if networkRuntime, ok := a.state.network.(interface{ SetEnabled(bool) }); ok {
		networkRuntime.SetEnabled(next.Network.Enabled)
	}
	return nil
}

func (a daemonSettingsRuntimeApplier) prepareLoopTargetHealthConfigChange(
	previous *compozyconfig.Config,
	next *compozyconfig.Config,
) (*deadentity.Service, *settingspkg.ApplyFailure) {
	if a.state.loopTargetHealth == nil || previous.Loops.Breaker == next.Loops.Breaker {
		return nil, nil
	}
	service, err := a.daemon.newLoopTargetHealthService(a.state, next.Loops.Breaker)
	if err == nil {
		return service, nil
	}
	failure := configApplyFailure(
		"loop_target_health",
		diagnosticcontract.CategoryConfig,
		"Loop target breaker policy sync failed",
		err,
	)
	return nil, &failure
}

func (a daemonSettingsRuntimeApplier) rollbackRuntimeDependencies(
	ctx context.Context,
	previous *compozyconfig.Config,
	failures []settingspkg.ApplyFailure,
) []settingspkg.ApplyFailure {
	if a.state.windowManagers != nil {
		if err := a.state.windowManagers.UpdateDefaults(windowManagerDefaults(previous.WindowManager)); err != nil {
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
	next *compozyconfig.Config,
) []settingspkg.ApplyFailure {
	var failures []settingspkg.ApplyFailure
	if a.state.windowManagers != nil {
		if err := a.state.windowManagers.UpdateDefaults(windowManagerDefaults(next.WindowManager)); err != nil {
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

func (a daemonSettingsRuntimeApplier) reconcileExtensionMarketplace(cfg *compozyconfig.Config) {
	if a.state == nil || cfg == nil {
		return
	}
	service, ok := a.state.deps.Extensions.(*daemonExtensionService)
	if !ok || service == nil {
		return
	}
	service.reconcileMarketplaceConfig(cfg.Extensions)
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
		Diagnostic: diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            "config.apply." + subsystem + "_sync_failed",
			Code:          diagnosticcontract.CodeConfigPartialFailure,
			Category:      category,
			Title:         summary,
			Message:       diagnostics.RedactAndBound(err.Error(), 1024),
			Severity:      diagnosticcontract.SeverityError,
			DataFreshness: diagnosticcontract.FreshnessLive,
		},
			diagnostics.WithSuggestedCommand("compozy config reload"),
		),
	}
}
