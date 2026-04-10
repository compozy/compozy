package exec_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/core/agent"
	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/agents/mcpserver"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/acpshared"
)

func TestRunAgentEngineExecutesRealNestedChildSession(t *testing.T) {
	workspaceRoot := t.TempDir()
	childDir := filepath.Join(workspaceRoot, model.WorkflowRootDirName, "agents", "child")
	if err := os.MkdirAll(childDir, 0o755); err != nil {
		t.Fatalf("mkdir child agent dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(childDir, "AGENT.md"),
		[]byte(strings.Join([]string{
			"---",
			"title: Child",
			"description: Child agent",
			"ide: codex",
			"access_mode: full",
			"---",
			"",
			"Child prompt.",
			"",
		}, "\n")),
		0o600,
	); err != nil {
		t.Fatalf("write child agent: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(childDir, "mcp.json"),
		[]byte(`{"mcpServers":{"filesystem":{"command":"/tmp/fs-mcp","args":["--serve"]}}}`),
		0o600,
	); err != nil {
		t.Fatalf("write child mcp.json: %v", err)
	}
	installRuntimeProbeStubForRunAgent(t, "codex-acp")

	var (
		mu           sync.Mutex
		gotClientCfg agent.ClientConfig
		gotReq       agent.SessionRequest
	)
	restore := acpshared.SwapNewAgentClientForTest(
		func(_ context.Context, cfg agent.ClientConfig) (agent.Client, error) {
			mu.Lock()
			gotClientCfg = cfg
			mu.Unlock()
			return &fakeRunAgentACPClient{
				createSessionFn: func(_ context.Context, req agent.SessionRequest) (agent.Session, error) {
					mu.Lock()
					gotReq = req
					mu.Unlock()
					return newSuccessfulRunAgentSession("sess-child", "child reply"), nil
				},
			}, nil
		},
	)
	t.Cleanup(restore)

	engine := mcpserver.NewEngine()
	result := engine.RunAgent(context.Background(), mcpserver.HostContext{
		BaseRuntime: reusableagents.NestedBaseRuntime{
			WorkspaceRoot:   workspaceRoot,
			IDE:             model.IDECodex,
			ReasoningEffort: "medium",
			AccessMode:      model.AccessModeFull,
		},
		Nested: reusableagents.NestedExecutionContext{
			Depth:            0,
			MaxDepth:         2,
			ParentAccessMode: model.AccessModeFull,
		},
	}, mcpserver.RunAgentRequest{
		Name:  "child",
		Input: "delegate this",
	})

	if !result.Success {
		t.Fatalf("expected nested child success, got %#v", result)
	}
	if result.Name != "child" || result.Source != string(reusableagents.ScopeWorkspace) {
		t.Fatalf("unexpected nested result identity: %#v", result)
	}
	if result.Output != "child reply" || strings.TrimSpace(result.RunID) == "" {
		t.Fatalf("unexpected nested result payload: %#v", result)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotClientCfg.AccessMode != model.AccessModeFull {
		t.Fatalf("unexpected nested child client access mode: %#v", gotClientCfg)
	}
	if len(gotReq.MCPServers) != 2 {
		t.Fatalf("expected reserved plus child-local MCP servers, got %#v", gotReq.MCPServers)
	}
	if gotReq.MCPServers[0].Stdio == nil || gotReq.MCPServers[0].Stdio.Name != reusableagents.ReservedMCPServerName {
		t.Fatalf("unexpected reserved MCP server wiring: %#v", gotReq.MCPServers)
	}
	if gotReq.MCPServers[1].Stdio == nil || gotReq.MCPServers[1].Stdio.Name != "filesystem" {
		t.Fatalf("unexpected child-local MCP server wiring: %#v", gotReq.MCPServers)
	}
}

func installRuntimeProbeStubForRunAgent(t *testing.T, command string) {
	t.Helper()

	binDir := t.TempDir()
	scriptPath := filepath.Join(binDir, command)
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write runtime probe stub: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

type fakeRunAgentACPClient struct {
	createSessionFn func(context.Context, agent.SessionRequest) (agent.Session, error)
}

func (c *fakeRunAgentACPClient) CreateSession(ctx context.Context, req agent.SessionRequest) (agent.Session, error) {
	return c.createSessionFn(ctx, req)
}

func (*fakeRunAgentACPClient) ResumeSession(context.Context, agent.ResumeSessionRequest) (agent.Session, error) {
	return nil, nil
}

func (*fakeRunAgentACPClient) SupportsLoadSession() bool { return false }
func (*fakeRunAgentACPClient) Close() error              { return nil }
func (*fakeRunAgentACPClient) Kill() error               { return nil }

type successfulRunAgentSession struct {
	id      string
	updates chan model.SessionUpdate
	done    chan struct{}
}

func newSuccessfulRunAgentSession(id, output string) *successfulRunAgentSession {
	session := &successfulRunAgentSession{
		id:      id,
		updates: make(chan model.SessionUpdate, 1),
		done:    make(chan struct{}),
	}
	go func() {
		session.updates <- model.SessionUpdate{
			Kind:   model.UpdateKindAgentMessageChunk,
			Status: model.StatusRunning,
			Blocks: []model.ContentBlock{runAgentTextContentBlock(output)},
		}
		close(session.updates)
		close(session.done)
	}()
	return session
}

func (s *successfulRunAgentSession) ID() string { return s.id }

func (s *successfulRunAgentSession) Identity() agent.SessionIdentity {
	return agent.SessionIdentity{ACPSessionID: s.id}
}

func (s *successfulRunAgentSession) Updates() <-chan model.SessionUpdate { return s.updates }
func (s *successfulRunAgentSession) Done() <-chan struct{}               { return s.done }
func (*successfulRunAgentSession) Err() error                            { return nil }
func (*successfulRunAgentSession) SlowPublishes() uint64                 { return 0 }
func (*successfulRunAgentSession) DroppedUpdates() uint64                { return 0 }

func runAgentTextContentBlock(text string) model.ContentBlock {
	payload, err := json.Marshal(model.TextBlock{
		Type: model.BlockText,
		Text: text,
	})
	if err != nil {
		panic(err)
	}
	return model.ContentBlock{
		Type: model.BlockText,
		Data: payload,
	}
}
