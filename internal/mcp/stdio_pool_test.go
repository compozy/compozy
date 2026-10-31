package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	toolspkg "github.com/compozy/compozy/internal/tools"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	stdioPoolHelperEnv       = "COMPOZY_MCP_STDIO_POOL_HELPER"
	stdioPoolLaunchLogEnv    = "COMPOZY_MCP_STDIO_POOL_LAUNCH_LOG"
	stdioPoolStartDelayEnv   = "COMPOZY_MCP_STDIO_POOL_START_DELAY"
	stdioPoolCrashOnStartEnv = "COMPOZY_MCP_STDIO_POOL_CRASH_ON_START"
	stdioPoolSecretEnv       = "COMPOZY_MCP_STDIO_POOL_SECRET"
	stdioPoolSecretSource    = "COMPOZY_MCP_STDIO_POOL_SECRET_SOURCE"
	stdioPoolSecretValue     = "pool-secret-value-7f3a"
	stdioPoolCrashMessage    = "crash"
)

func TestStdioSessionPool(t *testing.T) {
	t.Run("Should reuse one stdio child for discovery and concurrent tool calls", func(t *testing.T) {
		t.Parallel()

		fixture := newStdioPoolFixture(t, StdioPoolConfig{}, nil)
		descriptor := requireMCPDescriptor(t, fixture.executor, fixture.server.Name, "mcp__pooled__echo")
		var wg sync.WaitGroup
		errs := make(chan error, 12)
		for index := range 12 {
			wg.Go(func() {
				result, err := fixture.executor.CallTool(testContext(t), descriptor.Source, toolspkg.MCPToolCallRequest{
					ToolID:      descriptor.ID,
					RawToolName: descriptor.RawName,
					Input:       json.RawMessage(fmt.Sprintf(`{"message":"call-%d"}`, index)),
				})
				if err == nil && result.Preview != fmt.Sprintf("echo: call-%d", index) {
					err = fmt.Errorf("result.Preview = %q", result.Preview)
				}
				errs <- err
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("CallTool() error = %v", err)
			}
		}
		requireMCPDescriptor(t, fixture.executor, fixture.server.Name, "mcp__pooled__echo")
		if got := fixture.launches(); got != 1 {
			t.Fatalf("stdio launches = %d, want 1 for discovery plus 12 calls", got)
		}
	})

	t.Run("Should answer cached discovery without contacting the child", func(t *testing.T) {
		t.Parallel()

		fixture := newStdioPoolFixture(t, StdioPoolConfig{}, nil)
		requireMCPDescriptor(t, fixture.executor, fixture.server.Name, "mcp__pooled__echo")
		descriptor := requireMCPDescriptor(t, fixture.executor, fixture.server.Name, "mcp__pooled__echo")
		_, err := fixture.executor.CallTool(testContext(t), descriptor.Source, toolspkg.MCPToolCallRequest{
			ToolID: descriptor.ID, RawToolName: descriptor.RawName,
			Input: json.RawMessage(`{"message":"` + stdioPoolCrashMessage + `"}`),
		})
		if err == nil {
			t.Fatal("CallTool(crash) error = nil, want child exit failure")
		}
		fixture.waitForNoSession(t)
		requireMCPDescriptor(t, fixture.executor, fixture.server.Name, "mcp__pooled__echo")
		if got := fixture.launches(); got != 1 {
			t.Fatalf("stdio launches = %d, want cached discovery to skip relaunch", got)
		}
	})

	t.Run("Should keep a slow first launch alive past the caller deadline", func(t *testing.T) {
		t.Parallel()

		fixture := newStdioPoolFixture(t, StdioPoolConfig{}, map[string]string{
			stdioPoolStartDelayEnv: "600ms",
		})
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		_, err := fixture.executor.ListTools(ctx, fixture.source())
		requireReason(t, err, toolspkg.ReasonCallTimedOut)

		tools, err := fixture.executor.ListTools(testContext(t), fixture.source())
		if err != nil || len(tools) != 1 {
			t.Fatalf("ListTools() after slow start = %d tools, error %v; want the pending launch", len(tools), err)
		}
		if got := fixture.launches(); got != 1 {
			t.Fatalf("stdio launches = %d, want the slow launch reused", got)
		}
	})

	t.Run("Should back off after a child exits instead of relaunching per call", func(t *testing.T) {
		t.Parallel()

		fixture := newStdioPoolFixture(t, StdioPoolConfig{backoffInitial: 2 * time.Second}, nil)
		descriptor := requireMCPDescriptor(t, fixture.executor, fixture.server.Name, "mcp__pooled__echo")
		crash := toolspkg.MCPToolCallRequest{
			ToolID: descriptor.ID, RawToolName: descriptor.RawName,
			Input: json.RawMessage(`{"message":"` + stdioPoolCrashMessage + `"}`),
		}
		if _, err := fixture.executor.CallTool(testContext(t), descriptor.Source, crash); err == nil {
			t.Fatal("CallTool(crash) error = nil, want child exit failure")
		}
		fixture.waitForNoSession(t)
		echo := toolspkg.MCPToolCallRequest{
			ToolID: descriptor.ID, RawToolName: descriptor.RawName, Input: json.RawMessage(`{"message":"again"}`),
		}
		_, err := fixture.executor.CallTool(testContext(t), descriptor.Source, echo)
		requireReason(t, err, toolspkg.ReasonMCPUnreachable)
		if !strings.Contains(err.Error(), "backing off") {
			t.Fatalf("CallTool() during backoff error = %v, want backoff diagnostic", err)
		}
		if got := fixture.launches(); got != 1 {
			t.Fatalf("stdio launches during backoff = %d, want 1", got)
		}

		deadline := time.Now().Add(15 * time.Second)
		for {
			_, err := fixture.executor.CallTool(testContext(t), descriptor.Source, echo)
			if err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("CallTool() after backoff error = %v", err)
			}
			time.Sleep(100 * time.Millisecond)
		}
		if got := fixture.launches(); got != 2 {
			t.Fatalf("stdio launches after backoff = %d, want one relaunch", got)
		}
	})

	t.Run("Should keep a redacted stderr tail for a child that fails to start", func(t *testing.T) {
		t.Parallel()

		fixture := newStdioPoolFixture(t, StdioPoolConfig{}, map[string]string{stdioPoolCrashOnStartEnv: "1"})
		_, err := fixture.executor.ListTools(testContext(t), fixture.source())
		requireReason(t, err, toolspkg.ReasonMCPUnreachable)
		_, err = fixture.executor.ListTools(testContext(t), fixture.source())
		requireReason(t, err, toolspkg.ReasonMCPUnreachable)
		if got := fixture.launches(); got != 1 {
			t.Fatalf("stdio launches = %d, want the failed start backed off", got)
		}
		tail := fixture.pool.StderrTail(fixture.target())
		if !strings.Contains(tail, "fixture crashed on start") || !strings.Contains(tail, "fixture booting") {
			t.Fatalf("StderrTail() = %q, want the child's stderr", tail)
		}
		if strings.Contains(tail, stdioPoolSecretValue) {
			t.Fatalf("StderrTail() = %q, leaked the resolved secret_env value", tail)
		}
		fixture.requireNoHomes(t)
	})

	t.Run("Should launch a repaired definition without waiting out the old backoff", func(t *testing.T) {
		t.Parallel()

		fixture := newStdioPoolFixture(t, StdioPoolConfig{backoffInitial: time.Minute}, map[string]string{
			stdioPoolCrashOnStartEnv: "1",
		})
		_, err := fixture.executor.ListTools(testContext(t), fixture.source())
		requireReason(t, err, toolspkg.ReasonMCPUnreachable)
		repaired := fixture.server
		repaired.Env = cloneStringMap(repaired.Env)
		delete(repaired.Env, stdioPoolCrashOnStartEnv)
		requireMCPDescriptor(t, fixture.executorFor(t, repaired), repaired.Name, "mcp__pooled__echo")
		if got := fixture.launches(); got != 2 {
			t.Fatalf("stdio launches = %d, want the repaired definition launched once", got)
		}
	})

	t.Run("Should replace the child when the server definition changes", func(t *testing.T) {
		t.Parallel()

		fixture := newStdioPoolFixture(t, StdioPoolConfig{}, nil)
		requireMCPDescriptor(t, fixture.executor, fixture.server.Name, "mcp__pooled__echo")
		changed := fixture.server
		changed.Env = cloneStringMap(changed.Env)
		changed.Env["COMPOZY_MCP_STDIO_POOL_REVISION"] = "2"
		executor := fixture.executorFor(t, changed)
		requireMCPDescriptor(t, executor, changed.Name, "mcp__pooled__echo")
		if got := fixture.launches(); got != 2 {
			t.Fatalf("stdio launches = %d, want a relaunch for the changed definition", got)
		}
		fixture.waitForHomes(t, 1)
	})

	t.Run("Should close idle children and remove their homes", func(t *testing.T) {
		t.Parallel()

		fixture := newStdioPoolFixture(t, StdioPoolConfig{IdleTimeout: 100 * time.Millisecond}, nil)
		requireMCPDescriptor(t, fixture.executor, fixture.server.Name, "mcp__pooled__echo")
		fixture.waitForNoSession(t)
		fixture.requireNoHomes(t)
	})

	t.Run("Should stop children and remove homes on close", func(t *testing.T) {
		t.Parallel()

		fixture := newStdioPoolFixture(t, StdioPoolConfig{}, nil)
		requireMCPDescriptor(t, fixture.executor, fixture.server.Name, "mcp__pooled__echo")
		fixture.waitForHomes(t, 1)
		if err := fixture.pool.Close(testContext(t)); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		fixture.requireNoHomes(t)
		_, err := fixture.executor.ListTools(testContext(t), fixture.source())
		if err == nil {
			t.Fatal("ListTools() after Close error = nil, want closed pool")
		}
	})

	t.Run("Should point package caches at the persistent pool root", func(t *testing.T) {
		t.Parallel()

		fixture := newStdioPoolFixture(t, StdioPoolConfig{}, map[string]string{stdioEnvHelperEnv: "1"})
		fixture.server.Args = []string{"-test.run=^TestMCPStdioHelperProcess$"}
		executor := fixture.executorFor(t, fixture.server)
		descriptor := requireMCPDescriptor(t, executor, fixture.server.Name, "mcp__pooled__echo")
		for key, want := range map[string]string{
			"npm_config_cache": filepath.Join(fixture.root, "npm"),
			"UV_CACHE_DIR":     filepath.Join(fixture.root, "uv"),
		} {
			result := callMCPTool(t, executor, descriptor, json.RawMessage(`{"message":"`+key+`"}`))
			encoded := strings.TrimPrefix(result.Preview, "env: ")
			decoded, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				t.Fatalf("decode %s preview %q: %v", key, result.Preview, err)
			}
			if string(decoded) != want {
				t.Fatalf("%s = %q, want %q", key, decoded, want)
			}
		}
	})

	t.Run("Should sweep orphaned homes left by an earlier daemon", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		orphan := filepath.Join(root, "homes", mcpStdioHomeNamePrefix+"0011223344556677-orphan")
		if err := os.MkdirAll(filepath.Join(orphan, "cache"), 0o700); err != nil {
			t.Fatalf("create orphan home: %v", err)
		}
		pool, err := NewStdioSessionPool(StdioPoolConfig{Root: root})
		if err != nil {
			t.Fatalf("NewStdioSessionPool() error = %v", err)
		}
		t.Cleanup(func() {
			if err := pool.Close(context.WithoutCancel(t.Context())); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
		removed, err := pool.SweepOrphanHomes()
		if err != nil {
			t.Fatalf("SweepOrphanHomes() error = %v", err)
		}
		if removed < 1 {
			t.Fatalf("SweepOrphanHomes() removed = %d, want the orphan", removed)
		}
		if _, err := os.Stat(orphan); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("orphan home stat error = %v, want removed", err)
		}
	})
}

func TestStdioStderrRing(t *testing.T) {
	t.Run("Should keep only the newest bytes and redact launch secrets", func(t *testing.T) {
		t.Parallel()

		ring := newStdioStderrRing(64, []string{"s3cr3t-token"})
		for index := range 20 {
			if _, err := fmt.Fprintf(ring, "line %02d token=s3cr3t-token\n", index); err != nil {
				t.Fatalf("Write() error = %v", err)
			}
		}
		tail := ring.Tail()
		if strings.Contains(tail, "s3cr3t-token") || strings.Contains(tail, "line 00") {
			t.Fatalf("Tail() = %q, want redacted newest lines only", tail)
		}
		if !strings.Contains(tail, "line 19") {
			t.Fatalf("Tail() = %q, want the newest line", tail)
		}
	})
}

func TestMCPStdioPoolHelperProcess(t *testing.T) {
	t.Run("Should serve the pooled stdio fixture when requested", func(_ *testing.T) {
		if os.Getenv(stdioPoolHelperEnv) != "1" {
			return
		}
		if path := os.Getenv(stdioPoolLaunchLogEnv); path != "" {
			file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			if err != nil {
				os.Exit(4)
			}
			if _, err := file.WriteString("launch\n"); err != nil {
				os.Exit(4)
			}
			if err := file.Close(); err != nil {
				os.Exit(4)
			}
		}
		_, _ = fmt.Fprintf(os.Stderr, "fixture booting token=%s\n", os.Getenv(stdioPoolSecretEnv))
		if delay, err := time.ParseDuration(os.Getenv(stdioPoolStartDelayEnv)); err == nil {
			time.Sleep(delay)
		}
		if os.Getenv(stdioPoolCrashOnStartEnv) == "1" {
			_, _ = fmt.Fprintln(os.Stderr, "fixture crashed on start")
			os.Exit(23)
		}
		server := newFakeSDKServer(
			func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
				var input struct {
					Message string `json:"message"`
				}
				if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
					return nil, fmt.Errorf("decode echo input: %w", err)
				}
				if input.Message == stdioPoolCrashMessage {
					os.Exit(24)
				}
				return &mcpsdk.CallToolResult{
					Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "echo: " + input.Message}},
				}, nil
			},
		)
		server.AddReceivingMiddleware(withTestMCPListTTL)
		if err := server.Run(context.Background(), &mcpsdk.StdioTransport{}); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	})
}

type stdioPoolFixture struct {
	pool      *StdioSessionPool
	executor  *CallExecutor
	server    compozyconfig.MCPServer
	root      string
	launchLog string
}

func newStdioPoolFixture(t *testing.T, cfg StdioPoolConfig, env map[string]string) *stdioPoolFixture {
	t.Helper()

	root := t.TempDir()
	cfg.Root = root
	pool, err := NewStdioSessionPool(cfg)
	if err != nil {
		t.Fatalf("NewStdioSessionPool() error = %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Close(context.WithoutCancel(t.Context())); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	launchLog := filepath.Join(t.TempDir(), "launches.log")
	serverEnv := map[string]string{stdioPoolHelperEnv: "1", stdioPoolLaunchLogEnv: launchLog}
	maps.Copy(serverEnv, env)
	fixture := &stdioPoolFixture{
		pool: pool,
		server: compozyconfig.MCPServer{
			Name:      "pooled",
			Transport: compozyconfig.MCPServerTransportStdio,
			Command:   os.Args[0],
			Args:      []string{"-test.run=^TestMCPStdioPoolHelperProcess$"},
			Env:       serverEnv,
			SecretEnv: map[string]string{stdioPoolSecretEnv: "env:" + stdioPoolSecretSource},
		},
		root:      root,
		launchLog: launchLog,
	}
	fixture.executor = fixture.executorFor(t, fixture.server)
	return fixture
}

func (f *stdioPoolFixture) executorFor(t *testing.T, server compozyconfig.MCPServer) *CallExecutor {
	t.Helper()

	return newTestMCPExecutor(t, server, WithStdioSessionPool(f.pool), WithSecretLookup(func(key string) string {
		if key == stdioPoolSecretSource {
			return stdioPoolSecretValue
		}
		return ""
	}))
}

func (f *stdioPoolFixture) source() toolspkg.SourceRef {
	return toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: f.server.Name, RawServerName: f.server.Name}
}

func (f *stdioPoolFixture) target() mcpauth.Target {
	return userMCPExecutorTarget(f.server.Name)
}

func (f *stdioPoolFixture) launches() int {
	data, err := os.ReadFile(f.launchLog)
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "launch\n")
}

func (f *stdioPoolFixture) homes(t *testing.T) []os.DirEntry {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(f.root, "homes"))
	if err != nil {
		t.Fatalf("read stdio homes: %v", err)
	}
	return entries
}

func (f *stdioPoolFixture) requireNoHomes(t *testing.T) {
	t.Helper()

	f.waitForHomes(t, 0)
}

func (f *stdioPoolFixture) waitForHomes(t *testing.T, want int) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for {
		entries := f.homes(t)
		if len(entries) == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("stdio homes = %d, want %d", len(entries), want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (f *stdioPoolFixture) waitForNoSession(t *testing.T) {
	t.Helper()

	key, err := f.target().Key()
	if err != nil {
		t.Fatalf("target key: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		f.pool.mu.Lock()
		slot := f.pool.slots[key]
		live := slot != nil && slot.current != nil
		f.pool.mu.Unlock()
		if !live {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("stdio session still live, want it ended")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
