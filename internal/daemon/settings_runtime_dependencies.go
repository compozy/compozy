package daemon

import (
	"context"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

type settingsSecretRefResolver interface {
	ResolveRef(context.Context, string) (string, error)
}

func settingsRuntimeFunctions(d *Daemon) (func() time.Time, func() int, func() Info) {
	now := time.Now
	pid := func() int { return 0 }
	info := func() Info { return Info{} }
	if d == nil {
		return now, pid, info
	}
	if d.now != nil {
		now = d.now
	}
	if d.pid != nil {
		pid = d.pid
	}
	return now, pid, d.settingsInfoSnapshot
}

func settingsMCPAuthDependencies(
	state *bootState,
) (mcpauth.TokenStore, mcpauth.RegistrationStore, mcpauth.SecretRefResolver, settingsSecretRefResolver) {
	var tokenStore mcpauth.TokenStore
	if store, ok := state.registry.(mcpauth.TokenStore); ok {
		tokenStore = store
	}
	var registrationStore mcpauth.RegistrationStore
	if store, ok := state.registry.(mcpauth.RegistrationStore); ok {
		registrationStore = store
	}
	if state.providerVault == nil {
		return tokenStore, registrationStore, nil, nil
	}
	secretResolver := func(ctx context.Context, ref string) (string, error) {
		value, err := state.providerVault.ResolveRef(ctx, ref)
		if err != nil {
			return "", err
		}
		diagnostics.RegisterDynamicSecret(value)
		return value, nil
	}
	return tokenStore, registrationStore, secretResolver, state.providerVault
}

func (a daemonSettingsRuntimeApplier) rollbackRuntimeDependencies(
	ctx context.Context,
	previous *compozyconfig.Config,
	next *compozyconfig.Config,
	failures []settingspkg.ApplyFailure,
) []settingspkg.ApplyFailure {
	if failure := a.applyBusyInputDefault(next, previous); failure != nil {
		failures = append(failures, *failure)
	}
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
	if a.state.modelCatalog != nil && modelCatalogConfigChanged(previous, next) {
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
	previous *compozyconfig.Config,
	next *compozyconfig.Config,
) []settingspkg.ApplyFailure {
	var failures []settingspkg.ApplyFailure
	if failure := a.applyBusyInputDefault(previous, next); failure != nil {
		failures = append(failures, *failure)
	}
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
	if a.state.modelCatalog != nil && modelCatalogConfigChanged(previous, next) {
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
