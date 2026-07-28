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

	devcycle "github.com/compozy/compozy/extensions/dev-cycle"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/session"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

var devCycleIntegrationSkillNames = []string{
	"cy-create-prd",
	"cy-create-tasks",
	"cy-create-techspec",
	"cy-execute-task",
	"cy-final-verify",
	"cy-fix-reviews",
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
		driver := newAgentSkillIntegrationDriver(
			t,
			kernel,
			agentCodec,
			skillCodec,
			mcpCodec,
			initialAgentCatalog,
			initialSkillRegistry,
			initialMCPCatalog,
		)

		syncer := newAgentSkillSourceSyncer(
			kernel,
			agentStore,
			agentCodec,
			newAgentProjector(initialAgentCatalog),
			skillStore,
			skillCodec,
			newSkillProjector(initialSkillRegistry),
			mcpStore,
			mcpCodec,
			agentSkillSyncActor(),
			discardLogger(),
			func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
				return driver.Trigger(ctx, kind, reason)
			},
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
		)
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
		bootDriver := newAgentSkillIntegrationDriver(
			t,
			kernel,
			agentCodec,
			skillCodec,
			mcpCodec,
			rebuiltAgentCatalog,
			rebuiltSkillRegistry,
			rebuiltMCPCatalog,
		)
		if err := bootDriver.RunBoot(testutil.Context(t)); err != nil {
			t.Fatalf("bootDriver.RunBoot() error = %v", err)
		}

		resolved, err := workspaceResolver.Resolve(testutil.Context(t), workspace.ID)
		if err != nil {
			t.Fatalf("workspaceResolver.Resolve() error = %v", err)
		}
		agentCatalog := agentCatalogDependency(rebuiltAgentCatalog)
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
	})
}

func TestDevCycleBundledSkillPublicationAndBootRebuild(t *testing.T) {
	t.Parallel()

	t.Run("Should publish nine global skills while preserving workspace-local isolation", func(t *testing.T) {
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

		homePaths := agentSkillIntegrationHome(t)
		extensionRegistry := extensionpkg.NewRegistry(db.DB())
		if err := devcycle.EnsureManagedInstall(homePaths, extensionRegistry); err != nil {
			t.Fatalf("devcycle.EnsureManagedInstall() error = %v", err)
		}
		extensionSnapshot := agentSkillIntegrationDevCycleExtension(t, extensionRegistry)
		runtime := &agentSkillIntegrationRuntime{extension: extensionSnapshot}

		workspaceARoot := agentSkillIntegrationSkillWorkspace(t, true)
		workspaceBRoot := agentSkillIntegrationSkillWorkspace(t, false)
		now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
		for _, workspace := range []workspacepkg.Workspace{
			{ID: "ws_dev_cycle_a", RootDir: workspaceARoot, Name: "dev-cycle-a", CreatedAt: now, UpdatedAt: now},
			{ID: "ws_dev_cycle_b", RootDir: workspaceBRoot, Name: "dev-cycle-b", CreatedAt: now, UpdatedAt: now},
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
		)
		syncer := newAgentSkillSourceSyncer(
			kernel,
			agentStore,
			agentCodec,
			newAgentProjector(initialAgents),
			skillStore,
			skillCodec,
			newSkillProjector(initialSkills),
			mcpStore,
			mcpCodec,
			agentSkillSyncActor(),
			discardLogger(),
			func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
				return initialDriver.Trigger(ctx, kind, reason)
			},
			daemonAgentSkillDeclarationProvider(homePaths, db, workspaceResolver, initialSkills, discardLogger()),
			extensionAgentSkillDeclarationProvider(
				extensionRegistry,
				func() extensionRuntime { return runtime },
				discardLogger(),
			),
		)
		if err := syncer.Sync(ctx); err != nil {
			t.Fatalf("syncer.Sync() error = %v", err)
		}

		source := agentSkillSyncActor().Source
		records, err := skillStore.List(ctx, agentSkillSyncActor(), resources.ResourceFilter{Source: &source})
		if err != nil {
			t.Fatalf("skillStore.List() error = %v", err)
		}
		globalDevCycleCount := 0
		for _, record := range records {
			if record.Scope.Kind == resources.ResourceScopeKindGlobal &&
				record.Spec.InstalledFromExtension == devcycle.Name {
				globalDevCycleCount++
			}
		}
		if globalDevCycleCount != len(devCycleIntegrationSkillNames) {
			t.Fatalf(
				"global dev-cycle skill records = %d, want %d",
				globalDevCycleCount,
				len(devCycleIntegrationSkillNames),
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
		)
		if err := bootDriver.RunBoot(ctx); err != nil {
			t.Fatalf("bootDriver.RunBoot() error = %v", err)
		}

		resolvedA, err := workspaceResolver.Resolve(ctx, "ws_dev_cycle_a")
		if err != nil {
			t.Fatalf("workspaceResolver.Resolve(A) error = %v", err)
		}
		resolvedB, err := workspaceResolver.Resolve(ctx, "ws_dev_cycle_b")
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
		for _, name := range devCycleIntegrationSkillNames {
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
			t.Fatal("cy-capture-decisions is available, want excluded from dev-cycle bundle")
		}

		executeA := findIntegrationSkill(skillsA, "cy-execute-task")
		executeB := findIntegrationSkill(skillsB, "cy-execute-task")
		if executeA.Source != skillspkg.SourceWorkspace {
			t.Fatalf("workspace A cy-execute-task source = %v, want workspace", executeA.Source)
		}
		if executeB.Source != skillspkg.SourceBundled || executeB.InstalledFromExtension != devcycle.Name {
			t.Fatalf("workspace B cy-execute-task = %#v, want bundled dev-cycle source", executeB)
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
		if !strings.Contains(contentB, "# Execute PRD Task") {
			t.Fatalf("workspace B bundled content = %q, want cy-execute-task body", contentB)
		}

		augmenter := newSkillsCatalogAugmenter(rebuiltSkills, func() promptSkillsWorkspaceResolver {
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

				prompt, err := augmenter(ctx, promptCase.session, "Continue the task.")
				if err != nil {
					t.Fatalf("skills augmenter error = %v", err)
				}
				for _, name := range devCycleIntegrationSkillNames {
					if !strings.Contains(prompt, `name="`+name+`"`) {
						t.Fatalf("prompt missing dev-cycle skill %q: %s", name, prompt)
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
		)
		syncer := newAgentSkillSourceSyncer(
			kernel,
			agentStore,
			agentCodec,
			newAgentProjector(agentCatalog),
			skillStore,
			skillCodec,
			newSkillProjector(skillRegistry),
			mcpStore,
			mcpCodec,
			agentSkillSyncActor(),
			discardLogger(),
			func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
				return driver.Trigger(ctx, kind, reason)
			},
			daemonAgentSkillDeclarationProvider(homePaths, db, resolver, skillRegistry, discardLogger()),
		)
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

func (r *agentSkillIntegrationRuntime) Start(context.Context) error  { return nil }
func (r *agentSkillIntegrationRuntime) Stop(context.Context) error   { return nil }
func (r *agentSkillIntegrationRuntime) Reload(context.Context) error { return nil }

func (r *agentSkillIntegrationRuntime) Get(name string) (*extensionpkg.Extension, error) {
	if r.extension == nil || r.extension.Info.Name != name {
		return nil, &extensionpkg.ExtensionNotFoundError{Name: name}
	}
	return r.extension, nil
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
		[]resources.ProjectorRegistration{agentRegistration, skillRegistration, mcpRegistration},
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

	skillDir := filepath.Join(root, compozyconfig.DirName, compozyconfig.SkillsDirName, "workspace-review")
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
	agentPath := filepath.Join(dir, "agents", "ext-agent.md")
	writeAgentSkillIntegrationFile(t, agentPath, `---
name: ext-agent
provider: claude
mcp_servers:
  - name: ext-agent-mcp
    command: ext-agent-command
---

Use extension-provided context.
`)
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
	agent, err := compozyconfig.LoadAgentDefFile(agentPath)
	if err != nil {
		t.Fatalf("compozyconfig.LoadAgentDefFile(%q) error = %v", agentPath, err)
	}
	skill, err := skillspkg.ParseSkillFileWithSource(skillPath, skillspkg.SourceUser)
	if err != nil {
		t.Fatalf("skillspkg.ParseSkillFileWithSource(%q) error = %v", skillPath, err)
	}
	return &extensionpkg.Extension{
		Info:     *info,
		Manifest: manifest,
		RootDir:  dir,
		Agents:   []compozyconfig.AgentDef{agent},
		Skills:   []*skillspkg.Skill{skill},
		Status: extensionpkg.ExtensionStatus{
			Name:       info.Name,
			Version:    info.Version,
			Source:     info.Source,
			Enabled:    info.Enabled,
			Registered: true,
		},
	}
}

func agentSkillIntegrationDevCycleExtension(
	t *testing.T,
	registry *extensionpkg.Registry,
) *extensionpkg.Extension {
	t.Helper()

	info, err := registry.Get(devcycle.Name)
	if err != nil {
		t.Fatalf("registry.Get(%q) error = %v", devcycle.Name, err)
	}
	rootDir := filepath.Dir(info.ManifestPath)
	manifest, err := extensionpkg.LoadManifest(rootDir)
	if err != nil {
		t.Fatalf("extensionpkg.LoadManifest(%q) error = %v", rootDir, err)
	}
	agents, err := extensionpkg.LoadAgentResources(rootDir, manifest)
	if err != nil {
		t.Fatalf("extensionpkg.LoadAgentResources(%q) error = %v", rootDir, err)
	}
	skills := make([]*skillspkg.Skill, 0, len(devCycleIntegrationSkillNames))
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
			skill.InstalledFromExtension = devcycle.Name
			skills = append(skills, skill)
			return nil
		}); err != nil {
			t.Fatalf("filepath.WalkDir(%q) error = %v", resourceRoot, err)
		}
	}
	slices.SortFunc(skills, func(left, right *skillspkg.Skill) int {
		return strings.Compare(left.Meta.Name, right.Meta.Name)
	})
	if len(skills) != len(devCycleIntegrationSkillNames) {
		t.Fatalf("loaded dev-cycle skills = %d, want %d", len(skills), len(devCycleIntegrationSkillNames))
	}
	return &extensionpkg.Extension{
		Info:     *info,
		Manifest: manifest,
		RootDir:  rootDir,
		Agents:   agents,
		Skills:   skills,
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
			filepath.Join(skillsRoot, "cy-execute-task", "SKILL.md"),
			`---
name: cy-execute-task
description: Workspace A execution override
---

Workspace A execution override.
`,
		)
		writeAgentSkillIntegrationFile(
			t,
			filepath.Join(skillsRoot, "workspace-only-a", "SKILL.md"),
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
