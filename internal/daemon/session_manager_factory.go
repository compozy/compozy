package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/session"
	builtintools "github.com/compozy/agh/internal/tools/builtin"
)

func (d *Daemon) applySessionManagerFactoryDefault() {
	if d.newSessionManager != nil {
		return
	}
	d.newSessionManager = func(ctx context.Context, deps SessionManagerDeps) (SessionManager, error) {
		toolsets, err := builtintools.ToolsetCatalog()
		if err != nil {
			return nil, fmt.Errorf("daemon: build session toolset catalog: %w", err)
		}
		return session.NewManager(
			session.WithHomePaths(deps.HomePaths),
			session.WithLifecycleContext(ctx),
			session.WithLogger(deps.Logger),
			session.WithNotifier(deps.Notifier),
			session.WithHookSet(deps.Hooks),
			session.WithPromptAssembler(deps.PromptAssembler),
			session.WithStartupPromptOverlay(deps.StartupPromptOverlay),
			session.WithPromptInputAugmenter(deps.PromptInputAugmenter),
			session.WithWorkAdmissionChecker(deps.WorkAdmission),
			session.WithAgentResolver(deps.AgentResolver),
			session.WithSkillRegistry(deps.SkillRegistry),
			session.WithToolsetCatalog(toolsets),
			session.WithMCPResolver(deps.MCPResolver),
			session.WithModelCatalog(deps.ModelCatalog),
			session.WithWorkspaceResolver(deps.WorkspaceResolver),
			session.WithParticipationResolver(deps.ParticipationResolver),
			session.WithSandboxRegistry(deps.SandboxRegistry),
			session.WithSessionSupervision(deps.SessionSupervision),
			session.WithSessionBusyInputConfig(deps.SessionBusyInput),
			session.WithSessionCompactionConfig(deps.SessionCompaction),
			session.WithSessionInputQueueStore(deps.SessionInputQueue),
			session.WithSessionHealthConfig(deps.SessionHealthConfig),
			session.WithSessionHealthStore(deps.SessionHealthStore),
			session.WithSessionCatalog(deps.SessionCatalog),
			session.WithHostedMCPLauncher(deps.HostedMCP),
			session.WithProviderSecretResolver(deps.ProviderSecrets),
			session.WithSoulSnapshotStore(deps.SoulStore),
			session.WithSoulRunActivityChecker(deps.SoulRunChecker),
			session.WithLedgerMaterializer(deps.LedgerMaterializer),
			session.WithDriver(session.NewACPDriverAdapter(acp.New(
				acp.WithLogger(deps.Logger),
				acp.WithProcessRegistry(deps.ProcessRegistry),
				acp.WithSteerSource(sessionSteerSource{queue: deps.SessionInputQueue, now: d.now}),
			))),
		)
	}
}
