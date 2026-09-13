package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/api/core"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/marketplace/pluginsource"
)

func (d *Daemon) attachExtensionRuntime(
	ctx context.Context,
	state *bootState,
	extRegistry *extensionpkg.Registry,
	manager extensionRuntime,
) {
	state.deps.Extensions = d.newBootExtensionService(state, extRegistry, manager)
	d.syncExtensionRuntimeConsumers(ctx, state)
}

func (d *Daemon) newBootExtensionService(
	state *bootState,
	extRegistry *extensionpkg.Registry,
	manager extensionRuntime,
) core.ExtensionService {
	var envBindings extensionpkg.EnvBindingLifecycleStore
	if store, ok := any(state.registry).(extensionpkg.EnvBindingLifecycleStore); ok {
		envBindings = store
	}
	var acquirer extensionpkg.MarketplacePackageAcquirer
	var packageCache *pluginsource.PackageCache
	if state.marketplace != nil {
		acquirer = &marketplacePackageAcquirer{resolver: state.marketplace.resolver, logger: state.logger}
		packageCache = state.marketplace.resolver.Cache
	}
	return newDaemonExtensionService(&daemonExtensionServiceDeps{
		Registry:     extRegistry,
		Runtime:      manager,
		HookBindings: state.hookBindings,
		AgentSkill:   state.agentSkillResources,
		ToolMCP:      state.toolMCPResources,
		Loops:        state.loopResources,
		Sessions:     state.sessions,
		Profiles:     state.profiles,
		HomePaths:    d.homePaths,
		Logger:       state.logger,
		Now:          d.now,
		Getenv:       d.getenv,
	},
		withDaemonExtensionMarketplace(state.cfg.Extensions, nil),
		withDaemonExtensionCatalog(state.marketplace),
		withDaemonMarketplacePackageAcquirer(acquirer),
		withDaemonMarketplacePackageCache(packageCache),
		withDaemonExtensionEventWriter(extensionEventSummaryStore(state.registry)),
		withDaemonExtensionWorkspaceResolver(state.workspaceResolver),
		withDaemonExtensionKitPublisher(state.extensionKitResources),
		withDaemonExtensionSecrets(envBindings, state.providerVault),
		withDaemonExtensionInputs(state.extensionInputs),
		withDaemonExtensionMCPAllocations(state.extensionMCP),
		withDaemonExtensionMCPDetails(&extensionMCPDetails{state: state}),
		withDaemonExtensionAutomation(state.automation),
		withDaemonExtensionResources(state.resourceKernel, resourceReconcileActor(), state.resourceCodecs),
		withDaemonExtensionMCPRuntimeHealth(state.mcpRuntimeHealth),
		withDaemonExtensionPaletteNotifier(newExtensionPaletteNotifier(state)),
	)
}

func (d *Daemon) syncExtensionRuntimeConsumers(ctx context.Context, state *bootState) {
	for _, entry := range extensionResourcePublishers(state) {
		if entry.publisher == nil {
			continue
		}
		if err := entry.publisher.Sync(ctx); err != nil {
			state.logger.Error(
				"daemon: sync extension resources after extension boot failed",
				"publisher",
				entry.name,
				"error",
				err,
			)
		}
	}
	if state.hookBindings != nil {
		return
	}
	if rebuildable, ok := state.hooks.(interface {
		Rebuild(context.Context) error
	}); ok {
		if err := rebuildable.Rebuild(ctx); err != nil {
			state.logger.Error("daemon: rebuild hooks after extension boot failed", "error", err)
		}
	}
}
