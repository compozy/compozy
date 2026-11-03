package mcpbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	helperEnv        = "COMPOZY_MCPBRIDGE_HELPER"
	helperPIDFileEnv = "COMPOZY_MCPBRIDGE_HELPER_CHILD_PID_FILE"
	helperFailEnv    = "COMPOZY_MCPBRIDGE_HELPER_FAIL"
)

func TestBridge(t *testing.T) {
	t.Parallel()

	t.Run("Should relay stdio tools to an authorized loopback HTTP client", func(t *testing.T) {
		t.Parallel()
		bridge := startHelperBridge(t)

		endpoint, err := url.Parse(bridge.URL())
		if err != nil {
			t.Fatalf("parse bridge URL: %v", err)
		}
		if endpoint.Hostname() != "127.0.0.1" || endpoint.Scheme != "http" {
			t.Fatalf("bridge URL = %q, want loopback http", bridge.URL())
		}
		session := connectHTTPClient(t, bridge, bridge.AuthorizationHeader())
		if got := toolNames(t, session); !slices.Equal(got, []string{"echo", "grow", "slow"}) {
			t.Fatalf("tools = %#v, want the stdio server tools", got)
		}
		result, err := session.CallTool(testutil.Context(t), &sdkmcp.CallToolParams{
			Name: "echo", Arguments: map[string]any{"message": "via-http"},
		})
		if err != nil {
			t.Fatalf("CallTool(echo) error = %v", err)
		}
		if got := resultText(result); got != "echo: via-http" {
			t.Fatalf("CallTool(echo) text = %q, want the stdio server result", got)
		}
	})

	t.Run("Should reject requests without the bridge bearer token", func(t *testing.T) {
		t.Parallel()
		bridge := startHelperBridge(t)
		for _, header := range []string{"", "Bearer wrong", strings.TrimPrefix(bridge.AuthorizationHeader(), "Bearer ")} {
			request, err := http.NewRequestWithContext(
				testutil.Context(t), http.MethodPost, bridge.URL(),
				strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`),
			)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			if header != "" {
				request.Header.Set("Authorization", header)
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatalf("POST bridge: %v", err)
			}
			_ = response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status with Authorization %q = %d, want 401", header, response.StatusCode)
			}
		}
	})

	t.Run("Should notify the agent when the stdio server changes its tools", func(t *testing.T) {
		t.Parallel()
		bridge := startHelperBridge(t)
		changed := make(chan struct{}, 4)
		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "agent", Version: "1.0.0"}, &sdkmcp.ClientOptions{
			ToolListChangedHandler: func(context.Context, *sdkmcp.ToolListChangedRequest) {
				changed <- struct{}{}
			},
		})
		session, err := client.Connect(testutil.Context(t), &sdkmcp.StreamableClientTransport{
			Endpoint:   bridge.URL(),
			HTTPClient: &http.Client{Transport: headerTransport{authorization: bridge.AuthorizationHeader()}},
		}, nil)
		if err != nil {
			t.Fatalf("connect HTTP client: %v", err)
		}
		t.Cleanup(func() { _ = session.Close() })
		if _, err := session.CallTool(testutil.Context(t), &sdkmcp.CallToolParams{
			Name: "grow", Arguments: map[string]any{},
		}); err != nil {
			t.Fatalf("CallTool(grow) error = %v", err)
		}
		select {
		case <-changed:
		case <-time.After(5 * time.Second):
			t.Fatal("agent received no tools/list_changed after the stdio server added a tool")
		}
		if got := toolNames(t, session); !slices.Contains(got, "late") {
			t.Fatalf("tools = %#v, want the tool added after list_changed", got)
		}
	})

	t.Run("Should relay child progress to the agent under its own token", func(t *testing.T) {
		t.Parallel()
		bridge := startHelperBridge(t)
		progress := make(chan *sdkmcp.ProgressNotificationParams, 4)
		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "agent", Version: "1.0.0"}, &sdkmcp.ClientOptions{
			ProgressNotificationHandler: func(_ context.Context, req *sdkmcp.ProgressNotificationClientRequest) {
				progress <- req.Params
			},
		})
		session, err := client.Connect(testutil.Context(t), &sdkmcp.StreamableClientTransport{
			Endpoint:   bridge.URL(),
			HTTPClient: &http.Client{Transport: headerTransport{authorization: bridge.AuthorizationHeader()}},
		}, nil)
		if err != nil {
			t.Fatalf("connect HTTP client: %v", err)
		}
		t.Cleanup(func() { _ = session.Close() })
		params := &sdkmcp.CallToolParams{Name: "slow", Arguments: map[string]any{}}
		params.SetProgressToken("agent-token")
		if _, err := session.CallTool(testutil.Context(t), params); err != nil {
			t.Fatalf("CallTool(slow) error = %v", err)
		}
		select {
		case got := <-progress:
			if got.ProgressToken != "agent-token" || got.Message != "working" {
				t.Fatalf("progress = %#v, want the agent token and child message", got)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("no progress notification reached the agent")
		}
	})

	t.Run("Should stop serving and release the child on close", func(t *testing.T) {
		t.Parallel()
		bridge := startHelperBridge(t)
		if err := bridge.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if err := bridge.Close(); err != nil {
			t.Fatalf("second Close() error = %v, want idempotent", err)
		}
		request, err := http.NewRequestWithContext(testutil.Context(t), http.MethodPost, bridge.URL(), http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", bridge.AuthorizationHeader())
		if response, err := http.DefaultClient.Do(request); err == nil {
			_ = response.Body.Close()
			t.Fatalf("POST after Close status = %d, want connection failure", response.StatusCode)
		}
	})

	t.Run("Should stop processes the stdio server spawned when it closes", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("process-group signaling is covered by the Windows job object")
		}
		pidFile := filepath.Join(t.TempDir(), "child.pid")
		bridge := startBridge(t, Spec{
			Name: "helper", Command: os.Args[0],
			Args: []string{"-test.run=^TestBridgeStdioHelperProcess$"},
			Env:  append(os.Environ(), helperEnv+"=1", helperPIDFileEnv+"="+pidFile),
			Dir:  t.TempDir(),
		})
		raw, err := os.ReadFile(pidFile)
		if err != nil {
			t.Fatalf("read spawned child pid: %v", err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err != nil {
			t.Fatal(err)
		}
		if err := bridge.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for processAlive(pid) {
			if time.Now().After(deadline) {
				t.Fatalf("spawned child %d survived bridge Close", pid)
			}
			time.Sleep(20 * time.Millisecond)
		}
	})

	t.Run("Should resolve a bare command against the child PATH", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("PATHEXT resolution is covered by the subprocess package")
		}
		binDir := t.TempDir()
		if err := os.Symlink(os.Args[0], filepath.Join(binDir, "compozy-bridge-helper")); err != nil {
			t.Fatal(err)
		}
		bridge := startBridge(t, Spec{
			Name: "helper", Command: "compozy-bridge-helper",
			Args: []string{"-test.run=^TestBridgeStdioHelperProcess$"},
			Env:  append(os.Environ(), "PATH="+binDir, helperEnv+"=1"),
			Dir:  t.TempDir(),
		})
		session := connectHTTPClient(t, bridge, bridge.AuthorizationHeader())
		if got := toolNames(t, session); !slices.Contains(got, "echo") {
			t.Fatalf("tools = %#v, want the helper started from the child PATH", got)
		}
	})

	t.Run("Should report the stdio server's stderr when it fails to start", func(t *testing.T) {
		t.Parallel()
		_, err := Start(testutil.Context(t), Spec{
			Name: "helper", Command: os.Args[0],
			Args: []string{"-test.run=^TestBridgeStdioHelperProcess$"},
			Env:  append(os.Environ(), helperEnv+"=1", helperFailEnv+"=1"),
			Dir:  t.TempDir(),
		}, nil)
		if err == nil || !strings.Contains(err.Error(), "fixture refused to start") {
			t.Fatalf("Start() error = %v, want the server's stderr", err)
		}
	})

	t.Run("Should fail startup when the stdio server cannot start", func(t *testing.T) {
		t.Parallel()
		_, err := Start(testutil.Context(t), Spec{
			Name: "missing", Command: "/nonexistent/compozy-mcp-server",
		}, nil)
		if err == nil || !strings.Contains(err.Error(), `stdio server "missing"`) {
			t.Fatalf("Start() error = %v, want stdio server startup failure", err)
		}
	})
}

// TestBridgeStdioHelperProcess is the stdio MCP server child used above.
func TestBridgeStdioHelperProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "1" {
		t.Skip("helper process only")
	}
	if os.Getenv(helperFailEnv) == "1" {
		fmt.Fprintln(os.Stderr, "fixture refused to start")
		os.Exit(3)
	}
	if pidFile := os.Getenv(helperPIDFileEnv); pidFile != "" {
		child := exec.CommandContext(context.Background(), "sleep", "300")
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		if err := os.WriteFile(pidFile, []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
			os.Exit(2)
		}
	}
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "helper", Version: "1.0.0"}, &sdkmcp.ServerOptions{
		Capabilities: &sdkmcp.ServerCapabilities{Tools: &sdkmcp.ToolCapabilities{ListChanged: true}},
	})
	server.AddTool(&sdkmcp.Tool{
		Name:        "echo",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"}}}`),
	}, func(_ context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		var input struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
			return nil, err
		}
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "echo: " + input.Message}},
		}, nil
	})
	server.AddTool(&sdkmcp.Tool{
		Name:        "slow",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		if err := req.Session.NotifyProgress(ctx, &sdkmcp.ProgressNotificationParams{
			ProgressToken: req.Params.GetProgressToken(), Message: "working", Progress: 1,
		}); err != nil {
			return nil, err
		}
		// Real keepalives precede the result by seconds; a final notification racing
		// the result is dropped once the call has finished.
		time.Sleep(300 * time.Millisecond)
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "done"}}}, nil
	})
	server.AddTool(&sdkmcp.Tool{
		Name:        "grow",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		server.AddTool(&sdkmcp.Tool{Name: "late", InputSchema: json.RawMessage(`{"type":"object"}`)},
			func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				return &sdkmcp.CallToolResult{}, nil
			})
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "grown"}}}, nil
	})
	if err := server.Run(
		context.Background(),
		&sdkmcp.StdioTransport{},
	); err != nil &&
		!errors.Is(err, context.Canceled) {
		os.Exit(1)
	}
	os.Exit(0)
}

func startHelperBridge(t *testing.T) *Bridge {
	t.Helper()
	return startBridge(t, Spec{
		Name:    "helper",
		Command: os.Args[0],
		Args:    []string{"-test.run=^TestBridgeStdioHelperProcess$"},
		Env:     append(os.Environ(), helperEnv+"=1"),
		Dir:     t.TempDir(),
	})
}

func startBridge(t *testing.T, spec Spec) *Bridge {
	t.Helper()
	bridge, err := Start(testutil.Context(t), spec, nil)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := bridge.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return bridge
}

func connectHTTPClient(t *testing.T, bridge *Bridge, authorization string) *sdkmcp.ClientSession {
	t.Helper()
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "agent", Version: "1.0.0"}, nil)
	session, err := client.Connect(testutil.Context(t), &sdkmcp.StreamableClientTransport{
		Endpoint:   bridge.URL(),
		HTTPClient: &http.Client{Transport: headerTransport{authorization: authorization}},
	}, nil)
	if err != nil {
		t.Fatalf("connect HTTP client: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func toolNames(t *testing.T, session *sdkmcp.ClientSession) []string {
	t.Helper()
	names := make([]string, 0)
	for tool, err := range session.Tools(testutil.Context(t), nil) {
		if err != nil {
			t.Fatalf("list tools: %v", err)
		}
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return names
}

func resultText(result *sdkmcp.CallToolResult) string {
	for _, content := range result.Content {
		if text, ok := content.(*sdkmcp.TextContent); ok {
			return text.Text
		}
	}
	return ""
}

type headerTransport struct {
	authorization string
}

func (h headerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("Authorization", h.authorization)
	return http.DefaultTransport.RoundTrip(clone)
}

func processAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
