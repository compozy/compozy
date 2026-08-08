package httpapi

import (
	"io/fs"
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/doctor"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

const (
	handlersLocalhostKey               = "localhost"
	defaultGatewayIngressRequestLimit  = 60
	defaultGatewayIngressRequestWindow = time.Minute
)

type handlerConfig struct {
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
	resources          core.ResourceService
	windowManager      windowmanager.Service
	automation         core.AutomationManager
	loops              core.LoopService
	bridges            core.BridgeService
	notifications      core.NotificationPresetService
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
	workspaceAccess    workspaceaccess.Policy
	onboarding         core.OnboardingStore
	agentCatalog       core.AgentCatalog
	agentSync          core.AgentDefinitionSync
	modelCatalog       core.ModelCatalogService
	marketplaceCatalog core.MarketplaceCatalogService
	agentContext       core.AgentContextService
	coordinatorRole    core.CoordinatorRoleResolver
	roles              core.RolesStatusProvider
	soulAuthoring      core.SoulAuthoringService
	soulHistoryPurger  core.SoulHistoryPurger
	soulRefresher      core.SoulRefresher
	heartbeatAuthor    core.HeartbeatAuthoringService
	heartbeatPurger    core.HeartbeatHistoryPurger
	heartbeatStatus    core.HeartbeatStatusService
	heartbeatWake      core.HeartbeatWakeService
	sessionHealth      core.SessionHealthReader
	wakeEvents         core.HeartbeatWakeEventReader
	skillsRegistry     core.SkillsRegistry
	skillResources     core.SkillResourceSyncer
	memoryStore        *memory.Store
	dreamTrigger       core.DreamTrigger
	memoryExtractor    core.MemoryExtractorService
	memoryProviders    core.MemoryProviderService
	memoryLedger       core.MemorySessionLedgerService
	runtimeMemory      doctor.RuntimeMemorySnapshotSource
	deadEntities       doctor.DeadEntitySource
	gateway            core.GatewayService
	gatewayAdmission   gateway.AdmissionController
	deviceAuth         gateway.DeviceAuthenticator
	authLimiter        *gateway.AuthFailureLimiter
	ingressLimiter     *gateway.IngressRateLimiter
	surfaceSet         SurfaceSet
	staticFS           fs.FS
	homePaths          compozyconfig.HomePaths
	config             compozyconfig.Config
	boundHost          string
	logger             *slog.Logger
	startedAt          time.Time
	now                func() time.Time
	pollInterval       time.Duration
	agentLoader        core.AgentLoader
	httpPort           int
	resourceAuth       []gin.HandlerFunc
	extensions         ExtensionService
}

// Handlers expose request/response and SSE endpoints for the Compozy API.
type Handlers struct {
	*core.BaseHandlers
	staticFS         fs.FS
	resourceAuth     []gin.HandlerFunc
	Extensions       ExtensionService
	boundHost        string
	deviceAuth       gateway.DeviceAuthenticator
	gatewayAdmission gateway.AdmissionController
	authLimiter      *gateway.AuthFailureLimiter
	ingressLimiter   *gateway.IngressRateLimiter
}

const (
	defaultGatewayAuthFailureLimit  = 10
	defaultGatewayAuthFailureWindow = time.Minute
)

func newDefaultGatewayAuthFailureLimiter() *gateway.AuthFailureLimiter {
	return gateway.NewAuthFailureLimiter(
		defaultGatewayAuthFailureLimit,
		defaultGatewayAuthFailureWindow,
		func() time.Time { return time.Now().UTC() },
	)
}

func newHandlers(cfg *handlerConfig) *Handlers {
	if cfg == nil {
		cfg = &handlerConfig{}
	}

	if cfg.pollInterval <= 0 {
		cfg.pollInterval = defaultPollInterval
	}
	if cfg.httpPort <= 0 {
		cfg.httpPort = cfg.config.HTTP.Port
	}
	boundHost := handlerBoundHost(cfg)
	authLimiter := cfg.authLimiter
	if authLimiter == nil {
		authLimiter = newDefaultGatewayAuthFailureLimiter()
	}
	ingressLimiter := cfg.ingressLimiter
	if ingressLimiter == nil {
		ingressLimiter = gateway.NewIngressRateLimiter(
			defaultGatewayIngressRequestLimit,
			defaultGatewayIngressRequestWindow,
			func() time.Time { return time.Now().UTC() },
		)
	}
	gatewayAdmission := cfg.gatewayAdmission
	if gatewayAdmission == nil {
		if admission, ok := cfg.gateway.(gateway.AdmissionController); ok {
			gatewayAdmission = admission
		}
	}

	return &Handlers{
		BaseHandlers:     core.NewBaseHandlers(coreHandlerConfig(cfg, boundHost)),
		staticFS:         cfg.staticFS,
		resourceAuth:     append([]gin.HandlerFunc(nil), cfg.resourceAuth...),
		Extensions:       cfg.extensions,
		boundHost:        boundHost,
		deviceAuth:       cfg.deviceAuth,
		gatewayAdmission: gatewayAdmission,
		authLimiter:      authLimiter,
		ingressLimiter:   ingressLimiter,
	}
}

func coreHandlerConfig(cfg *handlerConfig, boundHost string) *core.BaseHandlerConfig {
	coreConfig := cfg.config
	coreConfig.HTTP.Host = boundHost
	return &core.BaseHandlerConfig{
		TransportName:                "httpapi",
		MaskInternalErrors:           true,
		IncludeSessionWorkspaceInSSE: true,
		Sessions:                     cfg.sessions,
		SessionAcceptance:            sessionAcceptanceManager(cfg.sessions),
		DrainController:              cfg.drainController,
		SessionCatalog:               cfg.sessionCatalog,
		Tasks:                        cfg.tasks,
		Network:                      cfg.network,
		NetworkStore:                 cfg.networkStore,
		NetworkUsage:                 cfg.networkUsage,
		Coordination:                 cfg.coordination,
		Observer:                     cfg.observer,
		SchemaStreams:                cfg.schemaStreams,
		Resources:                    cfg.resources,
		WindowManager:                cfg.windowManager,
		Extensions:                   cfg.extensions,
		Automation:                   cfg.automation,
		Loops:                        cfg.loops,
		Bridges:                      cfg.bridges,
		Notifications:                cfg.notifications,
		SupportBundles:               cfg.supportBundles,
		Tools:                        cfg.tools,
		ToolArtifacts:                cfg.toolArtifacts,
		Toolsets:                     cfg.toolsets,
		ToolApprovals:                cfg.toolApprovals,
		ApprovalGrants:               cfg.approvalGrants,
		Clarify:                      cfg.clarify,
		Settings:                     cfg.settings,
		SettingsRestart:              cfg.settingsRestart,
		SettingsUpdate:               cfg.settingsUpdate,
		Vault:                        cfg.vault,
		Workspaces:                   cfg.workspaces,
		WorkspaceAccess:              cfg.workspaceAccess,
		Onboarding:                   cfg.onboarding,
		AgentCatalog:                 cfg.agentCatalog,
		AgentDefinitionSync:          cfg.agentSync,
		ModelCatalog:                 cfg.modelCatalog,
		MarketplaceCatalog:           cfg.marketplaceCatalog,
		AgentContextService:          cfg.agentContext,
		CoordinatorRole:              cfg.coordinatorRole,
		Roles:                        cfg.roles,
		SoulAuthoring:                cfg.soulAuthoring,
		SoulHistoryPurger:            cfg.soulHistoryPurger,
		SoulRefresher:                cfg.soulRefresher,
		HeartbeatAuthoring:           cfg.heartbeatAuthor,
		HeartbeatHistoryPurger:       cfg.heartbeatPurger,
		HeartbeatStatus:              cfg.heartbeatStatus,
		HeartbeatWake:                cfg.heartbeatWake,
		SessionHealth:                cfg.sessionHealth,
		HeartbeatWakeEvents:          cfg.wakeEvents,
		SkillsRegistry:               cfg.skillsRegistry,
		SkillResources:               cfg.skillResources,
		MemoryStore:                  cfg.memoryStore,
		DreamTrigger:                 cfg.dreamTrigger,
		MemoryExtractor:              cfg.memoryExtractor,
		MemoryProviders:              cfg.memoryProviders,
		MemorySessionLedger:          cfg.memoryLedger,
		RuntimeMemory:                cfg.runtimeMemory,
		DeadEntities:                 cfg.deadEntities,
		Gateway:                      cfg.gateway,
		GatewayPairingSource:         gatewayPairingSource(cfg.surfaceSet),
		HomePaths:                    cfg.homePaths,
		Config:                       coreConfig,
		Logger:                       cfg.logger,
		StartedAt:                    cfg.startedAt,
		Now:                          cfg.now,
		PollInterval:                 cfg.pollInterval,
		AgentLoader:                  cfg.agentLoader,
		HTTPPort:                     cfg.httpPort,
	}
}

func gatewayPairingSource(surfaceSet SurfaceSet) string {
	switch surfaceSet {
	case SurfaceSetLocal:
		return string(gateway.PairingSourceLocal)
	case SurfaceSetPrivate:
		return string(gateway.PairingSourcePrivate)
	default:
		return ""
	}
}

func sessionAcceptanceManager(manager core.SessionManager) core.SessionAcceptanceManager {
	accepted, ok := manager.(core.SessionAcceptanceManager)
	if !ok {
		return nil
	}
	return accepted
}

func (h *Handlers) setStreamDone(done <-chan struct{}) {
	if h != nil && h.BaseHandlers != nil {
		h.SetStreamDone(done)
	}
}

func (h *Handlers) setHTTPPort(port int) {
	if h != nil && h.BaseHandlers != nil {
		h.SetHTTPPort(port)
	}
}

func (h *Handlers) resourceAuthMiddleware() []gin.HandlerFunc {
	if h == nil || len(h.resourceAuth) == 0 {
		return nil
	}
	return append([]gin.HandlerFunc(nil), h.resourceAuth...)
}

func (h *Handlers) privilegedMutationGuard() gin.HandlerFunc {
	boundHost := ""
	if h != nil {
		boundHost = h.boundHost
	}
	return loopbackMutationGuard(boundHost)
}
