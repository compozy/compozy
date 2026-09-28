package daemon

import "github.com/compozy/compozy/internal/api/core"

func (d *Daemon) supportBundleSnapshotHandlers(state *bootState) *core.BaseHandlers {
	return core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName: "support",
		Sessions:      state.deps.Sessions,
		Tasks:         state.deps.Tasks,

		Observer:      state.deps.Observer,
		SchemaStreams: state.deps.SchemaStreams,
		Resources:     state.deps.Resources,
		Automation:    state.deps.Automation,

		Settings:            state.deps.Settings,
		SettingsRestart:     state.deps.SettingsRestart,
		SettingsUpdate:      state.deps.SettingsUpdate,
		Vault:               state.deps.Vault,
		Workspaces:          state.deps.WorkspaceService,
		AgentCatalog:        state.deps.AgentCatalog,
		ModelCatalog:        state.deps.ModelCatalog,
		AgentContextService: state.deps.AgentContext,
		CoordinatorRole:     state.deps.CoordinatorRole,
		SoulAuthoring:       state.deps.SoulAuthoring,
		SoulRefresher:       state.deps.SoulRefresher,
		HeartbeatAuthoring:  state.deps.HeartbeatAuthor,
		HeartbeatStatus:     state.deps.HeartbeatStatus,
		HeartbeatWake:       state.deps.HeartbeatWake,
		SessionHealth:       state.deps.SessionHealth,
		HeartbeatWakeEvents: state.deps.WakeEvents,
		SkillsRegistry:      state.deps.SkillsRegistry,
		MemoryStore:         state.deps.MemoryStore,
		DreamTrigger:        state.deps.DreamTrigger,
		MemoryExtractor:     state.deps.MemoryExtractor,
		MemoryProviders:     state.deps.MemoryProviders,
		MemorySessionLedger: state.deps.MemorySessionLedger,
		HomePaths:           d.homePaths,
		Config:              state.cfg,
		Logger:              state.logger,
		StartedAt:           state.startedAt,
		Now:                 d.now,
	})
}
