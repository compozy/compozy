package daemon

import (
	"context"
	"errors"
	"fmt"

	automationpkg "github.com/compozy/compozy/internal/automation"
	memorypkg "github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/resources"
	taskpkg "github.com/compozy/compozy/internal/task"
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
	if state.dreamRuntime != nil {
		memoryObserver := manager.MemoryObserver()
		state.dreamRuntime.SetCompletionObserver(func(
			ctx context.Context,
			result memorypkg.ConsolidationResult,
		) error {
			return memoryObserver.OnMemoryConsolidated(ctx, automationpkg.MemoryConsolidatedEvent{
				WorkspaceID: result.WorkspaceID,
				Timestamp:   result.CompletedAt,
			})
		})
	}
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
