package daemon

import (
	"context"

	extensionpkg "github.com/compozy/agh/internal/extension"
)

func (d *Daemon) attachExtensionRuntime(
	ctx context.Context,
	state *bootState,
	extRegistry *extensionpkg.Registry,
	manager extensionRuntime,
) {
	state.deps.Extensions = newDaemonExtensionService(
		extRegistry,
		manager,
		state.hookBindings,
		state.agentSkillResources,
		state.toolMCPResources,
		state.bundleResources,
		state.loopResources,
		d.homePaths,
		state.logger,
		d.now,
		withDaemonExtensionMarketplace(state.cfg.Extensions.Marketplace, nil),
		withDaemonExtensionCatalog(state.marketplace),
		withDaemonExtensionEventWriter(extensionEventSummaryStore(state.registry)),
	)
	if state.agentSkillResources != nil {
		if err := state.agentSkillResources.Sync(ctx); err != nil {
			state.logger.Error(
				"daemon: sync agent/skill resources after extension boot failed",
				"error",
				err,
			)
		}
	}
	if state.hookBindings != nil {
		if err := state.hookBindings.Sync(ctx); err != nil {
			state.logger.Error(
				"daemon: sync hook bindings after extension boot failed",
				"error",
				err,
			)
		}
	}
	if state.toolMCPResources != nil {
		if err := state.toolMCPResources.Sync(ctx); err != nil {
			state.logger.Error(
				"daemon: sync tool/mcp resources after extension boot failed",
				"error",
				err,
			)
		}
	}
	if state.bundleResources != nil {
		if err := state.bundleResources.Sync(ctx); err != nil {
			state.logger.Error(
				"daemon: sync bundle resources after extension boot failed",
				"error",
				err,
			)
		}
	}
	if state.loopResources != nil {
		if err := state.loopResources.Sync(ctx); err != nil {
			state.logger.Error(
				"daemon: sync loop resources after extension boot failed",
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
