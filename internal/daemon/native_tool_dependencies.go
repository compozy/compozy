package daemon

import (
	"context"

	core "github.com/compozy/agh/internal/api/core"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	memorypkg "github.com/compozy/agh/internal/memory"
	skillspkg "github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/windowmanager"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type daemonNativeSkillsRegistry interface {
	core.SkillsRegistry
	ForAgentSession(
		ctx context.Context,
		resolved *workspacepkg.ResolvedWorkspace,
		agentName string,
		sessionID string,
	) ([]*skillspkg.Skill, error)
	ForAgentDefSession(
		ctx context.Context,
		resolved *workspacepkg.ResolvedWorkspace,
		agent aghconfig.AgentDef,
		sessionID string,
	) ([]*skillspkg.Skill, error)
}

type daemonNativeToolsDeps struct {
	Registry                   func() toolspkg.Registry
	ToolArtifacts              toolspkg.ToolArtifactStore
	Config                     aghconfig.Config
	Skills                     daemonNativeSkillsRegistry
	Sessions                   core.SessionManager
	Workspaces                 core.WorkspaceService
	WorkspaceResolver          workspacepkg.RuntimeResolver
	ModelCatalog               core.ModelCatalogService
	MarketplaceCatalog         core.MarketplaceCatalogService
	MarketplaceSkills          core.SkillMarketplaceService
	MarketplaceInstalledSkills core.InstalledSkillMarketplaceService
	Settings                   func() core.SettingsService
	Network                    core.NetworkService
	NetworkStore               core.NetworkStore
	NetworkUsage               store.NetworkUsageStore
	Tasks                      taskpkg.Manager
	MemoryStore                *memorypkg.Store
	MemoryToolWrites           memoryToolWriteRecorder
	DreamTrigger               core.DreamTrigger
	MemoryExtractor            core.MemoryExtractorService
	MemoryProviders            core.MemoryProviderService
	MemorySessionLedger        core.MemorySessionLedgerService
	Bridges                    core.BridgeService
	HomePaths                  aghconfig.HomePaths
	Observer                   core.Observer
	HookBindings               hookBindingPublisher
	AgentCatalog               core.AgentCatalog
	HeartbeatStatus            core.HeartbeatStatusService
	HeartbeatWake              core.HeartbeatWakeService
	SessionHealth              core.SessionHealthReader
	WakeEvents                 core.HeartbeatWakeEventReader
	Automation                 core.AutomationManager
	AutomationRuntime          func() core.AutomationManager
	ExtensionRegistry          *extensionpkg.Registry
	Extensions                 func() core.ExtensionService
	ExtensionRuntime           func() extensionRuntime
	ExtensionMarket            aghconfig.ExtensionsMarketplaceConfig
	ExtensionSources           extensionMarketplaceSourceLoader
	ExtensionEvents            store.EventSummaryStore
	AgentSkills                agentSkillPublisher
	AgentSkillsRuntime         func() agentSkillPublisher
	ToolMCP                    toolMCPPublisher
	MCPAuth                    func() toolspkg.MCPAuthStatusProvider
	ApprovalGrants             toolspkg.ApprovalGrantStore
	Clarify                    func() toolspkg.ClarifyBroker
	BundleResources            bundleResourcePublisher
	LoopResources              loopResourcePublisher
	BundleService              func() core.BundleService
	Loops                      func() core.LoopService
	Resources                  core.ResourceService
	WindowManager              windowmanager.Service
}

func (d *daemonNativeToolsDeps) agentSkills() agentSkillPublisher {
	if d == nil {
		return nil
	}
	if d.AgentSkillsRuntime != nil {
		if publisher := d.AgentSkillsRuntime(); publisher != nil {
			return publisher
		}
	}
	return d.AgentSkills
}
