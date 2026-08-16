//go:build integration

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	speccycle "github.com/compozy/compozy/extensions/spec-cycle"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/heartbeat"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/session"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/soul"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

var specCycleIntegrationSkillNames = []string{
	"cy-create-spec",
	"cy-create-tasks",
	"cy-execute-task",
	"cy-final-verify",
	"cy-fix-reviews",
	"cy-orchestrate-tasks",
	"cy-review-round",
	"cy-workflow-memory",
	"git-rebase",
}

func TestAgentSkillPublicationAndBootRebuild(t *testing.T) {
	t.Run("Should publish agent and skill resources and rebuild them on boot", func(t *testing.T) {
		db := openDaemonTestGlobalDB(t)
		kernel, err := resources.NewKernel(db.DB())
		if err != nil {
			t.Fatalf("resources.NewKernel() error = %v", err)
		}

		agentCodec, err := compozyconfig.NewAgentResourceCodec()
		if err != nil {
			t.Fatalf("compozyconfig.NewAgentResourceCodec() error = %v", err)
		}
		agentStore, err := resources.NewStore(kernel, agentCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(agent) error = %v", err)
		}
		skillCodec, err := skillspkg.NewResourceCodec()
		if err != nil {
			t.Fatalf("skillspkg.NewResourceCodec() error = %v", err)
		}
		skillStore, err := resources.NewStore(kernel, skillCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(skill) error = %v", err)
		}
		mcpCodec, err := compozyconfig.NewMCPServerResourceCodec()
		if err != nil {
			t.Fatalf("compozyconfig.NewMCPServerResourceCodec() error = %v", err)
		}
		mcpStore, err := resources.NewStore(kernel, mcpCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(mcp) error = %v", err)
		}
		soulCodec, err := soul.NewResourceCodec()
		if err != nil {
			t.Fatalf("soul.NewResourceCodec() error = %v", err)
		}
		soulStore, err := resources.NewStore(kernel, soulCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(soul) error = %v", err)
		}
		heartbeatCodec, err := heartbeat.NewResourceCodec()
		if err != nil {
			t.Fatalf("heartbeat.NewResourceCodec() error = %v", err)
		}
		heartbeatStore, err := resources.NewStore(kernel, heartbeatCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(heartbeat) error = %v", err)
		}

		homePaths := agentSkillIntegrationHome(t)
		workspaceRoot := agentSkillIntegrationWorkspace(t)
		now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
		workspace := workspacepkg.Workspace{
			ID:           "ws_agent_skill",
			RootDir:      workspaceRoot,
			Name:         "agent-skill",
			DefaultAgent: "coder",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := db.InsertWorkspace(testutil.Context(t), workspace); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
		}
		workspaceResolver, err := workspacepkg.NewResolver(
			db,
			workspacepkg.WithHomePaths(homePaths),
			workspacepkg.WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatalf("workspace.NewResolver() error = %v", err)
		}

		extensionRegistry := extensionpkg.NewRegistry(db.DB())
		extensionSnapshot := agentSkillIntegrationExtension(t, extensionRegistry)
		runtime := &agentSkillIntegrationRuntime{extension: extensionSnapshot}

		initialAgentCatalog := newResourceCatalog(cloneAgentDef)
		initialSkillRegistry := skillspkg.NewRegistry(
			agentSkillIntegrationSkillConfig(homePaths),
			skillspkg.WithLogger(discardLogger()),
		)
		initialMCPCatalog := newResourceCatalog(cloneDaemonMCPServer)
		initialSoulCatalog := newResourceCatalog(cloneSoulResourceSpec)
		initialHeartbeatCatalog := newResourceCatalog(cloneHeartbeatResourceSpec)
		driver := newAgentSkillIntegrationDriver(
			t,
			kernel,
			agentCodec,
			skillCodec,
			mcpCodec,
			initialAgentCatalog,
			initialSkillRegistry,
			initialMCPCatalog,
			&agentSkillIntegrationSidecars{
				soulCodec: soulCodec, soulCatalog: initialSoulCatalog,
				heartbeatCodec: heartbeatCodec, heartbeatCatalog: initialHeartbeatCatalog,
			},
		)

		syncer := newAgentSkillSourceSyncer(agentSkillSourceSyncerDeps{
			raw: kernel, agentStore: agentStore, agentCodec: agentCodec,
			agentProjector: newAgentProjector(initialAgentCatalog),
			soulStore:      soulStore, soulCodec: soulCodec,
			heartbeatStore: heartbeatStore, heartbeatCodec: heartbeatCodec,
			skillStore: skillStore, skillCodec: skillCodec,
			skillProjector: newSkillProjector(initialSkillRegistry),
			mcpStore:       mcpStore, mcpCodec: mcpCodec,
			actor: agentSkillSyncActor(), logger: discardLogger(),
			trigger: func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
				return driver.Trigger(ctx, kind, reason)
			},
			providers: []agentSkillDeclarationProvider{
				daemonAgentSkillDeclarationProvider(
					homePaths,
					db,
					workspaceResolver,
					initialSkillRegistry,
					discardLogger(),
				),
				extensionAgentSkillDeclarationProvider(
					extensionRegistry,
					func() extensionRuntime { return runtime },
					discardLogger(),
				),
			},
		})
		if err := syncer.Sync(testutil.Context(t)); err != nil {
			t.Fatalf("syncer.Sync() error = %v", err)
		}

		source := agentSkillSyncActor().Source
		agents, err := agentStore.List(
			testutil.Context(t),
			agentSkillSyncActor(),
			resources.ResourceFilter{Source: &source},
		)
		if err != nil {
			t.Fatalf("agentStore.List() error = %v", err)
		}
		if got, want := len(agents), 2; got != want {
			t.Fatalf("len(agentStore.List()) = %d, want %d (%#v)", got, want, agents)
		}
		assertExtensionAgentSidecarOwnership(
			t,
			testutil.Context(t),
			agents,
			soulStore,
			heartbeatStore,
			extensionSnapshot.Info.Name,
		)
		skills, err := skillStore.List(
			testutil.Context(t),
			agentSkillSyncActor(),
			resources.ResourceFilter{Source: &source},
		)
		if err != nil {
			t.Fatalf("skillStore.List() error = %v", err)
		}
		if got, want := len(skills), 2; got != want {
			t.Fatalf("len(skillStore.List()) = %d, want %d (%#v)", got, want, skills)
		}
		servers, err := mcpStore.List(
			testutil.Context(t),
			agentSkillSyncActor(),
			resources.ResourceFilter{Source: &source},
		)
		if err != nil {
			t.Fatalf("mcpStore.List() error = %v", err)
		}
		if got, want := len(servers), 4; got != want {
			t.Fatalf("len(mcpStore.List()) = %d, want %d (%#v)", got, want, servers)
		}
		if err := syncer.Sync(testutil.Context(t)); err != nil {
			t.Fatalf("second syncer.Sync() error = %v", err)
		}

		rebuiltAgentCatalog := newResourceCatalog(cloneAgentDef)
		rebuiltSkillRegistry := skillspkg.NewRegistry(
			agentSkillIntegrationSkillConfig(homePaths),
			skillspkg.WithLogger(discardLogger()),
		)
		rebuiltMCPCatalog := newResourceCatalog(cloneDaemonMCPServer)
		rebuiltSoulCatalog := newResourceCatalog(cloneSoulResourceSpec)
		rebuiltHeartbeatCatalog := newResourceCatalog(cloneHeartbeatResourceSpec)
		bootDriver := newAgentSkillIntegrationDriver(
			t,
			kernel,
			agentCodec,
			skillCodec,
			mcpCodec,
			rebuiltAgentCatalog,
			rebuiltSkillRegistry,
			rebuiltMCPCatalog,
			&agentSkillIntegrationSidecars{
				soulCodec: soulCodec, soulCatalog: rebuiltSoulCatalog,
				heartbeatCodec: heartbeatCodec, heartbeatCatalog: rebuiltHeartbeatCatalog,
			},
		)
		if err := bootDriver.RunBoot(testutil.Context(t)); err != nil {
			t.Fatalf("bootDriver.RunBoot() error = %v", err)
		}

		resolved, err := workspaceResolver.Resolve(testutil.Context(t), workspace.ID)
		if err != nil {
			t.Fatalf("workspaceResolver.Resolve() error = %v", err)
		}
		agentCatalog := agentCatalogDependency(rebuiltAgentCatalog, agentSidecarCatalogs{
			soul: rebuiltSoulCatalog, heartbeat: rebuiltHeartbeatCatalog,
		})
		coder, err := agentCatalog.ResolveAgent("coder", &resolved)
		if err != nil {
			t.Fatalf("ResolveAgent(coder) error = %v", err)
		}
		if !slices.Contains(coder.Tools, "compozy__lookup") {
			t.Fatalf("ResolveAgent(coder).Tools = %#v, want canonical lookup tool reference preserved", coder.Tools)
		}
		if !agentHasMCP(coder, "workspace-agent-mcp") {
			t.Fatalf("ResolveAgent(coder).MCPServers = %#v, want workspace-agent-mcp", coder.MCPServers)
		}
		extAgent, err := agentCatalog.ResolveAgent("ext-agent", &resolved)
		if err != nil {
			t.Fatalf("ResolveAgent(ext-agent) error = %v", err)
		}
		if !agentHasMCP(extAgent, "ext-agent-mcp") {
			t.Fatalf("ResolveAgent(ext-agent).MCPServers = %#v, want ext-agent-mcp", extAgent.MCPServers)
		}
		artifacts, err := agentCatalog.ResolveAgentArtifacts("ext-agent", &resolved)
		if err != nil {
			t.Fatalf("ResolveAgentArtifacts(ext-agent) error = %v", err)
		}
		if !artifacts.PackageOwned || artifacts.SoulBody != "Write with extension context.\n" ||
			artifacts.HeartbeatBody != "Check extension work.\n" {
			t.Fatalf("ResolveAgentArtifacts(ext-agent) = %#v, want extension-owned sidecars", artifacts)
		}
		policy, ok, err := agentCatalog.ResolveHeartbeatPolicy(testutil.Context(t), heartbeat.AuthoringTarget{
			AgentName: "ext-agent", WorkspaceID: resolved.ID, WorkspaceRoot: resolved.RootDir,
		})
		if err != nil {
			t.Fatalf("ResolveHeartbeatPolicy(ext-agent) error = %v", err)
		}
		if !ok || policy.SourcePath != "agents/ext-agent/HEARTBEAT.md" {
			t.Fatalf("ResolveHeartbeatPolicy(ext-agent) = %#v, %v", policy, ok)
		}
		resolved.Config.Providers["claude"] = compozyconfig.ProviderConfig{Command: "claude-acp"}
		resolved.Config.Roles.Dream.Agent = "ext-agent"
		roleResolver := newRoleResolver(
			&resolved.Config,
			roleWorkspaceResolverStub{configs: map[string]compozyconfig.Config{workspace.ID: resolved.Config}},
			agentCatalog,
		)
		resolvedRole, err := roleResolver.Resolve(testutil.Context(t), workspace.ID, compozyconfig.RoleDream)
		if err != nil {
			t.Fatalf("roleResolver.Resolve(dream) error = %v", err)
		}
		if resolvedRole.AgentName != "ext-agent" || !agentHasMCP(resolvedRole.AgentDef, "ext-agent-mcp") {
			t.Fatalf("roleResolver.Resolve(dream) = %#v, want projected ext-agent resource", resolvedRole)
		}

		projectedSkills, err := rebuiltSkillRegistry.ForWorkspace(testutil.Context(t), &resolved)
		if err != nil {
			t.Fatalf("rebuiltSkillRegistry.ForWorkspace() error = %v", err)
		}
		review := findIntegrationSkill(projectedSkills, "workspace-review")
		if review == nil {
			t.Fatalf("ForWorkspace() = %#v, want workspace-review", projectedSkills)
		}
		if !skillHasMCP(review, "workspace-skill-mcp") {
			t.Fatalf("workspace-review MCPServers = %#v, want workspace-skill-mcp", review.MCPServers)
		}
		extSkill := findIntegrationSkill(projectedSkills, "ext-skill")
		if extSkill == nil {
			t.Fatalf("ForWorkspace() = %#v, want ext-skill", projectedSkills)
		}
		if !skillHasMCP(extSkill, "ext-skill-mcp") {
			t.Fatalf("ext-skill MCPServers = %#v, want ext-skill-mcp", extSkill.MCPServers)
		}
		if !mcpCatalogHas(rebuiltMCPCatalog, "workspace-agent-mcp") ||
			!mcpCatalogHas(rebuiltMCPCatalog, "workspace-skill-mcp") ||
			!mcpCatalogHas(rebuiltMCPCatalog, "ext-agent-mcp") ||
			!mcpCatalogHas(rebuiltMCPCatalog, "ext-skill-mcp") {
			t.Fatalf("rebuilt MCP catalog = %#v, want all agent/skill MCP attachments", rebuiltMCPCatalog.Snapshot())
		}

		if err := extensionRegistry.Disable(extensionSnapshot.Info.Name); err != nil {
			t.Fatalf("extensionRegistry.Disable() error = %v", err)
		}
		if err := syncer.Sync(testutil.Context(t)); err != nil {
			t.Fatalf("syncer.Sync(after disable) error = %v", err)
		}
		if err := bootDriver.RunBoot(testutil.Context(t)); err != nil {
			t.Fatalf("bootDriver.RunBoot(after disable) error = %v", err)
		}
		owner := extensionOwner(extensionSnapshot.Info.Name)
		for label, count := range map[string]int{
			"agents":     countOwnedResourceRecords(t, testutil.Context(t), agentStore, owner),
			"souls":      countOwnedResourceRecords(t, testutil.Context(t), soulStore, owner),
			"heartbeats": countOwnedResourceRecords(t, testutil.Context(t), heartbeatStore, owner),
		} {
			if count != 0 {
				t.Fatalf("owned %s after disable = %d, want 0", label, count)
			}
		}
		if _, err := agentCatalog.ResolveAgent("ext-agent", &resolved); !errors.Is(
			err,
			workspacepkg.ErrAgentNotAvailable,
		) {
			t.Fatalf("ResolveAgent(ext-agent after disable) error = %v, want ErrAgentNotAvailable", err)
		}
	})

	t.Run("Should publish portable skills and MCP servers without mutating a running snapshot", func(t *testing.T) {
		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		kernel, err := resources.NewKernel(db.DB())
		if err != nil {
			t.Fatalf("resources.NewKernel() error = %v", err)
		}

		agentCodec, err := compozyconfig.NewAgentResourceCodec()
		if err != nil {
			t.Fatalf("compozyconfig.NewAgentResourceCodec() error = %v", err)
		}
		agentStore, err := resources.NewStore(kernel, agentCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(agent) error = %v", err)
		}
		skillCodec, err := skillspkg.NewResourceCodec()
		if err != nil {
			t.Fatalf("skillspkg.NewResourceCodec() error = %v", err)
		}
		skillStore, err := resources.NewStore(kernel, skillCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(skill) error = %v", err)
		}
		mcpCodec, err := compozyconfig.NewMCPServerResourceCodec()
		if err != nil {
			t.Fatalf("compozyconfig.NewMCPServerResourceCodec() error = %v", err)
		}
		mcpStore, err := resources.NewStore(kernel, mcpCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(mcp) error = %v", err)
		}
		toolCodec, err := toolspkg.NewResourceCodec()
		if err != nil {
			t.Fatalf("toolspkg.NewResourceCodec() error = %v", err)
		}
		toolStore, err := resources.NewStore(kernel, toolCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(tool) error = %v", err)
		}
		sidecars := newAgentSkillSourceSidecarStores(t, kernel)

		homePaths := agentSkillIntegrationHome(t)
		registry := extensionpkg.NewRegistry(db.DB())
		portableName := installPortablePublisherIntegrationExtension(t, homePaths, registry)
		manager := extensionpkg.NewManager(
			registry,
			extensionpkg.WithHomePaths(homePaths),
			extensionpkg.WithLogger(discardLogger()),
		)
		if err := manager.Start(ctx); err != nil {
			t.Fatalf("extension manager Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(context.Background()); err != nil {
				t.Errorf("extension manager Stop() error = %v", err)
			}
		})

		agentSkillSyncer := newAgentSkillSourceSyncer(agentSkillSourceSyncerDeps{
			raw: kernel, agentStore: agentStore, agentCodec: agentCodec,
			soulStore: sidecars.soulStore, soulCodec: sidecars.soulCodec,
			heartbeatStore: sidecars.heartbeatStore, heartbeatCodec: sidecars.heartbeatCodec,
			skillStore: skillStore, skillCodec: skillCodec,
			mcpStore: mcpStore, mcpCodec: mcpCodec,
			actor: agentSkillSyncActor(), logger: discardLogger(),
			providers: []agentSkillDeclarationProvider{
				extensionAgentSkillDeclarationProvider(
					registry,
					func() extensionRuntime { return manager },
					discardLogger(),
				),
			},
		})
		toolMCPSyncer := newToolMCPSourceSyncer(
			kernel,
			toolStore,
			toolCodec,
			mcpStore,
			mcpCodec,
			toolMCPSyncActor(),
			discardLogger(),
			nil,
			extensionManifestToolMCPDeclarationProvider(
				registry,
				func() extensionRuntime { return manager },
				nil,
				discardLogger(),
			),
		)
		if err := agentSkillSyncer.Sync(ctx); err != nil {
			t.Fatalf("agentSkillSyncer.Sync() error = %v", err)
		}
		if err := toolMCPSyncer.Sync(ctx); err != nil {
			t.Fatalf("toolMCPSyncer.Sync() error = %v", err)
		}

		owner := extensionOwner(portableName)
		if got, want := countOwnedResourceRecords(t, ctx, skillStore, owner), 30; got != want {
			t.Fatalf("owned portable skills = %d, want %d", got, want)
		}
		portableInfo, err := registry.Get(portableName)
		if err != nil {
			t.Fatalf("registry.Get(%q) error = %v", portableName, err)
		}
		if got, want := len(portableInfo.IngestDiagnostics), 1; got != want {
			t.Fatalf("portable ingest diagnostics = %d, want %d without truncation", got, want)
		}
		mcpRecords, err := mcpStore.List(ctx, toolMCPSyncActor(), resources.ResourceFilter{Owner: owner})
		if err != nil {
			t.Fatalf("mcpStore.List(portable owner) error = %v", err)
		}
		if got, want := len(mcpRecords), 1; got != want {
			t.Fatalf("owned portable MCP servers = %d, want %d (%#v)", got, want, mcpRecords)
		}
		dataPath, err := homePaths.ExtensionDataPath(portableName, "")
		if err != nil {
			t.Fatalf("homePaths.ExtensionDataPath() error = %v", err)
		}
		canonicalHome, err := filepath.EvalSymlinks(homePaths.HomeDir)
		if err != nil {
			t.Fatalf("filepath.EvalSymlinks(home) error = %v", err)
		}
		wantPluginRoot := filepath.Join(canonicalHome, "extensions", portableName)
		wantPluginData := filepath.Join(canonicalHome, "extension-data", portableName)
		server := mcpRecords[0].Spec
		if server.Env["PLUGIN_ROOT"] != wantPluginRoot ||
			server.Env["PLUGIN_DATA"] != wantPluginData || server.Env["LITERAL"] != "${UNKNOWN}" {
			t.Fatalf("portable MCP env = %#v, want absolute package/data roots and literal unknown placeholder", server.Env)
		}
		if _, err := os.Stat(dataPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(portable data path) error = %v, want not-exist before first launch", err)
		}
		runningSnapshot, err := manager.Get(portableName)
		if err != nil {
			t.Fatalf("manager.Get(%q) error = %v", portableName, err)
		}

		if err := registry.Disable(portableName); err != nil {
			t.Fatalf("registry.Disable(%q) error = %v", portableName, err)
		}
		if err := agentSkillSyncer.Sync(ctx); err != nil {
			t.Fatalf("agentSkillSyncer.Sync(after disable) error = %v", err)
		}
		if err := toolMCPSyncer.Sync(ctx); err != nil {
			t.Fatalf("toolMCPSyncer.Sync(after disable) error = %v", err)
		}
		if got := countOwnedResourceRecords(t, ctx, skillStore, owner); got != 0 {
			t.Fatalf("owned portable skills after disable = %d, want 0", got)
		}
		if got := countOwnedResourceRecords(t, ctx, mcpStore, owner); got != 0 {
			t.Fatalf("owned portable MCP servers after disable = %d, want 0", got)
		}
		if len(runningSnapshot.Skills) != 30 || len(runningSnapshot.Manifest.Resources.MCPServers) != 1 {
			t.Fatalf("running snapshot changed after disable: %#v", runningSnapshot)
		}

		degradedName := installFullyDegradedPublisherIntegrationExtension(t, homePaths, registry)
		if err := manager.Reload(ctx); err != nil {
			t.Fatalf("extension manager Reload(degraded) error = %v", err)
		}
		if err := agentSkillSyncer.Sync(ctx); err != nil {
			t.Fatalf("agentSkillSyncer.Sync(degraded) error = %v", err)
		}
		if err := toolMCPSyncer.Sync(ctx); err != nil {
			t.Fatalf("toolMCPSyncer.Sync(degraded) error = %v", err)
		}
		degradedOwner := extensionOwner(degradedName)
		if got := countOwnedResourceRecords(t, ctx, skillStore, degradedOwner); got != 0 {
			t.Fatalf("owned degraded skills = %d, want 0", got)
		}
		if got := countOwnedResourceRecords(t, ctx, mcpStore, degradedOwner); got != 0 {
			t.Fatalf("owned degraded MCP servers = %d, want 0", got)
		}
		degradedSnapshot, err := manager.Get(degradedName)
		if err != nil {
			t.Fatalf("manager.Get(%q) error = %v", degradedName, err)
		}
		if !degradedSnapshot.Status.Registered || !degradedSnapshot.Status.Enabled ||
			len(degradedSnapshot.Skills) != 0 || len(degradedSnapshot.Manifest.Resources.MCPServers) != 0 ||
			len(degradedSnapshot.Info.IngestDiagnostics) != 2 {
			t.Fatalf("degraded enabled snapshot = %#v, want registered zero-resource instance with two reasons", degradedSnapshot)
		}
	})
}

func TestSpecCycleBundledSkillPublicationAndBootRebuild(t *testing.T) {
	t.Parallel()

	t.Run("Should publish bundled skills while preserving workspace-local isolation", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		kernel, err := resources.NewKernel(db.DB())
		if err != nil {
			t.Fatalf("resources.NewKernel() error = %v", err)
		}
		agentCodec, err := compozyconfig.NewAgentResourceCodec()
		if err != nil {
			t.Fatalf("compozyconfig.NewAgentResourceCodec() error = %v", err)
		}
		agentStore, err := resources.NewStore(kernel, agentCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(agent) error = %v", err)
		}
		skillCodec, err := skillspkg.NewResourceCodec()
		if err != nil {
			t.Fatalf("skillspkg.NewResourceCodec() error = %v", err)
		}
		skillStore, err := resources.NewStore(kernel, skillCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(skill) error = %v", err)
		}
		mcpCodec, err := compozyconfig.NewMCPServerResourceCodec()
		if err != nil {
			t.Fatalf("compozyconfig.NewMCPServerResourceCodec() error = %v", err)
		}
		mcpStore, err := resources.NewStore(kernel, mcpCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(mcp) error = %v", err)
		}
		sidecarStores := newAgentSkillSourceSidecarStores(t, kernel)

		homePaths := agentSkillIntegrationHome(t)
		extensionRegistry := extensionpkg.NewRegistry(db.DB())
		if err := speccycle.EnsureManagedInstall(homePaths, extensionRegistry); err != nil {
			t.Fatalf("speccycle.EnsureManagedInstall() error = %v", err)
		}
		if err := extensionRegistry.Enable(speccycle.Name); err != nil {
			t.Fatalf("extensionRegistry.Enable(%q) error = %v", speccycle.Name, err)
		}
		extensionSnapshot := agentSkillIntegrationSpecCycleExtension(t, extensionRegistry)
		runtime := &agentSkillIntegrationRuntime{extension: extensionSnapshot}

		workspaceARoot := agentSkillIntegrationSkillWorkspace(t, true)
		workspaceBRoot := agentSkillIntegrationSkillWorkspace(t, false)
		now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
		for _, workspace := range []workspacepkg.Workspace{
			{ID: "ws_spec_cycle_a", RootDir: workspaceARoot, Name: "spec-cycle-a", CreatedAt: now, UpdatedAt: now},
			{ID: "ws_spec_cycle_b", RootDir: workspaceBRoot, Name: "spec-cycle-b", CreatedAt: now, UpdatedAt: now},
		} {
			if err := db.InsertWorkspace(ctx, workspace); err != nil {
				t.Fatalf("InsertWorkspace(%q) error = %v", workspace.ID, err)
			}
		}
		workspaceResolver, err := workspacepkg.NewResolver(
			db,
			workspacepkg.WithHomePaths(homePaths),
			workspacepkg.WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatalf("workspace.NewResolver() error = %v", err)
		}

		initialSkills := skillspkg.NewRegistry(
			agentSkillIntegrationSkillConfig(homePaths),
			skillspkg.WithLogger(discardLogger()),
		)
		initialAgents := newResourceCatalog(cloneAgentDef)
		initialMCP := newResourceCatalog(cloneDaemonMCPServer)
		initialDriver := newAgentSkillIntegrationDriver(
			t,
			kernel,
			agentCodec,
			skillCodec,
			mcpCodec,
			initialAgents,
			initialSkills,
			initialMCP,
			nil,
		)
		syncer := newAgentSkillSourceSyncer(agentSkillSourceSyncerDeps{
			raw: kernel, agentStore: agentStore, agentCodec: agentCodec,
			agentProjector: newAgentProjector(initialAgents),
			soulStore:      sidecarStores.soulStore, soulCodec: sidecarStores.soulCodec,
			heartbeatStore: sidecarStores.heartbeatStore, heartbeatCodec: sidecarStores.heartbeatCodec,
			skillStore: skillStore, skillCodec: skillCodec, skillProjector: newSkillProjector(initialSkills),
			mcpStore: mcpStore, mcpCodec: mcpCodec,
			actor: agentSkillSyncActor(), logger: discardLogger(),
			trigger: func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
				return initialDriver.Trigger(ctx, kind, reason)
			},
			providers: []agentSkillDeclarationProvider{
				daemonAgentSkillDeclarationProvider(homePaths, db, workspaceResolver, initialSkills, discardLogger()),
				extensionAgentSkillDeclarationProvider(
					extensionRegistry,
					func() extensionRuntime { return runtime },
					discardLogger(),
				),
			},
		})
		if err := syncer.Sync(ctx); err != nil {
			t.Fatalf("syncer.Sync() error = %v", err)
		}

		source := agentSkillSyncActor().Source
		records, err := skillStore.List(ctx, agentSkillSyncActor(), resources.ResourceFilter{Source: &source})
		if err != nil {
			t.Fatalf("skillStore.List() error = %v", err)
		}
		globalSpecCycleCount := 0
		for _, record := range records {
			if record.Scope.Kind == resources.ResourceScopeKindGlobal &&
				record.Spec.InstalledFromExtension == speccycle.Name {
				globalSpecCycleCount++
			}
		}
		if globalSpecCycleCount != len(specCycleIntegrationSkillNames) {
			t.Fatalf(
				"global spec-cycle skill records = %d, want %d",
				globalSpecCycleCount,
				len(specCycleIntegrationSkillNames),
			)
		}

		rebuiltSkills := skillspkg.NewRegistry(
			agentSkillIntegrationSkillConfig(homePaths),
			skillspkg.WithLogger(discardLogger()),
		)
		bootDriver := newAgentSkillIntegrationDriver(
			t,
			kernel,
			agentCodec,
			skillCodec,
			mcpCodec,
			newResourceCatalog(cloneAgentDef),
			rebuiltSkills,
			newResourceCatalog(cloneDaemonMCPServer),
			nil,
		)
		if err := bootDriver.RunBoot(ctx); err != nil {
			t.Fatalf("bootDriver.RunBoot() error = %v", err)
		}

		resolvedA, err := workspaceResolver.Resolve(ctx, "ws_spec_cycle_a")
		if err != nil {
			t.Fatalf("workspaceResolver.Resolve(A) error = %v", err)
		}
		resolvedB, err := workspaceResolver.Resolve(ctx, "ws_spec_cycle_b")
		if err != nil {
			t.Fatalf("workspaceResolver.Resolve(B) error = %v", err)
		}
		skillsA, err := rebuiltSkills.ForWorkspace(ctx, &resolvedA)
		if err != nil {
			t.Fatalf("rebuiltSkills.ForWorkspace(A) error = %v", err)
		}
		skillsB, err := rebuiltSkills.ForWorkspace(ctx, &resolvedB)
		if err != nil {
			t.Fatalf("rebuiltSkills.ForWorkspace(B) error = %v", err)
		}
		for _, name := range specCycleIntegrationSkillNames {
			if findIntegrationSkill(skillsA, name) == nil || findIntegrationSkill(skillsB, name) == nil {
				t.Fatalf("skill %q missing: workspace A=%#v workspace B=%#v", name, skillsA, skillsB)
			}
		}
		if findIntegrationSkill(skillsA, "workspace-only-a") == nil {
			t.Fatal("workspace A missing workspace-only-a")
		}
		if findIntegrationSkill(skillsB, "workspace-only-a") != nil {
			t.Fatal("workspace B contains workspace-only-a from workspace A")
		}
		if findIntegrationSkill(skillsA, "cy-capture-decisions") != nil ||
			findIntegrationSkill(skillsB, "cy-capture-decisions") != nil {
			t.Fatal("cy-capture-decisions is available, want excluded from spec-cycle bundle")
		}

		executeA := findIntegrationSkill(skillsA, "cy-execute-task")
		executeB := findIntegrationSkill(skillsB, "cy-execute-task")
		if executeA.Source != skillspkg.SourceWorkspace {
			t.Fatalf("workspace A cy-execute-task source = %v, want workspace", executeA.Source)
		}
		if executeB.Source != skillspkg.SourceBundled || executeB.InstalledFromExtension != speccycle.Name {
			t.Fatalf("workspace B cy-execute-task = %#v, want bundled spec-cycle source", executeB)
		}
		contentA, err := rebuiltSkills.LoadContent(ctx, executeA)
		if err != nil {
			t.Fatalf("LoadContent(workspace A override) error = %v", err)
		}
		contentB, err := rebuiltSkills.LoadContent(ctx, executeB)
		if err != nil {
			t.Fatalf("LoadContent(workspace B bundled) error = %v", err)
		}
		if strings.TrimSpace(contentA) != "Workspace A execution override." {
			t.Fatalf("workspace A content = %q, want local override", contentA)
		}
		if !strings.Contains(contentB, "# Execute Spec Task") {
			t.Fatalf("workspace B bundled content = %q, want cy-execute-task body", contentB)
		}

		augmenter := newSkillsCatalogAugmenter(rebuiltSkills, nil, func() promptSkillsWorkspaceResolver {
			return workspaceResolver
		})
		for _, promptCase := range []struct {
			name          string
			session       *session.Session
			wantLocalOnly bool
		}{
			{
				name: "Should augment workspace A session with its local override",
				session: &session.Session{
					ID: "session-a", WorkspaceID: resolvedA.ID, Workspace: resolvedA.RootDir,
				},
				wantLocalOnly: true,
			},
			{
				name: "Should augment workspace B session without workspace A resources",
				session: &session.Session{
					ID: "session-b", WorkspaceID: resolvedB.ID, Workspace: resolvedB.RootDir,
				},
			},
		} {
			t.Run(promptCase.name, func(t *testing.T) {
				t.Parallel()

				prompt, err := augmenter(testutil.Context(t), promptCase.session, "Continue the task.")
				if err != nil {
					t.Fatalf("skills augmenter error = %v", err)
				}
				for _, name := range specCycleIntegrationSkillNames {
					if !strings.Contains(prompt, `name="`+name+`"`) {
						t.Fatalf("prompt missing spec-cycle skill %q: %s", name, prompt)
					}
				}
				gotLocalOnly := strings.Contains(prompt, `name="workspace-only-a"`)
				if gotLocalOnly != promptCase.wantLocalOnly {
					t.Fatalf("prompt workspace-only-a presence = %t, want %t", gotLocalOnly, promptCase.wantLocalOnly)
				}
			})
		}
	})
}

func TestAgentDefinitionMutationLifecycleIntegration(t *testing.T) {
	t.Run("Should create update duplicate delete and rebuild an agent definition end to end", func(t *testing.T) {
		// not parallel: gin.SetMode mutates process-global state.
		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		kernel, err := resources.NewKernel(db.DB())
		if err != nil {
			t.Fatalf("resources.NewKernel() error = %v", err)
		}
		agentCodec, err := compozyconfig.NewAgentResourceCodec()
		if err != nil {
			t.Fatalf("compozyconfig.NewAgentResourceCodec() error = %v", err)
		}
		agentStore, err := resources.NewStore(kernel, agentCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(agent) error = %v", err)
		}
		skillCodec, err := skillspkg.NewResourceCodec()
		if err != nil {
			t.Fatalf("skillspkg.NewResourceCodec() error = %v", err)
		}
		skillStore, err := resources.NewStore(kernel, skillCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(skill) error = %v", err)
		}
		mcpCodec, err := compozyconfig.NewMCPServerResourceCodec()
		if err != nil {
			t.Fatalf("compozyconfig.NewMCPServerResourceCodec() error = %v", err)
		}
		mcpStore, err := resources.NewStore(kernel, mcpCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(mcp) error = %v", err)
		}
		sidecarStores := newAgentSkillSourceSidecarStores(t, kernel)
		homePaths := agentSkillIntegrationHome(t)
		resolver, err := workspacepkg.NewResolver(
			db,
			workspacepkg.WithHomePaths(homePaths),
			workspacepkg.WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatalf("workspace.NewResolver() error = %v", err)
		}
		agentCatalog := newResourceCatalog(cloneAgentDef)
		skillRegistry := skillspkg.NewRegistry(
			agentSkillIntegrationSkillConfig(homePaths),
			skillspkg.WithLogger(discardLogger()),
		)
		mcpCatalog := newResourceCatalog(cloneDaemonMCPServer)
		driver := newAgentSkillIntegrationDriver(
			t,
			kernel,
			agentCodec,
			skillCodec,
			mcpCodec,
			agentCatalog,
			skillRegistry,
			mcpCatalog,
			nil,
		)
		syncer := newAgentSkillSourceSyncer(agentSkillSourceSyncerDeps{
			raw: kernel, agentStore: agentStore, agentCodec: agentCodec,
			agentProjector: newAgentProjector(agentCatalog),
			soulStore:      sidecarStores.soulStore, soulCodec: sidecarStores.soulCodec,
			heartbeatStore: sidecarStores.heartbeatStore, heartbeatCodec: sidecarStores.heartbeatCodec,
			skillStore: skillStore, skillCodec: skillCodec, skillProjector: newSkillProjector(skillRegistry),
			mcpStore: mcpStore, mcpCodec: mcpCodec,
			actor: agentSkillSyncActor(), logger: discardLogger(),
			trigger: func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
				return driver.Trigger(ctx, kind, reason)
			},
			providers: []agentSkillDeclarationProvider{
				daemonAgentSkillDeclarationProvider(homePaths, db, resolver, skillRegistry, discardLogger()),
			},
		})
		catalog := agentCatalogDependency(agentCatalog)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName:       "agent-mutation-integration",
			HomePaths:           homePaths,
			Workspaces:          resolver,
			AgentCatalog:        catalog,
			AgentDefinitionSync: syncer,
			Logger:              discardLogger(),
		})
		gin.SetMode(gin.TestMode)
		engine := gin.New()
		engine.POST("/api/agents", handlers.CreateAgent)
		engine.PUT("/api/agents/:name", handlers.UpdateAgent)
		engine.DELETE("/api/agents/:name", handlers.DeleteAgent)
		engine.POST("/api/agents/:name/duplicate", handlers.DuplicateAgent)

		create := performAgentMutationIntegrationRequest(
			t,
			engine,
			http.MethodPost,
			"/api/agents",
			contract.CreateAgentRequest{
				Scope: contract.AgentCreateScopeGlobal,
				Agent: contract.CreateAgentPayload{
					Name: "coder", Provider: "claude", Prompt: "Create integration fixtures.",
				},
			},
		)
		if create.Code != http.StatusCreated {
			t.Fatalf("create status = %d, want %d; body=%s", create.Code, http.StatusCreated, create.Body.String())
		}
		var created contract.AgentResponse
		decodeAgentMutationIntegrationResponse(t, create, &created)
		if _, err := catalog.GetAgent(ctx, "coder"); err != nil {
			t.Fatalf("catalog.GetAgent(coder after create) error = %v", err)
		}

		update := performAgentMutationIntegrationRequest(
			t,
			engine,
			http.MethodPut,
			"/api/agents/coder",
			contract.UpdateAgentRequest{
				Agent: contract.CreateAgentPayload{
					Name: "coder", Provider: "claude", Model: "claude-opus-4-8", Prompt: "Updated integration fixture.",
				},
				ExpectedDigest: created.Agent.DefinitionDigest,
			},
		)
		if update.Code != http.StatusOK {
			t.Fatalf("update status = %d, want %d; body=%s", update.Code, http.StatusOK, update.Body.String())
		}
		updatedEntry, err := catalog.GetAgent(ctx, "coder")
		if err != nil {
			t.Fatalf("catalog.GetAgent(coder after update) error = %v", err)
		}
		if updatedEntry.Def.Model != "claude-opus-4-8" {
			t.Fatalf("updated catalog model = %q, want claude-opus-4-8", updatedEntry.Def.Model)
		}

		sourceDir := filepath.Join(homePaths.AgentsDir, "coder")
		sidecars := map[string]string{
			"SOUL.md":      "Integration soul.\n",
			"HEARTBEAT.md": "Integration heartbeat.\n",
			compozyconfig.MCPJSONName: `{
  "mcpServers": {
    "integration": {
      "command": "integration-mcp",
      "secret_env": {"TOKEN": "env:INTEGRATION_TOKEN"}
    }
  }
}`,
		}
		for name, content := range sidecars {
			writeAgentSkillIntegrationFile(t, filepath.Join(sourceDir, name), content)
		}
		duplicate := performAgentMutationIntegrationRequest(
			t,
			engine,
			http.MethodPost,
			"/api/agents/coder/duplicate",
			contract.DuplicateAgentRequest{
				Name: "reviewer",
				Overrides: &contract.DuplicateAgentOverrides{
					Prompt: "Review the integration fixture.",
				},
			},
		)
		if duplicate.Code != http.StatusCreated {
			t.Fatalf(
				"duplicate status = %d, want %d; body=%s",
				duplicate.Code,
				http.StatusCreated,
				duplicate.Body.String(),
			)
		}
		if _, err := catalog.GetAgent(ctx, "reviewer"); err != nil {
			t.Fatalf("catalog.GetAgent(reviewer after duplicate) error = %v", err)
		}
		for name, want := range sidecars {
			got, err := os.ReadFile(filepath.Join(homePaths.AgentsDir, "reviewer", name))
			if err != nil {
				t.Fatalf("os.ReadFile(duplicate %s) error = %v", name, err)
			}
			if string(got) != want {
				t.Fatalf("duplicate %s = %q, want %q", name, got, want)
			}
		}

		pollDone := make(chan error, 1)
		go func() {
			for range 8 {
				if err := syncer.Sync(ctx); err != nil {
					pollDone <- err
					return
				}
			}
			pollDone <- nil
		}()
		deleted := performAgentMutationIntegrationRequest(t, engine, http.MethodDelete, "/api/agents/coder", nil)
		if deleted.Code != http.StatusOK {
			t.Fatalf("delete status = %d, want %d; body=%s", deleted.Code, http.StatusOK, deleted.Body.String())
		}
		if err := <-pollDone; err != nil {
			t.Fatalf("poll-equivalent Sync() error = %v", err)
		}
		if err := syncer.Sync(ctx); err != nil {
			t.Fatalf("final Sync() error = %v", err)
		}
		if _, err := catalog.GetAgent(ctx, "coder"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("catalog.GetAgent(coder after delete) error = %v, want os.ErrNotExist", err)
		}
		if _, err := os.Stat(sourceDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(deleted source) error = %v, want os.ErrNotExist", err)
		}

		rebuiltCatalog := newResourceCatalog(cloneAgentDef)
		rebuiltSkills := skillspkg.NewRegistry(
			agentSkillIntegrationSkillConfig(homePaths),
			skillspkg.WithLogger(discardLogger()),
		)
		rebuiltMCP := newResourceCatalog(cloneDaemonMCPServer)
		bootDriver := newAgentSkillIntegrationDriver(
			t,
			kernel,
			agentCodec,
			skillCodec,
			mcpCodec,
			rebuiltCatalog,
			rebuiltSkills,
			rebuiltMCP,
			nil,
		)
		if err := bootDriver.RunBoot(ctx); err != nil {
			t.Fatalf("bootDriver.RunBoot() error = %v", err)
		}
		if _, err := agentCatalogDependency(rebuiltCatalog).GetAgent(ctx, "coder"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("rebuilt catalog GetAgent(coder) error = %v, want os.ErrNotExist", err)
		}
	})
}

func performAgentMutationIntegrationRequest(
	t *testing.T,
	engine http.Handler,
	method string,
	path string,
	payload any,
) *httptest.ResponseRecorder {
	t.Helper()
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal(request) error = %v", err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, req)
	return response
}

func decodeAgentMutationIntegrationResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("json.Unmarshal(response) error = %v; body=%s", err, response.Body.String())
	}
}

type agentSkillIntegrationRuntime struct {
	extension *extensionpkg.Extension
}

func assertExtensionAgentSidecarOwnership(
	t *testing.T,
	ctx context.Context,
	agents []resources.Record[compozyconfig.AgentDef],
	soulStore resources.Store[soul.ResourceSpec],
	heartbeatStore resources.Store[heartbeat.ResourceSpec],
	extensionName string,
) {
	t.Helper()
	wantOwner := extensionOwner(extensionName).Normalize()
	agentID := ""
	for _, record := range agents {
		if record.Owner.Normalize() == wantOwner {
			agentID = record.ID
			if record.Scope.Normalize().Kind != resources.ResourceScopeKindGlobal {
				t.Fatalf("extension agent scope = %#v, want global", record.Scope)
			}
		}
	}
	if agentID == "" {
		t.Fatalf("extension-owned agent not found in %#v", agents)
	}
	souls, err := soulStore.List(ctx, agentSkillSyncActor(), resources.ResourceFilter{Owner: &wantOwner})
	if err != nil {
		t.Fatalf("soulStore.List(extension owner) error = %v", err)
	}
	heartbeats, err := heartbeatStore.List(ctx, agentSkillSyncActor(), resources.ResourceFilter{Owner: &wantOwner})
	if err != nil {
		t.Fatalf("heartbeatStore.List(extension owner) error = %v", err)
	}
	if len(souls) != 1 || souls[0].Spec.AgentResourceID != agentID ||
		souls[0].Spec.SourcePath != "agents/ext-agent/SOUL.md" {
		t.Fatalf("extension soul records = %#v, want sidecar for %q", souls, agentID)
	}
	if len(heartbeats) != 1 || heartbeats[0].Spec.AgentResourceID != agentID ||
		heartbeats[0].Spec.SourcePath != "agents/ext-agent/HEARTBEAT.md" {
		t.Fatalf("extension heartbeat records = %#v, want sidecar for %q", heartbeats, agentID)
	}
}

func countOwnedResourceRecords[T any](
	t *testing.T,
	ctx context.Context,
	store resources.Store[T],
	owner *resources.ResourceOwner,
) int {
	t.Helper()
	records, err := store.List(ctx, agentSkillSyncActor(), resources.ResourceFilter{Owner: owner})
	if err != nil {
		t.Fatalf("store.List(extension owner) error = %v", err)
	}
	return len(records)
}

func (r *agentSkillIntegrationRuntime) Start(context.Context) error  { return nil }
func (r *agentSkillIntegrationRuntime) Stop(context.Context) error   { return nil }
func (r *agentSkillIntegrationRuntime) Reload(context.Context) error { return nil }

func (r *agentSkillIntegrationRuntime) Get(name string) (*extensionpkg.Extension, error) {
	if r.extension == nil || r.extension.Info.Name != name {
		return nil, &extensionpkg.ExtensionNotFoundError{Name: name}
	}
	return r.extension, nil
}

func (r *agentSkillIntegrationRuntime) InspectPackageResources(
	ctx context.Context,
	name string,
) (*extensionpkg.Extension, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.Get(name)
}

func (r *agentSkillIntegrationRuntime) HookDeclarations(context.Context) ([]hookspkg.HookDecl, error) {
	return nil, nil
}

func newAgentSkillIntegrationDriver(
	t *testing.T,
	kernel resources.RawStore,
	agentCodec resources.KindCodec[compozyconfig.AgentDef],
	skillCodec resources.KindCodec[skillspkg.SkillResourceSpec],
	mcpCodec resources.KindCodec[compozyconfig.MCPServer],
	agentCatalog *resourceCatalog[compozyconfig.AgentDef],
	skillRegistry *skillspkg.Registry,
	mcpCatalog *resourceCatalog[compozyconfig.MCPServer],
	sidecars *agentSkillIntegrationSidecars,
) resources.ReconcileDriver {
	t.Helper()

	agentRegistration, err := resources.NewTypedProjectorRegistration(agentCodec, newAgentProjector(agentCatalog))
	if err != nil {
		t.Fatalf("resources.NewTypedProjectorRegistration(agent) error = %v", err)
	}
	skillRegistration, err := resources.NewTypedProjectorRegistration(skillCodec, newSkillProjector(skillRegistry))
	if err != nil {
		t.Fatalf("resources.NewTypedProjectorRegistration(skill) error = %v", err)
	}
	mcpRegistration, err := resources.NewTypedProjectorRegistration(mcpCodec, newMCPServerProjector(mcpCatalog))
	if err != nil {
		t.Fatalf("resources.NewTypedProjectorRegistration(mcp) error = %v", err)
	}
	registrations := []resources.ProjectorRegistration{agentRegistration, skillRegistration, mcpRegistration}
	if sidecars != nil {
		if sidecars.soulCodec != nil && sidecars.soulCatalog != nil {
			soulRegistration, registerErr := resources.NewTypedProjectorRegistration(
				sidecars.soulCodec,
				newSoulProjector(sidecars.soulCatalog),
			)
			if registerErr != nil {
				t.Fatalf("resources.NewTypedProjectorRegistration(soul) error = %v", registerErr)
			}
			registrations = append(registrations, soulRegistration)
		}
		if sidecars.heartbeatCodec != nil && sidecars.heartbeatCatalog != nil {
			heartbeatRegistration, registerErr := resources.NewTypedProjectorRegistration(
				sidecars.heartbeatCodec,
				newHeartbeatProjector(sidecars.heartbeatCatalog),
			)
			if registerErr != nil {
				t.Fatalf("resources.NewTypedProjectorRegistration(heartbeat) error = %v", registerErr)
			}
			registrations = append(registrations, heartbeatRegistration)
		}
	}
	driver, err := resources.NewReconcileDriver(
		kernel,
		resources.MutationActor{
			Kind: resources.MutationActorKindDaemon,
			ID:   "agent-skill-integration",
			Source: resources.ResourceSource{
				Kind: resources.ResourceSourceKind("daemon"),
				ID:   "agent-skill-integration",
			},
			MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		},
		registrations,
		resources.WithReconcileLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("resources.NewReconcileDriver() error = %v", err)
	}
	t.Cleanup(func() {
		if err := driver.Close(context.Background()); err != nil {
			t.Fatalf("driver.Close() error = %v", err)
		}
	})
	return driver
}

type agentSkillIntegrationSidecars struct {
	soulCodec        resources.KindCodec[soul.ResourceSpec]
	soulCatalog      *resourceCatalog[soul.ResourceSpec]
	heartbeatCodec   resources.KindCodec[heartbeat.ResourceSpec]
	heartbeatCatalog *resourceCatalog[heartbeat.ResourceSpec]
}

type agentSkillSourceSidecarStores struct {
	soulStore      resources.Store[soul.ResourceSpec]
	soulCodec      resources.KindCodec[soul.ResourceSpec]
	heartbeatStore resources.Store[heartbeat.ResourceSpec]
	heartbeatCodec resources.KindCodec[heartbeat.ResourceSpec]
}

func newAgentSkillSourceSidecarStores(
	t *testing.T,
	kernel resources.RawStore,
) agentSkillSourceSidecarStores {
	t.Helper()

	soulCodec, err := soul.NewResourceCodec()
	if err != nil {
		t.Fatalf("soul.NewResourceCodec() error = %v", err)
	}
	soulStore, err := resources.NewStore(kernel, soulCodec)
	if err != nil {
		t.Fatalf("resources.NewStore(soul) error = %v", err)
	}
	heartbeatCodec, err := heartbeat.NewResourceCodec()
	if err != nil {
		t.Fatalf("heartbeat.NewResourceCodec() error = %v", err)
	}
	heartbeatStore, err := resources.NewStore(kernel, heartbeatCodec)
	if err != nil {
		t.Fatalf("resources.NewStore(heartbeat) error = %v", err)
	}
	return agentSkillSourceSidecarStores{
		soulStore: soulStore, soulCodec: soulCodec,
		heartbeatStore: heartbeatStore, heartbeatCodec: heartbeatCodec,
	}
}

func agentSkillIntegrationHome(t *testing.T) compozyconfig.HomePaths {
	t.Helper()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("compozyconfig.ResolveHomePathsFrom() error = %v", err)
	}
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("compozyconfig.EnsureHomeLayout() error = %v", err)
	}
	return homePaths
}

func agentSkillIntegrationSkillConfig(homePaths compozyconfig.HomePaths) skillspkg.RegistryConfig {
	return skillspkg.RegistryConfig{
		UserSkillsDir: homePaths.SkillsDir,
		UserAgentsDir: homePaths.AgentsDir,
	}
}

func agentSkillIntegrationWorkspace(t *testing.T) string {
	t.Helper()

	root := filepath.Join(t.TempDir(), "workspace")
	agentDir := filepath.Join(root, compozyconfig.DirName, compozyconfig.AgentsDirName, "coder")
	writeAgentSkillIntegrationFile(t, filepath.Join(agentDir, "AGENT.md"), `---
name: coder
provider: claude
tools: ["compozy__lookup"]
---

Use the workspace tool catalog.
`)
	writeAgentSkillIntegrationFile(t, filepath.Join(agentDir, compozyconfig.MCPJSONName), `{
  "mcpServers": {
    "workspace-agent-mcp": {
      "command": "workspace-agent-command"
    }
  }
}`)

	skillDir := filepath.Join(
		root,
		compozyconfig.DirName,
		compozyconfig.SkillsDirName,
		"review",
		"workspace-review",
	)
	writeAgentSkillIntegrationFile(t, filepath.Join(skillDir, "SKILL.md"), `---
name: workspace-review
description: Workspace review skill
---

Review workspace changes.
`)
	writeAgentSkillIntegrationFile(t, filepath.Join(skillDir, compozyconfig.MCPJSONName), `{
  "mcpServers": {
    "workspace-skill-mcp": {
      "command": "workspace-skill-command"
    }
  }
}`)

	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(%q) error = %v", root, err)
	}
	return canonical
}

func agentSkillIntegrationExtension(
	t *testing.T,
	registry *extensionpkg.Registry,
) *extensionpkg.Extension {
	t.Helper()

	dir := t.TempDir()
	writeAgentSkillIntegrationFile(t, filepath.Join(dir, "extension.toml"), `[extension]
name = "agent-skill-ext"
version = "0.1.0"
min_compozy_version = "0.5.0"

[resources]
skills = ["skills/"]
agents = ["agents/"]
`)
	agentPath := filepath.Join(dir, "agents", "ext-agent", "AGENT.md")
	writeAgentSkillIntegrationFile(t, agentPath, `---
name: ext-agent
provider: claude
mcp_servers:
  - name: ext-agent-mcp
    command: ext-agent-command
---

Use extension-provided context.
`)
	writeAgentSkillIntegrationFile(
		t,
		filepath.Join(dir, "agents", "ext-agent", soul.FileName),
		"Write with extension context.\n",
	)
	writeAgentSkillIntegrationFile(
		t,
		filepath.Join(dir, "agents", "ext-agent", heartbeat.FileName),
		"Check extension work.\n",
	)
	skillPath := filepath.Join(dir, "skills", "ext-skill.md")
	writeAgentSkillIntegrationFile(t, skillPath, `---
name: ext-skill
description: Extension skill
---

Use extension skill context.
`)
	writeAgentSkillIntegrationFile(t, filepath.Join(dir, "skills", compozyconfig.MCPJSONName), `{
  "mcpServers": {
    "ext-skill-mcp": {
      "command": "ext-skill-command"
    }
  }
}`)

	manifest, err := extensionpkg.LoadManifest(dir)
	if err != nil {
		t.Fatalf("extensionpkg.LoadManifest() error = %v", err)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(dir)
	if err != nil {
		t.Fatalf("extensionpkg.ComputeDirectoryChecksum() error = %v", err)
	}
	if err := registry.Install(manifest, dir, checksum); err != nil {
		t.Fatalf("registry.Install() error = %v", err)
	}
	info, err := registry.Get(manifest.Name)
	if err != nil {
		t.Fatalf("registry.Get(%q) error = %v", manifest.Name, err)
	}
	staticAgents, err := extensionpkg.LoadAgentResources(dir, manifest.Resources.Agents)
	if err != nil {
		t.Fatalf("extensionpkg.LoadAgentResources(%q) error = %v", dir, err)
	}
	agents := make([]compozyconfig.AgentDef, 0, len(staticAgents))
	for _, staticAgent := range staticAgents {
		agents = append(agents, compozyconfig.CloneAgentDef(staticAgent.Agent))
	}
	skill, err := skillspkg.ParseSkillFileWithSource(skillPath, skillspkg.SourceUser)
	if err != nil {
		t.Fatalf("skillspkg.ParseSkillFileWithSource(%q) error = %v", skillPath, err)
	}
	return &extensionpkg.Extension{
		Info:         *info,
		Manifest:     manifest,
		RootDir:      dir,
		Agents:       agents,
		StaticAgents: staticAgents,
		Skills:       []*skillspkg.Skill{skill},
		Status: extensionpkg.ExtensionStatus{
			Name:       info.Name,
			Version:    info.Version,
			Source:     info.Source,
			Enabled:    info.Enabled,
			Registered: true,
		},
	}
}

func installPortablePublisherIntegrationExtension(
	t *testing.T,
	homePaths compozyconfig.HomePaths,
	registry *extensionpkg.Registry,
) string {
	t.Helper()

	const name = "portable-publisher"
	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.CopyFS(
		sourceDir,
		os.DirFS(filepath.Join("..", "extension", "testdata", "agent-plugin-30-skills")),
	); err != nil {
		t.Fatalf("CopyFS(agent-plugin-30-skills) error = %v", err)
	}
	manifest, err := extensionpkg.LoadManifest(sourceDir)
	if err != nil {
		t.Fatalf("extensionpkg.LoadManifest(portable source) error = %v", err)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(sourceDir)
	if err != nil {
		t.Fatalf("extensionpkg.ComputeDirectoryChecksum(portable source) error = %v", err)
	}
	if err := extensionpkg.InstallLocalManaged(homePaths, registry, manifest, sourceDir, checksum); err != nil {
		t.Fatalf("extensionpkg.InstallLocalManaged(portable source) error = %v", err)
	}
	if err := registry.Enable(name); err != nil {
		t.Fatalf("registry.Enable(%q) error = %v", name, err)
	}
	return name
}

func installFullyDegradedPublisherIntegrationExtension(
	t *testing.T,
	homePaths compozyconfig.HomePaths,
	registry *extensionpkg.Registry,
) string {
	t.Helper()

	const name = "portable-degraded"
	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.CopyFS(
		sourceDir,
		os.DirFS(filepath.Join("..", "extension", "testdata", "agent-plugin-fully-degraded")),
	); err != nil {
		t.Fatalf("CopyFS(agent-plugin-fully-degraded) error = %v", err)
	}
	manifest, err := extensionpkg.LoadManifest(sourceDir)
	if err != nil {
		t.Fatalf("extensionpkg.LoadManifest(degraded source) error = %v", err)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(sourceDir)
	if err != nil {
		t.Fatalf("extensionpkg.ComputeDirectoryChecksum(degraded source) error = %v", err)
	}
	if err := extensionpkg.InstallLocalManaged(homePaths, registry, manifest, sourceDir, checksum); err != nil {
		t.Fatalf("extensionpkg.InstallLocalManaged(degraded source) error = %v", err)
	}
	if err := registry.Enable(name); err != nil {
		t.Fatalf("registry.Enable(%q) error = %v", name, err)
	}
	return name
}

func agentSkillIntegrationSpecCycleExtension(
	t *testing.T,
	registry *extensionpkg.Registry,
) *extensionpkg.Extension {
	t.Helper()

	info, err := registry.Get(speccycle.Name)
	if err != nil {
		t.Fatalf("registry.Get(%q) error = %v", speccycle.Name, err)
	}
	rootDir := filepath.Dir(info.ManifestPath)
	manifest, err := extensionpkg.LoadManifest(rootDir)
	if err != nil {
		t.Fatalf("extensionpkg.LoadManifest(%q) error = %v", rootDir, err)
	}
	staticAgents, err := extensionpkg.LoadAgentResources(rootDir, manifest.Resources.Agents)
	if err != nil {
		t.Fatalf("extensionpkg.LoadAgentResources(%q) error = %v", rootDir, err)
	}
	agents := make([]compozyconfig.AgentDef, 0, len(staticAgents))
	for _, staticAgent := range staticAgents {
		agents = append(agents, compozyconfig.CloneAgentDef(staticAgent.Agent))
	}
	skills := make([]*skillspkg.Skill, 0, len(specCycleIntegrationSkillNames))
	for _, resourcePath := range manifest.Resources.Skills {
		resourceRoot := filepath.Join(rootDir, filepath.FromSlash(resourcePath))
		if err := filepath.WalkDir(resourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || entry.Name() != "SKILL.md" {
				return nil
			}
			skill, err := skillspkg.ParseSkillFileWithSource(path, skillspkg.SourceBundled)
			if err != nil {
				return err
			}
			skill.InstalledFromExtension = speccycle.Name
			skills = append(skills, skill)
			return nil
		}); err != nil {
			t.Fatalf("filepath.WalkDir(%q) error = %v", resourceRoot, err)
		}
	}
	slices.SortFunc(skills, func(left, right *skillspkg.Skill) int {
		return strings.Compare(left.Meta.Name, right.Meta.Name)
	})
	if len(skills) != len(specCycleIntegrationSkillNames) {
		t.Fatalf("loaded spec-cycle skills = %d, want %d", len(skills), len(specCycleIntegrationSkillNames))
	}
	return &extensionpkg.Extension{
		Info:         *info,
		Manifest:     manifest,
		RootDir:      rootDir,
		Agents:       agents,
		StaticAgents: staticAgents,
		Skills:       skills,
		Status: extensionpkg.ExtensionStatus{
			Name:       info.Name,
			Version:    info.Version,
			Source:     info.Source,
			Enabled:    info.Enabled,
			Registered: true,
		},
	}
}

func agentSkillIntegrationSkillWorkspace(t *testing.T, withOverride bool) string {
	t.Helper()

	root := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", root, err)
	}
	if withOverride {
		skillsRoot := filepath.Join(root, compozyconfig.DirName, compozyconfig.SkillsDirName)
		writeAgentSkillIntegrationFile(
			t,
			filepath.Join(skillsRoot, "overrides", "cy-execute-task", "SKILL.md"),
			`---
name: cy-execute-task
description: Workspace A execution override
---

Workspace A execution override.
`,
		)
		writeAgentSkillIntegrationFile(
			t,
			filepath.Join(skillsRoot, "workspace-only", "workspace-only-a", "SKILL.md"),
			`---
name: workspace-only-a
description: Available only inside workspace A
---

Workspace A only.
`,
		)
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(%q) error = %v", root, err)
	}
	return canonical
}

func writeAgentSkillIntegrationFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}

func agentHasMCP(agent compozyconfig.AgentDef, name string) bool {
	for _, server := range agent.MCPServers {
		if server.Name == name {
			return true
		}
	}
	return false
}

func skillHasMCP(skill *skillspkg.Skill, name string) bool {
	if skill == nil {
		return false
	}
	for _, server := range skill.MCPServers {
		if server.Name == name {
			return true
		}
	}
	return false
}

func findIntegrationSkill(skills []*skillspkg.Skill, name string) *skillspkg.Skill {
	for _, skill := range skills {
		if skill != nil && skill.Meta.Name == name {
			return skill
		}
	}
	return nil
}

func mcpCatalogHas(catalog *resourceCatalog[compozyconfig.MCPServer], name string) bool {
	if catalog == nil {
		return false
	}
	for _, record := range catalog.Snapshot() {
		if record.Spec.Name == name {
			return true
		}
	}
	return false
}
