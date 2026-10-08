package daemon

import (
	"log/slog"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/admission"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/modelcatalog"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/toolruntime"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// SessionManagerDeps captures the composition-root dependencies needed to create a session manager.
type SessionManagerDeps struct {
	HomePaths               compozyconfig.HomePaths
	Logger                  *slog.Logger
	Notifier                session.Notifier
	SpawnWakeNotifier       session.SpawnWakeNotifier
	Hooks                   session.HookSet
	PromptAssembler         session.PromptAssembler
	StartupPromptOverlay    session.StartupPromptOverlay
	PromptInputAugmenter    session.PromptInputAugmenter
	CommandService          session.CommandService
	WorkAdmission           admission.Checker
	MemoryStore             *memory.Store
	AgentResolver           session.AgentResolver
	SkillRegistry           session.SkillRegistry
	MCPResolver             session.MCPResolver
	WorkspaceResolver       workspacepkg.RuntimeResolver
	WorktreeResolver        session.WorktreeResolver
	WindowReconciler        session.WindowReconciler
	SessionSupervision      compozyconfig.SessionSupervisionConfig
	SessionStop             compozyconfig.SessionStopConfig
	SessionBusyInput        compozyconfig.SessionBusyInputConfig
	SessionDerive           compozyconfig.SessionDeriveConfig
	SessionInputQueue       store.SessionInputQueueStore
	SessionPromptAdmission  store.SessionPromptAdmissionStore
	SessionAttachments      session.AttachmentOpener
	SessionHealthConfig     compozyconfig.HeartbeatConfig
	AttentionConfig         compozyconfig.AttentionConfig
	AttentionWorkspaceMutes session.AttentionWorkspaceMuteReader
	SessionCatalog          store.SessionCatalog
	EventLedger             store.EventSummaryStore
	ProcessRegistry         *toolruntime.Registry
	Terminals               acp.TerminalHost
	HostedMCP               session.HostedMCPLauncher
	ProviderSecrets         session.ProviderSecretResolver
	ProfileNames            session.ProfileNameResolver
	ModelCatalog            modelcatalog.Service
	SoulStore               session.SoulSnapshotStore
	SoulRunChecker          session.SoulRunActivityChecker
	SessionHealthStore      session.HealthStore
}

func (d *Daemon) sessionManagerDeps(state *bootState) SessionManagerDeps {
	reconciler := windowManagerSessionReconcilerDependency(state.windowManagers)
	state.sessionWindowReconciler = reconciler
	return SessionManagerDeps{
		HomePaths:         d.homePaths,
		Logger:            state.logger,
		Notifier:          d.sessionNotifier(state),
		SpawnWakeNotifier: state.sessionWakeBridge,
		Hooks: session.HookSet{
			Session:         state.notifier,
			RuntimeRecovery: state.notifier,
			Prompt:          state.notifier,
			Events:          state.notifier,
			Agent:           state.notifier,
			Conversation:    state.notifier,
			Tools:           state.notifier,
			Compaction:      state.notifier,
			Spawn:           state.notifier,
			AuthoredContext: state.notifier,
			Attention:       state.notifier,
		},
		PromptAssembler:      state.promptAssembler,
		StartupPromptOverlay: state.startupOverlay,
		PromptInputAugmenter: state.promptAugmenter,
		CommandService:       state.commandService,
		WorkAdmission:        &d.admission,
		MemoryStore:          state.memoryStore,
		AgentResolver: agentCatalogDependency(state.agentCatalog, agentSidecarCatalogs{
			soul:      state.soulCatalog,
			heartbeat: state.heartbeatCatalog,
		}),
		SkillRegistry:           skillRegistryDependency(state.skillsRegistry),
		MCPResolver:             mcpResolverDependency(state.mcpResolver),
		WorkspaceResolver:       state.workspaceResolver,
		WorktreeResolver:        daemonSessionWorktreeResolver{state: state},
		WindowReconciler:        reconciler,
		SessionSupervision:      state.cfg.Session.Supervision,
		SessionStop:             state.cfg.Session.Stop,
		SessionBusyInput:        state.cfg.Session.BusyInput,
		SessionDerive:           state.cfg.Session.Derive,
		SessionInputQueue:       sessionInputQueueStoreDependency(state.registry),
		SessionPromptAdmission:  sessionPromptAdmissionStoreDependency(state.registry),
		SessionAttachments:      state.sessionAttachments,
		SessionHealthConfig:     state.cfg.Agents.Heartbeat,
		AttentionConfig:         state.cfg.Attention,
		AttentionWorkspaceMutes: state.registry,
		SessionCatalog:          state.registry,
		EventLedger:             sessionEventLedgerDependency(state.registry),
		ProcessRegistry:         state.processRegistry,
		Terminals:               state.terminals,
		HostedMCP:               hostedMCPLauncher(state.hostedMCP),
		ProviderSecrets:         sessionProviderVaultDependency(state.providerVault),
		ProfileNames:            state.profiles,
		ModelCatalog:            state.modelCatalog,
		SoulStore:               soulSnapshotStoreDependency(state.registry),
		SoulRunChecker:          soulRunActivityCheckerDependency(state.registry),
		SessionHealthStore:      sessionHealthStoreDependency(state.registry),
	}
}

// sessionEventLedgerDependency exposes the daemon ledger that records
// session.fallback.used before each session-owned fallback attempt.
func sessionEventLedgerDependency(value any) store.EventSummaryStore {
	ledger, ok := value.(store.EventSummaryStore)
	if !ok {
		return nil
	}
	return ledger
}
