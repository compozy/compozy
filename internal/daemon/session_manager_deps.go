package daemon

import (
	"log/slog"

	"github.com/compozy/compozy/internal/admission"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/sandbox"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/toolruntime"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// SessionManagerDeps captures the composition-root dependencies needed to create a session manager.
type SessionManagerDeps struct {
	HomePaths              compozyconfig.HomePaths
	Logger                 *slog.Logger
	Notifier               session.Notifier
	Hooks                  session.HookSet
	PromptAssembler        session.PromptAssembler
	StartupPromptOverlay   session.StartupPromptOverlay
	PromptInputAugmenter   session.PromptInputAugmenter
	CommandService         session.CommandService
	WorkAdmission          admission.Checker
	MemoryStore            *memory.Store
	LedgerMaterializer     session.LedgerMaterializer
	AgentResolver          session.AgentResolver
	SkillRegistry          session.SkillRegistry
	MCPResolver            session.MCPResolver
	WorkspaceResolver      workspacepkg.RuntimeResolver
	WorktreeResolver       session.SessionWorktreeResolver
	ParticipationResolver  participation.Resolver
	SandboxRegistry        *sandbox.Registry
	SessionSupervision     compozyconfig.SessionSupervisionConfig
	SessionBusyInput       compozyconfig.SessionBusyInputConfig
	SessionCompaction      compozyconfig.SessionCompactionConfig
	SessionInputQueue      store.SessionInputQueueStore
	SessionPromptAdmission store.SessionPromptAdmissionStore
	SessionHealthConfig    compozyconfig.HeartbeatConfig
	SessionCatalog         store.SessionCatalog
	ProcessRegistry        *toolruntime.Registry
	HostedMCP              session.HostedMCPLauncher
	ProviderSecrets        session.ProviderSecretResolver
	ModelCatalog           modelcatalog.Service
	SoulStore              session.SoulSnapshotStore
	SoulRunChecker         session.SoulRunActivityChecker
	SessionHealthStore     session.HealthStore
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
		CommandService:       state.commandService,
		WorkAdmission:        &d.admission,
		MemoryStore:          state.memoryStore,
		LedgerMaterializer:   state.ledgerMaterializer,
		AgentResolver: agentCatalogDependency(state.agentCatalog, agentSidecarCatalogs{
			soul:      state.soulCatalog,
			heartbeat: state.heartbeatCatalog,
		}),
		SkillRegistry:          skillRegistryDependency(state.skillsRegistry),
		MCPResolver:            mcpResolverDependency(state.mcpResolver),
		WorkspaceResolver:      state.workspaceResolver,
		WorktreeResolver:       daemonSessionWorktreeResolver{state: state},
		ParticipationResolver:  state.participationResolver,
		SandboxRegistry:        state.sandboxRegistry,
		SessionSupervision:     state.cfg.Session.Supervision,
		SessionBusyInput:       state.cfg.Session.BusyInput,
		SessionCompaction:      state.cfg.Session.Compaction,
		SessionInputQueue:      sessionInputQueueStoreDependency(state.registry),
		SessionPromptAdmission: sessionPromptAdmissionStoreDependency(state.registry),
		SessionHealthConfig:    state.cfg.Agents.Heartbeat,
		SessionCatalog:         state.registry,
		ProcessRegistry:        state.processRegistry,
		HostedMCP:              hostedMCPLauncher(state.hostedMCP),
		ProviderSecrets:        sessionProviderVaultDependency(state.providerVault),
		ModelCatalog:           state.modelCatalog,
		SoulStore:              soulSnapshotStoreDependency(state.registry),
		SoulRunChecker:         soulRunActivityCheckerDependency(state.registry),
		SessionHealthStore:     sessionHealthStoreDependency(state.registry),
	}
}
