package daemon

import (
	"log/slog"

	"github.com/compozy/agh/internal/admission"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/toolruntime"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// SessionManagerDeps captures the composition-root dependencies needed to create a session manager.
type SessionManagerDeps struct {
	HomePaths             aghconfig.HomePaths
	Logger                *slog.Logger
	Notifier              session.Notifier
	Hooks                 session.HookSet
	PromptAssembler       session.PromptAssembler
	StartupPromptOverlay  session.StartupPromptOverlay
	PromptInputAugmenter  session.PromptInputAugmenter
	WorkAdmission         admission.Checker
	MemoryStore           *memory.Store
	LedgerMaterializer    session.LedgerMaterializer
	AgentResolver         session.AgentResolver
	SkillRegistry         session.SkillRegistry
	MCPResolver           session.MCPResolver
	ModelCatalog          modelcatalog.Service
	WorkspaceResolver     workspacepkg.RuntimeResolver
	ParticipationResolver participation.Resolver
	SandboxRegistry       *sandbox.Registry
	SessionSupervision    aghconfig.SessionSupervisionConfig
	SessionBusyInput      aghconfig.SessionBusyInputConfig
	SessionCompaction     aghconfig.SessionCompactionConfig
	SessionInputQueue     store.SessionInputQueueStore
	SessionHealthConfig   aghconfig.HeartbeatConfig
	SessionCatalog        store.SessionCatalog
	ProcessRegistry       *toolruntime.Registry
	HostedMCP             session.HostedMCPLauncher
	ProviderSecrets       session.ProviderSecretResolver
	SoulStore             session.SoulSnapshotStore
	SoulRunChecker        session.SoulRunActivityChecker
	SessionHealthStore    session.HealthStore
}

func (d *Daemon) sessionManagerDeps(state *bootState) SessionManagerDeps {
	return SessionManagerDeps{
		HomePaths: d.homePaths,
		Logger:    state.logger,
		Notifier:  d.sessionNotifier(state),
		Hooks: session.HookSet{
			Session:         state.notifier,
			Sandbox:         state.notifier,
			Prompt:          state.notifier,
			Events:          state.notifier,
			Agent:           state.notifier,
			Conversation:    state.notifier,
			Tools:           state.notifier,
			Compaction:      state.notifier,
			Spawn:           state.notifier,
			AuthoredContext: state.notifier,
		},
		PromptAssembler:      state.promptAssembler,
		StartupPromptOverlay: state.startupOverlay,
		PromptInputAugmenter: state.promptAugmenter,
		WorkAdmission:        &d.admission,
		MemoryStore:          state.memoryStore,
		LedgerMaterializer:   state.ledgerMaterializer,
		AgentResolver: agentCatalogDependency(state.agentCatalog, agentSidecarCatalogs{
			soul:      state.soulCatalog,
			heartbeat: state.heartbeatCatalog,
		}),
		SkillRegistry:         skillRegistryDependency(state.skillsRegistry),
		MCPResolver:           mcpResolverDependency(state.mcpResolver),
		ModelCatalog:          state.modelCatalog,
		WorkspaceResolver:     state.workspaceResolver,
		ParticipationResolver: state.participationResolver,
		SandboxRegistry:       state.sandboxRegistry,
		SessionSupervision:    state.cfg.Session.Supervision,
		SessionBusyInput:      state.cfg.Session.BusyInput,
		SessionCompaction:     state.cfg.Session.Compaction,
		SessionInputQueue:     sessionInputQueueStoreDependency(state.registry),
		SessionHealthConfig:   state.cfg.Agents.Heartbeat,
		SessionCatalog:        state.registry,
		ProcessRegistry:       state.processRegistry,
		HostedMCP:             hostedMCPLauncher(state.hostedMCP),
		ProviderSecrets:       sessionProviderVaultDependency(state.providerVault),
		SoulStore:             soulSnapshotStoreDependency(state.registry),
		SoulRunChecker:        soulRunActivityCheckerDependency(state.registry),
		SessionHealthStore:    sessionHealthStoreDependency(state.registry),
	}
}
