package daemon

import (
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/cmdpalette"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/skills"
	skillmarketplace "github.com/compozy/compozy/internal/skills/marketplace"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (d *Daemon) nativeToolsDeps(
	state *bootState,
	registryRef func() toolspkg.Registry,
) daemonNativeToolsDeps {
	agentCatalog := nativeAgentCatalogDependency(state)
	marketplaceSkills := d.nativeMarketplaceSkills(state)
	deps := daemonNativeToolsDeps{
		Logger:                     state.logger,
		Now:                        d.now,
		Registry:                   registryRef,
		CmdPalette:                 func() cmdpalette.Registry { return state.cmdPalette },
		ToolArtifacts:              state.toolArtifacts,
		Config:                     state.cfg,
		Skills:                     skillsRegistryAPI(state.skillsRegistry),
		SkillExposures:             state.registry,
		SkillExposureEvents:        state.registry,
		Sessions:                   state.sessions,
		Profiles:                   state.profiles,
		ProfileManager:             state.profiles,
		SessionAttachments:         state.sessionAttachments,
		Workspaces:                 state.workspaceResolver,
		Worktrees:                  state.worktrees,
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
		TaskClaimHandoff:           taskClaimHandoffForState(state),
		MemoryStore:                state.memoryStore,
		MemoryToolWrites:           state.memoryExtractor,
		DreamTrigger:               state.deps.DreamTrigger,
		MemoryExtractor:            state.deps.MemoryExtractor,
		MemoryProviders:            state.deps.MemoryProviders,
		MemorySessionLedger:        state.deps.MemorySessionLedger,
		Bridges:                    state.deps.Bridges,
		Gateway:                    nativeGatewayDependency(state),
		GatewayPermissionMode:      nativeGatewayPermissionModeSource(state.sessions),
		HomePaths:                  d.homePaths,
		Observer:                   state.observer,
		HookBindings:               state.hookBindings,
		AgentCatalog:               agentCatalog,
		Vault:                      state.providerVault,
		AgentResolver:              agentCatalog,
		HeartbeatStatus:            state.deps.HeartbeatStatus,
		HeartbeatWake:              state.deps.HeartbeatWake,
		SessionHealth:              state.deps.SessionHealth,
		WakeEvents:                 state.deps.WakeEvents,
		Automation:                 state.deps.Automation,
		AutomationRuntime: func() core.AutomationManager {
			return state.deps.Automation
		},
		Calls: func() core.CallsService {
			return state.deps.Calls
		},
	}
	d.populateNativeExtensionDeps(&deps, state)
	return deps
}

func (d *Daemon) populateNativeExtensionDeps(deps *daemonNativeToolsDeps, state *bootState) {
	deps.ExtensionRegistry = extensionRegistryDependency(state.registry)
	deps.Extensions = func() core.ExtensionService { return state.deps.Extensions }
	deps.ExtensionRuntime = state.currentExtensionRuntime
	deps.ExtensionConfig = state.cfg.Extensions
	deps.ExtensionEvents = extensionEventSummaryStore(state.registry)
	deps.ExtensionSecrets = state.providerVault
	deps.AgentSkillsRuntime = func() agentSkillPublisher { return state.agentSkillResources }
	deps.ToolMCP = state.toolMCPResources
	deps.ApprovalGrants = state.deps.ApprovalGrants
	deps.Clarify = func() toolspkg.ClarifyBroker { return state.clarify }
	deps.LoopResources = state.loopResources
	deps.Loops = func() core.LoopService { return state.deps.Loops }
	deps.Resources = state.deps.Resources
	deps.WindowManagers = state.windowManagers
}

func nativeGatewayDependency(state *bootState) func() core.GatewayService {
	return func() core.GatewayService {
		if state.deps.Gateway == nil {
			return nil
		}
		return state.deps.Gateway
	}
}

func nativeAgentCatalogDependency(state *bootState) *resourceAgentCatalog {
	return agentCatalogDependency(state.agentCatalog, agentSidecarCatalogs{
		soul: state.soulCatalog, heartbeat: state.heartbeatCatalog,
	})
}

func (d *Daemon) nativeMarketplaceSkills(state *bootState) *skillmarketplace.Service {
	exposures := skills.NewExposeManager(
		state.registry,
		compozyconfig.ResolveGlobalSkillRoots(&state.cfg.Skills, d.homePaths),
		skills.WithExposureEventStore(state.registry),
		skills.WithExposureLogger(state.logger),
	)
	return skillmarketplace.NewService(
		d.homePaths,
		state.cfg.Skills,
		skillmarketplace.WithLogger(state.logger),
		skillmarketplace.WithNow(d.now),
		skillmarketplace.WithExposureLifecycle(exposures),
	)
}
