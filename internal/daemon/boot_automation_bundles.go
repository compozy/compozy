package daemon

import (
	"context"
	"errors"
	"fmt"

	devcycle "github.com/compozy/agh/extensions/dev-cycle"

	automationpkg "github.com/compozy/agh/internal/automation"

	bundlepkg "github.com/compozy/agh/internal/bundles"

	extensionpkg "github.com/compozy/agh/internal/extension"

	"github.com/compozy/agh/internal/resources"

	taskpkg "github.com/compozy/agh/internal/task"
)

func (d *Daemon) bootAutomation(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state == nil {
		return nil
	}
	if !state.cfg.Automation.Enabled {
		state.logger.Info("daemon: automation disabled")
		return nil
	}

	store, ok := state.registry.(automationpkg.Store)
	if !ok {
		return errors.New("daemon: global registry does not implement automation store")
	}
	if d.newAutomationManager == nil {
		return errors.New("daemon: automation manager factory is required")
	}

	var tasks taskpkg.Manager
	if state.tasks != nil {
		tasks = state.tasks.manager
	}

	manager, err := d.newAutomationManager(automationManagerDeps{
		Store:                 store,
		Sessions:              state.sessions,
		Tasks:                 tasks,
		WorkspaceResolver:     state.workspaceResolver,
		Config:                state.cfg.Automation,
		Hooks:                 state.hooks,
		WebhookSecrets:        state.providerVault,
		Logger:                state.logger.With("component", "automation"),
		GlobalWorkspacePath:   d.homePaths.HomeDir,
		ResourceStore:         resourceRawStore(state.resourceKernel),
		ResourceCodecs:        state.resourceCodecs,
		LoopCatalog:           state.loopCatalog,
		ToolRegistry:          state.deps.ToolRegistry,
		ParticipationResolver: state.participationResolver,
		ResourceTrigger: func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
			if state.resourceReconcile == nil {
				return nil
			}
			return state.resourceReconcile.Trigger(ctx, kind, reason)
		},
	})
	if err != nil {
		return fmt.Errorf("daemon: create automation manager: %w", err)
	}
	if manager == nil {
		return errors.New("daemon: automation manager factory returned nil")
	}
	if err := manager.Start(ctx); err != nil {
		return fmt.Errorf("daemon: start automation manager: %w", err)
	}

	cleanup.add(func(ctx context.Context) error {
		return manager.Shutdown(ctx)
	})
	if state.lifecycleObservers != nil {
		state.lifecycleObservers.Add(manager.SessionObserver())
	}
	if state.hookTelemetrySinks != nil {
		state.hookTelemetrySinks.Add(manager.HookTelemetrySink())
	}

	state.automation = manager
	state.deps.Automation = manager
	return nil
}

func (d *Daemon) bootBundles(_ context.Context, state *bootState) error {
	if state == nil {
		return errors.New("daemon: boot bundle state is required")
	}

	dbSource, ok := state.registry.(interface {
		extensionDBSource
	})
	if !ok {
		return nil
	}

	extRegistry := extensionpkg.NewRegistry(dbSource.DB())
	resourceStore, err := newBundleResourceStore(state, d.now)
	if err != nil {
		return err
	}
	if resourceStore == nil {
		return nil
	}
	service := bundlepkg.NewService(
		resourceStore,
		extRegistry,
		func(ctx context.Context, name string) (*extensionpkg.Extension, error) {
			return loadExtensionSnapshot(
				ctx,
				extRegistry,
				state.currentExtensionRuntime(),
				state.logger,
				name,
			)
		},
		bundlepkg.WithWorkspaceResolver(state.workspaceResolver),
		bundlepkg.WithLogger(state.logger),
		bundlepkg.WithNow(d.now),
	)
	if service == nil {
		return nil
	}
	state.bundles = service
	state.deps.Bundles = service
	return nil
}

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
	if err := devcycle.EnsureManagedInstall(d.homePaths, extRegistry); err != nil {
		return fmt.Errorf("daemon: enroll dev-cycle extension: %w", err)
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
	bundleResources, err := d.newBundlePublisher(state, extRegistry)
	if err != nil {
		return err
	}
	state.bundleResources = bundleResources
	loopResources, err := d.newLoopPublisher(state, extRegistry)
	if err != nil {
		return err
	}
	state.loopResources = loopResources
	return nil
}

func syncExtensionResourcePublishers(ctx context.Context, state *bootState) error {
	if state.agentSkillResources != nil {
		if err := state.agentSkillResources.Sync(ctx); err != nil {
			return err
		}
	}
	if state.hookBindings != nil {
		if err := state.hookBindings.Sync(ctx); err != nil {
			return err
		}
	}
	if state.toolMCPResources != nil {
		if err := state.toolMCPResources.Sync(ctx); err != nil {
			return err
		}
	}
	if state.bundleResources != nil {
		if err := state.bundleResources.Sync(ctx); err != nil {
			return err
		}
	}
	if state.loopResources != nil {
		if err := state.loopResources.Sync(ctx); err != nil {
			return err
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
