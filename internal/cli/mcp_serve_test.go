package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	compozydaemon "github.com/compozy/compozy/internal/daemon"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func TestMCPServeCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should dispatch serve when the workspace override is omitted", func(t *testing.T) {
		t.Parallel()

		called := false
		cmd := newMCPServeCommand(commandDeps{runMCPServe: func(context.Context, mcpServeOptions) error {
			called = true
			return nil
		}})
		cmd.SetArgs(nil)
		if err := cmd.ExecuteContext(t.Context()); err != nil {
			t.Fatalf("ExecuteContext() error = %v", err)
		}
		if !called {
			t.Fatal("runMCPServe was not called")
		}
	})

	t.Run("Should forward transport options and command streams", func(t *testing.T) {
		t.Parallel()

		var captured mcpServeOptions
		deps := commandDeps{runMCPServe: func(_ context.Context, opts mcpServeOptions) error {
			captured = opts
			return nil
		}}
		cmd := newMCPServeCommand(deps)
		stdin := strings.NewReader("input")
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.SetIn(stdin)
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{
			"--workspace", "alpha",
			"--transport", "http",
			"--listen", "127.0.0.1:3131",
			"--token-env", "CUSTOM_TOKEN",
		})
		if err := cmd.ExecuteContext(t.Context()); err != nil {
			t.Fatalf("ExecuteContext() error = %v", err)
		}
		if captured.Workspace != "alpha" || captured.Transport != "http" ||
			captured.Listen != "127.0.0.1:3131" || captured.TokenEnv != "CUSTOM_TOKEN" {
			t.Fatalf("captured options = %#v, want command flags", captured)
		}
		if captured.Stdin != stdin || captured.Stdout != stdout || captured.Stderr != stderr {
			t.Fatalf("captured streams = %#v, want command streams", captured)
		}
		if cmd.Flags().Lookup("token") != nil {
			t.Fatal("serve command exposes a token value flag")
		}
	})

	t.Run("Should validate HTTP listener before daemon discovery", func(t *testing.T) {
		t.Parallel()

		deps := commandDeps{}.withDefaults()
		err := runMCPServe(t.Context(), deps, mcpServeOptions{
			Workspace: "alpha",
			Transport: mcpServeTransportHTTP,
		})
		if err == nil || !strings.Contains(err.Error(), "--listen is required") {
			t.Fatalf("runMCPServe() error = %v, want required listener", err)
		}
	})

	t.Run("Should reject listen on stdio before daemon discovery", func(t *testing.T) {
		t.Parallel()

		deps := commandDeps{}.withDefaults()
		err := runMCPServe(t.Context(), deps, mcpServeOptions{
			Workspace: "alpha",
			Transport: mcpServeTransportStdio,
			Listen:    "127.0.0.1:3131",
		})
		if err == nil || !strings.Contains(err.Error(), "only valid") {
			t.Fatalf("runMCPServe() error = %v, want stdio listener rejection", err)
		}
	})

	t.Run("Should pass the cwd-resolved workspace through the real serve flow", func(t *testing.T) {
		t.Parallel()

		var workspaceRefMu sync.Mutex
		var workspaceRef string
		client := &mcpServeRelayClient{stubClient: &stubClient{
			getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
				workspaceRefMu.Lock()
				workspaceRef = ref
				workspaceRefMu.Unlock()
				return WorkspaceDetailRecord{
					Workspace: WorkspaceRecord{ID: "ws-canonical", RootDir: "/workspace/project"},
				}, nil
			},
		}}
		deps := newTestDeps(t, client)
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
			return compozydaemon.Info{PID: 42, StartedAt: fixedTestNow}, nil
		}
		deps.processAlive = func(int) bool { return true }
		requests := strings.Join([]string{
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"` +
				mcpgo.LATEST_PROTOCOL_VERSION +
				`","capabilities":{},"clientInfo":{"name":"cli-test","version":"1.0.0"}}}`,
			`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"compozy_host__sessions__list","arguments":{}}}`,
		}, "\n") + "\n"

		err := runMCPServe(t.Context(), deps, mcpServeOptions{
			Transport: mcpServeTransportStdio,
			Stdin:     strings.NewReader(requests),
			Stdout:    &bytes.Buffer{},
			Stderr:    &bytes.Buffer{},
		})
		if err != nil {
			t.Fatalf("runMCPServe() error = %v", err)
		}
		workspaceRefMu.Lock()
		gotWorkspaceRef := workspaceRef
		workspaceRefMu.Unlock()
		if gotWorkspaceRef != "/workspace/project" {
			t.Fatalf("GetWorkspace() ref = %q, want cwd", gotWorkspaceRef)
		}
		if got := client.invokedWorkspace(); got != "ws-canonical" {
			t.Fatalf("InvokeHostAPI() workspace = %q, want ws-canonical", got)
		}
	})
}

type mcpServeRelayClient struct {
	*stubClient

	mu        sync.Mutex
	workspace string
}

func (c *mcpServeRelayClient) InvokeHostAPI(
	_ context.Context,
	_ string,
	workspace string,
	_ string,
	_ json.RawMessage,
) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workspace = workspace
	return json.RawMessage(`{"sessions":[]}`), nil
}

func (c *mcpServeRelayClient) CloseHostAPISession(context.Context, string) error {
	return nil
}

func (c *mcpServeRelayClient) invokedWorkspace() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.workspace
}
