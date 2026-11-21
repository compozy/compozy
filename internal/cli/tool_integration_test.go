//go:build integration

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/api/udsapi"
	compozyconfig "github.com/compozy/compozy/internal/config"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/store/globaldb"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"

	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCLIToolCommandsMatchUDSContractsIntegration(t *testing.T) {
	t.Parallel()

	homePaths := testutil.NewTestHomePaths(t)
	cfg := compozyconfig.DefaultWithHome(homePaths)
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
	artifactContent := []byte(`{"version":"compozy.tool-result/v1","tail":"D6-TAIL"}`)
	artifactRef, err := artifactStore.Put(t.Context(), workspaceaccess.ScopeTarget("ws-1", ""), artifactContent)
	if err != nil {
		t.Fatalf("artifactStore.Put() error = %v", err)
	}
	workspaceRoot := t.TempDir()
	server, err := udsapi.New(
		udsapi.WithHomePaths(homePaths),
		udsapi.WithProfileService(newToolIntegrationProfiles(t, homePaths)),
		udsapi.WithConfig(&cfg),
		udsapi.WithSocketPath(cfg.Daemon.Socket),
		udsapi.WithLogger(discardLogger()),
		udsapi.WithSessionManager(&testutil.StubSessionManager{}),
		udsapi.WithTaskService(&testutil.StubTaskManager{}),
		udsapi.WithObserver(testutil.StubObserver{}),
		udsapi.WithWorkspaceResolver(testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					ID:          ref,
					RootDir:     workspaceRoot,
					Name:        ref,
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
	if err := server.Start(t.Context()); err != nil {
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
		expectedPayload.Count = 1
		expectedPayload.Total = 2
		expectedPayload.NextOffset = 1
		expectedPayload.HasMore = true

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
		next, _, err := executeRootCommand(
			t,
			deps,
			"tool",
			"search",
			"skill",
			"--limit",
			"1",
			"--offset",
			"1",
			"--workspace",
			"ws-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatal(err)
		}
		var nextPage ToolsResponseRecord
		decodeJSONOutput(t, next, &nextPage)
		if nextPage.Offset != 1 || nextPage.Count != 1 || nextPage.Total != 2 || nextPage.HasMore ||
			nextPage.NextOffset != 0 || len(nextPage.Tools) != 1 || nextPage.Tools[0].Descriptor.ToolID == cliPayload.Tools[0].Descriptor.ToolID {
			t.Fatalf("incorrect search continuation: %#v", nextPage)
		}

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
		expected, err := artifactStore.ReadPage(
			t.Context(),
			workspaceaccess.ScopeTarget("ws-1", ""),
			artifactRef.URI,
			2,
			7,
		)
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

// The production proxy and daemon UDS route own hosted wait progress framing and socket cancellation.
func TestCLIHostedMCPCallLifetimeIntegration(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		withProgress bool
	}{
		{"Should return a tool error and close a silent hosted call", false},
		{"Should deliver a progress frame before the final tool result", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			received := make(chan struct{})
			canceled := make(chan struct{})
			registry := &cliHostedWaitRegistry{
				cliToolIntegrationRegistry: newCLIToolIntegrationRegistry(),
				canceled:                   canceled,
			}
			if tc.withProgress {
				registry.release = received
			}
			client, nonce := startCLIHostedWaitDaemon(t, registry)
			client.hostedMCPCallTimeout = 2 * time.Second
			transport := client.streamClient.Transport
			progress := sync.OnceFunc(func() { close(received) })
			client.streamClient.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/api/internal/hosted-mcp/tools/call" {
					trace := &httptrace.ClientTrace{Got1xxResponse: func(status int, _ textproto.MIMEHeader) error {
						if status == http.StatusProcessing {
							progress()
						}
						return nil
					}}
					req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
				}
				return transport.RoundTrip(req)
			})

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			watchdog := time.AfterFunc(10*time.Second, cancel)
			defer watchdog.Stop()
			session := startCLIHostedWaitProxy(t, ctx, client, nonce)
			result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
				Name: toolspkg.ToolIDSkillView.String(), Arguments: map[string]any{},
			})
			if err != nil {
				t.Fatalf("CallTool() = %v, want MCP tool result", err)
			}
			if len(result.Content) == 0 {
				t.Fatal("CallTool() returned no content")
			}
			message, ok := result.Content[0].(*sdkmcp.TextContent)
			if tc.withProgress {
				if result.IsError || !ok || message.Text != "ok" {
					t.Fatalf("CallTool() = %#v, want successful result after progress", result)
				}
				return
			}
			if !result.IsError || !ok || !strings.Contains(message.Text, context.DeadlineExceeded.Error()) {
				t.Fatalf("CallTool() = %#v, want hosted deadline error", result)
			}
			select {
			case <-canceled:
			case <-ctx.Done():
				t.Fatal("daemon did not observe cancellation of the hosted request")
			}
		})
	}
}

type cliHostedWaitRegistry struct {
	*cliToolIntegrationRegistry
	release  <-chan struct{}
	canceled chan<- struct{}
}

func (r *cliHostedWaitRegistry) Call(
	ctx context.Context, scope toolspkg.Scope, request toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	if r.release != nil {
		defer toolspkg.BeginHostedWaitProgress(ctx)()
	}
	select {
	case <-r.release:
		return r.cliToolIntegrationRegistry.Call(ctx, scope, request)
	case <-ctx.Done():
		close(r.canceled)
		return toolspkg.ToolResult{}, ctx.Err()
	}
}

const cliHostedWaitSessionID = "hosted-lifetime-session"

func startCLIHostedWaitDaemon(t *testing.T, registry *cliHostedWaitRegistry) (*daemonClient, string) {
	t.Helper()

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	service, err := mcppkg.NewHostedService(&mcppkg.HostedConfig{
		Enabled: true, ExpectedBinary: executable, BindNonceTTL: time.Minute,
		Registry: func() toolspkg.Registry { return registry },
	})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := service.Launch(t.Context(), mcppkg.HostedLaunchRequest{
		SessionID: cliHostedWaitSessionID, WorkspaceID: "ws-1", AgentName: "coder", EagerTools: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ArmLaunch(t.Context(), cliHostedWaitSessionID); err != nil {
		t.Fatal(err)
	}
	index := slices.Index(launch.Args, "--bind-nonce")
	if index < 0 || index+1 >= len(launch.Args) {
		t.Fatal("hosted launch did not include a bind nonce")
	}
	router := gin.New()
	udsapi.RegisterRoutes(router, &udsapi.Handlers{BaseHandlers: core.NewBaseHandlers(nil), HostedMCP: service})
	server := &http.Server{
		Handler: router, ReadHeaderTimeout: 5 * time.Second,
		ConnContext: func(ctx context.Context, conn net.Conn) context.Context {
			peer, err := mcppkg.PeerInfoFromConn(conn)
			return mcppkg.ContextWithPeerInfo(ctx, peer, err)
		},
	}
	socket := shortSocketPath(t)
	listen := net.ListenConfig{}
	listener, err := listen.Listen(t.Context(), clientUnixNetwork, socket)
	if err != nil {
		t.Fatal(err)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
		if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("serve hosted socket: %v", err)
		}
	})

	daemon, err := NewClient(LocalClientTarget(socket))
	if err != nil {
		t.Fatal(err)
	}
	client, ok := daemon.(*daemonClient)
	if !ok {
		t.Fatalf("daemon client = %T, want *daemonClient", daemon)
	}
	t.Cleanup(client.streamClient.CloseIdleConnections)
	return client, launch.Args[index+1]
}

func startCLIHostedWaitProxy(
	t *testing.T, ctx context.Context, cliClient *daemonClient, nonce string,
) *sdkmcp.ClientSession {
	t.Helper()

	clientConn, proxyConn := net.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- mcppkg.RunHostedProxy(ctx, cliClient, mcppkg.HostedProxyOptions{
			SessionID: cliHostedWaitSessionID, Nonce: nonce, Stdin: proxyConn, Stdout: proxyConn,
		})
	}()
	t.Cleanup(func() {
		for _, conn := range []net.Conn{clientConn, proxyConn} {
			if err := conn.Close(); err != nil {
				t.Error(err)
			}
		}
		select {
		case err := <-done:
			if err != nil && ctx.Err() == nil {
				t.Errorf("RunHostedProxy(): %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("timed out waiting for hosted proxy shutdown")
		}
	})
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "hosted-wait-test", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &sdkmcp.IOTransport{Reader: clientConn, Writer: clientConn}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return session
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

	views, err := registry.List(t.Context(), scope)
	if err != nil {
		t.Fatalf("registry.List() error = %v", err)
	}
	return ToolsResponseRecord{Tools: core.ToolPayloadsFromViews(views), Count: len(views), Total: len(views)}
}

func expectedCLIToolSearchPayload(
	t *testing.T,
	registry *cliToolIntegrationRegistry,
	scope toolspkg.Scope,
	query toolspkg.SearchQuery,
) ToolsResponseRecord {
	t.Helper()

	views, err := registry.Search(t.Context(), scope, query)
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

	result, err := registry.Call(t.Context(), scope, request)
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

	toolset, err := registry.GetToolset(t.Context(), scope, id)
	if err != nil {
		t.Fatalf("registry.GetToolset() error = %v", err)
	}
	return ToolsetResponseRecord{Toolset: core.ToolsetPayloadFromView(toolset)}
}

func toolIntegrationDeps(
	t *testing.T,
	homePaths compozyconfig.HomePaths,
	cfg compozyconfig.Config,
) commandDeps {
	t.Helper()

	return commandDeps{
		loadConfig: func() (compozyconfig.Config, error) {
			return cfg, nil
		},
		resolveHome: func() (compozyconfig.HomePaths, error) {
			return homePaths, nil
		},
		ensureHome: func(compozyconfig.HomePaths) error { return nil },
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
		cliToolIntegrationView("compozy__operator_diag", false),
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
		Toolset:       toolspkg.Toolset{ID: toolspkg.ToolsetIDCatalog, Tools: []string{"compozy__skill_view"}},
		ExpandedTools: []toolspkg.ToolID{toolspkg.ToolIDSkillView},
	}
}

func limitCLIToolIntegrationViews(views []toolspkg.ToolView, limit int) []toolspkg.ToolView {
	if limit <= 0 || limit >= len(views) {
		return views
	}
	return views[:limit]
}

func newToolIntegrationProfiles(t *testing.T, home compozyconfig.HomePaths) *profilepkg.Manager {
	t.Helper()
	database, err := globaldb.OpenGlobalDB(t.Context(), home.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB(): %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			t.Errorf("Close(): %v", err)
		}
	})
	manager, err := profilepkg.NewManager(profilepkg.WithStore(database), profilepkg.WithHomePaths(home))
	if err != nil {
		t.Fatalf("NewManager(): %v", err)
	}
	return manager
}
