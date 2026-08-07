package daemon

import (
	core "github.com/compozy/compozy/internal/api/core"
	skillmarketplace "github.com/compozy/compozy/internal/skills/marketplace"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (d *Daemon) nativeToolsDeps(
	state *bootState,
	registryRef func() toolspkg.Registry,
) daemonNativeToolsDeps {
	marketplaceSkills := skillmarketplace.NewService(
		d.homePaths,
		state.cfg.Skills,
		skillmarketplace.WithLogger(state.logger),
		skillmarketplace.WithNow(d.now),
	)
	return daemonNativeToolsDeps{
		Registry:                   registryRef,
		ToolArtifacts:              state.toolArtifacts,
		Config:                     state.cfg,
		Skills:                     skillsRegistryAPI(state.skillsRegistry),
		Sessions:                   state.sessions,
		Workspaces:                 state.workspaceResolver,
		WorkspaceResolver:          state.workspaceResolver,
		ModelCatalog:               state.deps.ModelCatalog,
		MarketplaceCatalog:         state.deps.MarketplaceCatalog,
		MarketplaceSkills:          marketplaceSkills,
		MarketplaceInstalledSkills: marketplaceSkills,
		Settings:                   func() core.SettingsService { return state.deps.Settings },
		Network:                    state.deps.Network,
		NetworkStore:               state.registry,
		NetworkUsage:               state.registry,
		Tasks:                      state.deps.Tasks,
		MemoryStore:                state.memoryStore,
		MemoryToolWrites:           state.memoryExtractor,
		DreamTrigger:               state.deps.DreamTrigger,
		MemoryExtractor:            state.deps.MemoryExtractor,
		MemoryProviders:            state.deps.MemoryProviders,
		MemorySessionLedger:        state.deps.MemorySessionLedger,
		Bridges:                    state.deps.Bridges,
		Gateway:                    state.deps.Gateway,
		GatewayPermissionMode:      nativeGatewayPermissionModeSource(state.sessions),
		HomePaths:                  d.homePaths,
		Observer:                   state.observer,
		HookBindings:               state.hookBindings,
		AgentCatalog: agentCatalogDependency(state.agentCatalog, agentSidecarCatalogs{
			soul:      state.soulCatalog,
			heartbeat: state.heartbeatCatalog,
		}),
		HeartbeatStatus: state.deps.HeartbeatStatus,
		HeartbeatWake:   state.deps.HeartbeatWake,
		SessionHealth:   state.deps.SessionHealth,
		WakeEvents:      state.deps.WakeEvents,
		Automation:      state.deps.Automation,
		AutomationRuntime: func() core.AutomationManager {
			return state.deps.Automation
		},
		ExtensionRegistry: extensionRegistryDependency(state.registry),
		Extensions: func() core.ExtensionService {
			return state.deps.Extensions
		},
		ExtensionRuntime: state.currentExtensionRuntime,
		ExtensionConfig:  state.cfg.Extensions,
		ExtensionEvents:  extensionEventSummaryStore(state.registry),
		ExtensionSecrets: state.providerVault,
		AgentSkillsRuntime: func() agentSkillPublisher {
			return state.agentSkillResources
		},
		ToolMCP:        state.toolMCPResources,
		ApprovalGrants: state.deps.ApprovalGrants,
		Clarify: func() toolspkg.ClarifyBroker {
			return state.clarify
		},
		LoopResources: state.loopResources,
		Loops: func() core.LoopService {
			return state.deps.Loops
		},
		Resources:     state.deps.Resources,
		WindowManager: state.windowManager,
	}
}
