package httpapi

import (
	"io/fs"

	"github.com/gin-gonic/gin"
)

func (s *Server) handlerConfig(staticFS fs.FS) *handlerConfig {
	config := s.handlerDependencies()
	config.staticFS = staticFS
	config.boundHost = s.host
	config.httpPort = s.port
	config.resourceAuth = append([]gin.HandlerFunc(nil), s.resourceAuth...)
	return &config
}

func (s *Server) handlerDependencies() handlerConfig {
	return handlerConfig{
		sessions:            s.sessions,
		drainController:     s.drainController,
		sessionCatalog:      s.sessionCatalog,
		tasks:               s.tasks,
		network:             s.network,
		networkStore:        s.networkStore,
		networkUsage:        s.networkUsage,
		coordination:        s.coordination,
		observer:            s.observer,
		schemaStreams:       s.schemaStreams,
		resources:           s.resources,
		windowManager:       s.windowManager,
		terminal:            s.terminal,
		automation:          s.automation,
		loops:               s.loops,
		bridges:             s.bridges,
		notifications:       s.notifications,
		profiles:            s.profiles,
		supportBundles:      s.supportBundles,
		tools:               s.tools,
		toolArtifacts:       s.toolArtifacts,
		sessionAttachments:  s.sessionAttachments,
		toolsets:            s.toolsets,
		toolApprovals:       s.toolApprovals,
		approvalGrants:      s.approvalGrants,
		approvalCoordinator: s.approvalCoordinator,
		cmdPalette:          s.cmdPalette,
		clarify:             s.clarify,
		settings:            s.settings,
		settingsRestart:     s.settingsRestart,
		settingsUpdate:      s.settingsUpdate,
		vault:               s.vault,
		workspaces:          s.workspaces,
		worktrees:           s.worktrees,
		workspaceAccess:     s.workspaceAccess,
		onboarding:          s.onboarding,
		agentCatalog:        s.agentCatalog,
		agentSync:           s.agentSync,
		modelCatalog:        s.modelCatalog,
		marketplaceCatalog:  s.marketplaceCatalog,
		agentContext:        s.agentContext,
		coordinatorRole:     s.coordinatorRole,
		roles:               s.roles,
		soulAuthoring:       s.soulAuthoring,
		soulHistoryPurger:   s.soulHistoryPurger,
		soulRefresher:       s.soulRefresher,
		heartbeatAuthor:     s.heartbeatAuthor,
		heartbeatPurger:     s.heartbeatPurger,
		heartbeatStatus:     s.heartbeatStatus,
		heartbeatWake:       s.heartbeatWake,
		sessionHealth:       s.sessionHealth,
		wakeEvents:          s.wakeEvents,
		skillsRegistry:      s.skillsRegistry,
		skillResources:      s.skillResources,
		memoryStore:         s.memoryStore,
		dreamTrigger:        s.dreamTrigger,
		memoryExtractor:     s.memoryExtractor,
		memoryProviders:     s.memoryProviders,
		memoryLedger:        s.memoryLedger,
		runtimeMemory:       s.runtimeMemory,
		deadEntities:        s.deadEntities,
		gateway:             s.gateway,
		gatewayAdmission:    s.gatewayAdmission,
		deviceAuth:          s.deviceAuth,
		authLimiter:         s.authLimiter,
		ingressLimiter:      s.ingressLimiter,
		surfaceSet:          s.surfaceSet,
		homePaths:           s.homePaths,
		config:              s.config,
		logger:              s.logger,
		startedAt:           s.startedAt,
		now:                 s.now,
		pollInterval:        s.pollInterval,
		agentLoader:         s.agentLoader,
		extensions:          s.extensions,
	}
}
