//go:build integration && !windows

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
	builtintools "github.com/compozy/compozy/internal/tools/builtin"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const agentNativeExtensionE2EName = "agent-native-e2e"

func TestDaemonE2EAgentCompletesExtensionAuthoringThroughNativeTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	deps, extensionRegistry, _, _ := newNativeExtensionToolDeps(t)
	workspaceRoot := t.TempDir()
	workspaceID := "workspace-agent-native-e2e"
	resolver := &daemonExtensionWorkspaceResolverStub{resolved: workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      workspaceID,
			Name:    "agent-native-e2e-workspace",
			RootDir: workspaceRoot,
		},
		WorkspaceID: workspaceID,
	}}
	installNativeGlobalResourceExtension(t, extensionRegistry, agentNativeExtensionE2EName)
	manager := extensionpkg.NewManager(extensionRegistry, extensionpkg.WithWorkspaceResolver(resolver))
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("extension manager Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(context.Background()); err != nil {
			t.Errorf("extension manager Stop() error = %v", err)
		}
	})

	service, ok := newDaemonExtensionService(
		extensionRegistry,
		manager,
		nil,
		nil,
		nil,
		nil,
		nil,
		deps.HomePaths,
		nil,
		nil,
		withDaemonExtensionMarketplace(deps.ExtensionConfig, deps.ExtensionSources),
		withDaemonExtensionEventWriter(deps.ExtensionEvents),
	).(*daemonExtensionService)
	if !ok {
		t.Fatal("newDaemonExtensionService() did not return daemon extension service")
	}
	deps.ExtensionRuntime = func() extensionRuntime { return manager }
	deps.WorkspaceResolver = resolver
	deps.Extensions = func() core.ExtensionService { return service }

	approvals := &agentNativeExtensionApprovalBridge{}
	registry := newAgentNativeExtensionRegistry(t, deps, extensionRegistry, manager, resolver, approvals)
	scope := toolspkg.Scope{
		WorkspaceID: workspaceID,
		SessionID:   "session-agent-native-e2e",
		ActorKind:   string(taskpkg.ActorKindAgentSession),
	}
	sourceDir := filepath.Join(workspaceRoot, agentNativeExtensionE2EName)

	initResult := callAgentNativeExtensionTool(t, ctx, registry, scope, toolspkg.ToolIDExtensionsInit, map[string]any{
		"name": agentNativeExtensionE2EName, "template": "tool-provider-go", "directory": sourceDir,
	})
	requireAgentNativeStructuredContains(t, initResult, []byte(agentNativeExtensionE2EName))
	configureExtensionAuthoringSDKReplaceWithoutShell(t, sourceDir, extensionAuthoringE2ERepoRoot(t))
	rewriteExtensionAuthoringGeneration(t, sourceDir, "No results for ", "agent-v1:", "agent-v1-observed")
	configureAgentNativeStructuredToolResult(t, sourceDir)

	firstBuild := decodeAgentNativeBuild(t, callAgentNativeExtensionTool(
		t, ctx, registry, scope, toolspkg.ToolIDExtensionsBuild, map[string]any{"source_dir": sourceDir},
	))
	validation := callAgentNativeExtensionTool(
		t, ctx, registry, scope, toolspkg.ToolIDExtensionsValidate, map[string]any{"path": firstBuild.GenerationDir},
	)
	requireAgentNativeStructuredContains(t, validation, []byte(`"issues":[]`))
	dev := callAgentNativeExtensionTool(t, ctx, registry, scope, toolspkg.ToolIDExtensionsDev, map[string]any{
		"origin_path": sourceDir, "generation_hash": firstBuild.GenerationHash,
	})
	requireAgentNativeStructuredContains(t, dev, []byte(`"workspace_id":"`+workspaceID+`"`))

	extensionToolID, err := toolspkg.CanonicalToolID("ext", agentNativeExtensionE2EName, "search")
	if err != nil {
		t.Fatalf("CanonicalToolID() error = %v", err)
	}
	firstInvocation := callAgentNativeExtensionTool(
		t, ctx, registry, scope, extensionToolID, map[string]any{"query": "alpha"},
	)
	requireAgentNativeStructuredContains(t, firstInvocation, []byte("agent-v1:alpha"))

	rewriteAgentNativeStructuredGeneration(t, sourceDir, "agent-v1:", "agent-v2:")
	secondBuild := decodeAgentNativeBuild(t, callAgentNativeExtensionTool(
		t, ctx, registry, scope, toolspkg.ToolIDExtensionsBuild, map[string]any{"source_dir": sourceDir},
	))
	if secondBuild.GenerationHash == firstBuild.GenerationHash {
		t.Fatalf("second generation hash = %q, want changed generation", secondBuild.GenerationHash)
	}
	reload := callAgentNativeExtensionTool(t, ctx, registry, scope, toolspkg.ToolIDExtensionsReload, map[string]any{
		"name": agentNativeExtensionE2EName, "generation_hash": secondBuild.GenerationHash,
	})
	requireAgentNativeStructuredContains(t, reload, []byte(secondBuild.GenerationHash))
	secondInvocation := callAgentNativeExtensionTool(
		t, ctx, registry, scope, extensionToolID, map[string]any{"query": "alpha"},
	)
	requireAgentNativeStructuredContains(t, secondInvocation, []byte("agent-v2:alpha"))

	status := callAgentNativeExtensionTool(
		t, ctx, registry, scope, toolspkg.ToolIDExtensionsInfo, map[string]any{"name": agentNativeExtensionE2EName},
	)
	requireAgentNativeStructuredContains(t, status, []byte(`"dev":true`))
	requireAgentNativeStructuredContains(t, status, []byte(`"generation_hash":"`+secondBuild.GenerationHash+`"`))
	provenance := callAgentNativeExtensionTool(
		t,
		ctx,
		registry,
		scope,
		toolspkg.ToolIDExtensionsProvenance,
		map[string]any{"name": agentNativeExtensionE2EName},
	)
	requireAgentNativeStructuredContains(t, provenance, []byte(`"installed_from":"local_path"`))

	removed := callAgentNativeExtensionTool(
		t, ctx, registry, scope, toolspkg.ToolIDExtensionsRemove, map[string]any{"name": agentNativeExtensionE2EName},
	)
	requireAgentNativeStructuredContains(t, removed, []byte(`"status":"removed"`))
	if _, err := extensionRegistry.GetDevLink(agentNativeExtensionE2EName, workspaceID); !errors.Is(
		err,
		extensionpkg.ErrExtensionNotDevLinked,
	) {
		t.Fatalf("GetDevLink(after remove) error = %v, want ErrExtensionNotDevLinked", err)
	}
	if _, err := registry.Call(ctx, scope, toolspkg.CallRequest{
		ToolID: extensionToolID,
		Input:  json.RawMessage(`{"query":"alpha"}`),
	}); !errors.Is(err, toolspkg.ErrToolNotFound) {
		t.Fatalf("Registry.Call(contributed tool after remove) error = %v, want ErrToolNotFound", err)
	}
	approvals.requireCounts(t, map[toolspkg.ToolID]int{
		toolspkg.ToolIDExtensionsInit:   1,
		toolspkg.ToolIDExtensionsBuild:  2,
		toolspkg.ToolIDExtensionsDev:    1,
		toolspkg.ToolIDExtensionsReload: 1,
		toolspkg.ToolIDExtensionsRemove: 1,
	})
}

func configureAgentNativeStructuredToolResult(t *testing.T, sourceDir string) {
	t.Helper()
	mainPath := filepath.Join(sourceDir, "main.go")
	data, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", mainPath, err)
	}
	oldResult := `return compozysdk.TextResult("agent-v1:" + req.Input.Query), nil`
	newResult := `resultText := "agent-v1:" + req.Input.Query
			structured, resultErr := compozysdk.StructuredResult(map[string]any{"result": resultText})
			if resultErr != nil {
				return compozysdk.ToolResult{}, resultErr
			}
			structured.Preview = resultText
			return structured, nil`
	content := strings.Replace(string(data), oldResult, newResult, 1)
	if content == string(data) {
		t.Fatalf("scaffold source %q is missing the expected text result", mainPath)
	}
	if err := os.WriteFile(mainPath, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", mainPath, err)
	}
}

func rewriteAgentNativeStructuredGeneration(t *testing.T, sourceDir, oldResult, newResult string) {
	t.Helper()
	mainPath := filepath.Join(sourceDir, "main.go")
	data, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", mainPath, err)
	}
	content := strings.Replace(string(data), oldResult, newResult, 1)
	if content == string(data) {
		t.Fatalf("scaffold source %q is missing result %q", mainPath, oldResult)
	}
	if err := os.WriteFile(mainPath, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", mainPath, err)
	}
}

func newAgentNativeExtensionRegistry(
	t *testing.T,
	deps *daemonNativeToolsDeps,
	extensionRegistry *extensionpkg.Registry,
	manager *extensionpkg.Manager,
	resolver workspacepkg.RuntimeResolver,
	approvals toolspkg.ApprovalBridge,
) *toolspkg.RuntimeRegistry {
	t.Helper()
	var registry *toolspkg.RuntimeRegistry
	deps.Registry = func() toolspkg.Registry { return registry }
	nativeProvider, err := newDaemonNativeProvider(deps)
	if err != nil {
		t.Fatalf("newDaemonNativeProvider() error = %v", err)
	}
	extensionProvider, err := extensionpkg.NewExtensionToolProvider(
		extensionRegistry,
		func() extensionpkg.ExtensionToolRuntime { return manager },
	)
	if err != nil {
		t.Fatalf("NewExtensionToolProvider() error = %v", err)
	}
	toolsets, err := builtintools.ToolsetCatalog()
	if err != nil {
		t.Fatalf("builtin.ToolsetCatalog() error = %v", err)
	}
	sourceGrant := toolspkg.SourceGrant{Kind: toolspkg.SourceExtension, Owner: agentNativeExtensionE2EName}
	registry, err = toolspkg.NewRegistry(
		toolspkg.WithProviders(nativeProvider, newDaemonScopedExtensionToolProvider(extensionProvider, resolver)),
		toolspkg.WithPolicyInputs(toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveReads,
			ApprovalAvailable:    true,
			TrustedSources:       []toolspkg.SourceGrant{sourceGrant},
			AllowSources:         []toolspkg.SourceGrant{sourceGrant},
		}, toolsets),
		toolspkg.WithApprovalBridge(approvals),
		toolspkg.WithDefaultMaxResultBytes(compozyconfig.DefaultToolsMaxResultBytes),
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	return registry
}

func callAgentNativeExtensionTool(
	t *testing.T,
	ctx context.Context,
	registry toolspkg.Registry,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	input any,
) toolspkg.ToolResult {
	t.Helper()
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("json.Marshal(%s input) error = %v", id, err)
	}
	result, err := registry.Call(ctx, scope, toolspkg.CallRequest{ToolID: id, Input: encoded})
	if err != nil {
		t.Fatalf("Registry.Call(%s) error = %v", id, err)
	}
	if !json.Valid(result.Structured) || result.Preview == "" {
		t.Fatalf("Registry.Call(%s) result = %#v, want structured JSON and preview", id, result)
	}
	return result
}

func decodeAgentNativeBuild(t *testing.T, result toolspkg.ToolResult) extensionpkg.BuildResult {
	t.Helper()
	var envelope struct {
		Build extensionpkg.BuildResult `json:"build"`
	}
	if err := json.Unmarshal(result.Structured, &envelope); err != nil {
		t.Fatalf("json.Unmarshal(extensions_build) error = %v", err)
	}
	if envelope.Build.GenerationDir == "" || len(envelope.Build.GenerationHash) != 64 {
		t.Fatalf("extensions_build result = %#v, want immutable generation", envelope.Build)
	}
	return envelope.Build
}

func requireAgentNativeStructuredContains(t *testing.T, result toolspkg.ToolResult, want []byte) {
	t.Helper()
	if !json.Valid(result.Structured) || !bytes.Contains(result.Structured, want) {
		t.Fatalf("structured result = %s, want %s", result.Structured, want)
	}
}

type agentNativeExtensionApprovalBridge struct {
	counts map[toolspkg.ToolID]int
}

var _ toolspkg.ApprovalBridge = (*agentNativeExtensionApprovalBridge)(nil)

func (b *agentNativeExtensionApprovalBridge) RequestToolApproval(
	_ context.Context,
	_ toolspkg.Scope,
	call toolspkg.CallRequest,
	view *toolspkg.ToolView,
) error {
	if view == nil || !view.Decision.ApprovalRequired {
		return fmt.Errorf("approval request for %s did not carry an approval-required decision", call.ToolID)
	}
	if b.counts == nil {
		b.counts = make(map[toolspkg.ToolID]int)
	}
	b.counts[call.ToolID]++
	return nil
}

func (b *agentNativeExtensionApprovalBridge) requireCounts(t *testing.T, want map[toolspkg.ToolID]int) {
	t.Helper()
	if len(b.counts) != len(want) {
		t.Fatalf("approved native tool IDs = %#v, want %#v", b.counts, want)
	}
	for id, count := range want {
		if b.counts[id] != count {
			t.Fatalf("approval count for %s = %d, want %d", id, b.counts[id], count)
		}
	}
}
