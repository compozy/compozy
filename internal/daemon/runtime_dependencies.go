package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/api/core"
)

func (d *Daemon) runtimeDeps(
	ctx context.Context,
	state *bootState,
	sessions SessionManager,
) RuntimeDeps {
	if state.agentProbeConfig == nil {
		state.agentProbeConfig = newAgentProbeConfigState(&state.cfg)
	}
	d.initializeDreamRuntime(state, sessions)
	authoredContext := authoredContextRuntimeDeps(ctx, state, sessions)
	state.runtimeWorkers.authoredHeartbeatWake = authoredContext.wakePrompter
	var memoryProviders core.MemoryProviderService
	if state.memoryProviderRegistry != nil {
		memoryProviders = daemonMemoryProviderService{registry: state.memoryProviderRegistry}
	}
	var worktrees core.WorktreeService
	if state.worktrees != nil {
		worktrees = state.worktrees
	}
	roles := roleResolverForState(state)
	agentCatalog := agentCatalogDependency(state.agentCatalog, agentSidecarCatalogs{
		soul:      state.soulCatalog,
		heartbeat: state.heartbeatCatalog,
	})
	return RuntimeDeps{
		Config:              state.cfg,
		AgentProbeConfig:    state.agentProbeConfig,
		HomePaths:           d.homePaths,
		Logger:              state.logger,
		Sessions:            sessions,
		SessionAttachments:  state.sessionAttachments,
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
		Worktrees:           worktrees,
		WorkspaceAccess:     state.accessPolicy,
		WindowManager:       state.windowManager,
		ModelCatalog:        state.modelCatalog,
		MarketplaceCatalog:  state.marketplace,
		AgentCatalog:        agentCatalog,
		AgentResolver:       agentCatalog,
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
		ApprovalCoordinator: state.approvalCoordinator,
		CmdPalette:          state.cmdPalette,
		ApprovalGrants:      state.deps.ApprovalGrants,
		Clarify:             state.clarify,
		HostedMCP:           state.hostedMCP,
		MCPHostAPI:          newMCPHostAPIRuntimeInvoker(state.currentExtensionRuntime),
		DreamTrigger:        dreamTriggerFromRuntime(state.dreamRuntime),
		Vault:               state.providerVault,
		StartedAt:           state.startedAt,
	}
}
