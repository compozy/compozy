package daemon

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/session"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/heartbeat"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/resources"

	"github.com/compozy/compozy/internal/situation"
	"github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/soul"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
)

// daemonRuntimeState is one published daemon generation. Assigning or clearing
// this value moves every runtime-owned handle through the same transition.
type daemonRuntimeState struct {
	subagents         session.SubagentService
	lock              *Lock
	harnessResolver   *HarnessContextResolver
	registry          Registry
	profiles          *profile.Manager
	runtimeWorkers    daemonRuntimeWorkers
	terminals         *terminalpkg.Service
	situationContext  *situation.Service
	sessions          SessionManager
	sessionWakeBridge *sessionWakeBridge
	tasks             *taskRuntime
	coordinator       *coordinatorRuntime
	spawnReaper       *spawnReaper
	scheduler         *schedulerRuntime
	gateway           gateway.Policy
	toolRegistry      toolspkg.Registry
	clarify           *clarifyBridge
	hooks             hookRuntime
	extensions        extensionRuntime
	observer          Observer
	resourceReconcile resources.ReconcileDriver
	supportBundles    supportBundleShutdowner
	backgroundUpdates *backgroundUpdateRuntime
	agentCatalog      *resourceCatalog[compozyconfig.AgentDef]
	soulCatalog       *resourceCatalog[soul.ResourceSpec]
	heartbeatCatalog  *resourceCatalog[heartbeat.ResourceSpec]
	toolCatalog       *resourceCatalog[toolspkg.Tool]
	mcpServerCatalog  *resourceCatalog[compozyconfig.MCPServer]
	loopCatalog       *resourceCatalog[looppkg.ResourceSpec]
	automation        automationRuntime
	httpServer        Server
	udsServer         Server
	workspaceRuntimeState
	worktrees *worktree.Service
	windowManagerRuntime
	skillsRegistry    *skills.Registry
	modelCatalog      *modelCatalogRuntime
	marketplace       *marketplaceRuntime
	skillsCancel      context.CancelFunc
	skillsDone        chan struct{}
	loopsCancel       context.CancelFunc
	loopsDone         chan struct{}
	goalOutboxCancel  context.CancelFunc
	goalOutboxDone    chan struct{}
	effectRelayCancel context.CancelFunc
	effectRelayDone   chan struct{}
	viewPatches       *extensionCmdPaletteProvider
	startedAt         time.Time
	info              Info
}

type workspaceUnregisterFinalizer interface {
	DrainUnregisters(context.Context) error
}

type workspaceRuntimeState struct {
	workspaceResolver  workspacepkg.RuntimeResolver
	workspaceFinalizer workspaceUnregisterFinalizer
}
