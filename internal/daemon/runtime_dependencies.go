package daemon

import (
	"context"

	"github.com/compozy/agh/internal/api/core"
)

func (d *Daemon) runtimeDeps(
	ctx context.Context,
	state *bootState,
	sessions SessionManager,
) RuntimeDeps {
	d.initializeDreamRuntime(state, sessions)
	authoredContext := authoredContextRuntimeDeps(ctx, state, sessions)
	var memoryProviders core.MemoryProviderService
	if state.memoryProviderRegistry != nil {
		memoryProviders = daemonMemoryProviderService{registry: state.memoryProviderRegistry}
	}
	roles := roleResolverForState(state)
	return RuntimeDeps{
		Config:              state.cfg,
		HomePaths:           d.homePaths,
		Logger:              state.logger,
		Sessions:            sessions,
		DrainController:     d,
		Bridges:             state.bridges,
		Notifications:       state.notificationPresets,
		Registry:            state.registry,
		SchemaStreams:       newDaemonSchemaStreamStatusReader(state.registry),
		MemoryStore:         state.memoryStore,
		MemoryExtractor:     state.memoryExtractor,
		MemoryProviders:     memoryProviders,
		MemorySessionLedger: newDaemonMemorySessionLedgerService(state, d.now),
		RuntimeMemory:       state.runtimeWorkers.runtimeMemory,
		DeadEntities:        state.deadEntities,
		WorkspaceResolver:   state.workspaceResolver,
		WorkspaceService:    state.workspaceResolver,
		WindowManager:       state.windowManager,
		ModelCatalog:        state.modelCatalog,
		MarketplaceCatalog:  state.marketplace,
		AgentCatalog: agentCatalogDependency(state.agentCatalog, agentSidecarCatalogs{
			soul:      state.soulCatalog,
			heartbeat: state.heartbeatCatalog,
		}),
		AgentContext:        state.situationContext,
		AgentDefinitionSync: state.agentSkillResources,
		SoulAuthoring:       authoredContext.SoulAuthoring,
		SoulHistoryPurger:   authoredContext.SoulHistoryPurger,
		SoulRefresher:       authoredContext.SoulRefresher,
		HeartbeatAuthor:     authoredContext.HeartbeatAuthoring,
		HeartbeatPurger:     authoredContext.HeartbeatPurger,
		HeartbeatStatus:     authoredContext.HeartbeatStatus,
		HeartbeatWake:       authoredContext.HeartbeatWake,
		SessionHealth:       authoredContext.SessionHealth,
		WakeEvents:          authoredContext.WakeEvents,
		CoordinatorRole:     coordinatorRoleResolverFor(roles),
		Roles:               roles,
		SkillsRegistry:      skillsRegistryAPI(state.skillsRegistry),
		ToolRegistry:        state.toolRegistry,
		Toolsets:            state.toolsets,
		ToolApprovals:       state.toolApprovals,
		ApprovalGrants:      state.deps.ApprovalGrants,
		Clarify:             state.clarify,
		HostedMCP:           state.hostedMCP,
		MCPHostAPI:          newMCPHostAPIRuntimeInvoker(state.currentExtensionRuntime),
		DreamTrigger:        dreamTriggerFromRuntime(state.dreamRuntime),
		Vault:               state.providerVault,
		StartedAt:           state.startedAt,
	}
}
