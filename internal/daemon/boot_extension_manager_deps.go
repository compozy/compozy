package daemon

import (
	"context"

	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/resources"
)

func (d *Daemon) extensionManagerDeps(
	state *bootState,
	extRegistry *extensionpkg.Registry,
) extensionManagerDeps {
	return extensionManagerDeps{
		Registry:   extRegistry,
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
			return state.resourceReconcile.Trigger(ctx, kind, reason)
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
		AGHExecutable:   d.executable,
	}
}
