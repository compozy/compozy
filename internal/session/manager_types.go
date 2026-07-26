package session

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/agh/internal/admission"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/session/inputqueue"
	"github.com/compozy/agh/internal/store"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// CreateOpts defines the inputs required to create a new session.
type CreateOpts struct {
	DesiredSessionID     string
	AgentName            string
	Provider             string
	Model                string
	ReasoningEffort      string
	CWD                  string
	SandboxRef           string
	DisableSandbox       bool
	Permissions          aghconfig.PermissionMode
	Name                 string
	Workspace            string
	WorkspacePath        string
	NetworkParticipation *participation.Request
	// ResolvedNetworkParticipation binds an internal worker session to the immutable owner snapshot.
	// Callers must not set it together with NetworkParticipation.
	ResolvedNetworkParticipation *participation.Spec
	// NetworkOwnerKey binds an internal worker session to its task or loop budget owner.
	// Empty values default to the session's own identity.
	NetworkOwnerKey string
	// NetworkAuthority carries the concrete delegated channel scope for child-owned resolution.
	NetworkAuthority *participation.AuthorityScope
	PromptOverlay    string
	ContractOverlay  string
	RuntimeMode      string
	Type             Type
	Lineage          *store.SessionLineage
	ParentSoulDigest string
	// AllowedToolsOverride is a concrete subset narrowing of the resolved agent tool policy.
	AllowedToolsOverride []string
	CreationProfile      *store.SessionCreationProfile
	CreationIdentity     *store.SessionCreationIdentity
}

// CreateAcceptedOpts combines session creation options with an optional first
// prompt that must be durably staged before the starting session is returned.
type CreateAcceptedOpts struct {
	Session       CreateOpts
	InitialPrompt string
}

// StoreOpener opens the per-session events store for a session directory.
type StoreOpener func(ctx context.Context, sessionID string, path string) (EventRecorder, error)

// QueryStoreOpener opens a read-only per-session events store for query paths.
type QueryStoreOpener func(ctx context.Context, sessionID string, path string) (EventReadCloser, error)

type sessionMetaReader func(path string) (store.SessionMeta, error)

// IDGenerator returns unique identifiers for sessions and prompt turns.
type IDGenerator func() string

// HostedMCPLauncher mints and releases session-bound hosted MCP launch records.
type HostedMCPLauncher interface {
	Launch(ctx context.Context, req HostedMCPLaunchRequest) (aghconfig.MCPServer, error)
	CancelLaunch(sessionID string)
	ReleaseSession(sessionID string)
}

// HostedMCPLaunchRequest describes the session identity for a hosted MCP entry.
type HostedMCPLaunchRequest struct {
	SessionID   string
	WorkspaceID string
	AgentName   string
}

// ProviderSecretResolver resolves provider-bound secret refs at launch time.
type ProviderSecretResolver interface {
	ResolveRef(ctx context.Context, ref string) (string, error)
}

// ModelCatalog exposes the provider/model projection needed for startup preflight.
type ModelCatalog interface {
	ListModels(ctx context.Context, opts modelcatalog.ListOptions) ([]modelcatalog.Model, error)
}

// Option customizes the session manager.
type Option func(*Manager)

type sessionReservation struct {
	workspaceID string
}

// Manager owns active session lifecycle and runtime orchestration.
type Manager struct {
	mu                 sync.RWMutex
	lifecycleMu        sync.Mutex
	sessions           map[string]*Session
	pending            map[string]sessionReservation
	finalizing         map[string]*sessionFinalization
	promptDrains       map[chan struct{}]struct{}
	spawnMu            sync.Mutex
	managedInputMu     sync.Mutex
	managedInputLeases map[string]managedInputLease
	goalCommandMu      sync.RWMutex
	resumeReplayMu     sync.Mutex
	resumeReplays      map[string]string
	interruptSalvageMu sync.Mutex
	interruptSalvages  map[string]interruptedPromptSalvage
	compactionMu       sync.Mutex
	compactions        map[string]*sessionCompactionState
	compactionWG       sync.WaitGroup
	compactionClosing  bool
	startMu            sync.Mutex
	startRuns          map[string]*sessionStartRun
	startWG            sync.WaitGroup
	startClosing       bool

	syntheticMu           sync.Mutex
	syntheticQueues       map[string][]queuedSyntheticPrompt
	syntheticDispatching  map[string]bool
	soulLocksMu           sync.Mutex
	soulLocks             map[string]chan struct{}
	sessionHealthHookMu   sync.Mutex
	sessionHealthHookLast map[string]time.Time
	streamEventsMu        sync.Mutex
	streamEvents          *sessionEventBroadcaster
	catalogEventsMu       sync.Mutex
	catalogEvents         *sessionCatalogBroadcaster

	logger                       *slog.Logger
	driver                       AgentDriver
	notifier                     Notifier
	networkPeers                 NetworkPeerLifecycle
	participationResolver        participation.Resolver
	turnEndNotifier              TurnEndNotifier
	inputAugmenter               PromptInputAugmenter
	inputQueue                   *inputqueue.Service
	inputQueueStore              store.SessionInputQueueStore
	managedInputLifecycle        ManagedInputLifecycle
	workAdmission                admission.Checker
	goalCommandHandler           GoalCommandHandler
	startupOverlay               StartupPromptOverlay
	hooks                        HookSet
	sandbox                      *sandbox.Registry
	agentResolver                AgentResolver
	providerSecrets              ProviderSecretResolver
	modelCatalog                 ModelCatalog
	skillRegistry                SkillRegistry
	toolsetCatalog               toolspkg.ToolsetCatalog
	mcpResolver                  MCPResolver
	hostedMCP                    HostedMCPLauncher
	soulStore                    SoulSnapshotStore
	soulRunChecker               SoulRunActivityChecker
	sessionHealthStore           HealthStore
	sessionCatalog               store.SessionCatalog
	creationStore                store.SessionCreationStore
	transcriptEpochStore         store.SessionTranscriptEpochStore
	ledgerMaterializer           LedgerMaterializer
	homePaths                    aghconfig.HomePaths
	workspace                    workspacepkg.RuntimeResolver
	readSessionMeta              sessionMetaReader
	openStore                    StoreOpener
	openQueryStore               QueryStoreOpener
	queryStoreExplicit           bool
	queryStoreRuntime            *queryStoreRuntime
	assembler                    PromptAssembler
	supervision                  aghconfig.SessionSupervisionConfig
	busyInput                    aghconfig.SessionBusyInputConfig
	compaction                   aghconfig.SessionCompactionConfig
	compactionHandler            CompactionHandler
	sessionHealthStaleAfter      time.Duration
	lifecycleCtx                 context.Context
	now                          func() time.Time
	newSessionID                 IDGenerator
	newSandboxID                 IDGenerator
	newTurnID                    IDGenerator
	renamePath                   func(oldPath string, newPath string) error
	removeAllPath                func(path string) error
	promptBufSize                int
	soulRefreshTimeout           time.Duration
	sessionHealthHookMinInterval time.Duration
}
