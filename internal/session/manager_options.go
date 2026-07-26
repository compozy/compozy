package session

import (
	"context"
	"log/slog"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/store"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// WithSandboxRegistry injects the runtime sandbox provider registry.
func WithSandboxRegistry(registry *sandbox.Registry) Option {
	return func(manager *Manager) {
		manager.sandbox = registry
	}
}

// WithDriver injects the runtime driver used for session lifecycle operations.
func WithDriver(driver AgentDriver) Option {
	return func(manager *Manager) {
		manager.driver = driver
	}
}

// WithStore injects the opener used to create per-session event recorders.
func WithStore(opener StoreOpener) Option {
	return func(manager *Manager) {
		manager.openStore = opener
		if !manager.queryStoreExplicit {
			manager.openQueryStore = func(
				ctx context.Context,
				sessionID string,
				path string,
			) (EventReadCloser, error) {
				return opener(ctx, sessionID, path)
			}
		}
	}
}

// WithQueryStore injects the opener used for stopped-session transcript/event
// reads. Production uses a read-only no-create opener so stale viewers cannot
// recreate events.db during clear/delete races.
func WithQueryStore(opener QueryStoreOpener) Option {
	return func(manager *Manager) {
		manager.openQueryStore = opener
		manager.queryStoreExplicit = true
	}
}

// WithPromptAssembler injects prompt assembly for session startup.
func WithPromptAssembler(assembler PromptAssembler) Option {
	return func(manager *Manager) {
		manager.assembler = assembler
	}
}

// WithLifecycleContext injects the daemon-owned lifecycle context used by background goroutines.
func WithLifecycleContext(ctx context.Context) Option {
	return func(manager *Manager) {
		manager.lifecycleCtx = ctx
	}
}

// WithNotifier injects the async notification fan-out implementation.
func WithNotifier(notifier Notifier) Option {
	return func(manager *Manager) {
		manager.notifier = notifier
	}
}

// WithHookSet injects the grouped hook dispatch surface used by the session
// manager for lifecycle and runtime hook points.
func WithHookSet(hooks HookSet) Option {
	return func(manager *Manager) {
		manager.hooks = hooks
	}
}

// WithSkillRegistry injects the active-skill registry used during session start.
func WithSkillRegistry(registry SkillRegistry) Option {
	return func(manager *Manager) {
		manager.skillRegistry = registry
	}
}

// WithAgentResolver injects the daemon-authoritative agent definition resolver.
func WithAgentResolver(resolver AgentResolver) Option {
	return func(manager *Manager) {
		manager.agentResolver = resolver
	}
}

// WithProviderSecretResolver injects the launch-time provider secret resolver.
func WithProviderSecretResolver(resolver ProviderSecretResolver) Option {
	return func(manager *Manager) {
		manager.providerSecrets = resolver
	}
}

// WithModelCatalog injects the authoritative provider/model catalog used before startup.
func WithModelCatalog(catalog ModelCatalog) Option {
	return func(manager *Manager) {
		manager.modelCatalog = catalog
	}
}

// WithMCPResolver injects the skill MCP resolver used during session start.
func WithMCPResolver(resolver MCPResolver) Option {
	return func(manager *Manager) {
		manager.mcpResolver = resolver
	}
}

// WithHostedMCPLauncher injects the session-bound AGH-hosted MCP launcher.
func WithHostedMCPLauncher(launcher HostedMCPLauncher) Option {
	return func(manager *Manager) {
		manager.hostedMCP = launcher
	}
}

// WithSoulSnapshotStore injects durable Soul snapshot/session provenance storage.
func WithSoulSnapshotStore(store SoulSnapshotStore) Option {
	return func(manager *Manager) {
		manager.soulStore = store
	}
}

// WithSoulRunActivityChecker injects the active-run predicate used by Soul refresh.
func WithSoulRunActivityChecker(checker SoulRunActivityChecker) Option {
	return func(manager *Manager) {
		manager.soulRunChecker = checker
	}
}

// WithSessionHealthStore injects durable metadata-only session health storage.
func WithSessionHealthStore(store HealthStore) Option {
	return func(manager *Manager) {
		manager.sessionHealthStore = store
	}
}

// WithSessionCatalog injects the global session catalog updated by lifecycle transitions.
func WithSessionCatalog(catalog store.SessionCatalog) Option {
	return func(manager *Manager) {
		manager.sessionCatalog = catalog
		if creationStore, ok := catalog.(store.SessionCreationStore); ok {
			manager.creationStore = creationStore
		}
	}
}

// WithManagedInputLifecycle injects the daemon-owned managed prompt callbacks.
func WithManagedInputLifecycle(lifecycle ManagedInputLifecycle) Option {
	return func(manager *Manager) {
		manager.managedInputLifecycle = lifecycle
	}
}

// WithGoalCommandHandler injects the daemon-owned authenticated `/goal` dispatcher.
func WithGoalCommandHandler(handler GoalCommandHandler) Option {
	return func(manager *Manager) {
		manager.goalCommandHandler = handler
	}
}

// WithSessionCreationStore injects the immutable session creation catalog.
func WithSessionCreationStore(creationStore store.SessionCreationStore) Option {
	return func(manager *Manager) {
		manager.creationStore = creationStore
	}
}

// WithLedgerMaterializer injects the forensic session-ledger materializer.
func WithLedgerMaterializer(materializer LedgerMaterializer) Option {
	return func(manager *Manager) {
		manager.ledgerMaterializer = materializer
	}
}

// WithSessionHealthConfig injects Agent Heartbeat bounds used by session health.
func WithSessionHealthConfig(config aghconfig.HeartbeatConfig) Option {
	return func(manager *Manager) {
		manager.sessionHealthStaleAfter = config.SessionHealthStaleAfter
		manager.sessionHealthHookMinInterval = config.SessionHealthHookMinInterval
	}
}

// WithLogger injects the logger used by the session manager.
func WithLogger(logger *slog.Logger) Option {
	return func(manager *Manager) {
		manager.logger = logger
	}
}

// WithHomePaths overrides the resolved AGH home directory layout.
func WithHomePaths(homePaths aghconfig.HomePaths) Option {
	return func(manager *Manager) {
		manager.homePaths = homePaths
	}
}

// WithWorkspaceResolver injects workspace resolution for create/resume flows.
func WithWorkspaceResolver(resolver workspacepkg.RuntimeResolver) Option {
	return func(manager *Manager) {
		manager.workspace = resolver
	}
}

// WithParticipationResolver injects the shared resolver used before session creation writes.
func WithParticipationResolver(resolver participation.Resolver) Option {
	return func(manager *Manager) {
		manager.participationResolver = resolver
	}
}

// WithPromptInputAugmenter injects a bounded pre-dispatch message augmenter.
func WithPromptInputAugmenter(augmenter PromptInputAugmenter) Option {
	return func(manager *Manager) {
		manager.inputAugmenter = augmenter
	}
}

// WithSessionInputQueueStore injects durable busy-input queue storage.
func WithSessionInputQueueStore(queueStore store.SessionInputQueueStore) Option {
	return func(manager *Manager) {
		manager.inputQueueStore = queueStore
	}
}

// WithTranscriptEpochStore injects durable transcript reset epoch storage.
func WithTranscriptEpochStore(epochStore store.SessionTranscriptEpochStore) Option {
	return func(manager *Manager) {
		manager.transcriptEpochStore = epochStore
	}
}

// WithStartupPromptOverlay injects a daemon-owned startup prompt overlay.
func WithStartupPromptOverlay(overlay StartupPromptOverlay) Option {
	return func(manager *Manager) {
		manager.startupOverlay = overlay
	}
}

// WithNow overrides the manager clock, mainly for tests.
func WithNow(now func() time.Time) Option {
	return func(manager *Manager) {
		manager.now = now
	}
}

// WithSessionIDGenerator overrides session id allocation.
func WithSessionIDGenerator(generator IDGenerator) Option {
	return func(manager *Manager) {
		manager.newSessionID = generator
	}
}

// WithSandboxIDGenerator overrides sandbox id allocation.
func WithSandboxIDGenerator(generator IDGenerator) Option {
	return func(manager *Manager) {
		manager.newSandboxID = generator
	}
}

// WithTurnIDGenerator overrides prompt turn id allocation.
func WithTurnIDGenerator(generator IDGenerator) Option {
	return func(manager *Manager) {
		manager.newTurnID = generator
	}
}

// WithPromptBufferSize overrides the size of the returned prompt event buffer.
func WithPromptBufferSize(size int) Option {
	return func(manager *Manager) {
		manager.promptBufSize = size
	}
}

// WithSessionSupervision overrides runtime activity supervision settings.
func WithSessionSupervision(config aghconfig.SessionSupervisionConfig) Option {
	return func(manager *Manager) {
		manager.supervision = config
	}
}

// WithSessionBusyInputConfig overrides busy-input queue behavior.
func WithSessionBusyInputConfig(config aghconfig.SessionBusyInputConfig) Option {
	return func(manager *Manager) {
		manager.busyInput = config.Normalize()
	}
}

// WithSessionCompactionConfig overrides pressure-triggered context compaction guards.
func WithSessionCompactionConfig(config aghconfig.SessionCompactionConfig) Option {
	return func(manager *Manager) {
		manager.compaction = config
	}
}

// WithCompactionHandler injects the durable summary boundary used before archiving.
func WithCompactionHandler(handler CompactionHandler) Option {
	return func(manager *Manager) {
		manager.compactionHandler = handler
	}
}
