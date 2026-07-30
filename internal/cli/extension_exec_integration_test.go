//go:build integration

package cli

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/api/udsapi"
	"github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestExtensionExecInvokesCanonicalToolExactlyOnceAcrossOutputFormatsIntegration(t *testing.T) {
	t.Parallel()

	homePaths := testutil.NewTestHomePaths(t)
	cfg := testutil.ConfigWithDisabledNetwork(homePaths)
	cfg.Daemon.Socket = shortSocketPath(t)
	workspaceRoot := t.TempDir()
	registry := newCommandInvokeSpyRegistry()
	extensions := &commandIntegrationExtensionService{
		integrationExtensionService: &integrationExtensionService{},
		response:                    commandCLICatalog(),
	}
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
					Workspace:   workspacepkg.Workspace{ID: ref, RootDir: workspaceRoot, Name: ref},
					WorkspaceID: ref,
				}, nil
			},
		}),
		udsapi.WithExtensionService(extensions),
		udsapi.WithToolRegistry(registry),
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
	markExtensionDaemonRunning(&deps)
	formats := []struct {
		name   string
		argv   []string
		assert func(*testing.T, string)
	}{
		{
			name: "Should render human output",
			assert: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "Tool Invocation") || !strings.Contains(output, "ok") {
					t.Fatalf("human output = %q", output)
				}
			},
		},
		{
			name: "Should render JSON output",
			argv: []string{"-o", "json"},
			assert: func(t *testing.T, output string) {
				t.Helper()
				var response ToolInvokeResponseRecord
				decodeJSONOutput(t, output, &response)
				if response.ToolID != "ext__cmd_fixture__greet" || response.Status != "completed" {
					t.Fatalf("JSON response = %#v", response)
				}
			},
		},
		{
			name: "Should render JSONL output",
			argv: []string{"-o", "jsonl"},
			assert: func(t *testing.T, output string) {
				t.Helper()
				var response ToolInvokeResponseRecord
				if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &response); err != nil {
					t.Fatalf("json.Unmarshal(JSONL) error = %v; output=%q", err, output)
				}
				if response.ToolID != "ext__cmd_fixture__greet" {
					t.Fatalf("JSONL response = %#v", response)
				}
			},
		},
		{
			name: "Should render TOON output",
			argv: []string{"-o", "toon"},
			assert: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "tool_invocation") || !strings.Contains(output, "ext__cmd_fixture__greet") {
					t.Fatalf("TOON output = %q", output)
				}
			},
		},
	}
	for _, tt := range formats {
		t.Run(tt.name, func(t *testing.T) {
			registry.resetCalls()
			argv := []string{
				"extension", "exec", "cmd-fixture", "--cmd", "greet",
				"--name", "Ada", "--round", "2", "--workspace", "ws-1",
			}
			argv = append(argv, tt.argv...)
			stdout, _, err := executeRootCommand(t, deps, argv...)
			if err != nil {
				t.Fatalf("extension exec error = %v", err)
			}
			tt.assert(t, stdout)
			calls := registry.callsSnapshot()
			if len(calls) != 1 {
				t.Fatalf("registry.Call() calls = %d, want 1", len(calls))
			}
			call := calls[0]
			if call.ToolID != "ext__cmd_fixture__greet" || string(call.Input) != `{"name":"Ada","round":2}` {
				t.Fatalf("registry.Call() request = %#v", call)
			}
		})
	}

	t.Run("Should preserve unavailable backend reason codes", func(t *testing.T) {
		registry.resetCalls()
		registry.setCallError(toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			"ext__cmd_fixture__greet",
			"fixture backend is unavailable",
			toolspkg.ErrToolUnavailable,
			toolspkg.ReasonBackendUnhealthy,
		))
		t.Cleanup(func() { registry.setCallError(nil) })
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"extension", "exec", "cmd-fixture", "--cmd", "greet",
			"--name", "Ada", "--round", "2", "--workspace", "ws-1", "-o", "json",
		)
		if err == nil {
			t.Fatal("extension exec unavailable error = nil")
		}
		var response ToolErrorResponseRecord
		decodeJSONOutput(t, stdout, &response)
		if response.Error.Code != toolspkg.ErrorCodeUnavailable ||
			!slices.Contains(response.Error.ReasonCodes, toolspkg.ReasonBackendUnhealthy) {
			t.Fatalf("extension exec unavailable response = %#v", response.Error)
		}
		if calls := registry.callsSnapshot(); len(calls) != 1 {
			t.Fatalf("unavailable registry calls = %d, want 1", len(calls))
		}
	})
}

type commandIntegrationExtensionService struct {
	*integrationExtensionService
	response contract.ExtensionCommandsResponse
}

func (s *commandIntegrationExtensionService) Commands(
	context.Context,
	string,
) (contract.ExtensionCommandsResponse, error) {
	return s.response, nil
}

func (s *commandIntegrationExtensionService) CommandsScoped(
	_ context.Context,
	filter string,
	actor task.ActorContext,
) (contract.ExtensionCommandsResponse, error) {
	if filter != "cmd-fixture" || actor.Scope.WorkspaceID != "ws-1" {
		return contract.ExtensionCommandsResponse{}, nil
	}
	return s.response, nil
}

type commandInvokeSpyRegistry struct {
	*cliToolIntegrationRegistry
	callsMu sync.Mutex
	calls   []toolspkg.CallRequest
	callErr error
}

func newCommandInvokeSpyRegistry() *commandInvokeSpyRegistry {
	base := &cliToolIntegrationRegistry{views: []toolspkg.ToolView{
		cliToolIntegrationView("ext__cmd_fixture__greet", true),
	}}
	return &commandInvokeSpyRegistry{cliToolIntegrationRegistry: base}
}

func (r *commandInvokeSpyRegistry) Call(
	ctx context.Context,
	scope toolspkg.Scope,
	request toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	r.callsMu.Lock()
	r.calls = append(r.calls, request)
	callErr := r.callErr
	r.callsMu.Unlock()
	if callErr != nil {
		return toolspkg.ToolResult{}, callErr
	}
	return r.cliToolIntegrationRegistry.Call(ctx, scope, request)
}

func (r *commandInvokeSpyRegistry) resetCalls() {
	r.callsMu.Lock()
	defer r.callsMu.Unlock()
	r.calls = nil
}

func (r *commandInvokeSpyRegistry) callsSnapshot() []toolspkg.CallRequest {
	r.callsMu.Lock()
	defer r.callsMu.Unlock()
	return append([]toolspkg.CallRequest(nil), r.calls...)
}

func (r *commandInvokeSpyRegistry) setCallError(err error) {
	r.callsMu.Lock()
	defer r.callsMu.Unlock()
	r.callErr = err
}
