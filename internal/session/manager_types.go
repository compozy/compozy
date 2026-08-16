package session

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/admission"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/sandbox"
	"github.com/compozy/compozy/internal/session/inputqueue"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// CreateOpts defines the inputs required to create a new session.
type CreateOpts struct {
	DesiredSessionID string
	AgentName        string
	Provider         string
	Model            string
	ReasoningEffort  string
	Speed            speedpkg.Speed
	CWD              string
	SandboxRef       string
	DisableSandbox   bool
	Permissions      compozyconfig.PermissionMode
	Name             string
	Workspace        string
	WorkspacePath    string
	// Worktree names an existing ready worktree within the resolved parent workspace.
	Worktree             string
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
	// DiscardStartFailure prevents internal retry attempts from leaving durable
	// session artifacts when provider startup fails before Create returns.
	DiscardStartFailure bool
}

// CreateAcceptedOpts carries one logical user-session creation request.
type CreateAcceptedOpts struct {
	Session CreateOpts
}

// StoreOpener opens the per-session events store for the exact immutable owner.
// Implementations must forward both owner fields unchanged to the storage boundary.
type StoreOpener func(ctx context.Context, owner store.SessionDBOwner, path string) (EventRecorder, error)

// QueryStoreOpener opens the read-only per-session events store for the exact
// immutable owner. Implementations must forward both owner fields unchanged.
type QueryStoreOpener func(ctx context.Context, owner store.SessionDBOwner, path string) (EventReadCloser, error)

type sessionDBFamilyLeaseAcquirer func(context.Context, string) (*sessiondb.FamilyLease, error)

type sessionMetaReader func(path string) (store.SessionMeta, error)

// IDGenerator returns a unique identifier or the entropy failure that prevented allocation.
type IDGenerator func() (string, error)

// HostedMCPLauncher mints and releases session-bound hosted MCP launch records.
type HostedMCPLauncher interface {
	Launch(ctx context.Context, req HostedMCPLaunchRequest) (compozyconfig.MCPServer, error)
	ArmLaunch(ctx context.Context, sessionID string) error
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

// Option customizes the session manager.
type Option func(*Manager)

type sessionReservation struct {
	workspaceID string
}

type sessionResumeRun struct {
	done    chan struct{}
	session *Session
	err     error
}

// Manager owns active session lifecycle and runtime orchestration.
type Manager struct {
	mu                         sync.RWMutex
	lifecycleMu                sync.Mutex
	sessions                   map[string]*Session
	pending                    map[string]sessionReservation
	finalizing                 map[string]*sessionFinalization
	conversationFinalizing     map[string]chan struct{}
	conversationOperationMu    sync.Mutex
	conversationOperationLocks map[string]*conversationOperationLock
	promptDrains               map[chan struct{}]struct{}
	spawnMu                    sync.Mutex
	managedInputMu             sync.Mutex
	managedInputLeases         map[string]managedInputLease
	goalCommandMu              sync.RWMutex
	promptAdmissionMu          sync.Mutex
	promptAdmissionLocks       map[string]*promptAdmissionLock
	resumeReplayMu             sync.Mutex
	resumeReplays              map[string]string
	compactionLifecycle        sessionCompactionLifecycle
	startLifecycle             sessionStartLifecycle
	resumeLifecycle            sessionResumeLifecycle
	processWatchLifecycle      sessionProcessWatchLifecycle

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
	presenceMu            sync.Mutex
	presenceLeases        map[sessionPresenceKey]sessionPresenceLease
	notifyMu              sync.Mutex
	notifyConfig          compozyconfig.AttentionConfig
	notifyLastBySession   map[string]time.Time

	logger                       *slog.Logger
	driver                       AgentDriver
	notifier                     Notifier
	networkPeers                 NetworkPeerLifecycle
	participationResolver        participation.Resolver
	turnEndNotifier              TurnEndNotifier
	inputAugmenter               PromptInputAugmenter
	commandService               CommandService
	inputQueue                   *inputqueue.Service
	inputQueueStore              store.SessionInputQueueStore
	promptAdmissionStore         store.SessionPromptAdmissionStore
	attachmentOpener             AttachmentOpener
	attachmentScopeLease         attachmentScopeLease
	managedInputLifecycle        ManagedInputLifecycle
	workAdmission                admission.Checker
	goalCommandHandler           GoalCommandHandler
	startupOverlay               StartupPromptOverlay
	hooks                        HookSet
	sandbox                      *sandbox.Registry
	agentResolver                AgentResolver
	providerSecrets              ProviderSecretResolver
	modelCatalog                 modelcatalog.Service
	skillRegistry                SkillRegistry
	toolsetCatalog               toolspkg.ToolsetCatalog
	mcpResolver                  MCPResolver
	hostedMCP                    HostedMCPLauncher
	soulStore                    SoulSnapshotStore
	soulRunChecker               SoulRunActivityChecker
	sessionHealthStore           HealthStore
	sessionCatalog               store.SessionCatalog
	attentionStore               store.SessionAttentionStore
	creationStore                store.SessionCreationStore
	transcriptEpochStore         store.SessionTranscriptEpochStore
	ledgerMaterializer           LedgerMaterializer
	homePaths                    compozyconfig.HomePaths
	workspace                    workspacepkg.RuntimeResolver
	worktreeResolver             WorktreeResolver
	workspaceAccess              workspaceaccess.Policy
	readSessionMeta              sessionMetaReader
	openStore                    StoreOpener
	openQueryStore               QueryStoreOpener
	queryStoreExplicit           bool
	queryStoreRuntime            *queryStoreRuntime
	assembler                    PromptAssembler
	supervision                  compozyconfig.SessionSupervisionConfig
	busyInput                    compozyconfig.SessionBusyInputConfig
	compaction                   compozyconfig.SessionCompactionConfig
	compactionHandler            CompactionHandler
	sessionHealthStaleAfter      time.Duration
	lifecycleCtx                 context.Context
	now                          func() time.Time
	newSessionID                 IDGenerator
	newSandboxID                 IDGenerator
	newTurnID                    IDGenerator
	newRepairEventID             IDGenerator
	newInteractionID             IDGenerator
	newPresenceLeaseID           IDGenerator
	newNotificationID            IDGenerator
	acquireSessionDBFamilyLease  sessionDBFamilyLeaseAcquirer
	removeAllPath                func(path string) error
	promptBufSize                int
	soulRefreshTimeout           time.Duration
	sessionHealthHookMinInterval time.Duration
}
