package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
	builtintools "github.com/compozy/compozy/internal/tools/builtin"
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
		descriptors := builtintools.NativeDescriptors()
		toolUniverse := make([]toolspkg.ToolID, 0, len(descriptors))
		for _, descriptor := range descriptors {
			toolUniverse = append(toolUniverse, descriptor.ID)
		}
		return session.NewManager(
			session.WithHomePaths(deps.HomePaths),
			session.WithLifecycleContext(ctx),
			session.WithLogger(deps.Logger),
			session.WithNotifier(deps.Notifier),
			session.WithSpawnWakeNotifier(deps.SpawnWakeNotifier),
			session.WithHookSet(deps.Hooks),
			session.WithPromptAssembler(deps.PromptAssembler),
			session.WithStartupPromptOverlay(deps.StartupPromptOverlay),
			session.WithPromptInputAugmenter(deps.PromptInputAugmenter),
			session.WithCommandService(deps.CommandService),
			session.WithWorkAdmissionChecker(deps.WorkAdmission),
			session.WithAgentResolver(deps.AgentResolver),
			session.WithSkillRegistry(deps.SkillRegistry),
			session.WithToolsetCatalog(toolsets),
			session.WithToolUniverse(toolUniverse),
			session.WithMCPResolver(deps.MCPResolver),
			session.WithWorkspaceResolver(deps.WorkspaceResolver),
			session.WithWorktreeResolver(deps.WorktreeResolver),
			session.WithParticipationResolver(deps.ParticipationResolver),
			session.WithWindowReconciler(deps.WindowReconciler),
			session.WithSandboxRegistry(deps.SandboxRegistry),
			session.WithSessionSupervision(deps.SessionSupervision),
			session.WithSessionBusyInputConfig(deps.SessionBusyInput),
			session.WithSessionCompactionConfig(deps.SessionCompaction),
			session.WithSessionInputQueueStore(deps.SessionInputQueue),
			session.WithSessionPromptAdmissionStore(deps.SessionPromptAdmission),
			session.WithAttachmentOpener(deps.SessionAttachments),
			session.WithSessionHealthConfig(deps.SessionHealthConfig),
			session.WithAttentionConfig(deps.AttentionConfig),
			session.WithAttentionWorkspaceMuteReader(deps.AttentionWorkspaceMutes),
			session.WithSessionHealthStore(deps.SessionHealthStore),
			session.WithSessionCatalog(deps.SessionCatalog),
			session.WithHostedMCPLauncher(deps.HostedMCP),
			session.WithProviderSecretResolver(deps.ProviderSecrets),
			session.WithProfileNameResolver(deps.ProfileNames),
			session.WithModelCatalog(deps.ModelCatalog),
			session.WithSoulSnapshotStore(deps.SoulStore),
			session.WithSoulRunActivityChecker(deps.SoulRunChecker),
			session.WithLedgerMaterializer(deps.LedgerMaterializer),
			session.WithDriver(session.NewACPDriverAdapter(acp.New(
				acp.WithLogger(deps.Logger),
				acp.WithProcessRegistry(deps.ProcessRegistry),
				acp.WithProviderPreStarter(d.providerPreStarter),
			))),
		)
	}
}
