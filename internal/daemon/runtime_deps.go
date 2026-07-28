package daemon

import (
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/udsapi"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/doctor"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/situation"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// RuntimeDeps captures the composition-root dependencies available to server factories.
type RuntimeDeps struct {
	Config              compozyconfig.Config
	HomePaths           compozyconfig.HomePaths
	Logger              *slog.Logger
	Sessions            SessionManager
	DrainController     core.DaemonDrainController
	Tasks               taskpkg.Manager
	Network             core.NetworkService
	ToolRegistry        toolspkg.Registry
	Toolsets            core.ToolsetRegistry
	ToolArtifacts       toolspkg.ToolArtifactStore
	ToolApprovals       toolspkg.ApprovalTokenIssuer
	ApprovalGrants      toolspkg.ApprovalGrantStore
	Clarify             toolspkg.ClarifyBroker
	HostedMCP           *mcppkg.HostedService
	MCPHostAPI          mcppkg.HostAPIInvoker
	Observer            Observer
	SchemaStreams       core.SchemaStreamStatusReader
	Automation          core.AutomationManager
	Loops               core.LoopService
	Bridges             core.BridgeService
	Notifications       core.NotificationPresetService
	Registry            Registry
	MemoryStore         *memory.Store
	MemoryExtractor     core.MemoryExtractorService
	MemoryProviders     core.MemoryProviderService
	MemorySessionLedger core.MemorySessionLedgerService
	RuntimeMemory       doctor.RuntimeMemorySnapshotSource
	DeadEntities        doctor.DeadEntitySource
	WorkspaceResolver   workspacepkg.RuntimeResolver
	WorkspaceService    core.WorkspaceService
	AgentCatalog        core.AgentCatalog
	AgentDefinitionSync core.AgentDefinitionSync
	ModelCatalog        core.ModelCatalogService
	MarketplaceCatalog  core.MarketplaceCatalogService
	AgentContext        *situation.Service
	SoulAuthoring       core.SoulAuthoringService
	SoulHistoryPurger   core.SoulHistoryPurger
	SoulRefresher       core.SoulRefresher
	HeartbeatAuthor     core.HeartbeatAuthoringService
	HeartbeatPurger     core.HeartbeatHistoryPurger
	HeartbeatStatus     core.HeartbeatStatusService
	HeartbeatWake       core.HeartbeatWakeService
	SessionHealth       core.SessionHealthReader
	WakeEvents          core.HeartbeatWakeEventReader
	CoordinatorRole     CoordinatorRoleResolver
	Roles               core.RolesStatusProvider
	SkillsRegistry      core.SkillsRegistry
	SkillResources      core.SkillResourceSyncer
	DreamTrigger        DreamTrigger
	Settings            core.SettingsService
	SettingsRestart     core.SettingsRestartController
	SettingsUpdate      core.SettingsUpdateController
	SupportBundles      core.SupportBundleService
	Vault               core.VaultService
	Extensions          udsapi.ExtensionService
	Bundles             core.BundleService
	Resources           core.ResourceService
	WindowManager       windowmanager.Service
	StartedAt           time.Time
}
