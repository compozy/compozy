package udsapi

import (
	"log/slog"
	"time"

	"github.com/compozy/agh/internal/api/core"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/doctor"
	mcppkg "github.com/compozy/agh/internal/mcp"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/store"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/windowmanager"
	workspacepkg "github.com/compozy/agh/internal/workspace"
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
	homePaths          aghconfig.HomePaths
	config             aghconfig.Config
	logger             *slog.Logger
	startedAt          time.Time
	now                func() time.Time
	pollInterval       time.Duration
	agentLoader        core.AgentLoader
	extensions         ExtensionService
	hostedMCP          *mcppkg.HostedService
	mcpHostAPI         mcppkg.HostAPIInvoker
}
