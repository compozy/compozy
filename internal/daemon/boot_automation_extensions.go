package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/extensions/connectivity/tailscale"
	forgegithub "github.com/compozy/compozy/extensions/forge/github"
	speccycle "github.com/compozy/compozy/extensions/spec-cycle"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/resources"
)

func (d *Daemon) bootExtensions(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state == nil || state.registry == nil {
		return nil
	}

	dbSource, ok := state.registry.(extensionDBSource)
	if !ok || dbSource.DB() == nil {
		state.logger.Warn(
			"daemon: skipping extensions because global registry does not expose a SQL database handle",
		)
		return nil
	}

	extRegistry := extensionpkg.NewRegistry(dbSource.DB())
	if err := extensionpkg.ReconcileManagedExtensionArtifacts(d.homePaths, extRegistry); err != nil {
		return fmt.Errorf("daemon: reconcile managed extension artifacts: %w", err)
	}
	if err := speccycle.EnsureManagedInstall(d.homePaths, extRegistry); err != nil {
		return fmt.Errorf("daemon: enroll spec-cycle extension: %w", err)
	}
	if err := tailscale.EnsureManagedInstall(d.homePaths, extRegistry); err != nil {
		return fmt.Errorf("daemon: enroll Tailscale connectivity extension: %w", err)
	}
	if err := forgegithub.EnsureManagedInstall(d.homePaths, extRegistry); err != nil {
		return fmt.Errorf("daemon: enroll GitHub forge extension: %w", err)
	}
	if err := d.configureExtensionResourcePublishers(state, extRegistry); err != nil {
		return err
	}
	manager := d.newExtensionManager(d.extensionManagerDeps(state, extRegistry))
	if manager == nil {
		state.logger.Warn("daemon: extension manager factory returned nil; skipping extensions")
		return syncExtensionResourcePublishers(ctx, state)
	}

	cleanup.add(func(ctx context.Context) error {
		return manager.Stop(ctx)
	})

	if err := ctx.Err(); err != nil {
		return err
	}
	if err := manager.Start(ctx); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
			return err
		}

		if extensionRuntimeHasRegisteredEntries(ctx, extRegistry, manager) {
			state.logger.Error(
				"daemon: extension manager start failed; continuing with healthy extensions only",
				"error",
				err,
			)
		} else {
			state.logger.Error("daemon: extension manager start failed; continuing without blocking boot", "error", err)
		}
	}
	if state.bridges != nil {
		state.bridges.setExtensionRuntime(manager)
		state.bridges.startTargetDirectoryRefresh(ctx)
	}
	state.setExtensionRuntime(manager)
	d.attachExtensionRuntime(ctx, state, extRegistry, manager)

	return nil
}

func (d *Daemon) configureExtensionResourcePublishers(
	state *bootState,
	extRegistry *extensionpkg.Registry,
) error {
	agentSkillResources, err := d.newAgentSkillPublisher(state, extRegistry)
	if err != nil {
		return err
	}
	wireAgentSkillResources(state, agentSkillResources)
	toolMCPResources, err := d.newToolMCPPublisher(state, extRegistry)
	if err != nil {
		return err
	}
	state.toolMCPResources = toolMCPResources
	loopResources, err := d.newLoopPublisher(state, extRegistry)
	if err != nil {
		return err
	}
	state.loopResources = loopResources
	extensionKitResources, err := d.newExtensionKitResourcePublisher(state, extRegistry)
	if err != nil {
		return err
	}
	state.extensionKitResources = extensionKitResources
	return nil
}

type extensionResourcePublisher interface {
	Sync(context.Context) error
}

type extensionResourcePublisherEntry struct {
	name      string
	publisher extensionResourcePublisher
}

func extensionResourcePublishers(state *bootState) []extensionResourcePublisherEntry {
	if state == nil {
		return nil
	}
	return []extensionResourcePublisherEntry{
		{name: "agent/skill", publisher: state.agentSkillResources},
		{name: "hook bindings", publisher: state.hookBindings},
		{name: "tool/MCP", publisher: state.toolMCPResources},
		{name: "loops", publisher: state.loopResources},
		{name: "extension kit", publisher: state.extensionKitResources},
	}
}

func syncExtensionResourcePublishers(ctx context.Context, state *bootState) error {
	for _, entry := range extensionResourcePublishers(state) {
		if entry.publisher == nil {
			continue
		}
		if err := entry.publisher.Sync(ctx); err != nil {
			return fmt.Errorf("daemon: sync %s resources: %w", entry.name, err)
		}
	}
	return nil
}

func syncWorkspaceDerivedResources(ctx context.Context, state *bootState) error {
	return syncExtensionResourcePublishers(ctx, state)
}

func resourceRawStore(kernel *resources.Kernel) resources.RawStore {
	if kernel == nil {
		return nil
	}
	return kernel
}

func resourceSourceSessions(kernel *resources.Kernel) resources.SourceSessionManager {
	if kernel == nil {
		return nil
	}
	return kernel
}
