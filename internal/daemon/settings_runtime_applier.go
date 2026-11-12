package daemon

import (
	"context"
	"reflect"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/deadentity"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

type daemonSettingsRuntimeApplier struct {
	daemon *Daemon
	state  *bootState
}

func (a daemonSettingsRuntimeApplier) ApplyActiveConfig(
	ctx context.Context,
	snap *compozyconfig.Config,
) []settingspkg.ApplyFailure {
	if a.daemon == nil || a.state == nil || snap == nil {
		return nil
	}
	next := *snap

	previous, err := a.previousActiveConfig(ctx)
	if err != nil {
		return []settingspkg.ApplyFailure{
			configApplyFailure(
				"active_config",
				"Active config snapshot unavailable",
				err,
			),
		}
	}

	failures := a.applyRuntimeDependencies(ctx, &previous, &next)
	if len(failures) > 0 {
		return a.rollbackRuntimeDependencies(ctx, &previous, &next, failures)
	}
	nextLoopTargetHealth, failure := a.prepareLoopTargetHealthConfigChange(&previous, &next)
	if failure != nil {
		return a.rollbackRuntimeDependencies(ctx, &previous, &next, []settingspkg.ApplyFailure{*failure})
	}
	if failures := a.applyGatewayTuningChange(&previous, &next); len(failures) > 0 {
		failures = a.rollbackRuntimeDependencies(ctx, &previous, &next, failures)
		return a.rollbackGateway(ctx, &previous, &next, failures)
	}
	if failures := a.applyGatewayCeilingChange(ctx, &previous, &next); len(failures) > 0 {
		failures = a.rollbackRuntimeDependencies(ctx, &previous, &next, failures)
		return a.rollbackGateway(ctx, &previous, &next, failures)
	}
	if failure := a.applyAttentionConfigChange(&previous, &next); failure != nil {
		failures := a.rollbackRuntimeDependencies(ctx, &previous, &next, []settingspkg.ApplyFailure{*failure})
		return a.rollbackGateway(ctx, &previous, &next, failures)
	}
	if failures := a.applySkillSourceConfigChange(ctx, &previous, &next); len(failures) > 0 {
		if rollback := a.applyAttentionConfigChange(&next, &previous); rollback != nil {
			failures = append(failures, *rollback)
		}
		failures = a.rollbackRuntimeDependencies(ctx, &previous, &next, failures)
		return a.rollbackGateway(ctx, &previous, &next, failures)
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
	a.daemon.providerAuthVerdicts.Forget(changedProviderNames(&previous, &next)...)
	if a.state.providerUsage != nil {
		a.state.providerUsage.ApplyConsent(ctx, previous.ProviderUsage, next.ProviderUsage)
	}
	if a.state.toolProjectionEpoch != nil {
		a.state.toolProjectionEpoch.Advance()
	}
	// Infrequent config application compares values rather than cloned pointer identities.
	if previous.Defaults != next.Defaults || !reflect.DeepEqual(previous.Agents, next.Agents) ||
		!reflect.DeepEqual(previous.Providers, next.Providers) {
		if checks := a.state.runtimeWorkers.proactiveChecks; checks != nil {
			if err := checks.RecoverConfiguration(ctx, nil); err != nil {
				a.state.logger.ErrorContext(ctx, "daemon: recover config rejected checks", "error", err)
			}
		}
	}
	return nil
}

// previousActiveConfig uses the fully projected Settings snapshot, including user sidecars at boot.
func (a daemonSettingsRuntimeApplier) previousActiveConfig(ctx context.Context) (compozyconfig.Config, error) {
	if active, ok := a.state.deps.Settings.(interface {
		ActiveConfig(context.Context) (compozyconfig.Config, error)
	}); ok {
		return active.ActiveConfig(ctx)
	}
	a.daemon.mu.Lock()
	defer a.daemon.mu.Unlock()
	return a.state.cfg, nil
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

func (a daemonSettingsRuntimeApplier) rollbackGateway(
	ctx context.Context,
	previous *compozyconfig.Config,
	next *compozyconfig.Config,
	failures []settingspkg.ApplyFailure,
) []settingspkg.ApplyFailure {
	failures = a.rollbackGatewayTuning(previous, next, failures)
	failures = a.rollbackGatewayCeiling(ctx, previous, next, failures)
	return failures
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
				"Gateway ceiling rollback failed",
				err,
			))
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
			"Gateway ceiling sync failed",
			err,
		)}
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
		"Loop target breaker policy sync failed",
		err,
	)
	return nil, &failure
}

func marketplaceConfigChanged(previous, next *compozyconfig.Config) bool {
	// Configuration application is infrequent; deep comparison includes optional source choices
	// without treating cloned pointers as changes and replaying a stale snapshot over live sources.
	return !reflect.DeepEqual(previous.Marketplace, next.Marketplace)
}

func modelCatalogConfigChanged(previous, next *compozyconfig.Config) bool {
	if previous == nil || next == nil {
		return previous != next
	}
	return !reflect.DeepEqual(previous.Providers, next.Providers) ||
		!reflect.DeepEqual(previous.ModelCatalog, next.ModelCatalog)
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
