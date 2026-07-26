package udsapi

import core "github.com/compozy/agh/internal/api/core"

func newHandlers(cfg *handlerConfig) *Handlers {
	if cfg == nil {
		cfg = &handlerConfig{}
	}

	if cfg.pollInterval <= 0 {
		cfg.pollInterval = defaultPollInterval
	}

	return &Handlers{
		BaseHandlers: core.NewBaseHandlers(udsCoreHandlerConfig(cfg)),
		Extensions:   cfg.extensions,
		HostedMCP:    cfg.hostedMCP,
		MCPHostAPI:   cfg.mcpHostAPI,
	}
}

func udsCoreHandlerConfig(cfg *handlerConfig) *core.BaseHandlerConfig {
	return &core.BaseHandlerConfig{
		TransportName:                "udsapi",
		MaskInternalErrors:           false,
		IncludeSessionWorkspaceInSSE: true,
		Sessions:                     cfg.sessions,
		SessionAcceptance:            sessionAcceptanceManager(cfg.sessions),
		DrainController:              cfg.drainController,
		SessionCatalog:               cfg.sessionCatalog,
		Tasks:                        cfg.tasks,
		Network:                      cfg.network,
		NetworkStore:                 cfg.networkStore,
		NetworkUsage:                 cfg.networkUsage,
		Coordination:                 cfg.coordination,
		Observer:                     cfg.observer,
		SchemaStreams:                cfg.schemaStreams,
		Resources:                    cfg.resources,
		WindowManager:                cfg.windowManager,
		Extensions:                   cfg.extensions,
		Automation:                   cfg.automation,
		Loops:                        cfg.loops,
		Bridges:                      cfg.bridges,
		Notifications:                cfg.notifications,
		Bundles:                      cfg.bundles,
		SupportBundles:               cfg.supportBundles,
		Tools:                        cfg.tools,
		ToolArtifacts:                cfg.toolArtifacts,
		Toolsets:                     cfg.toolsets,
		ToolApprovals:                cfg.toolApprovals,
		ApprovalGrants:               cfg.approvalGrants,
		Clarify:                      cfg.clarify,
		Settings:                     cfg.settings,
		SettingsRestart:              cfg.settingsRestart,
		SettingsUpdate:               cfg.settingsUpdate,
		Vault:                        cfg.vault,
		Workspaces:                   cfg.workspaces,
		Onboarding:                   cfg.onboarding,
		AgentCatalog:                 cfg.agentCatalog,
		AgentDefinitionSync:          cfg.agentSync,
		ModelCatalog:                 cfg.modelCatalog,
		MarketplaceCatalog:           cfg.marketplaceCatalog,
		AgentContextService:          cfg.agentContext,
		SoulAuthoring:                cfg.soulAuthoring,
		SoulHistoryPurger:            cfg.soulHistoryPurger,
		SoulRefresher:                cfg.soulRefresher,
		HeartbeatAuthoring:           cfg.heartbeatAuthor,
		HeartbeatHistoryPurger:       cfg.heartbeatPurger,
		HeartbeatStatus:              cfg.heartbeatStatus,
		HeartbeatWake:                cfg.heartbeatWake,
		SessionHealth:                cfg.sessionHealth,
		HeartbeatWakeEvents:          cfg.wakeEvents,
		CoordinatorRole:              cfg.coordinatorRole,
		Roles:                        cfg.roles,
		SkillsRegistry:               cfg.skillsRegistry,
		SkillResources:               cfg.skillResources,
		MemoryStore:                  cfg.memoryStore,
		DreamTrigger:                 cfg.dreamTrigger,
		MemoryExtractor:              cfg.memoryExtractor,
		MemoryProviders:              cfg.memoryProviders,
		MemorySessionLedger:          cfg.memoryLedger,
		RuntimeMemory:                cfg.runtimeMemory,
		DeadEntities:                 cfg.deadEntities,
		HomePaths:                    cfg.homePaths,
		Config:                       cfg.config,
		Logger:                       cfg.logger,
		StartedAt:                    cfg.startedAt,
		Now:                          cfg.now,
		PollInterval:                 cfg.pollInterval,
		AgentLoader:                  cfg.agentLoader,
	}
}

func sessionAcceptanceManager(manager core.SessionManager) core.SessionAcceptanceManager {
	accepted, ok := manager.(core.SessionAcceptanceManager)
	if !ok {
		return nil
	}
	return accepted
}

func (h *Handlers) setStreamDone(done <-chan struct{}) {
	if h != nil && h.BaseHandlers != nil {
		h.SetStreamDone(done)
	}
}
