package httpapi

import (
	"io/fs"
	"log/slog"
	"time"

	"github.com/compozy/agh/internal/api/core"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/doctor"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/store"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/windowmanager"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

const (
	handlersLocalhostKey = "localhost"
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
	staticFS           fs.FS
	homePaths          aghconfig.HomePaths
	config             aghconfig.Config
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

// Handlers expose request/response and SSE endpoints for the AGH API.
type Handlers struct {
	*core.BaseHandlers
	staticFS     fs.FS
	resourceAuth []gin.HandlerFunc
	Extensions   ExtensionService
	boundHost    string
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

	return &Handlers{
		BaseHandlers: core.NewBaseHandlers(coreHandlerConfig(cfg, boundHost)),
		staticFS:     cfg.staticFS,
		resourceAuth: append([]gin.HandlerFunc(nil), cfg.resourceAuth...),
		Extensions:   cfg.extensions,
		boundHost:    boundHost,
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
		Bundles:                      cfg.bundles,
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
