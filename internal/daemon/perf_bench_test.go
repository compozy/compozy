package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/resources"
	skillspkg "github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

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
		Workspace: workspacepkg.Workspace{ID: workspaceID},
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
	rawStore, agentStore, agentCodec, skillStore, skillCodec, mcpStore, mcpCodec := daemonBenchmarkAgentSkillStores(b)
	desired := daemonBenchmarkAgentSkillDesiredResources(24)
	syncer := newAgentSkillSourceSyncer(
		rawStore,
		agentStore,
		agentCodec,
		nil,
		skillStore,
		skillCodec,
		nil,
		mcpStore,
		mcpCodec,
		agentSkillSyncActor(),
		discardLogger(),
		nil,
		func(context.Context) (agentSkillDesiredResources, error) {
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

func daemonBenchmarkAgentRecords(count int, workspaceID string) []resources.Record[aghconfig.AgentDef] {
	records := make([]resources.Record[aghconfig.AgentDef], 0, count*2)
	for i := range count {
		name := fmt.Sprintf("agent-%03d", i)
		global := aghconfig.AgentDef{
			Name:        name,
			Prompt:      fmt.Sprintf("global prompt %d", i),
			Tools:       []string{toolspkg.ToolIDToolInfo.String(), toolspkg.ToolIDToolList.String()},
			Permissions: string(aghconfig.PermissionModeApproveAll),
			MCPServers: []aghconfig.MCPServer{{
				Name:    fmt.Sprintf("global-mcp-%03d", i),
				Command: "global-command",
				Args:    []string{"--mode", "global"},
			}},
		}
		records = append(records, resources.Record[aghconfig.AgentDef]{
			ID:      fmt.Sprintf("global:%s", name),
			Version: int64(i + 1),
			Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			Source: resources.ResourceSource{
				Kind: resources.ResourceSourceKind("bench"),
				ID:   fmt.Sprintf("global-%03d", i),
			},
			Spec: global,
		})

		workspace := global
		workspace.Prompt = fmt.Sprintf("workspace prompt %d", i)
		workspace.MCPServers = []aghconfig.MCPServer{{
			Name:    fmt.Sprintf("workspace-mcp-%03d", i),
			Command: "workspace-command",
			Args:    []string{"--mode", "workspace"},
		}}
		records = append(records, resources.Record[aghconfig.AgentDef]{
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

func daemonBenchmarkAgentSkillDesiredResources(count int) agentSkillDesiredResources {
	desired := agentSkillDesiredResources{
		agents:     make([]agentPublicationInput, 0, count),
		skills:     make([]skillPublicationInput, 0, count),
		mcpServers: make([]mcpServerPublicationInput, 0, count),
	}
	scope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
	for i := range count {
		desired.agents = append(desired.agents, agentPublicationInput{
			sourceKey: fmt.Sprintf("bench/agent/%03d", i),
			scope:     scope,
			spec: aghconfig.AgentDef{
				Name:        fmt.Sprintf("agent-%03d", i),
				Prompt:      fmt.Sprintf("Agent prompt %d", i),
				Tools:       []string{toolspkg.ToolIDToolInfo.String()},
				Permissions: string(aghconfig.PermissionModeApproveAll),
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
			spec: aghconfig.MCPServer{
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
	scope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
	for i := range count {
		desired.tools = append(desired.tools, toolPublicationInput{
			sourceKey: fmt.Sprintf("bench/tool/%03d", i),
			scope:     scope,
			spec: toolspkg.Tool{
				ID:           toolspkg.ToolID(fmt.Sprintf("agh__bench_tool_%03d", i)),
				DisplayTitle: fmt.Sprintf("tool-%03d", i),
				Description:  fmt.Sprintf("Tool %d", i),
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
			spec: aghconfig.MCPServer{
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
	resources.Store[aghconfig.AgentDef],
	resources.KindCodec[aghconfig.AgentDef],
	resources.Store[skillspkg.SkillResourceSpec],
	resources.KindCodec[skillspkg.SkillResourceSpec],
	resources.Store[aghconfig.MCPServer],
	resources.KindCodec[aghconfig.MCPServer],
) {
	b.Helper()

	kernel := daemonBenchmarkKernel(b)
	agentCodec, err := aghconfig.NewAgentResourceCodec()
	if err != nil {
		b.Fatalf("aghconfig.NewAgentResourceCodec() error = %v", err)
	}
	agentStore, err := resources.NewStore(kernel, agentCodec)
	if err != nil {
		b.Fatalf("resources.NewStore(agent) error = %v", err)
	}
	skillCodec, err := skillspkg.NewResourceCodec()
	if err != nil {
		b.Fatalf("skillspkg.NewResourceCodec() error = %v", err)
	}
	skillStore, err := resources.NewStore(kernel, skillCodec)
	if err != nil {
		b.Fatalf("resources.NewStore(skill) error = %v", err)
	}
	mcpCodec, err := aghconfig.NewMCPServerResourceCodec()
	if err != nil {
		b.Fatalf("aghconfig.NewMCPServerResourceCodec() error = %v", err)
	}
	mcpStore, err := resources.NewStore(kernel, mcpCodec)
	if err != nil {
		b.Fatalf("resources.NewStore(mcp) error = %v", err)
	}
	return kernel, agentStore, agentCodec, skillStore, skillCodec, mcpStore, mcpCodec
}

func daemonBenchmarkToolMCPStores(
	b *testing.B,
) (
	resources.RawStore,
	resources.Store[toolspkg.Tool],
	resources.KindCodec[toolspkg.Tool],
	resources.Store[aghconfig.MCPServer],
	resources.KindCodec[aghconfig.MCPServer],
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
	mcpCodec, err := aghconfig.NewMCPServerResourceCodec()
	if err != nil {
		b.Fatalf("aghconfig.NewMCPServerResourceCodec() error = %v", err)
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
	db, err := globaldb.OpenGlobalDB(ctx, filepath.Join(b.TempDir(), store.GlobalDatabaseName))
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
