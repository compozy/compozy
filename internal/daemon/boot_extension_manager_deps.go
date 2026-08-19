package daemon

import (
	"context"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/resources"
)

func (d *Daemon) extensionManagerDeps(
	state *bootState,
	extRegistry *extensionpkg.Registry,
) extensionManagerDeps {
	envBindings := state.extensionEnvBindings
	if envBindings == nil {
		if store, ok := any(state.registry).(extensionpkg.EnvBindingStore); ok {
			envBindings = store
		}
	}
	return extensionManagerDeps{
		Registry:   extRegistry,
		HomePaths:  d.homePaths,
		Extensions: state.cfg.Extensions,
		Sessions:   state.sessions,
		Clarify:    state.clarify,
		Automation: func() extensionpkg.HostAPIAutomationManager {
			return state.automation
		},
		Tasks:                  state.deps.Tasks,
		Network:                state.deps.Network,
		NetworkStore:           state.registry,
		ModelCatalog:           state.modelCatalog,
		MemoryStore:            state.memoryStore,
		MemoryProviderRegistry: state.memoryProviderRegistry,
		Observer:               state.observer,
		SkillsRegistry:         state.skillsRegistry,
		WorkspaceResolver:      state.workspaceResolver,
		Logger:                 state.logger,
		BridgeRegistry:         state.bridges,
		BridgeDedupStore:       bridgeRuntimeDedupStore(state.bridges),
		BridgeBroker:           bridgeRuntimeBroker(state.bridges),
		BridgeRuntime:          state.bridges,
		ResourceStore:          resourceRawStore(state.resourceKernel),
		SourceSessions:         resourceSourceSessions(state.resourceKernel),
		ResourceCodecs:         state.resourceCodecs,
		ResourceTrigger: func(
			ctx context.Context,
			kind resources.ResourceKind,
			reason resources.ReconcileReason,
		) error {
			if state.resourceReconcile == nil {
				return nil
			}
			_, err := state.resourceReconcile.Trigger(ctx, kind, reason)
			return err
		},
		SoulAuthoring:   state.deps.SoulAuthoring,
		SoulRefresher:   state.deps.SoulRefresher,
		HeartbeatAuthor: state.deps.HeartbeatAuthor,
		HeartbeatStatus: state.deps.HeartbeatStatus,
		HeartbeatWake:   state.deps.HeartbeatWake,
		SessionHealth:   state.deps.SessionHealth,
		WakeEvents:      state.deps.WakeEvents,
		ProcessRegistry: state.processRegistry,
		SecretResolver:  state.providerVault,
		EnvBindings:     envBindings,
		LifecycleEvents: extensionPaletteLifecycleEventSink{
			primary: extensionLifecycleEventStoreSink{
				writer: extensionEventSummaryStore(state.registry),
				now:    d.now,
			},
			notifier: newExtensionPaletteNotifier(state),
		},
		CompozyExecutable: d.executable,
	}
}
