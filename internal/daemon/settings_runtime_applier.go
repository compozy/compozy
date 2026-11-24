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
	if failure := a.applyAttentionConfigChange(&previous, &next); failure != nil {
		return a.rollbackRuntimeDependencies(ctx, &previous, &next, []settingspkg.ApplyFailure{*failure})
	}
	if failures := a.applySkillSourceConfigChange(ctx, &previous, &next); len(failures) > 0 {
		if rollback := a.applyAttentionConfigChange(&next, &previous); rollback != nil {
			failures = append(failures, *rollback)
		}
		return a.rollbackRuntimeDependencies(ctx, &previous, &next, failures)
	}

	a.daemon.mu.Lock()
	if nextLoopTargetHealth != nil {
		a.state.loopTargetHealth.Swap(nextLoopTargetHealth)
	}
	a.state.deps.Config = next
	a.state.cfg = next
	a.daemon.config = next
	a.daemon.localComputers.Store(new(next.Computers))
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
	a.recoverHeartbeatConfig(ctx, &previous, &next)
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
