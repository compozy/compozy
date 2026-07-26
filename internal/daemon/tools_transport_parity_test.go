package daemon_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/httpapi"
	"github.com/compozy/agh/internal/api/testutil"
	"github.com/compozy/agh/internal/api/udsapi"
	"github.com/compozy/agh/internal/session"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestToolRoutesStayHTTPAndUDSBehaviorallyAligned(t *testing.T) {
	t.Parallel()

	artifactID := fmt.Sprintf("art_%x", sha256.Sum256([]byte(toolParityArtifactContent)))
	requests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{name: "ShouldListTools", method: http.MethodGet, path: "/api/tools"},
		{
			name:   "ShouldSearchTools",
			method: http.MethodPost,
			path:   "/api/tools/search",
			body:   []byte(`{"query":"skill"}`),
		},
		{name: "ShouldGetTool", method: http.MethodGet, path: "/api/tools/agh__skill_view"},
		{
			name:   "ShouldInvokeTool",
			method: http.MethodPost,
			path:   "/api/tools/agh__skill_view/invoke",
			body:   []byte(`{"session_id":"sess-1","input":{"message":"hello"}}`),
		},
		{
			name:   "ShouldInvokeNetworkStatusTool",
			method: http.MethodPost,
			path:   "/api/tools/agh__network_status/invoke",
			body:   []byte(`{"session_id":"sess-1","input":{}}`),
		},
		{
			name:   "ShouldInvokeSessionEventsTool",
			method: http.MethodPost,
			path:   "/api/tools/agh__session_events/invoke",
			body: []byte(
				`{"session_id":"sess-1","workspace_id":"ws-1","input":{"workspace_id":"ws-1","session_id":"sess-1","limit":1}}`,
			),
		},
		{
			name:   "ShouldInvokeWorkspaceDescribeTool",
			method: http.MethodPost,
			path:   "/api/tools/agh__workspace_describe/invoke",
			body:   []byte(`{"session_id":"sess-1","workspace_id":"ws-1","input":{"workspace":"ws-1"}}`),
		},
		{name: "ShouldListSessionTools", method: http.MethodGet, path: "/api/workspaces/ws-1/sessions/sess-1/tools"},
		{
			name:   "ShouldSearchSessionTools",
			method: http.MethodPost,
			path:   "/api/workspaces/ws-1/sessions/sess-1/tools/search",
			body:   []byte(`{"query":"skill"}`),
		},
		{name: "ShouldListToolsets", method: http.MethodGet, path: "/api/toolsets"},
		{name: "ShouldGetToolset", method: http.MethodGet, path: "/api/toolsets/agh__catalog"},
		{
			name:   "ShouldReadToolArtifact",
			method: http.MethodGet,
			path:   "/api/workspaces/ws-1/tool-artifacts/" + artifactID + "?offset=2&limit=7",
		},
	}

	for _, request := range requests {
		t.Run(request.name, func(t *testing.T) {
			t.Parallel()

			httpEngine := newToolParityHTTPEngine(t)
			udsEngine := newToolParityUDSEngine(t)
			httpResp := testutil.PerformRequest(t, httpEngine, request.method, request.path, request.body)
			udsResp := testutil.PerformRequest(t, udsEngine, request.method, request.path, request.body)
			if httpResp.Code != udsResp.Code {
				t.Fatalf("status mismatch http=%d uds=%d", httpResp.Code, udsResp.Code)
			}
			if !jsonBodiesEqual(t, httpResp.Body.Bytes(), udsResp.Body.Bytes()) {
				t.Fatalf("body mismatch\nhttp=%s\nuds=%s", httpResp.Body.String(), udsResp.Body.String())
			}
			if request.name == "ShouldReadToolArtifact" {
				if httpResp.Code != http.StatusOK {
					t.Fatalf("artifact status = %d, want %d; body=%s", httpResp.Code, http.StatusOK, httpResp.Body)
				}
				var page contract.ToolArtifactPageResponse
				if err := json.Unmarshal(httpResp.Body.Bytes(), &page); err != nil {
					t.Fatalf("decode artifact page: %v", err)
				}
				decoded, err := base64.StdEncoding.DecodeString(page.DataBase64)
				if err != nil {
					t.Fatalf("decode artifact data: %v", err)
				}
				if got, want := string(decoded), toolParityArtifactContent[2:9]; got != want {
					t.Fatalf("artifact page = %q, want %q", got, want)
				}
				if page.Offset != 2 || page.Bytes != 7 || page.NextOffset != 9 || page.EOF {
					t.Fatalf("artifact page metadata = %#v, want exact non-terminal page", page)
				}
			}
		})
	}
}

func newToolParityHTTPEngine(t *testing.T) *gin.Engine {
	t.Helper()
	homePaths := testutil.NewTestHomePaths(t)
	cfg := testutil.ConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = 2123
	registry := newToolParityRegistry()
	engine := gin.New()
	if _, err := httpapi.New(
		httpapi.WithEngine(engine),
		httpapi.WithHomePaths(homePaths),
		httpapi.WithConfig(&cfg),
		httpapi.WithHost(cfg.HTTP.Host),
		httpapi.WithPort(cfg.HTTP.Port),
		httpapi.WithSessionManager(toolParitySessionManager()),
		httpapi.WithObserver(testutil.StubObserver{}),
		httpapi.WithTaskService(&testutil.StubTaskManager{}),
		httpapi.WithWorkspaceResolver(toolParityWorkspaceService(t)),
		httpapi.WithToolRegistry(registry),
		httpapi.WithToolArtifactStore(newToolParityArtifactStore(t)),
		httpapi.WithToolsetRegistry(registry),
		httpapi.WithLogger(testutil.DiscardLogger()),
		httpapi.WithStartedAt(time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)),
		httpapi.WithNow(func() time.Time { return time.Date(2026, 4, 29, 12, 0, 1, 0, time.UTC) }),
	); err != nil {
		t.Fatalf("httpapi.New() error = %v", err)
	}
	return engine
}

func newToolParityUDSEngine(t *testing.T) *gin.Engine {
	t.Helper()
	engine := gin.New()
	udsapi.RegisterRoutes(engine, &udsapi.Handlers{BaseHandlers: newToolParityBaseHandlers(t)})
	return engine
}

func newToolParityBaseHandlers(t *testing.T) *core.BaseHandlers {
	t.Helper()
	homePaths := testutil.NewTestHomePaths(t)
	registry := newToolParityRegistry()
	return core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName: "tool-parity-test",
		Sessions:      toolParitySessionManager(),
		Observer:      testutil.StubObserver{},
		Tasks:         &testutil.StubTaskManager{},
		Workspaces:    toolParityWorkspaceService(t),
		Tools:         registry,
		ToolArtifacts: newToolParityArtifactStore(t),
		Toolsets:      registry,
		HomePaths:     homePaths,
		Config:        testutil.ConfigWithDisabledNetwork(homePaths),
		Logger:        testutil.DiscardLogger(),
		StartedAt:     time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
		Now:           func() time.Time { return time.Date(2026, 4, 29, 12, 0, 1, 0, time.UTC) },
		StreamDone:    make(chan struct{}),
	})
}

const toolParityArtifactContent = `{"version":"agh.tool-result/v1","tail":"D6-TAIL"}`

func newToolParityArtifactStore(t *testing.T) *toolspkg.FilesystemToolArtifactStore {
	t.Helper()
	store, err := toolspkg.OpenFilesystemToolArtifactStore(
		t.Context(),
		t.TempDir(),
		toolspkg.ToolArtifactRetention{MaxCount: 10, MaxBytes: 1 << 20, MaxAge: time.Hour},
	)
	if err != nil {
		t.Fatalf("OpenFilesystemToolArtifactStore() error = %v", err)
	}
	if _, err := store.Put(t.Context(), "ws-1", []byte(toolParityArtifactContent)); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	return store
}

func toolParitySessionManager() testutil.StubSessionManager {
	return testutil.StubSessionManager{
		StatusFn: func(ctx context.Context, id string) (*session.Info, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if strings.TrimSpace(id) != "sess-1" {
				return nil, session.ErrSessionNotFound
			}
			return &session.Info{ID: "sess-1", WorkspaceID: "ws-1", AgentName: "coder"}, nil
		},
	}
}

func toolParityWorkspaceService(t *testing.T) testutil.StubWorkspaceService {
	t.Helper()

	root := t.TempDir()
	return testutil.StubWorkspaceService{
		ResolveFn: func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
			if err := ctx.Err(); err != nil {
				return workspacepkg.ResolvedWorkspace{}, err
			}
			if strings.TrimSpace(ref) != "ws-1" {
				return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
			}
			return workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{
					ID:      "ws-1",
					RootDir: root,
					Name:    "ws-1",
				},
				WorkspaceID: "ws-1",
			}, nil
		},
	}
}

type toolParityRegistry struct {
	mu    sync.Mutex
	views []toolspkg.ToolView
}

func newToolParityRegistry() *toolParityRegistry {
	return &toolParityRegistry{views: []toolspkg.ToolView{
		toolParityView(toolspkg.ToolIDSkillView, toolspkg.VisibilityModel, true),
		toolParityView(toolspkg.ToolIDNetworkStatus, toolspkg.VisibilityModel, true),
		toolParityView(toolspkg.ToolIDSessionEvents, toolspkg.VisibilityModel, true),
		toolParityView(toolspkg.ToolIDWorkspaceDescribe, toolspkg.VisibilityModel, true),
		toolParityView("agh__operator_diag", toolspkg.VisibilityOperator, false),
	}}
}

func (r *toolParityRegistry) List(_ context.Context, scope toolspkg.Scope) ([]toolspkg.ToolView, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	views := make([]toolspkg.ToolView, 0, len(r.views))
	for i := range r.views {
		view := &r.views[i]
		if scope.Operator || (view.Decision.VisibleToSession && view.Decision.Callable) {
			views = append(views, r.views[i])
		}
	}
	return views, nil
}

func (r *toolParityRegistry) Search(
	ctx context.Context,
	scope toolspkg.Scope,
	q toolspkg.SearchQuery,
) ([]toolspkg.ToolView, error) {
	views, err := r.List(ctx, scope)
	if err != nil {
		return nil, err
	}
	needle := strings.TrimSpace(strings.ToLower(q.Query))
	if needle == "" {
		return views, nil
	}
	filtered := make([]toolspkg.ToolView, 0, len(views))
	for i := range views {
		view := &views[i]
		if strings.Contains(strings.ToLower(view.Descriptor.ID.String()+" "+view.Descriptor.Description), needle) {
			filtered = append(filtered, views[i])
		}
	}
	return filtered, nil
}

func (r *toolParityRegistry) Get(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
) (toolspkg.ToolView, error) {
	views, err := r.List(ctx, scope)
	if err != nil {
		return toolspkg.ToolView{}, err
	}
	for i := range views {
		view := &views[i]
		if view.Descriptor.ID == id {
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

func (r *toolParityRegistry) Call(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	if _, err := r.Get(ctx, toolspkg.Scope{Operator: true}, req.ToolID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return toolspkg.ToolResult{
		Content:    []toolspkg.ToolContent{{Type: "text", Text: "ok"}},
		Structured: json.RawMessage(`{"ok":true}`),
		DurationMS: 10,
	}, nil
}

func (r *toolParityRegistry) ListToolsets(context.Context, toolspkg.Scope) ([]toolspkg.ToolsetView, error) {
	return []toolspkg.ToolsetView{toolParityToolset()}, nil
}

func (r *toolParityRegistry) GetToolset(
	_ context.Context,
	_ toolspkg.Scope,
	id toolspkg.ToolsetID,
) (toolspkg.ToolsetView, error) {
	if id == "agh__catalog" {
		return toolParityToolset(), nil
	}
	return toolspkg.ToolsetView{}, toolspkg.NewToolError(
		toolspkg.ErrorCodeNotFound,
		toolspkg.ToolID(id),
		fmt.Sprintf("toolset %q not found", id),
		toolspkg.ErrToolNotFound,
		toolspkg.ReasonToolsetUnknown,
	)
}

func toolParityView(id toolspkg.ToolID, visibility toolspkg.Visibility, callable bool) toolspkg.ToolView {
	return toolspkg.ToolView{
		Descriptor: toolspkg.Descriptor{
			ID:           id,
			Backend:      toolspkg.BackendRef{Kind: toolspkg.BackendNativeGo, NativeName: id.String()},
			DisplayTitle: id.String(),
			Description:  "Skill registry test tool",
			InputSchema:  json.RawMessage(`{"type":"object"}`),
			Source:       toolspkg.SourceRef{Kind: toolspkg.SourceBuiltin, Owner: "agh"},
			Visibility:   visibility,
			Risk:         toolspkg.RiskRead,
			ReadOnly:     true,
			Toolsets:     []toolspkg.ToolsetID{"agh__catalog"},
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

func toolParityToolset() toolspkg.ToolsetView {
	return toolspkg.ToolsetView{
		Toolset:       toolspkg.Toolset{ID: "agh__catalog", Tools: []string{"agh__skill_view"}},
		ExpandedTools: []toolspkg.ToolID{toolspkg.ToolIDSkillView},
	}
}

func jsonBodiesEqual(t *testing.T, left []byte, right []byte) bool {
	t.Helper()
	var leftValue any
	if err := json.Unmarshal(left, &leftValue); err != nil {
		t.Fatalf("json.Unmarshal(left) error = %v; body=%s", err, left)
	}
	var rightValue any
	if err := json.Unmarshal(right, &rightValue); err != nil {
		t.Fatalf("json.Unmarshal(right) error = %v; body=%s", err, right)
	}
	return reflect.DeepEqual(leftValue, rightValue)
}
