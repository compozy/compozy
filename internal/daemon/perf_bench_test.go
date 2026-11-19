package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentinstance"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/session"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/situation"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/soul"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	toolspkg "github.com/compozy/compozy/internal/tools"
	builtintools "github.com/compozy/compozy/internal/tools/builtin"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func BenchmarkBoundSessionMCPSummaryIdleTick(b *testing.B) {
	fixture := newInstanceMCPProjectionFixture(b, 22, 0, 806, false)
	fixture.model.state.sessions = fixture.sessions.Manager
	fixture.warmProjections(b)
	b.ReportAllocs()
	for b.Loop() {
		if err := fixture.model.refresh(b.Context()); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionMCPSummaryIdleTick(b *testing.B) {
	const toolsPerServer = 403
	executor := &sessionMCPExecutorStub{descriptors: make([]toolspkg.MCPToolDescriptor, toolsPerServer)}
	for i := range executor.descriptors {
		executor.descriptors[i] = toolspkg.MCPToolDescriptor{
			RawName: fmt.Sprintf("tool_%03d", i), ReadOnly: true, InputSchema: json.RawMessage(`{"type":"object"}`),
		}
	}
	model, manager, _, _ := newSessionMCPReadFixtureWithExecutor(b, executor, 2*toolsPerServer)
	now := time.Now()
	inventory := newTestRuntimeMCPInventory(b)
	inventory.now = func() time.Time { return now }
	root := b.TempDir()
	benchmarkManager := &inventorySessionMCPManager{
		sessionMCPManagerStub: manager, inventory: inventory, root: root,
	}
	for i := range 22 {
		manager.boundIDs = append(manager.boundIDs, fmt.Sprintf("sess-%d", i))
	}
	model.state.sessions, model.state.runtimeMCPInventory = benchmarkManager, inventory
	for range 2 {
		if err := model.refresh(b.Context()); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		now = now.Add(model.summaryIdleTTL())
		inventory.Poll(b.Context())
		if err := model.refresh(b.Context()); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResourceCatalogSnapshotAgentRecords(b *testing.B) {
	b.ReportAllocs()

	catalog := newResourceCatalog(cloneAgentDef)
	records := daemonBenchmarkAgentRecords(256, "ws-bench")
	catalog.Replace(int64(len(records)), records)

	for b.Loop() {
		snapshot := catalog.Snapshot()
		if len(snapshot) != len(records) {
			b.Fatalf("len(snapshot) = %d, want %d", len(snapshot), len(records))
		}
	}
}

func BenchmarkResourceAgentCatalogResolveAgentWorkspaceHit(b *testing.B) {
	b.ReportAllocs()

	workspaceID := "ws-bench"
	catalog := newResourceCatalog(cloneAgentDef)
	catalog.Replace(1, daemonBenchmarkAgentRecords(256, workspaceID))
	dependency := agentCatalogDependency(catalog)
	resolved := &workspacepkg.ResolvedWorkspace{
		ID: workspaceID,
	}

	for b.Loop() {
		agent, err := dependency.ResolveAgent("agent-255", resolved)
		if err != nil {
			b.Fatalf("ResolveAgent() error = %v", err)
		}
		if got, want := agent.Prompt, "workspace prompt 255"; got != want {
			b.Fatalf("ResolveAgent().Prompt = %q, want %q", got, want)
		}
	}
}

func BenchmarkAgentSkillSourceSyncerSyncNoop(b *testing.B) {
	b.ReportAllocs()

	ctx := b.Context()
	rawStore, agentStore, agentCodec, soulStore, soulCodec, heartbeatStore, heartbeatCodec,
		skillStore, skillCodec, mcpStore, mcpCodec := daemonBenchmarkAgentSkillStores(b)
	desired := daemonBenchmarkAgentSkillDeclarations(24)
	syncer := newAgentSkillSourceSyncer(agentSkillSourceSyncerDeps{
		raw: rawStore, agentStore: agentStore, agentCodec: agentCodec,
		soulStore: soulStore, soulCodec: soulCodec,
		heartbeatStore: heartbeatStore, heartbeatCodec: heartbeatCodec,
		skillStore: skillStore, skillCodec: skillCodec,
		mcpStore: mcpStore, mcpCodec: mcpCodec,
		actor: agentSkillSyncActor(), logger: discardLogger(),
		providers: []agentSkillDeclarationProvider{func(context.Context) (agentSkillDeclarations, error) {
			return desired, nil
		}},
	})
	if err := syncer.Sync(ctx); err != nil {
		b.Fatalf("initial Sync() error = %v", err)
	}

	for b.Loop() {
		if err := syncer.Sync(ctx); err != nil {
			b.Fatalf("Sync() error = %v", err)
		}
	}
}

func BenchmarkToolMCPSourceSyncerSyncNoop(b *testing.B) {
	b.ReportAllocs()

	ctx := b.Context()
	raw, toolStore, toolCodec, mcpStore, mcpCodec := daemonBenchmarkToolMCPStores(b)
	desired := daemonBenchmarkToolMCPDesiredResources(32)
	syncer := newToolMCPSourceSyncer(
		raw,
		toolStore,
		toolCodec,
		mcpStore,
		mcpCodec,
		toolMCPSyncActor(),
		discardLogger(),
		nil,
		func(context.Context) (toolMCPDesiredResources, error) {
			return desired, nil
		},
	)
	if err := syncer.Sync(ctx); err != nil {
		b.Fatalf("initial Sync() error = %v", err)
	}

	for b.Loop() {
		if err := syncer.Sync(ctx); err != nil {
			b.Fatalf("Sync() error = %v", err)
		}
	}
}

func daemonBenchmarkAgentRecords(count int, workspaceID string) []resources.Record[compozyconfig.AgentDef] {
	records := make([]resources.Record[compozyconfig.AgentDef], 0, count*2)
	for i := range count {
		name := fmt.Sprintf("agent-%03d", i)
		global := compozyconfig.AgentDef{
			Name:        name,
			Prompt:      fmt.Sprintf("global prompt %d", i),
			Tools:       []string{toolspkg.ToolIDToolInfo.String(), toolspkg.ToolIDToolList.String()},
			Permissions: string(compozyconfig.PermissionModeApproveAll),
			MCPServers: []compozyconfig.MCPServer{{
				Name:    fmt.Sprintf("global-mcp-%03d", i),
				Command: "global-command",
				Args:    []string{"--mode", "global"},
			}},
		}
		records = append(records, resources.Record[compozyconfig.AgentDef]{
			ID:      fmt.Sprintf("global:%s", name),
			Version: int64(i + 1),
			Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			Source: resources.ResourceSource{
				Kind: resources.ResourceSourceKind("bench"),
				ID:   fmt.Sprintf("global-%03d", i),
			},
			Spec: global,
		})

		workspace := global
		workspace.Prompt = fmt.Sprintf("workspace prompt %d", i)
		workspace.MCPServers = []compozyconfig.MCPServer{{
			Name:    fmt.Sprintf("workspace-mcp-%03d", i),
			Command: "workspace-command",
			Args:    []string{"--mode", "workspace"},
		}}
		records = append(records, resources.Record[compozyconfig.AgentDef]{
			ID:      fmt.Sprintf("workspace:%s", name),
			Version: int64(i + 1),
			Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: workspaceID},
			Source: resources.ResourceSource{
				Kind: resources.ResourceSourceKind("bench"),
				ID:   fmt.Sprintf("workspace-%03d", i),
			},
			Spec: workspace,
		})
	}
	return records
}

func daemonBenchmarkAgentSkillDeclarations(count int) agentSkillDeclarations {
	desired := agentSkillDeclarations{
		agents:     make([]agentPublicationInput, 0, count),
		skills:     make([]skillPublicationInput, 0, count),
		mcpServers: make([]mcpServerPublicationInput, 0, count),
	}
	scope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
	for i := range count {
		desired.agents = append(desired.agents, agentPublicationInput{
			sourceKey: fmt.Sprintf("bench/agent/%03d", i),
			scope:     scope,
			spec: compozyconfig.AgentDef{
				Name:        fmt.Sprintf("agent-%03d", i),
				Prompt:      fmt.Sprintf("Agent prompt %d", i),
				Tools:       []string{toolspkg.ToolIDToolInfo.String()},
				Permissions: string(compozyconfig.PermissionModeApproveAll),
			},
		})
		desired.skills = append(desired.skills, skillPublicationInput{
			sourceKey: fmt.Sprintf("bench/skill/%03d", i),
			scope:     scope,
			spec: skillspkg.SkillResourceSpec{
				Name:        fmt.Sprintf("skill-%03d", i),
				Description: fmt.Sprintf("Skill %d", i),
				Source:      skillspkg.SkillSourceName(skillspkg.SourceUser),
				Enabled:     true,
			},
		})
		desired.mcpServers = append(desired.mcpServers, mcpServerPublicationInput{
			sourceKey: fmt.Sprintf("bench/mcp/%03d", i),
			scope:     scope,
			spec: compozyconfig.MCPServer{
				Name:    fmt.Sprintf("mcp-%03d", i),
				Command: "bench-mcp",
				Args:    []string{"--id", fmt.Sprintf("%03d", i)},
			},
		})
	}
	return desired
}

func daemonBenchmarkToolMCPDesiredResources(count int) toolMCPDesiredResources {
	desired := toolMCPDesiredResources{
		tools:      make([]toolPublicationInput, 0, count),
		mcpServers: make([]mcpServerPublicationInput, 0, count),
	}
	scope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
	for i := range count {
		desired.tools = append(desired.tools, toolPublicationInput{
			sourceKey: fmt.Sprintf("bench/tool/%03d", i),
			scope:     scope,
			spec: toolspkg.Tool{
				ID: toolspkg.ToolID(fmt.Sprintf("compozy__bench_tool_%03d", i)),
				ToolPresentation: toolspkg.NewToolPresentation(
					fmt.Sprintf("tool-%03d", i), "", "",
				),
				Description: fmt.Sprintf("Tool %d", i),
				Backend: toolspkg.BackendRef{
					Kind:       toolspkg.BackendNativeGo,
					NativeName: fmt.Sprintf("bench_tool_%03d", i),
				},
				InputSchema: json.RawMessage(`{"type":"object"}`),
				Source: toolspkg.SourceRef{
					Kind:  toolspkg.SourceBuiltin,
					Owner: "daemon",
				},
				Visibility: toolspkg.VisibilityOperator,
				Risk:       toolspkg.RiskRead,
				ReadOnly:   true,
			},
		})
		desired.mcpServers = append(desired.mcpServers, mcpServerPublicationInput{
			sourceKey: fmt.Sprintf("bench/mcp/%03d", i),
			scope:     scope,
			spec: compozyconfig.MCPServer{
				Name:    fmt.Sprintf("tool-mcp-%03d", i),
				Command: "bench-tool-mcp",
				Args:    []string{"--id", fmt.Sprintf("%03d", i)},
			},
		})
	}
	return desired
}

func daemonBenchmarkAgentSkillStores(
	b *testing.B,
) (
	resources.RawStore,
	resources.Store[compozyconfig.AgentDef],
	resources.KindCodec[compozyconfig.AgentDef],
	resources.Store[soul.ResourceSpec],
	resources.KindCodec[soul.ResourceSpec],
	resources.Store[heartbeat.ResourceSpec],
	resources.KindCodec[heartbeat.ResourceSpec],
	resources.Store[skillspkg.SkillResourceSpec],
	resources.KindCodec[skillspkg.SkillResourceSpec],
	resources.Store[compozyconfig.MCPServer],
	resources.KindCodec[compozyconfig.MCPServer],
) {
	b.Helper()

	kernel := daemonBenchmarkKernel(b)
	agentCodec, err := compozyconfig.NewAgentResourceCodec()
	if err != nil {
		b.Fatalf("compozyconfig.NewAgentResourceCodec() error = %v", err)
	}
	agentStore, err := resources.NewStore(kernel, agentCodec)
	if err != nil {
		b.Fatalf("resources.NewStore(agent) error = %v", err)
	}
	soulCodec, err := soul.NewResourceCodec()
	if err != nil {
		b.Fatalf("soul.NewResourceCodec() error = %v", err)
	}
	soulStore, err := resources.NewStore(kernel, soulCodec)
	if err != nil {
		b.Fatalf("resources.NewStore(soul) error = %v", err)
	}
	heartbeatCodec, err := heartbeat.NewResourceCodec()
	if err != nil {
		b.Fatalf("heartbeat.NewResourceCodec() error = %v", err)
	}
	heartbeatStore, err := resources.NewStore(kernel, heartbeatCodec)
	if err != nil {
		b.Fatalf("resources.NewStore(heartbeat) error = %v", err)
	}
	skillCodec, err := skillspkg.NewResourceCodec()
	if err != nil {
		b.Fatalf("skillspkg.NewResourceCodec() error = %v", err)
	}
	skillStore, err := resources.NewStore(kernel, skillCodec)
	if err != nil {
		b.Fatalf("resources.NewStore(skill) error = %v", err)
	}
	mcpCodec, err := compozyconfig.NewMCPServerResourceCodec()
	if err != nil {
		b.Fatalf("compozyconfig.NewMCPServerResourceCodec() error = %v", err)
	}
	mcpStore, err := resources.NewStore(kernel, mcpCodec)
	if err != nil {
		b.Fatalf("resources.NewStore(mcp) error = %v", err)
	}
	return kernel, agentStore, agentCodec, soulStore, soulCodec, heartbeatStore, heartbeatCodec,
		skillStore, skillCodec, mcpStore, mcpCodec
}

func daemonBenchmarkToolMCPStores(
	b *testing.B,
) (
	resources.RawStore,
	resources.Store[toolspkg.Tool],
	resources.KindCodec[toolspkg.Tool],
	resources.Store[compozyconfig.MCPServer],
	resources.KindCodec[compozyconfig.MCPServer],
) {
	b.Helper()

	kernel := daemonBenchmarkKernel(b)
	toolCodec, err := toolspkg.NewResourceCodec()
	if err != nil {
		b.Fatalf("toolspkg.NewResourceCodec() error = %v", err)
	}
	toolStore, err := resources.NewStore(kernel, toolCodec)
	if err != nil {
		b.Fatalf("resources.NewStore(tool) error = %v", err)
	}
	mcpCodec, err := compozyconfig.NewMCPServerResourceCodec()
	if err != nil {
		b.Fatalf("compozyconfig.NewMCPServerResourceCodec() error = %v", err)
	}
	mcpStore, err := resources.NewStore(kernel, mcpCodec)
	if err != nil {
		b.Fatalf("resources.NewStore(mcp) error = %v", err)
	}
	return kernel, toolStore, toolCodec, mcpStore, mcpCodec
}

func daemonBenchmarkKernel(b *testing.B) *resources.Kernel {
	b.Helper()

	ctx := b.Context()
	db, err := openDaemonTestGlobalDBAtPath(ctx, filepath.Join(b.TempDir(), store.GlobalDatabaseName))
	if err != nil {
		b.Fatalf("OpenGlobalDB() error = %v", err)
	}
	b.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		if err := db.Close(closeCtx); err != nil {
			b.Fatalf("GlobalDB.Close() error = %v", err)
		}
	})
	kernel, err := resources.NewKernel(db.DB())
	if err != nil {
		b.Fatalf("resources.NewKernel() error = %v", err)
	}
	return kernel
}

func BenchmarkSchedulerSessionCapabilities(b *testing.B) {
	db, err := openDaemonTestGlobalDBAtPath(b.Context(), filepath.Join(b.TempDir(), store.GlobalDatabaseName))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			b.Error(err)
		}
	})
	infos := make([]*session.Info, 128)
	for i := range infos {
		infos[i] = &session.Info{
			ID:        fmt.Sprintf("session-%d", i),
			ProfileID: store.DefaultProfileID,
			State:     session.StateActive,
		}
	}
	source := schedulerSessionSource{
		sessions:  &fakeSessionManager{infos: infos},
		situation: situation.NewService(situation.Deps{TaskStore: db}),
	}
	b.ReportAllocs()
	for b.Loop() {
		snapshots, err := source.Sessions(b.Context())
		if err != nil || len(snapshots) != len(infos) {
			b.Fatalf("Sessions() = %d, %v", len(snapshots), err)
		}
	}
}

func BenchmarkSchedulerTaskReadProjection(b *testing.B) {
	db, err := openDaemonTestGlobalDBAtPath(b.Context(), filepath.Join(b.TempDir(), store.GlobalDatabaseName))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			b.Error(err)
		}
	})
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	runs := make([]taskpkg.Run, 0, 256)
	for taskIndex := range 64 {
		record := daemonTaskRecordForTest(fmt.Sprintf("task-%d", taskIndex), now)
		if err := db.CreateTask(b.Context(), record); err != nil {
			b.Fatal(err)
		}
		for runIndex := range 4 {
			runs = append(runs, taskpkg.Run{ID: fmt.Sprintf("run-%d-%d", taskIndex, runIndex), TaskID: record.ID})
		}
	}
	source := schedulerTaskSource{store: db}
	b.ReportAllocs()
	for b.Loop() {
		snapshots, err := source.joinRunsWithTasks(b.Context(), runs)
		if err != nil {
			b.Fatal(err)
		}
		snapshots, err = source.filterPausedRuns(b.Context(), snapshots)
		if err != nil || len(snapshots) != len(runs) {
			b.Fatalf("scheduler projection = %d, %v", len(snapshots), err)
		}
	}
}

// The daemon owns this fixture: it drives the real session, repository, policy and
// hosted read paths, with a fake ACP process and a deterministic upstream catalog.
type instanceMCPProjectionFixture struct {
	model                            *sessionMCPReadModel
	config                           *compozyconfig.Config
	policy                           *projectionFixturePolicy
	hosted                           *mcppkg.HostedService
	db                               *globaldb.GlobalDB
	sessions                         *projectionFixtureSessions
	now                              time.Time
	ids, instanceIDs, binds, digests []string
	agentRoot                        string
	provider                         *toolspkg.MCPProvider
	executor                         *sessionMCPExecutorStub
	settings                         *nativeMCPSettingsService
	auth                             *projectionFixtureAuth
	peer                             mcppkg.PeerInfo
	policyReads                      map[string]int
}

func newInstanceMCPProjectionFixture(
	t testing.TB, count, instances, toolCount int, adoption bool,
) *instanceMCPProjectionFixture {
	t.Helper()
	ctx := t.Context()
	home, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := compozyconfig.EnsureHomeLayout(home); err != nil {
		t.Fatal(err)
	}
	cfg := compozyconfig.DefaultWithHome(home)
	cfg.Providers["acpmock"] = acpmock.ProviderConfig("mock")
	cfg.MCPServers = []compozyconfig.MCPServer{{Name: "fixture", Command: "mock-mcp"}}
	cfg.Permissions.Mode = compozyconfig.PermissionModeApproveAll
	if adoption {
		cfg.Permissions.Mode = compozyconfig.PermissionModeApproveEdits
	}
	cfg.Tools.Policy.ExternalDefault = compozyconfig.ToolsExternalDefaultEnabled
	db, err := openDaemonTestGlobalDBAtPath(ctx, filepath.Join(home.HomeDir, store.GlobalDatabaseName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	workspace := workspacepkg.Workspace{ID: "ws-instance-perf", Name: "instance-perf", RootDir: t.TempDir()}
	if err := db.InsertWorkspace(ctx, workspace); err != nil {
		t.Fatal(err)
	}
	resolver, err := workspacepkg.NewResolver(
		db,
		workspacepkg.WithHomePaths(home),
		workspacepkg.WithProfileConfigLoader(
			func(string, string) (compozyconfig.Config, error) { return cfg, nil },
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	agents := projectionFixtureAgents{root: home.AgentsDir}
	manager, err := session.NewManager(
		session.WithHomePaths(home),
		session.WithDriver(new(projectionFixtureDriver)),
		session.WithLogger(discardLogger()),
		session.WithSessionCatalog(db),
		session.WithWorkspaceResolver(resolver),
		session.WithAgentResolver(agents),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	fixture := &instanceMCPProjectionFixture{
		db: db, config: &cfg, sessions: &projectionFixtureSessions{Manager: manager, reads: make(map[string]int)},
		now: time.Now(), policyReads: make(map[string]int), agentRoot: agents.root,
	}
	state := &bootState{cfg: cfg, registry: db, sessions: manager, workspaceResolver: resolver,
		toolProjectionEpoch: mcppkg.NewProjectionEpoch(), mcpAuthGeneration: mcpauth.NewMutationGeneration()}
	if _, _, err := (&Daemon{}).bootToolApprovalServices(state); err != nil {
		t.Fatal(err)
	}
	policy, err := newNativeToolPolicyResolverForBoot(state)
	if err != nil {
		t.Fatal(err)
	}
	// Substitute fixture configuration and the agent/catalog sources. The approval
	// service, instance repository and policy observer retain their boot wiring.
	policy.cfg, policy.agentResolver, policy.approvalAvailable = &cfg, agents, adoption
	policy.extensionRegistry, policy.mcpSourceGrants, policy.mcpResourceNames = nil, nil, nil
	policy.profileNames = nil
	state.sessions = fixture.sessions

	manager.SetMCPAuthorityResolver(policy)
	for i := range count {
		name := fmt.Sprintf("fixture-%d", i)
		instanceID := ""
		sessionType := session.SessionTypeUser
		parent := ""
		if instances > 1 && i == instances-1 {
			name, instanceID = "fixture-0", fixture.instanceIDs[0]
			sessionType, parent = session.SessionTypeTriage, fixture.ids[0]
		} else if i < instances {
			instance, created, err := db.Instances().AdmitCreate(ctx, agentinstance.Instance{
				ProfileID: store.DefaultProfileID, WorkspaceID: workspace.ID, AgentName: name,
				PermissionCeiling: "{}", CreatedByKind: "human",
			}, count+1)
			if err != nil {
				t.Fatal(err)
			}
			if !created {
				t.Fatal("fixture instance already exists")
			}
			instanceID = instance.ID
		}
		if i == 1 && adoption {
			sessionType, parent = session.SessionTypeSystem, fixture.ids[0]
		}
		if err := os.MkdirAll(filepath.Join(agents.root, name), 0o700); err != nil {
			t.Fatal(err)
		}
		started, err := manager.Create(ctx, &session.CreateOpts{
			AgentName: name, Workspace: workspace.ID, InstanceID: instanceID,
			Type: sessionType, ProvenanceParentSessionID: parent,
		})
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case sessionType == session.SessionTypeTriage:
			admitted, err := db.Instances().Admit(ctx, heartbeat.AdmitRequest{
				InstanceID: instanceID, OccurrenceID: "fixture-occurrence", ManualCheck: true,
				Now: fixture.now, NextWakeAt: fixture.now.Add(time.Hour), MaxConcurrent: 1,
			})
			if err != nil || !admitted {
				t.Fatalf("admit check-in = %v, %v", admitted, err)
			}
			bound, err := db.Instances().BindTriage(ctx, instanceID, "fixture-occurrence", started.ID)
			if err != nil || !bound {
				t.Fatalf("bind triage = %v, %v", bound, err)
			}
			// Match the durable server ceiling a triage launch receives from its instance.
			if _, err := db.DB().ExecContext(ctx, "UPDATE sessions SET permission_policy_json = ? WHERE id = ?",
				`{"mcp_servers":["fixture"],"mcp_servers_mode":"list"}`, started.ID); err != nil {
				t.Fatal(err)
			}
		case instanceID != "":
			if _, err := db.Instances().BindMain(ctx, instanceID, started.ID); err != nil {
				t.Fatal(err)
			}
		}
		fixture.ids = append(fixture.ids, started.ID)
		fixture.instanceIDs = append(fixture.instanceIDs, instanceID)
	}

	source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "fixture", RawServerName: "fixture", Scope: "user"}
	executor := &sessionMCPExecutorStub{descriptors: make([]toolspkg.MCPToolDescriptor, toolCount)}
	for i := range executor.descriptors {
		executor.descriptors[i] = toolspkg.MCPToolDescriptor{
			RawName: fmt.Sprintf(
				"tool_%03d",
				i,
			),
			ReadOnly:    instances == 0 || i != 0,
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}
	}
	fixture.auth = &projectionFixtureAuth{status: "unconfigured"}
	provider, err := toolspkg.NewMCPProvider(toolspkg.MCPSourceListerFunc(
		func(context.Context) ([]toolspkg.SourceRef, error) { return []toolspkg.SourceRef{source}, nil },
	), executor, fixture.auth, toolspkg.WithMCPBackgroundDiscovery(func() { state.toolProjectionEpoch.Advance() }, discardLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.List(
		ctx,
		toolspkg.Scope{ProfileID: store.DefaultProfileID, WorkspaceID: workspace.ID},
	); err != nil {
		t.Fatal(err)
	}
	providers := []toolspkg.Provider{provider}
	if adoption {
		native := make([]toolspkg.NativeTool, 0, 2)
		for _, descriptor := range builtintools.NativeDescriptors() {
			switch descriptor.ID {
			case toolspkg.ToolIDSessionApprove, toolspkg.ToolIDSessionClarifyAnswer:
				native = append(native, toolspkg.NativeTool{Descriptor: descriptor,
					Call: func(context.Context, toolspkg.Scope, toolspkg.CallRequest) (toolspkg.ToolResult, error) {
						return toolspkg.ToolResult{}, errors.New("unexpected native call")
					},
				})
			}
		}
		coordination, err := toolspkg.NewNativeProvider(builtintools.Source(), native...)
		if err != nil {
			t.Fatal(err)
		}
		providers = append(providers, coordination)
		toolCount += len(native)
	}
	fixture.policy = &projectionFixturePolicy{nativeToolPolicyResolver: policy, reads: fixture.policyReads}
	registry, err := toolspkg.NewRegistry(toolspkg.WithProviders(providers...), toolspkg.WithPolicyInputResolver(
		fixture.policy, toolspkg.ToolsetCatalog{},
	))
	if err != nil {
		t.Fatal(err)
	}
	state.mcpToolProvider = provider
	fixture.provider, fixture.executor = provider, executor
	fixture.settings = &nativeMCPSettingsService{
		servers: []settingspkg.MCPServerItem{{Name: "fixture", Owner: "manual"}},
	}
	state.deps.Settings = fixture.settings
	bindSessionMCPAuthority(state, policy, registry)
	fixture.model = state.sessionMCPReadModel

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	fixture.hosted, err = mcppkg.NewHostedService(&mcppkg.HostedConfig{
		Enabled: true, ExpectedBinary: executable, Registry: func() toolspkg.Registry { return registry },
		ProjectionEpoch: state.toolProjectionEpoch.Current, Now: func() time.Time { return fixture.now },
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.peer = mcppkg.PeerInfo{PID: os.Getpid(), UID: os.Getuid(), ExecutablePath: executable, Supported: true}
	for i, id := range fixture.ids {
		info, err := manager.StoredStatus(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		launch, err := fixture.hosted.Launch(ctx, mcppkg.HostedLaunchRequest{
			SessionID: id, WorkspaceID: workspace.ID, ProfileID: store.DefaultProfileID, AgentName: info.AgentName,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := fixture.hosted.ArmLaunch(ctx, id); err != nil {
			t.Fatal(err)
		}
		bind, err := fixture.hosted.Bind(
			ctx,
			mcppkg.HostedBindRequest{SessionID: id, Nonce: launch.Args[len(launch.Args)-1]},
			fixture.peer,
		)
		if err != nil {
			t.Fatal(err)
		}
		first, err := fixture.hosted.StreamProjection(ctx, bind.BindID, fixture.peer, "")
		if err != nil || first.Digest == "" || len(first.Tools) > toolCount ||
			(info.Type != session.SessionTypeTriage && len(first.Tools) != toolCount) {
			t.Fatalf("initial bind %d: %d tools, %v", i, len(first.Tools), err)
		}
		fixture.binds = append(fixture.binds, bind.BindID)
		fixture.digests = append(fixture.digests, first.Digest)
	}
	return fixture
}

// warmProjections fills both caches after asynchronous startup publication.
func (f *instanceMCPProjectionFixture) warmProjections(t testing.TB) {
	t.Helper()
	for range 3 {
		if err := f.model.refresh(t.Context()); err != nil {
			t.Fatal(err)
		}
		f.now = f.now.Add(f.hosted.StreamRefreshInterval())
		for i, bind := range f.binds {
			result, err := f.hosted.StreamProjection(t.Context(), bind, f.peer, f.digests[i])
			if err != nil {
				t.Fatal(err)
			}
			f.digests[i] = result.Digest
		}
	}
}

type projectionFixtureAuth struct{ status string }

func (f *projectionFixtureAuth) Status(context.Context, toolspkg.SourceRef, string) (toolspkg.MCPAuthStatus, error) {
	return toolspkg.MCPAuthStatus{Status: f.status}, nil
}

type projectionFixturePolicy struct {
	*nativeToolPolicyResolver
	reads         map[string]int
	readsMu       sync.Mutex
	aroundResolve func(toolspkg.Scope) func()
}

func (p *projectionFixturePolicy) Resolve(ctx context.Context, scope toolspkg.Scope) (toolspkg.PolicyInputs, error) {
	p.readsMu.Lock()
	p.reads[scope.SessionID]++
	p.readsMu.Unlock()
	if p.aroundResolve != nil {
		after := p.aroundResolve(scope)
		if after != nil {
			defer after()
		}
	}
	return p.nativeToolPolicyResolver.Resolve(ctx, scope)
}

type projectionFixtureSessions struct {
	*session.Manager
	reads map[string]int
}

func (s *projectionFixtureSessions) Status(ctx context.Context, id string) (*session.Info, error) {
	return s.StoredStatus(ctx, id)
}

func (s *projectionFixtureSessions) MCPReadSnapshot(
	ctx context.Context, id string, catalog toolspkg.AdvertisedCatalog,
) (session.MCPRuntimeSnapshot, error) {
	s.reads[id]++
	return s.Manager.MCPReadSnapshot(ctx, id, catalog)
}

type projectionFixtureAgents struct{ root string }

func (a projectionFixtureAgents) ResolveAgent(
	name string,
	_ *workspacepkg.ResolvedWorkspace,
) (compozyconfig.AgentDef, error) {
	path := filepath.Join(a.root, name, "AGENT.md")
	agent, err := compozyconfig.LoadAgentDefFile(path)
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		return agent, err
	}
	return compozyconfig.AgentDef{
		Name: name, Provider: "acpmock", Model: "fixture", Prompt: "Fixture agent.", SourcePath: path,
	}, nil
}

func (a projectionFixtureAgents) ResolvePolicyAgent(
	_ context.Context,
	name string,
	_ *workspacepkg.ResolvedAgentConfig,
) (compozyconfig.AgentDef, error) {
	return a.ResolveAgent(name, nil)
}

type projectionFixtureDriver struct {
	mu      sync.Mutex
	running map[*session.AgentProcess]chan struct{}
}

func (d *projectionFixtureDriver) Start(_ context.Context, opts acp.StartOpts) (*session.AgentProcess, error) {
	done := make(chan struct{})
	proc := session.NewAgentProcess(session.AgentProcessOptions{
		AgentName: opts.AgentName, SessionID: opts.CompozySessionID, Done: done,
	})
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running == nil {
		d.running = make(map[*session.AgentProcess]chan struct{})
	}
	d.running[proc] = done
	return proc, nil
}

func (*projectionFixtureDriver) Prompt(
	context.Context,
	*session.AgentProcess,
	acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	return nil, errors.New("unexpected prompt")
}
func (*projectionFixtureDriver) Cancel(context.Context, *session.AgentProcess) error { return nil }
func (d *projectionFixtureDriver) Stop(_ context.Context, proc *session.AgentProcess) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if done := d.running[proc]; done != nil {
		delete(d.running, proc)
		close(done)
	}
	return nil
}
func (d *projectionFixtureDriver) VerifyExit(proc *session.AgentProcess) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.running[proc] == nil, nil
}

func BenchmarkInstanceMCPIdleTick(b *testing.B) {
	for _, path := range []string{"Summary", "Stream"} {
		b.Run(path, func(b *testing.B) {
			fixture := newInstanceMCPProjectionFixture(b, 22, 5, 806, false)
			fixture.model.state.sessions = fixture.sessions.Manager
			fixture.warmProjections(b)
			b.ReportAllocs()
			for b.Loop() {
				fixture.now = fixture.now.Add(fixture.hosted.StreamRefreshInterval())
				if path == "Summary" {
					if err := fixture.model.refresh(b.Context()); err != nil {
						b.Fatal(err)
					}
					continue
				}
				for i, bind := range fixture.binds {
					response, err := fixture.hosted.StreamProjection(
						b.Context(),
						bind,
						fixture.peer,
						fixture.digests[i],
					)
					if err != nil || response.Digest != fixture.digests[i] || len(response.Tools) != 0 {
						b.Fatalf("idle bind %d: %d tools, %v", i, len(response.Tools), err)
					}
				}
			}
		})
	}
}

// Invariant: a concurrent epoch replacement converges even while unrelated
// commits land. Owner: daemon-composed hosted projections; canonical perf suite.
func TestHostedMCPProjectionConvergence(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		write bool
	}{
		{"Should converge after an epoch advance", false},
		{"Should converge under unrelated writes", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			fixture := newInstanceMCPProjectionFixture(t, 22, 5, 806, false)
			fixture.warmProjections(t)
			fixture.model.state.toolProjectionEpoch.Advance()
			var write func() error
			if scenario.write {
				write = fixture.convergenceWriter(t)
			}
			const limit = 2
			var misses []int
			for range limit + 1 {
				misses = append(misses, fixture.projectionWave(t, write))
				if misses[len(misses)-1] == 0 {
					t.Logf("converged after %d refresh windows (rebuilds %v)", len(misses)-1, misses)
					return
				}
			}
			t.Fatalf("still cold after %d refresh windows (rebuilds %v)", limit, misses)
		})
	}
}

func (f *instanceMCPProjectionFixture) projectionWave(t *testing.T, write func() error) int {
	t.Helper()
	f.now = f.now.Add(f.hosted.StreamRefreshInterval())
	clear(f.policyReads)
	release := make(chan struct{})
	ready := make(chan struct{}, len(f.binds))
	results := make(chan error, len(f.binds))
	entered := make(map[string]*sync.Once, len(f.ids))
	for _, id := range f.ids {
		entered[id] = new(sync.Once)
	}
	f.policy.aroundResolve = func(scope toolspkg.Scope) func() {
		if marker, ok := entered[scope.SessionID]; ok {
			marker.Do(func() { ready <- struct{}{} })
			<-release
		}
		return nil
	}
	for i, bind := range f.binds {
		go func() {
			_, err := f.hosted.StreamProjection(t.Context(), bind, f.peer, f.digests[i])
			entered[f.ids[i]].Do(func() { ready <- struct{}{} })
			results <- err
		}()
	}
	for range f.binds {
		<-ready
	}
	f.policy.readsMu.Lock()
	misses := len(f.policyReads)
	f.policy.readsMu.Unlock()
	var writeErr error
	if misses > 0 && write != nil {
		writeErr = write()
	}
	close(release)
	for range f.binds {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
	f.policy.aroundResolve = nil
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	return misses
}

func (f *instanceMCPProjectionFixture) convergenceWriter(t *testing.T) func() error {
	t.Helper()
	request := make(chan chan error)
	finished := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(func() {
		cancel()
		<-finished
	})
	go func() {
		defer close(finished)
		// One commit during each cold wave, paced at 10/s to model roughly
		// one event every two seconds per active session on a 22-bind daemon.
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		repo := f.db.Instances()
		for {
			select {
			case <-ctx.Done():
				return
			case result := <-request:
				<-ticker.C
				_, err := repo.MarkSeen(ctx, f.instanceIDs[0], agentinstance.LocalOperator, time.Now())
				result <- err
			}
		}
	}()
	return func() error {
		result := make(chan error, 1)
		request <- result
		return <-result
	}
}
