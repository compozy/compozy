package daemon

import (
	"context"
	"errors"

	"log/slog"
	"os"

	"sync"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/admission"
	core "github.com/compozy/compozy/internal/api/core"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/heartbeat"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/memory/consolidation"
	"github.com/compozy/compozy/internal/network"
	"github.com/compozy/compozy/internal/observe"
	"github.com/compozy/compozy/internal/providers"

	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/soul"
	"github.com/compozy/compozy/internal/store"

	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/toolruntime"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const defaultShutdownTimeout = 10 * time.Second

var (
	errDaemonBootInProgress         = errors.New("daemon: boot in progress")
	errDaemonShutdownInProgress     = errors.New("daemon: shutdown in progress")
	errMissingNetworkBindingSurface = errors.New(
		"daemon: session manager does not implement the network binding surface",
	)
	errMissingWorkspaceRemovalPreparation = errors.New(
		"daemon: session manager does not implement workspace removal preparation",
	)
)

// Option customizes daemon construction.
type Option func(*Daemon)

// ConfigLoader resolves the daemon-level runtime configuration.
type ConfigLoader func() (compozyconfig.Config, error)

// SessionManager is the shared transport-facing session surface consumed by daemon/.
type SessionManager = core.SessionManager

// Observer is the daemon observer surface used for transport wiring and reconciliation.
type Observer interface {
	core.Observer
	session.Notifier
	Reconcile(ctx context.Context) (store.ReconcileResult, error)
}

type daemonObserveRegistry interface {
	observe.Registry
	store.EventSummaryBatchWriter
	store.SessionCatalog
}

type daemonNetworkRegistry interface {
	store.NetworkAuditStore
	store.NetworkChannelStore
	store.NetworkConversationStore
	store.NetworkMessageStore
	store.NetworkPreferenceStore
	store.NetworkAvailabilityStore
	store.NetworkUsageStore
}

type daemonWorkspaceRegistry interface {
	workspacepkg.Store
	workspacepkg.CoordinationSettings
	workspacepkg.CoordinationCommandStore
}

// Registry is the composition-root database surface grouped by owning domain.
// Consumers depend on their local narrow interfaces instead of this aggregate.
type Registry interface {
	daemonObserveRegistry
	daemonNetworkRegistry
	daemonWorkspaceRegistry
	gateway.Store
	store.OnboardingStore
}

// Server is a daemon-owned runtime component with explicit start and shutdown phases.
type Server interface {
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// ServerFactory constructs runtime components such as HTTP and UDS servers.
type ServerFactory func(ctx context.Context, deps RuntimeDeps) (Server, error)

// GatewayTierServerFactory constructs one daemon-owned loopback listener for a tier and its active surfaces.
type GatewayTierServerFactory func(
	ctx context.Context,
	deps *RuntimeDeps,
	tier gateway.Tier,
	surfaces []gateway.Surface,
) (Server, error)

// DreamTrigger exposes consolidation controls and health state to transport layers.
type DreamTrigger = core.DreamTrigger

type registryOpener func(ctx context.Context, path string) (Registry, error)
type sessionManagerFactory func(ctx context.Context, deps SessionManagerDeps) (SessionManager, error)
type observerFactory func(ctx context.Context, deps RuntimeDeps) (Observer, error)
type extensionManagerFactory func(deps extensionManagerDeps) extensionRuntime
type automationManagerFactory func(deps automationManagerDeps) (automationRuntime, error)
type resourceReconcileDriverFactory func(
	ctx context.Context,
	deps resourceReconcileDriverDeps,
) (resources.ReconcileDriver, error)

type networkRuntime interface {
	core.NetworkService
	session.NetworkPeerLifecycle
	Shutdown(context.Context) error
	OnTurnEnd(string)
	SendFromRuntimePeer(context.Context, network.RuntimeSendRequest) (string, error)
}

type networkBindableSessionManager interface {
	Resume(ctx context.Context, sessionID string) (*session.Session, error)
	PromptNetwork(
		ctx context.Context,
		sessionID string,
		message string,
		meta ...acp.PromptNetworkMeta,
	) (<-chan acp.AgentEvent, error)
	CancelPrompt(ctx context.Context, sessionID string) error
	IsPrompting(sessionID string) bool
	SetNetworkPeerLifecycle(session.NetworkPeerLifecycle)
	SetTurnEndNotifier(session.TurnEndNotifier)
}

type memoryProviderShutdowner interface {
	Shutdown(context.Context) error
}

type supportBundleShutdowner interface {
	Shutdown(context.Context) error
}

type observerRetentionStarter interface {
	StartRetention(context.Context) error
}

type observerRetentionStopper interface {
	ShutdownRetention(context.Context) error
}

type resourceReconcileDriverDeps struct {
	Config              compozyconfig.Config
	Logger              *slog.Logger
	Registry            Registry
	ResourceStore       resources.RawStore
	CodecRegistry       *resources.CodecRegistry
	Hooks               *hookspkg.Hooks
	AgentCatalog        *resourceCatalog[compozyconfig.AgentDef]
	SoulCatalog         *resourceCatalog[soul.ResourceSpec]
	HeartbeatCatalog    *resourceCatalog[heartbeat.ResourceSpec]
	ToolCatalog         *resourceCatalog[toolspkg.Tool]
	MCPServerCatalog    *resourceCatalog[compozyconfig.MCPServer]
	LoopCatalog         *resourceCatalog[looppkg.ResourceSpec]
	WindowLayoutCatalog *resourceCatalog[windowmanager.LayoutResource]
	SkillsRegistry      *skills.Registry
	Automation          automationResourceProjectorTarget
	Bridges             bridgeResourceProjectorTarget
}

type extensionRuntime interface {
	Start(context.Context) error
	Stop(context.Context) error
	Reload(context.Context) error
	Get(string) (*extensionpkg.Extension, error)
	HookDeclarations(context.Context) ([]hookspkg.HookDecl, error)
	InspectPackageResources(context.Context, string) (*extensionpkg.Extension, error)
}

type extensionDevRuntime interface {
	GetForInstance(extensionpkg.InstanceKey) (*extensionpkg.Extension, error)
	ListForWorkspace(string) []extensionpkg.ExtensionInfo
	InspectDevelopmentGeneration(context.Context, string, string, string) (extensionpkg.DevelopmentGeneration, error)
	LinkDevelopmentFromOrigin(context.Context, string, string, string) (*extensionpkg.Extension, error)
	StageDevelopmentLink(context.Context, extensionpkg.InstanceKey, string, string) (*extensionpkg.DevLink, error)
	ActivateDevelopmentLink(context.Context, extensionpkg.InstanceKey) (*extensionpkg.Extension, error)
	ReloadExtension(context.Context, extensionpkg.InstanceKey, string) (*extensionpkg.Extension, error)
	UnlinkDevelopment(context.Context, extensionpkg.InstanceKey) error
	Logs(extensionpkg.InstanceKey, int64) ([]extensionpkg.ExtensionLogEntry, error)
}

type extensionManagerDeps struct {
	Registry               *extensionpkg.Registry
	Extensions             compozyconfig.ExtensionsConfig
	Sessions               SessionManager
	Clarify                toolspkg.ClarifyBroker
	Automation             func() extensionpkg.HostAPIAutomationManager
	Tasks                  taskpkg.Manager
	Network                core.NetworkService
	NetworkStore           store.NetworkConversationStore
	ModelCatalog           core.ModelCatalogService
	MemoryStore            *memory.Store
	MemoryProviderRegistry *extensionpkg.MemoryProviderRegistry
	Observer               Observer
	SkillsRegistry         *skills.Registry
	WorkspaceResolver      workspacepkg.RuntimeResolver
	Logger                 *slog.Logger
	BridgeRegistry         bridgepkg.Registry
	BridgeDedupStore       bridgeDedupStore
	BridgeBroker           *bridgepkg.Broker
	BridgeRuntime          extensionpkg.BridgeRuntimeResolver
	ResourceStore          resources.RawStore
	SourceSessions         resources.SourceSessionManager
	ResourceCodecs         *resources.CodecRegistry
	ResourceTrigger        func(context.Context, resources.ResourceKind, resources.ReconcileReason) error
	SoulAuthoring          core.SoulAuthoringService
	SoulRefresher          core.SoulRefresher
	HeartbeatAuthor        core.HeartbeatAuthoringService
	HeartbeatStatus        core.HeartbeatStatusService
	HeartbeatWake          core.HeartbeatWakeService
	SessionHealth          core.SessionHealthReader
	WakeEvents             core.HeartbeatWakeEventReader
	ProcessRegistry        *toolruntime.Registry
	SecretResolver         extensionpkg.SecretRefResolver
	EnvBindings            extensionpkg.EnvBindingStore
	LifecycleEvents        extensionpkg.LifecycleEventSink
	CompozyExecutable      func() (string, error)
}

// Daemon is the sole Compozy composition root.
type Daemon struct {
	mu sync.Mutex

	homePaths                    compozyconfig.HomePaths
	loadConfig                   ConfigLoader
	logger                       *slog.Logger
	closeLogger                  func() error
	now                          func() time.Time
	pid                          func() int
	processStartedAt             func(int) (time.Time, error)
	acquireLock                  func(path string, pid int) (*Lock, error)
	openRegistry                 registryOpener
	newSessionManager            sessionManagerFactory
	newDreamService              consolidation.ServiceFactory
	newObserver                  observerFactory
	newExtensionManager          extensionManagerFactory
	newAutomationManager         automationManagerFactory
	newResourceReconcile         resourceReconcileDriverFactory
	httpFactory                  ServerFactory
	udsFactory                   ServerFactory
	gatewayTierFactory           GatewayTierServerFactory
	gatewayProviderEffects       gateway.EffectDriver
	listProcesses                func(context.Context) ([]processInfo, error)
	signalProcess                func(int, syscall.Signal) error
	processAlive                 func(int) bool
	executable                   func() (string, error)
	startDetached                detachedStartFunc
	signalCh                     <-chan os.Signal
	verifyBoundaries             bool
	boundaryRoot                 string
	getenv                       func(string) string
	bridgeSecretResolver         BridgeSecretResolver
	bridgeSecretResolverExplicit bool
	readyCh                      chan struct{}
	readyClosed                  bool
	booting                      bool
	shutdown                     *daemonShutdownOperation
	orphanGraceWait              time.Duration
	orphanPollWait               time.Duration
	config                       compozyconfig.Config
	admission                    admission.Gate
	providerPreStarter           *providers.PreStarter
	daemonRuntimeState
}
