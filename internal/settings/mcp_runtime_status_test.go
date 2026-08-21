package settings

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestMCPServerItemsIncludeRuntimeStatusAndRemainIsolated(t *testing.T) {
	t.Run("Should attach daemon-backed runtime status to configured MCP servers", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, strings.Join([]string{
			"[[mcp_servers]]",
			"name = \"ready-docs\"",
			"command = \"docs-mcp\"",
			"",
			"[[mcp_servers]]",
			"name = \"linear\"",
			"transport = \"http\"",
			"url = \"https://mcp.linear.example/mcp\"",
			"",
			"[mcp_servers.auth]",
			"registration = \"pre_registered\"",
			"issuer_url = \"https://auth.linear.example\"",
			"client_id = \"compozy-desktop\"",
		}, "\n"))
		runtime := &fakeMCPRuntimeProvider{
			statuses: map[string]MCPServerRuntimeStatus{
				"ready-docs": {
					Configured:      true,
					Initialized:     true,
					State:           MCPServerRuntimeStateReady,
					Probe:           MCPServerProbeSucceeded,
					ToolCount:       2,
					ProtocolVersion: "2025-06-18",
				},
				"linear": {
					Configured: true,
					State:      MCPServerRuntimeStateAuthRequired,
					Probe:      MCPServerProbeSkipped,
					Reason:     "mcp_auth_required",
				},
			},
		}
		service := testService(t, homePaths, Dependencies{MCPRuntime: runtime})

		envelope, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionMCPServers})
		if err != nil {
			t.Fatalf("ListCollection(mcp) error = %v", err)
		}

		ready := findMCPItem(t, envelope.MCPServers, "ready-docs")
		if ready.RuntimeStatus == nil {
			t.Fatal("ready-docs RuntimeStatus = nil, want probe status")
		}
		if got, want := ready.RuntimeStatus.State, MCPServerRuntimeStateReady; got != want {
			t.Fatalf("ready-docs RuntimeStatus.State = %q, want %q", got, want)
		}
		if !ready.RuntimeStatus.Initialized || ready.RuntimeStatus.ToolCount != 2 {
			t.Fatalf("ready-docs RuntimeStatus = %#v, want initialized with 2 tools", ready.RuntimeStatus)
		}
		if got, want := ready.RuntimeStatus.ProtocolVersion, "2025-06-18"; got != want {
			t.Fatalf("ready-docs RuntimeStatus.ProtocolVersion = %q, want %q", got, want)
		}

		linear := findMCPItem(t, envelope.MCPServers, "linear")
		if linear.RuntimeStatus == nil {
			t.Fatal("linear RuntimeStatus = nil, want auth-blocked status")
		}
		if got, want := linear.RuntimeStatus.State, MCPServerRuntimeStateAuthRequired; got != want {
			t.Fatalf("linear RuntimeStatus.State = %q, want %q", got, want)
		}
		if got, want := linear.RuntimeStatus.Probe, MCPServerProbeSkipped; got != want {
			t.Fatalf("linear RuntimeStatus.Probe = %q, want %q", got, want)
		}
		if got, want := linear.RuntimeStatus.Reason, "mcp_auth_required"; got != want {
			t.Fatalf("linear RuntimeStatus.Reason = %q, want %q", got, want)
		}
	})

	t.Run("Should deep-clone every mutable MCP item field", func(t *testing.T) {
		t.Parallel()

		expiresAt := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
		updatedAt := expiresAt.Add(-time.Minute)
		source := MCPServerItem{
			Args:          []string{"serve"},
			EnvKeys:       []string{"PROJECT"},
			SecretEnvKeys: []string{"TOKEN"},
			Auth: compozyconfig.MCPAuthConfig{
				Scopes: []string{"read"},
			},
			AuthStatus: &mcpauth.Status{
				Scopes:    []string{"read"},
				ExpiresAt: &expiresAt,
				UpdatedAt: &updatedAt,
			},
			RuntimeStatus: &MCPServerRuntimeStatus{Reason: "ready"},
			SourceMetadata: SourceMetadata{
				ShadowedSources:  []SourceRef{{Kind: SourceKindGlobalConfig}},
				AvailableTargets: []WriteTargetKind{WriteTargetGlobalConfig},
			},
		}

		cloned := cloneMCPServerItem(source)
		cloned.Args[0] = "changed"
		cloned.EnvKeys[0] = "changed"
		cloned.SecretEnvKeys[0] = "changed"
		cloned.Auth.Scopes[0] = "changed"
		cloned.AuthStatus.Scopes[0] = "changed"
		*cloned.AuthStatus.ExpiresAt = time.Time{}
		*cloned.AuthStatus.UpdatedAt = time.Time{}
		cloned.RuntimeStatus.Reason = "changed"
		cloned.SourceMetadata.ShadowedSources[0].Kind = SourceKindGlobalMCPSidecar
		cloned.SourceMetadata.AvailableTargets[0] = WriteTargetGlobalMCPSidecar

		if source.Args[0] != "serve" || source.EnvKeys[0] != "PROJECT" ||
			source.SecretEnvKeys[0] != "TOKEN" || source.Auth.Scopes[0] != "read" {
			t.Fatalf("clone mutated source MCP config fields: %#v", source)
		}
		if source.AuthStatus.Scopes[0] != "read" || !source.AuthStatus.ExpiresAt.Equal(expiresAt) ||
			!source.AuthStatus.UpdatedAt.Equal(updatedAt) {
			t.Fatalf("clone mutated source auth status: %#v", source.AuthStatus)
		}
		if source.RuntimeStatus.Reason != "ready" ||
			source.SourceMetadata.ShadowedSources[0].Kind != SourceKindGlobalConfig ||
			source.SourceMetadata.AvailableTargets[0] != WriteTargetGlobalConfig {
			t.Fatalf("clone mutated source runtime or metadata: %#v", source)
		}
	})

	t.Run("Should pass the effective workspace source target to the runtime", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		workspaceRoot := t.TempDir()
		writeFile(t, filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.ConfigName), `
[[mcp_servers]]
name = "workspace-docs"
command = "docs-mcp"
`)
		runtime := &fakeMCPRuntimeProvider{
			statuses: map[string]MCPServerRuntimeStatus{
				"workspace-docs": {
					Configured: true,
					State:      MCPServerRuntimeStateDead,
					Probe:      MCPServerProbeSkipped,
					Reason:     "backend_dead",
				},
			},
			targets: make(map[string]mcpauth.Target),
		}
		service := testService(t, homePaths, Dependencies{
			MCPRuntime: runtime,
			WorkspaceResolver: fakeWorkspaceResolver{resolved: map[string]workspacepkg.ResolvedWorkspace{
				"ws-runtime": {
					Workspace: workspacepkg.Workspace{ID: "ws-runtime", RootDir: workspaceRoot},
				},
			}},
		})

		envelope, err := service.ListCollection(ctx, CollectionRequest{
			Collection:  CollectionMCPServers,
			Scope:       ScopeWorkspace,
			WorkspaceID: "ws-runtime",
		})
		if err != nil {
			t.Fatalf("ListCollection(workspace mcp) error = %v", err)
		}
		item := findMCPItem(t, envelope.MCPServers, "workspace-docs")
		if item.RuntimeStatus == nil || item.RuntimeStatus.State != MCPServerRuntimeStateDead {
			t.Fatalf("workspace runtime status = %#v, want dead", item.RuntimeStatus)
		}
		if got, want := runtime.targets["workspace-docs"], (mcpauth.Target{
			Scope:       mcpauth.ScopeWorkspace,
			WorkspaceID: "ws-runtime",
			ServerName:  "workspace-docs",
		}); got != want {
			t.Fatalf("runtime target = %#v, want %#v", got, want)
		}
	})
}

func TestMCPServerCollectionBoundsRuntimeProbes(t *testing.T) {
	t.Parallel()

	t.Run("Should probe no more than four configured MCP servers concurrently", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, strings.Join([]string{
			"[[mcp_servers]]", "name = \"alpha\"", "command = \"alpha-mcp\"", "",
			"[[mcp_servers]]", "name = \"bravo\"", "command = \"bravo-mcp\"", "",
			"[[mcp_servers]]", "name = \"charlie\"", "command = \"charlie-mcp\"", "",
			"[[mcp_servers]]", "name = \"delta\"", "command = \"delta-mcp\"", "",
			"[[mcp_servers]]", "name = \"echo\"", "command = \"echo-mcp\"",
		}, "\n"))
		runtime := newBlockingMCPRuntimeProvider()
		service := testService(t, homePaths, Dependencies{MCPRuntime: runtime})
		type listResult struct {
			envelope CollectionEnvelope
			err      error
		}
		results := make(chan listResult, 1)
		go func() {
			envelope, err := service.ListCollection(
				context.Background(),
				CollectionRequest{Collection: CollectionMCPServers},
			)
			results <- listResult{envelope: envelope, err: err}
		}()

		for range maxConcurrentMCPRuntimeProbes {
			select {
			case <-runtime.entered:
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for bounded MCP runtime probes")
			}
		}
		if got := runtime.maxInFlight(); got != maxConcurrentMCPRuntimeProbes {
			t.Fatalf("max concurrent MCP runtime probes = %d, want %d", got, maxConcurrentMCPRuntimeProbes)
		}
		select {
		case name := <-runtime.entered:
			t.Fatalf("MCP runtime probe %q exceeded the concurrency bound", name)
		default:
		}
		close(runtime.release)

		select {
		case result := <-results:
			if result.err != nil {
				t.Fatalf("ListCollection(mcp) error = %v", result.err)
			}
			if got, want := mcpItemNames(
				result.envelope.MCPServers,
			), []string{
				"alpha",
				"bravo",
				"charlie",
				"delta",
				"echo",
			}; !slices.Equal(
				got,
				want,
			) {
				t.Fatalf("MCP item order = %#v, want %#v", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for MCP runtime probe collection")
		}
	})

	t.Run("Should return no partial collection after a structural probe error", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, strings.Join([]string{
			"[[mcp_servers]]", "name = \"alpha\"", "command = \"alpha-mcp\"", "",
			"[[mcp_servers]]", "name = \"bravo\"", "command = \"bravo-mcp\"", "",
			"[[mcp_servers]]", "name = \"charlie\"", "command = \"charlie-mcp\"", "",
			"[[mcp_servers]]", "name = \"broken\"", "command = \"broken-mcp\"", "",
			"[[mcp_servers]]", "name = \"delta\"", "command = \"delta-mcp\"",
		}, "\n"))
		runtime := newBlockingMCPRuntimeProvider()
		runtime.failName = "broken"
		service := testService(t, homePaths, Dependencies{MCPRuntime: runtime})
		type listResult struct {
			envelope CollectionEnvelope
			err      error
		}
		results := make(chan listResult, 1)
		go func() {
			envelope, err := service.ListCollection(
				context.Background(),
				CollectionRequest{Collection: CollectionMCPServers},
			)
			results <- listResult{envelope: envelope, err: err}
		}()

		for range maxConcurrentMCPRuntimeProbes {
			select {
			case <-runtime.entered:
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for structural MCP runtime probes")
			}
		}
		close(runtime.release)

		select {
		case result := <-results:
			if !errors.Is(result.err, errBlockingMCPRuntime) {
				t.Fatalf("ListCollection(mcp) error = %v, want structural probe error", result.err)
			}
			if result.envelope.Collection != "" || len(result.envelope.MCPServers) != 0 {
				t.Fatalf("ListCollection(mcp) result = %#v, want no partial collection", result.envelope)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for structural MCP runtime probe cleanup")
		}
		if got, want := runtime.finishedCount(), runtime.startedCount(); got != want {
			t.Fatalf("completed MCP runtime probes = %d, want %d after cancellation join", got, want)
		}
	})
}

func TestMCPAuthOperationsResolveExactWorkspaceSidecarTarget(t *testing.T) {
	t.Parallel()
	t.Run("Should resolve the exact workspace sidecar target for every auth operation", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		workspaceRoot := t.TempDir()
		writeFile(t, homePaths.ConfigFile, strings.Join([]string{
			"[[mcp_servers]]",
			"name = \"linear\"",
			"transport = \"http\"",
			"url = \"https://global.linear.example/mcp\"",
			"",
			"[mcp_servers.auth]",
			"registration = \"pre_registered\"",
			"issuer_url = \"https://global.linear.example\"",
			"client_id = \"global-client\"",
		}, "\n"))
		writeFile(t, filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.MCPJSONName), `{
  "mcpServers": {
    "linear": {
	  "transport": "http",
      "url": "https://workspace.linear.example/mcp",
      "auth": {
        "registration": "pre_registered",
        "issuer_url": "https://workspace.linear.example",
        "client_id": "workspace-client"
      }
    }
  }
}`)
		runtime := &recordingMCPAuthRuntime{}
		service := testService(t, homePaths, Dependencies{
			WorkspaceResolver: fakeWorkspaceResolver{
				resolved: map[string]workspacepkg.ResolvedWorkspace{
					"workspace-a": {
						Workspace: workspacepkg.Workspace{ID: "workspace-a", RootDir: workspaceRoot},
					},
				},
			},
			MCPAuth: runtime,
		})
		workspaceTarget := MCPAuthTargetRequest{
			Scope: ScopeWorkspace, WorkspaceID: "workspace-a", Name: "linear",
		}
		authStatus, err := service.GetMCPAuthStatus(ctx, workspaceTarget)
		if err != nil {
			t.Fatalf("GetMCPAuthStatus(workspace sidecar) error = %v", err)
		}
		if authStatus.Scope != mcpauth.ScopeWorkspace || authStatus.WorkspaceID != "workspace-a" ||
			authStatus.ServerName != "linear" || authStatus.Status != mcpauth.StatusAuthenticated {
			t.Fatalf("GetMCPAuthStatus(workspace sidecar) = %#v", authStatus)
		}
		if runtime.statusTarget != (mcpauth.Target{
			Scope: mcpauth.ScopeWorkspace, WorkspaceID: "workspace-a", ServerName: "linear",
		}) ||
			runtime.statusServer.URL != "https://workspace.linear.example/mcp" ||
			runtime.statusServer.Auth.ClientID != "workspace-client" {
			t.Fatalf("workspace status resolution = target:%#v server:%#v", runtime.statusTarget, runtime.statusServer)
		}
		begin, err := service.BeginMCPAuth(ctx, MCPAuthBeginRequest{
			MCPAuthTargetRequest: workspaceTarget,
			CallbackURL:          "http://127.0.0.1:2123/api/mcp/oauth/callback",
		})
		if err != nil {
			t.Fatalf("BeginMCPAuth(workspace sidecar) error = %v", err)
		}
		if begin.AuthorizationURL != "https://workspace.linear.example/authorize?state=public" {
			t.Fatalf("BeginMCPAuth() = %#v", begin)
		}
		if runtime.beginTarget.Scope != mcpauth.ScopeWorkspace ||
			runtime.beginTarget.WorkspaceID != "workspace-a" ||
			runtime.beginServer.URL != "https://workspace.linear.example/mcp" ||
			runtime.beginServer.Auth.ClientID != "workspace-client" {
			t.Fatalf("workspace begin resolution = target:%#v server:%#v", runtime.beginTarget, runtime.beginServer)
		}

		status, err := service.ExchangeMCPAuth(ctx, MCPAuthExchangeRequest{
			MCPAuthTargetRequest: workspaceTarget,
			RedirectURL:          "https://callback.example/?code=opaque&state=public",
		})
		if err != nil {
			t.Fatalf("ExchangeMCPAuth(workspace) error = %v", err)
		}
		if status.WorkspaceID != "workspace-a" || !status.TokenPresent {
			t.Fatalf("ExchangeMCPAuth(workspace) status = %#v", status)
		}
		if runtime.exchangeInput.RedirectURL != "https://callback.example/?code=opaque&state=public" {
			t.Fatalf("exchange input = %#v", runtime.exchangeInput)
		}
		if runtime.exchangeTarget != runtime.beginTarget ||
			runtime.exchangeServer.URL != "https://workspace.linear.example/mcp" ||
			runtime.exchangeServer.Auth.ClientID != "workspace-client" {
			t.Fatalf(
				"workspace exchange resolution = target:%#v server:%#v",
				runtime.exchangeTarget,
				runtime.exchangeServer,
			)
		}

		if _, err := service.LogoutMCPAuth(ctx, workspaceTarget); err != nil {
			t.Fatalf("LogoutMCPAuth(workspace sidecar) error = %v", err)
		}
		if runtime.logoutTarget != runtime.beginTarget || runtime.logoutServer.URL != runtime.beginServer.URL {
			t.Fatalf("logout resolution = target:%#v server:%#v", runtime.logoutTarget, runtime.logoutServer)
		}

		if _, err := service.CompleteMCPAuthCallback(
			ctx,
			"http://127.0.0.1:2123/api/mcp/oauth/callback?code=opaque&state=public",
		); err != nil {
			t.Fatalf("CompleteMCPAuthCallback() error = %v", err)
		}
		if !strings.Contains(runtime.callbackURL, "code=opaque") {
			t.Fatalf("callback URL = %q", runtime.callbackURL)
		}
		if runtime.callbackTarget != runtime.beginTarget ||
			runtime.callbackServer.URL != "https://workspace.linear.example/mcp" ||
			runtime.callbackServer.Auth.ClientID != "workspace-client" {
			t.Fatalf(
				"workspace callback resolution = target:%#v server:%#v",
				runtime.callbackTarget,
				runtime.callbackServer,
			)
		}
	})

	t.Run("Should resolve the exact workspace-profile sidecar target", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		workspaceRoot := t.TempDir()
		writeFile(
			t,
			filepath.Join(
				workspaceRoot,
				compozyconfig.DirName,
				compozyconfig.ProfilesDirName,
				"marketing",
				compozyconfig.MCPJSONName,
			),
			`{"mcpServers":{"linear":{"transport":"http","url":"https://marketing.linear.example/mcp","auth":{"registration":"pre_registered","issuer_url":"https://marketing.linear.example","client_id":"marketing-client"}}}}`,
		)
		runtime := &recordingMCPAuthRuntime{}
		service := testService(t, homePaths, Dependencies{
			WorkspaceResolver: fakeWorkspaceResolver{resolved: map[string]workspacepkg.ResolvedWorkspace{
				"workspace-a": {
					Workspace: workspacepkg.Workspace{ID: "workspace-a", RootDir: workspaceRoot},
				},
			}},
			MCPAuth: runtime,
		})
		request := MCPAuthTargetRequest{
			Scope: ScopeProfile, WorkspaceID: "workspace-a", ProfileName: "marketing", Name: "linear",
		}
		if _, err := service.GetMCPAuthStatus(ctx, request); err != nil {
			t.Fatalf("GetMCPAuthStatus(workspace profile) error = %v", err)
		}
		want := mcpauth.Target{
			Scope: mcpauth.ScopeWorkspaceProfile, WorkspaceID: "workspace-a@pf:marketing", ServerName: "linear",
		}
		if runtime.statusTarget != want || runtime.statusServer.URL != "https://marketing.linear.example/mcp" {
			t.Fatalf(
				"workspace-profile status resolution = target:%#v server:%#v",
				runtime.statusTarget,
				runtime.statusServer,
			)
		}
		if _, err := service.BeginMCPAuth(ctx, MCPAuthBeginRequest{
			MCPAuthTargetRequest: request,
			CallbackURL:          "http://127.0.0.1:2123/api/mcp/oauth/callback",
		}); err != nil {
			t.Fatalf("BeginMCPAuth(workspace profile) error = %v", err)
		}
		if runtime.beginTarget != want {
			t.Fatalf("workspace-profile begin target = %#v, want %#v", runtime.beginTarget, want)
		}
	})
}

type recordingMCPAuthRuntime struct {
	statusTarget    mcpauth.Target
	statusServer    compozyconfig.MCPServer
	beginTarget     mcpauth.Target
	beginServer     compozyconfig.MCPServer
	exchangeTarget  mcpauth.Target
	exchangeServer  compozyconfig.MCPServer
	exchangeInput   mcpauth.ExchangeInput
	logoutTarget    mcpauth.Target
	logoutServer    compozyconfig.MCPServer
	callbackTarget  mcpauth.Target
	callbackServer  compozyconfig.MCPServer
	callbackURL     string
	operations      []recordingMCPAuthOperation
	invalidateCalls int
	deleteCalls     int
	invalidateErrs  map[int]error
	deleteStateErrs map[int]error
}

type recordingMCPAuthOperation struct {
	kind   string
	target mcpauth.Target
}

const (
	recordingMCPAuthInvalidate  = "invalidate"
	recordingMCPAuthDeleteState = "delete_state"
)

func (r *recordingMCPAuthRuntime) MCPAuthStatus(
	_ context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
) (mcpauth.Status, error) {
	r.statusTarget = target
	r.statusServer = server
	return confirmedMCPAuthRuntimeStatus(target), nil
}

func (r *recordingMCPAuthRuntime) MCPAuthBegin(
	_ context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
	callbackURL string,
) (mcpauth.BeginResult, error) {
	r.beginTarget = target
	r.beginServer = server
	return mcpauth.BeginResult{
		AuthorizationURL: "https://workspace.linear.example/authorize?state=public",
		State:            "public",
		ExpiresAt:        time.Date(2026, 7, 13, 16, 5, 0, 0, time.UTC),
		CallbackURL:      callbackURL,
		ManualSupported:  true,
	}, nil
}

func (r *recordingMCPAuthRuntime) MCPAuthBeginStepUp(
	ctx context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
	callbackURL string,
	_ []string,
	approved bool,
) (mcpauth.BeginResult, error) {
	if !approved {
		return mcpauth.BeginResult{}, errors.New("scope escalation requires approval")
	}
	return r.MCPAuthBegin(ctx, target, server, callbackURL)
}

func (r *recordingMCPAuthRuntime) MCPAuthExchange(
	_ context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
	input mcpauth.ExchangeInput,
) (mcpauth.Status, error) {
	r.exchangeTarget = target
	r.exchangeServer = server
	r.exchangeInput = input
	return confirmedMCPAuthRuntimeStatus(target), nil
}

func (r *recordingMCPAuthRuntime) MCPAuthCallbackTarget(string) (mcpauth.Target, error) {
	return mcpauth.Target{
		Scope: mcpauth.ScopeWorkspace, WorkspaceID: "workspace-a", ServerName: "linear",
	}, nil
}

func (r *recordingMCPAuthRuntime) MCPAuthCompleteCallback(
	_ context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
	callbackURL string,
) (mcpauth.Status, error) {
	r.callbackTarget = target
	r.callbackServer = server
	r.callbackURL = callbackURL
	return confirmedMCPAuthRuntimeStatus(target), nil
}

func (r *recordingMCPAuthRuntime) MCPAuthInvalidate(target mcpauth.Target) error {
	r.operations = append(r.operations, recordingMCPAuthOperation{kind: recordingMCPAuthInvalidate, target: target})
	r.invalidateCalls++
	if err := r.invalidateErrs[r.invalidateCalls]; err != nil {
		return err
	}
	return nil
}

func (r *recordingMCPAuthRuntime) MCPAuthDeleteState(_ context.Context, target mcpauth.Target) error {
	r.operations = append(r.operations, recordingMCPAuthOperation{kind: recordingMCPAuthDeleteState, target: target})
	r.deleteCalls++
	if err := r.deleteStateErrs[r.deleteCalls]; err != nil {
		return err
	}
	return nil
}

func (r *recordingMCPAuthRuntime) MCPAuthLogout(
	_ context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
) (mcpauth.Status, error) {
	r.logoutTarget = target
	r.logoutServer = server
	status := confirmedMCPAuthRuntimeStatus(target)
	status.Status = mcpauth.StatusNeedsLogin
	status.TokenPresent = false
	return status, nil
}

func confirmedMCPAuthRuntimeStatus(target mcpauth.Target) mcpauth.Status {
	return mcpauth.Status{
		ServerName:   target.ServerName,
		Scope:        target.Scope,
		WorkspaceID:  target.WorkspaceID,
		Status:       mcpauth.StatusAuthenticated,
		TokenPresent: true,
	}
}

type fakeMCPRuntimeProvider struct {
	statuses map[string]MCPServerRuntimeStatus
	targets  map[string]mcpauth.Target
}

var errBlockingMCPRuntime = errors.New("blocking MCP runtime failure")

type blockingMCPRuntimeProvider struct {
	entered  chan string
	release  chan struct{}
	failName string

	mu       sync.Mutex
	inFlight int
	max      int
	started  int
	finished int
}

func newBlockingMCPRuntimeProvider() *blockingMCPRuntimeProvider {
	return &blockingMCPRuntimeProvider{
		entered: make(chan string, maxConcurrentMCPRuntimeProbes+1),
		release: make(chan struct{}),
	}
}

func (f *blockingMCPRuntimeProvider) MCPServerRuntimeStatus(
	ctx context.Context,
	_ mcpauth.Target,
	server compozyconfig.MCPServer,
) (MCPServerRuntimeStatus, error) {
	f.mu.Lock()
	f.inFlight++
	f.started++
	if f.inFlight > f.max {
		f.max = f.inFlight
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.inFlight--
		f.finished++
		f.mu.Unlock()
	}()

	name := strings.TrimSpace(server.Name)
	select {
	case f.entered <- name:
	case <-ctx.Done():
		return MCPServerRuntimeStatus{}, ctx.Err()
	}
	select {
	case <-f.release:
		if name == f.failName {
			return MCPServerRuntimeStatus{}, errBlockingMCPRuntime
		}
		return MCPServerRuntimeStatus{
			Configured: true, Initialized: true, State: MCPServerRuntimeStateReady, Probe: MCPServerProbeSucceeded,
		}, nil
	case <-ctx.Done():
		return MCPServerRuntimeStatus{}, ctx.Err()
	}
}

func (f *blockingMCPRuntimeProvider) maxInFlight() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.max
}

func (f *blockingMCPRuntimeProvider) finishedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.finished
}

func (f *blockingMCPRuntimeProvider) startedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.started
}

func mcpItemNames(items []MCPServerItem) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

func (f *fakeMCPRuntimeProvider) MCPServerRuntimeStatus(
	_ context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
) (MCPServerRuntimeStatus, error) {
	if f.targets != nil {
		f.targets[strings.TrimSpace(server.Name)] = target
	}
	if status, ok := f.statuses[strings.TrimSpace(server.Name)]; ok {
		return status, nil
	}
	return MCPServerRuntimeStatus{
		Configured: true,
		State:      MCPServerRuntimeStateRuntimeUnavailable,
		Probe:      MCPServerProbeFailed,
		Reason:     "test_missing_runtime_status",
	}, nil
}
