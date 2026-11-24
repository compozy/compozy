package daemon

import (
	"context"
	"errors"
	"fmt"

	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/resources"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/jonboulle/clockwork"
)

// WithAutomationSchedulerClock sets the clock for scheduled automation.
func WithAutomationSchedulerClock(clock clockwork.Clock) Option {
	return func(d *Daemon) { d.automationClock = clock }
}

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

	manager, err := d.newAutomationManager(d.automationManagerDependencies(state, store))
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
	wireAutomationObservers(state, manager)
	state.automation = manager
	state.deps.Automation = manager
	return nil
}

func (d *Daemon) automationManagerDependencies(
	state *bootState,
	store automationpkg.Store,
) automationManagerDeps {
	var tasks taskpkg.Manager
	var waitForSignIn func(context.Context) error
	if service := state.runtimeWorkers.account; service != nil {
		waitForSignIn = service.WaitForWorkAdmission
	}
	if state.tasks != nil {
		tasks = state.tasks.manager
	}
	return automationManagerDeps{
		InstanceEvents:    automationInstanceEventSink(state),
		Store:             store,
		Sessions:          state.sessions,
		Tasks:             tasks,
		WorkspaceResolver: state.workspaceResolver,
		Config:            state.cfg.Automation,
		Hooks:             state.hooks,
		WebhookSecrets:    state.providerVault,
		Logger:            state.logger.With("component", "automation"),
		WaitForSignIn:     waitForSignIn,

		ResourceStore:     resourceRawStore(state.resourceKernel),
		ResourceCodecs:    state.resourceCodecs,
		LoopCatalog:       state.loopCatalog,
		ToolRegistry:      state.deps.ToolRegistry,
		LoopInputEntities: daemonLoopInputEntityCatalog{state: state},
		Profiles:          state.profiles,
		LoopRuntimeCatalog: loopRuntimeCatalogFactory{
			homePaths: d.homePaths, workspaceResolver: state.workspaceResolver,
		},
		ResourceTrigger: func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
			if state.resourceReconcile == nil {
				return nil
			}
			_, err := state.resourceReconcile.Trigger(ctx, kind, reason)
			return err
		},
	}
}

func wireAutomationObservers(state *bootState, manager automationRuntime) {
	if state.lifecycleObservers != nil {
		state.lifecycleObservers.Add(manager.SessionObserver())
	}
	if state.hookTelemetrySinks != nil {
		state.hookTelemetrySinks.Add(manager.HookTelemetrySink())
	}
}
