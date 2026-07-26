//go:build integration

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	"github.com/compozy/agh/internal/api/udsapi"
	aghconfig "github.com/compozy/agh/internal/config"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestCLIToolCommandsMatchUDSContractsIntegration(t *testing.T) {
	t.Parallel()

	homePaths := testutil.NewTestHomePaths(t)
	cfg := testutil.ConfigWithDisabledNetwork(homePaths)
	cfg.Daemon.Socket = shortSocketPath(t)
	registry := newCLIToolIntegrationRegistry()
	artifactStore, err := toolspkg.OpenFilesystemToolArtifactStore(
		t.Context(),
		homePaths.ToolArtifactsDir,
		toolspkg.ToolArtifactRetention{MaxCount: 10, MaxBytes: 1 << 20, MaxAge: time.Hour},
	)
	if err != nil {
		t.Fatalf("OpenFilesystemToolArtifactStore() error = %v", err)
	}
	artifactContent := []byte(`{"version":"agh.tool-result/v1","tail":"D6-TAIL"}`)
	artifactRef, err := artifactStore.Put(t.Context(), "ws-1", artifactContent)
	if err != nil {
		t.Fatalf("artifactStore.Put() error = %v", err)
	}
	workspaceRoot := t.TempDir()
	server, err := udsapi.New(
		udsapi.WithHomePaths(homePaths),
		udsapi.WithConfig(&cfg),
		udsapi.WithSocketPath(cfg.Daemon.Socket),
		udsapi.WithLogger(discardLogger()),
		udsapi.WithSessionManager(testutil.StubSessionManager{}),
		udsapi.WithTaskService(&testutil.StubTaskManager{}),
		udsapi.WithObserver(testutil.StubObserver{}),
		udsapi.WithWorkspaceResolver(testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      ref,
						RootDir: workspaceRoot,
						Name:    ref,
					},
					WorkspaceID: ref,
				}, nil
			},
		}),
		udsapi.WithToolRegistry(registry),
		udsapi.WithToolArtifactStore(artifactStore),
		udsapi.WithToolsetRegistry(registry),
	)
	if err != nil {
		t.Fatalf("udsapi.New() error = %v", err)
	}
	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("server.Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			t.Errorf("server.Shutdown() error = %v", err)
		}
	})

	deps := toolIntegrationDeps(t, homePaths, cfg)

	t.Run("Should match tool list payload", func(t *testing.T) {
		expectedPayload := expectedCLIToolListPayload(t, registry, toolspkg.Scope{WorkspaceID: "ws-1", Operator: true})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"tool",
			"list",
			"--workspace",
			"ws-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool list error = %v", err)
		}
		var cliPayload ToolsResponseRecord
		decodeJSONOutput(t, stdout, &cliPayload)
		assertContractJSONEqual(t, cliPayload, expectedPayload)
	})

	t.Run("Should match tool search payload", func(t *testing.T) {
		expectedPayload := expectedCLIToolSearchPayload(
			t,
			registry,
			toolspkg.Scope{WorkspaceID: "ws-1", Operator: true},
			toolspkg.SearchQuery{Query: "skill", Limit: 1},
		)
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"tool",
			"search",
			"skill",
			"--limit",
			"1",
			"--workspace",
			"ws-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool search error = %v", err)
		}
		var cliPayload ToolsResponseRecord
		decodeJSONOutput(t, stdout, &cliPayload)
		assertContractJSONEqual(t, cliPayload, expectedPayload)
	})

	t.Run("Should match tool invoke payload", func(t *testing.T) {
		input := json.RawMessage(`{"message":"hello"}`)
		expectedPayload := expectedCLIToolInvokePayload(
			t,
			registry,
			toolspkg.Scope{SessionID: "sess-1", Operator: true},
			toolspkg.CallRequest{
				ToolID:        toolspkg.ToolIDSkillView,
				SessionID:     "sess-1",
				Input:         input,
				ApprovalToken: "approval-ref",
			},
		)
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"tool",
			"invoke",
			toolspkg.ToolIDSkillView.String(),
			"--session",
			"sess-1",
			"--input",
			`{"message":"hello"}`,
			"--approval-token",
			" approval-ref ",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool invoke error = %v", err)
		}
		var cliPayload ToolInvokeResponseRecord
		decodeJSONOutput(t, stdout, &cliPayload)
		assertContractJSONEqual(t, cliPayload, expectedPayload)
	})

	t.Run("Should match artifact paging and decoded human output", func(t *testing.T) {
		expected, err := artifactStore.ReadPage(t.Context(), "ws-1", artifactRef.URI, 2, 7)
		if err != nil {
			t.Fatalf("artifactStore.ReadPage() error = %v", err)
		}
		expectedPayload := ToolArtifactPageRecord{
			Artifact:   expected.Artifact,
			Offset:     expected.Offset,
			Bytes:      expected.Bytes,
			TotalBytes: expected.TotalBytes,
			DataBase64: expected.DataBase64,
			NextOffset: expected.NextOffset,
			EOF:        expected.EOF,
		}
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"tool",
			"artifact",
			"read",
			artifactRef.URI,
			"--workspace",
			"ws-1",
			"--offset",
			"2",
			"--limit",
			"7",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool artifact JSON read error = %v", err)
		}
		var page ToolArtifactPageRecord
		decodeJSONOutput(t, stdout, &page)
		assertContractJSONEqual(t, page, expectedPayload)

		stdout, _, err = executeRootCommand(
			t,
			deps,
			"tool",
			"artifact",
			"read",
			artifactRef.URI,
			"--workspace",
			"ws-1",
			"--offset",
			"2",
			"--limit",
			"7",
		)
		if err != nil {
			t.Fatalf("tool artifact human read error = %v", err)
		}
		if got, want := stdout, string(artifactContent[2:9]); got != want {
			t.Fatalf("tool artifact human output = %q, want %q", got, want)
		}
	})

	t.Run("Should match toolset info payload", func(t *testing.T) {
		expectedPayload := expectedCLIToolsetPayload(
			t,
			registry,
			toolspkg.Scope{Operator: true},
			toolspkg.ToolsetIDCatalog,
		)
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"toolsets",
			"info",
			toolspkg.ToolsetIDCatalog.String(),
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("toolsets info error = %v", err)
		}
		var cliPayload ToolsetResponseRecord
		decodeJSONOutput(t, stdout, &cliPayload)
		assertContractJSONEqual(t, cliPayload, expectedPayload)
	})
}

func assertContractJSONEqual(t *testing.T, got any, want any) {
	t.Helper()

	var gotValue any
	if err := json.Unmarshal(mustJSON(t, got), &gotValue); err != nil {
		t.Fatalf("json.Unmarshal(got) error = %v", err)
	}
	var wantValue any
	if err := json.Unmarshal(mustJSON(t, want), &wantValue); err != nil {
		t.Fatalf("json.Unmarshal(want) error = %v", err)
	}
	if gotJSON, wantJSON := mustJSON(t, gotValue), mustJSON(t, wantValue); string(gotJSON) != string(wantJSON) {
		t.Fatalf("contract JSON = %s, want %s", gotJSON, wantJSON)
	}
}

func expectedCLIToolListPayload(
	t *testing.T,
	registry *cliToolIntegrationRegistry,
	scope toolspkg.Scope,
) ToolsResponseRecord {
	t.Helper()

	views, err := registry.List(context.Background(), scope)
	if err != nil {
		t.Fatalf("registry.List() error = %v", err)
	}
	return ToolsResponseRecord{Tools: core.ToolPayloadsFromViews(views)}
}

func expectedCLIToolSearchPayload(
	t *testing.T,
	registry *cliToolIntegrationRegistry,
	scope toolspkg.Scope,
	query toolspkg.SearchQuery,
) ToolsResponseRecord {
	t.Helper()

	views, err := registry.Search(context.Background(), scope, query)
	if err != nil {
		t.Fatalf("registry.Search() error = %v", err)
	}
	return ToolsResponseRecord{Tools: core.ToolPayloadsFromViews(views)}
}

func expectedCLIToolInvokePayload(
	t *testing.T,
	registry *cliToolIntegrationRegistry,
	scope toolspkg.Scope,
	request toolspkg.CallRequest,
) ToolInvokeResponseRecord {
	t.Helper()

	result, err := registry.Call(context.Background(), scope, request)
	if err != nil {
		t.Fatalf("registry.Call() error = %v", err)
	}
	return ToolInvokeResponseRecord{
		ToolID:     request.ToolID,
		Status:     "completed",
		Result:     result,
		Truncated:  result.Truncated,
		DurationMS: result.DurationMS,
		Events:     []contract.ToolCallEventPayload{},
	}
}

func expectedCLIToolsetPayload(
	t *testing.T,
	registry *cliToolIntegrationRegistry,
	scope toolspkg.Scope,
	id toolspkg.ToolsetID,
) ToolsetResponseRecord {
	t.Helper()

	toolset, err := registry.GetToolset(context.Background(), scope, id)
	if err != nil {
		t.Fatalf("registry.GetToolset() error = %v", err)
	}
	return ToolsetResponseRecord{Toolset: core.ToolsetPayloadFromView(toolset)}
}

func toolIntegrationDeps(
	t *testing.T,
	homePaths aghconfig.HomePaths,
	cfg aghconfig.Config,
) commandDeps {
	t.Helper()

	return commandDeps{
		loadConfig: func() (aghconfig.Config, error) {
			return cfg, nil
		},
		resolveHome: func() (aghconfig.HomePaths, error) {
			return homePaths, nil
		},
		ensureHome: func(aghconfig.HomePaths) error { return nil },
		newClient:  NewClient,
		getwd: func() (string, error) {
			return "/workspace/project", nil
		},
		getenv: func(string) string { return "" },
		now: func() time.Time {
			return time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
		},
	}
}

type cliToolIntegrationRegistry struct {
	mu    sync.Mutex
	views []toolspkg.ToolView
}

var _ core.ToolRegistry = (*cliToolIntegrationRegistry)(nil)
var _ core.ToolsetRegistry = (*cliToolIntegrationRegistry)(nil)

func newCLIToolIntegrationRegistry() *cliToolIntegrationRegistry {
	return &cliToolIntegrationRegistry{views: []toolspkg.ToolView{
		cliToolIntegrationView(toolspkg.ToolIDSkillView, true),
		cliToolIntegrationView("agh__operator_diag", false),
	}}
}

func (r *cliToolIntegrationRegistry) List(
	_ context.Context,
	scope toolspkg.Scope,
) ([]toolspkg.ToolView, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	views := make([]toolspkg.ToolView, 0, len(r.views))
	for i := range r.views {
		view := r.views[i]
		if scope.Operator || (view.Decision.VisibleToSession && view.Decision.Callable) {
			views = append(views, view)
		}
	}
	return views, nil
}

func (r *cliToolIntegrationRegistry) Search(
	ctx context.Context,
	scope toolspkg.Scope,
	query toolspkg.SearchQuery,
) ([]toolspkg.ToolView, error) {
	views, err := r.List(ctx, scope)
	if err != nil {
		return nil, err
	}
	needle := strings.TrimSpace(strings.ToLower(query.Query))
	if needle == "" {
		return limitCLIToolIntegrationViews(views, query.Limit), nil
	}
	filtered := make([]toolspkg.ToolView, 0, len(views))
	for i := range views {
		view := views[i]
		if strings.Contains(strings.ToLower(view.Descriptor.ID.String()+" "+view.Descriptor.Description), needle) {
			filtered = append(filtered, view)
		}
	}
	return limitCLIToolIntegrationViews(filtered, query.Limit), nil
}

func (r *cliToolIntegrationRegistry) Get(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
) (toolspkg.ToolView, error) {
	if err := id.Validate(); err != nil {
		return toolspkg.ToolView{}, err
	}
	views, err := r.List(ctx, scope)
	if err != nil {
		return toolspkg.ToolView{}, err
	}
	for i := range views {
		if views[i].Descriptor.ID == id {
			return views[i], nil
		}
	}
	return toolspkg.ToolView{}, toolspkg.NewToolError(
		toolspkg.ErrorCodeNotFound,
		id,
		fmt.Sprintf("tool %q not found", id),
		toolspkg.ErrToolNotFound,
		toolspkg.ReasonToolUnknown,
	)
}

func (r *cliToolIntegrationRegistry) Call(
	ctx context.Context,
	scope toolspkg.Scope,
	request toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	if _, err := r.Get(ctx, scope, request.ToolID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if request.ApprovalToken != "" && request.ApprovalToken != "approval-ref" {
		return toolspkg.ToolResult{}, toolspkg.NewValidationError(
			"approval_token",
			toolspkg.ReasonApprovalTokenMismatch,
			"approval token did not match integration fixture",
		)
	}
	return toolspkg.ToolResult{
		Content: []toolspkg.ToolContent{{Type: "text", Text: "ok"}},
		Structured: json.RawMessage(fmt.Sprintf(
			`{"approval_token_present":%t,"ok":true}`,
			request.ApprovalToken != "",
		)),
		Preview:    "ok",
		DurationMS: 5,
	}, nil
}

func (r *cliToolIntegrationRegistry) ListToolsets(
	context.Context,
	toolspkg.Scope,
) ([]toolspkg.ToolsetView, error) {
	return []toolspkg.ToolsetView{cliToolIntegrationToolset()}, nil
}

func (r *cliToolIntegrationRegistry) GetToolset(
	_ context.Context,
	_ toolspkg.Scope,
	id toolspkg.ToolsetID,
) (toolspkg.ToolsetView, error) {
	if id == toolspkg.ToolsetIDCatalog {
		return cliToolIntegrationToolset(), nil
	}
	return toolspkg.ToolsetView{}, toolspkg.NewToolError(
		toolspkg.ErrorCodeNotFound,
		toolspkg.ToolID(id),
		fmt.Sprintf("toolset %q not found", id),
		toolspkg.ErrToolNotFound,
		toolspkg.ReasonToolsetUnknown,
	)
}

func cliToolIntegrationView(id toolspkg.ToolID, callable bool) toolspkg.ToolView {
	visibility := toolspkg.VisibilityModel
	if !callable {
		visibility = toolspkg.VisibilityOperator
	}
	return toolspkg.ToolView{
		Descriptor: toolspkg.Descriptor{
			ID:          id,
			Backend:     toolspkg.BackendRef{Kind: toolspkg.BackendNativeGo, NativeName: id.String()},
			Description: "Skill registry integration tool",
			InputSchema: json.RawMessage(`{"type":"object"}`),
			Source:      toolspkg.SourceRef{Kind: toolspkg.SourceBuiltin, Owner: toolspkg.BuiltinSourceOwner},
			Visibility:  visibility,
			Risk:        toolspkg.RiskRead,
			ReadOnly:    true,
			Toolsets:    []toolspkg.ToolsetID{toolspkg.ToolsetIDCatalog},
		},
		Availability: toolspkg.Availability{
			Registered: true,
			Enabled:    true,
			Available:  true,
			Authorized: true,
			Executable: callable,
		},
		Decision: toolspkg.EffectiveToolDecision{
			VisibleToOperator: true,
			VisibleToSession:  callable,
			Callable:          callable,
		},
	}
}

func cliToolIntegrationToolset() toolspkg.ToolsetView {
	return toolspkg.ToolsetView{
		Toolset:       toolspkg.Toolset{ID: toolspkg.ToolsetIDCatalog, Tools: []string{"agh__skill_view"}},
		ExpandedTools: []toolspkg.ToolID{toolspkg.ToolIDSkillView},
	}
}

func limitCLIToolIntegrationViews(views []toolspkg.ToolView, limit int) []toolspkg.ToolView {
	if limit <= 0 || limit >= len(views) {
		return views
	}
	return views[:limit]
}
