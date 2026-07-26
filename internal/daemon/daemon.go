package daemon

import (
	"context"
	"errors"

	"log/slog"
	"os"

	"sync"
	"syscall"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/admission"
	core "github.com/compozy/agh/internal/api/core"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	bundlepkg "github.com/compozy/agh/internal/bundles"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/heartbeat"
	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/memory/consolidation"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/observe"

	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/situation"
	"github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/store"

	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/toolruntime"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/windowmanager"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const defaultShutdownTimeout = 10 * time.Second

var (
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
type ConfigLoader func() (aghconfig.Config, error)

// SessionManager is the shared transport-facing session surface consumed by daemon/.
type SessionManager = core.SessionManager

// Observer is the daemon observer surface used for transport wiring and reconciliation.
type Observer interface {
	core.Observer
	session.Notifier
	Reconcile(ctx context.Context) (store.ReconcileResult, error)
}

// Registry is the narrowed global database surface shared by observe and workspace.
type Registry interface {
	observe.Registry
	store.SessionCatalog
	store.NetworkAuditStore
	store.NetworkChannelStore
	store.NetworkConversationStore
	store.NetworkMessageStore
	store.NetworkPreferenceStore
	store.NetworkAvailabilityStore
	store.NetworkUsageStore
	store.OnboardingStore
	workspacepkg.Store
	workspacepkg.CoordinationSettings
	workspacepkg.CoordinationCommandStore
}

// Server is a daemon-owned runtime component with explicit start and shutdown phases.
type Server interface {
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// ServerFactory constructs runtime components such as HTTP and UDS servers.
type ServerFactory func(ctx context.Context, deps RuntimeDeps) (Server, error)

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

type observerRetentionStarter interface {
	StartRetention(context.Context) error
}

type observerRetentionStopper interface {
	ShutdownRetention(context.Context) error
}

type resourceReconcileDriverDeps struct {
	Config              aghconfig.Config
	Logger              *slog.Logger
	Registry            Registry
	ResourceStore       resources.RawStore
	CodecRegistry       *resources.CodecRegistry
	Hooks               *hookspkg.Hooks
	AgentCatalog        *resourceCatalog[aghconfig.AgentDef]
	SoulCatalog         *resourceCatalog[soul.ResourceSpec]
	HeartbeatCatalog    *resourceCatalog[heartbeat.ResourceSpec]
	ToolCatalog         *resourceCatalog[toolspkg.Tool]
	MCPServerCatalog    *resourceCatalog[aghconfig.MCPServer]
	LoopCatalog         *resourceCatalog[looppkg.ResourceSpec]
	WindowLayoutCatalog *resourceCatalog[windowmanager.LayoutResource]
	SkillsRegistry      *skills.Registry
	Automation          automationResourceProjectorTarget
	Bridges             bridgeResourceProjectorTarget
	Bundles             resources.BundleActivationProjector[bundlepkg.ActivationResourceSpec, bundlepkg.BundleResourceSpec]
}

type extensionRuntime interface {
	Start(context.Context) error
	Stop(context.Context) error
	Reload(context.Context) error
	Get(string) (*extensionpkg.Extension, error)
	HookDeclarations(context.Context) ([]hookspkg.HookDecl, error)
}

type extensionManagerDeps struct {
	Registry               *extensionpkg.Registry
	Extensions             aghconfig.ExtensionsConfig
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
	AGHExecutable          func() (string, error)
}

// Daemon is the sole AGH composition root.
type Daemon struct {
	mu sync.Mutex

	homePaths                    aghconfig.HomePaths
	loadConfig                   ConfigLoader
	logger                       *slog.Logger
	closeLogger                  func() error
	now                          func() time.Time
	pid                          func() int
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
	orphanGraceWait              time.Duration
	orphanPollWait               time.Duration
	config                       aghconfig.Config
	startedAt                    time.Time
	info                         Info
	admission                    admission.Gate
	lock                         *Lock
	harnessResolver              *HarnessContextResolver
	registry                     Registry
	memoryStore                  *memory.Store
	memoryProviderRegistry       *extensionpkg.MemoryProviderRegistry
	memoryExtractor              *daemonMemoryExtractor
	runtimeWorkers               daemonRuntimeWorkers
	localMemoryProvider          memoryProviderShutdowner
	situationContext             *situation.Service
	sessions                     SessionManager
	tasks                        *taskRuntime
	coordinator                  *coordinatorRuntime
	spawnReaper                  *spawnReaper
	scheduler                    *schedulerRuntime
	network                      networkRuntime
	networkWakeRunner            *networkWakeRunner
	toolRegistry                 toolspkg.Registry
	clarify                      *clarifyBridge
	hooks                        hookRuntime
	extensions                   extensionRuntime
	observer                     Observer
	resourceReconcile            resources.ReconcileDriver
	agentCatalog                 *resourceCatalog[aghconfig.AgentDef]
	soulCatalog                  *resourceCatalog[soul.ResourceSpec]
	heartbeatCatalog             *resourceCatalog[heartbeat.ResourceSpec]
	toolCatalog                  *resourceCatalog[toolspkg.Tool]
	mcpServerCatalog             *resourceCatalog[aghconfig.MCPServer]
	loopCatalog                  *resourceCatalog[looppkg.ResourceSpec]
	automation                   automationRuntime
	bridges                      *bridgeRuntime
	httpServer                   Server
	udsServer                    Server
	dreamRuntime                 *consolidation.Runtime
	workspaceResolver            workspacepkg.RuntimeResolver
	sandboxRegistry              *sandbox.Registry
	windowManagerRuntime
	skillsRegistry   *skills.Registry
	modelCatalog     *modelCatalogRuntime
	marketplace      *marketplaceRuntime
	skillsCancel     context.CancelFunc
	skillsDone       chan struct{}
	loopsCancel      context.CancelFunc
	loopsDone        chan struct{}
	goalOutboxCancel context.CancelFunc
	goalOutboxDone   chan struct{}
}
