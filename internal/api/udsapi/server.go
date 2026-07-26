package udsapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	core "github.com/compozy/agh/internal/api/core"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/doctor"
	mcppkg "github.com/compozy/agh/internal/mcp"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/store"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

const (
	defaultPollInterval      = 100 * time.Millisecond
	defaultReadHeaderTimeout = 5 * time.Second
	defaultIdleTimeout       = 60 * time.Second
	maxSocketPathBytes       = 103
)

var (
	ErrSessionManagerRequired    = errors.New("udsapi: session manager is required")
	ErrTaskServiceRequired       = errors.New("udsapi: task service is required")
	ErrObserverRequired          = errors.New("udsapi: observer is required")
	ErrWorkspaceResolverRequired = errors.New("udsapi: workspace resolver is required")
	ErrSocketPathTooLong         = errors.New("udsapi: socket path too long")
)

type serverState uint8

const (
	serverStateStopped serverState = iota
	serverStateRunning
	serverStateStopping
)

// Option customizes UDS server construction.
type Option func(*Server)

// Server exposes the daemon API over a Unix domain socket.
type Server struct {
	mu sync.Mutex

	homePaths          aghconfig.HomePaths
	config             aghconfig.Config
	configSet          bool
	socketPath         string
	logger             *slog.Logger
	startedAt          time.Time
	now                func() time.Time
	pollInterval       time.Duration
	sessions           core.SessionManager
	drainController    core.DaemonDrainController
	sessionCatalog     core.SessionCatalog
	tasks              core.TaskService
	network            core.NetworkService
	networkStore       core.NetworkStore
	networkUsage       store.NetworkUsageStore
	coordination       workspacepkg.CoordinationCommands
	observer           core.Observer
	schemaStreams      core.SchemaStreamStatusReader
	automation         core.AutomationManager
	loops              core.LoopService
	bridges            core.BridgeService
	notifications      core.NotificationPresetService
	bundles            core.BundleService
	supportBundles     core.SupportBundleService
	tools              core.ToolRegistry
	toolArtifacts      toolspkg.ToolArtifactStore
	toolsets           core.ToolsetRegistry
	toolApprovals      core.ToolApprovalIssuer
	approvalGrants     core.ToolApprovalGrantService
	clarify            toolspkg.ClarifyBroker
	settings           core.SettingsService
	settingsRestart    core.SettingsRestartController
	settingsUpdate     core.SettingsUpdateController
	vault              core.VaultService
	workspaces         core.WorkspaceService
	onboarding         core.OnboardingStore
	agentCatalog       core.AgentCatalog
	agentSync          core.AgentDefinitionSync
	modelCatalog       core.ModelCatalogService
	marketplaceCatalog core.MarketplaceCatalogService
	agentContext       core.AgentContextService
	soulAuthoring      core.SoulAuthoringService
	soulHistoryPurger  core.SoulHistoryPurger
	soulRefresher      core.SoulRefresher
	heartbeatAuthor    core.HeartbeatAuthoringService
	heartbeatPurger    core.HeartbeatHistoryPurger
	heartbeatStatus    core.HeartbeatStatusService
	heartbeatWake      core.HeartbeatWakeService
	sessionHealth      core.SessionHealthReader
	wakeEvents         core.HeartbeatWakeEventReader
	coordinatorRole    core.CoordinatorRoleResolver
	roles              core.RolesStatusProvider
	skillsRegistry     core.SkillsRegistry
	skillResources     core.SkillResourceSyncer
	memoryStore        *memory.Store
	dreamTrigger       core.DreamTrigger
	memoryExtractor    core.MemoryExtractorService
	memoryProviders    core.MemoryProviderService
	memoryLedger       core.MemorySessionLedgerService
	runtimeMemory      doctor.RuntimeMemorySnapshotSource
	deadEntities       doctor.DeadEntitySource
	agentLoader        core.AgentLoader
	udsExtendedServices

	engine       *gin.Engine
	handlers     *Handlers
	httpServer   *http.Server
	listener     net.Listener
	serveDone    chan struct{}
	serveErr     error
	streamCancel context.CancelFunc
	state        serverState
}

// Handlers expose request/response and SSE endpoints for the AGH API.
type Handlers struct {
	*core.BaseHandlers
	Extensions ExtensionService
	HostedMCP  *mcppkg.HostedService
	MCPHostAPI mcppkg.HostAPIInvoker
}

// WithResourceService injects the shared operator-facing desired-state resource service.
func WithResourceService(service core.ResourceService) Option {
	return func(server *Server) {
		server.resources = service
	}
}

// WithBridgeService injects the daemon-owned bridge runtime.
func WithBridgeService(bridges core.BridgeService) Option {
	return func(server *Server) {
		server.bridges = bridges
	}
}

// WithNotificationPresetService injects the daemon-owned notification preset runtime.
func WithNotificationPresetService(service core.NotificationPresetService) Option {
	return func(server *Server) {
		server.notifications = service
	}
}

// WithBundleService injects the daemon-owned bundle runtime.
func WithBundleService(service core.BundleService) Option {
	return func(server *Server) {
		server.bundles = service
	}
}

// WithSupportBundleService injects the daemon-owned support bundle operation service.
func WithSupportBundleService(service core.SupportBundleService) Option {
	return func(server *Server) {
		server.supportBundles = service
	}
}

// WithToolRegistry injects the executable tool registry.
func WithToolRegistry(registry core.ToolRegistry) Option {
	return func(server *Server) {
		server.tools = registry
	}
}

// WithToolsetRegistry injects the named toolset projection registry.
func WithToolsetRegistry(registry core.ToolsetRegistry) Option {
	return func(server *Server) {
		server.toolsets = registry
	}
}

// WithSettingsService injects the daemon-owned settings service.
func WithSettingsService(service core.SettingsService) Option {
	return func(server *Server) {
		server.settings = service
	}
}

// WithSettingsRestartController injects the daemon-owned restart action surface for settings handlers.
func WithSettingsRestartController(controller core.SettingsRestartController) Option {
	return func(server *Server) {
		server.settingsRestart = controller
	}
}

// WithSettingsUpdateController injects the daemon-owned update status surface for settings handlers.
func WithSettingsUpdateController(controller core.SettingsUpdateController) Option {
	return func(server *Server) {
		server.settingsUpdate = controller
	}
}

// WithVaultService injects the daemon-owned vault control surface.
func WithVaultService(service core.VaultService) Option {
	return func(server *Server) {
		server.vault = service
	}
}

// WithWorkspaceResolver injects the runtime workspace resolver/service.
func WithWorkspaceResolver(workspaces core.WorkspaceService) Option {
	return func(server *Server) {
		server.workspaces = workspaces
	}
}

// WithOnboardingStore injects the first-run onboarding completion store.
func WithOnboardingStore(store core.OnboardingStore) Option {
	return func(server *Server) {
		server.onboarding = store
	}
}

// WithSkillsRegistry injects the skills registry surfaced by the daemon.
func WithSkillsRegistry(registry core.SkillsRegistry) Option {
	return func(server *Server) {
		server.skillsRegistry = registry
	}
}

// WithSkillResourceSyncer injects the resource-backed skill syncer surfaced by the daemon.
func WithSkillResourceSyncer(syncer core.SkillResourceSyncer) Option {
	return func(server *Server) {
		server.skillResources = syncer
	}
}

// WithAgentContext injects the bounded agent situation context service.
func WithAgentContext(service core.AgentContextService) Option {
	return func(server *Server) {
		server.agentContext = service
	}
}

// WithSoulAuthoring injects the managed Soul authoring surface.
func WithSoulAuthoring(service core.SoulAuthoringService) Option {
	return func(server *Server) {
		server.soulAuthoring = service
	}
}

// WithSoulRefresher injects the session Soul refresh surface.
func WithSoulRefresher(service core.SoulRefresher) Option {
	return func(server *Server) {
		server.soulRefresher = service
	}
}

// WithHeartbeatAuthoring injects the managed Heartbeat authoring surface.
func WithHeartbeatAuthoring(service core.HeartbeatAuthoringService) Option {
	return func(server *Server) {
		server.heartbeatAuthor = service
	}
}

// WithHeartbeatStatus injects the Heartbeat status/read surface.
func WithHeartbeatStatus(service core.HeartbeatStatusService) Option {
	return func(server *Server) {
		server.heartbeatStatus = service
	}
}

// WithHeartbeatWake injects the manual Heartbeat wake surface.
func WithHeartbeatWake(service core.HeartbeatWakeService) Option {
	return func(server *Server) {
		server.heartbeatWake = service
	}
}

// WithSessionHealthReader injects the metadata-only session health reader.
func WithSessionHealthReader(reader core.SessionHealthReader) Option {
	return func(server *Server) {
		server.sessionHealth = reader
	}
}

// WithHeartbeatWakeEventReader injects the retained Heartbeat wake audit reader.
func WithHeartbeatWakeEventReader(reader core.HeartbeatWakeEventReader) Option {
	return func(server *Server) {
		server.wakeEvents = reader
	}
}

// WithCoordinatorRole injects the resolved coordinator policy reader.
func WithCoordinatorRole(resolver core.CoordinatorRoleResolver) Option {
	return func(server *Server) {
		server.coordinatorRole = resolver
	}
}

// WithRolesStatusProvider injects the effective role configuration reader.
func WithRolesStatusProvider(provider core.RolesStatusProvider) Option {
	return func(server *Server) {
		server.roles = provider
	}
}

// WithAgentLoader overrides agent definition loading.
func WithAgentLoader(loader core.AgentLoader) Option {
	return func(server *Server) {
		server.agentLoader = loader
	}
}

// WithEngine overrides the Gin engine used by the server, mainly for tests.
func WithEngine(engine *gin.Engine) Option {
	return func(server *Server) {
		server.engine = engine
	}
}

// New constructs a Unix domain socket API server.
func New(opts ...Option) (*Server, error) {
	homePaths, err := aghconfig.ResolveHomePaths()
	if err != nil {
		return nil, fmt.Errorf("udsapi: resolve home paths: %w", err)
	}

	server := newDefaultServer(homePaths)
	applyOptions(server, opts)
	if err := server.finalize(); err != nil {
		return nil, err
	}

	server.ensureEngine()
	server.handlers = newHandlers(server.handlerConfig())
	registerRoutes(server.engine, server.handlers)

	return server, nil
}
