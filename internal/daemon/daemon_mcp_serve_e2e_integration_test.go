//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
	mcpclient "github.com/mark3labs/mcp-go/client"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
)

func TestDaemonE2EMCPServeProjectsWorkspaceBoundHostAPI(t *testing.T) {
	t.Parallel()
	acpmock.RequireDriver(t)

	const agentName = "mcp-serve-agent"
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "multi_agent_fixture.json"),
			FixtureAgent: "alpha",
			AgentName:    agentName,
		}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	workspaceA := harness.WorkspaceRoot
	clientA := startMCPServeClient(t, ctx, harness, workspaceA)

	createdSession := callMCPServeToolJSON[struct {
		SessionID string `json:"session_id"`
	}](t, ctx, clientA, "agh_host__sessions__create", map[string]any{
		"agent": agentName,
	})
	if createdSession.SessionID == "" {
		t.Fatal("sessions/create returned an empty session ID")
	}
	if _, err := harness.GetSession(ctx, createdSession.SessionID); err != nil {
		t.Fatalf("native HTTP GetSession(%q) error = %v", createdSession.SessionID, err)
	}

	listedSessions := callMCPServeToolJSON[[]struct {
		ID string `json:"id"`
	}](t, ctx, clientA, "agh_host__sessions__list", map[string]any{})
	if !mcpServeSessionListContains(listedSessions, createdSession.SessionID) {
		t.Fatalf("sessions/list = %#v, want %q", listedSessions, createdSession.SessionID)
	}

	createdTask := callMCPServeToolJSON[struct {
		ID string `json:"id"`
	}](t, ctx, clientA, "agh_host__tasks__create", map[string]any{
		"title": "MCP-created task",
	})
	if createdTask.ID == "" {
		t.Fatal("tasks/create returned an empty task ID")
	}
	var nativeTask aghcontract.TaskDetailResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, "/api/tasks/"+createdTask.ID, nil, &nativeTask); err != nil {
		t.Fatalf("native HTTP GetTask(%q) error = %v", createdTask.ID, err)
	}
	if nativeTask.Task.Task.ID != createdTask.ID || nativeTask.Task.Task.WorkspaceID != harness.WorkspaceID {
		t.Fatalf("native task = %#v, want MCP task in workspace A", nativeTask.Task)
	}

	workspaceB := t.TempDir()
	resolvedB, err := harness.ResolveWorkspace(ctx, workspaceB)
	if err != nil {
		t.Fatalf("ResolveWorkspace(B) error = %v", err)
	}
	clientB := startMCPServeClient(t, ctx, harness, workspaceB)
	isolatedSessions := callMCPServeToolJSON[[]struct {
		ID string `json:"id"`
	}](t, ctx, clientB, "agh_host__sessions__list", map[string]any{})
	if mcpServeSessionListContains(isolatedSessions, createdSession.SessionID) {
		t.Fatalf("workspace B %q listed workspace A session %q", resolvedB.ID, createdSession.SessionID)
	}

	callMCPServeToolJSON[map[string]any](
		t,
		ctx,
		clientA,
		"agh_host__sessions__stop",
		map[string]any{"session_id": createdSession.SessionID},
	)
}

func startMCPServeClient(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspace string,
) *mcpclient.Client {
	t.Helper()
	client, err := mcpclient.NewStdioMCPClientWithOptions(
		harness.BinaryPath,
		[]string{
			"AGH_HOME=" + harness.HomePaths.HomeDir,
			"HOME=" + harness.HomePaths.HomeDir,
		},
		[]string{"mcp", "serve", "--workspace", workspace},
	)
	if err != nil {
		t.Fatalf("NewStdioMCPClientWithOptions() error = %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("MCP client.Close() error = %v", err)
		}
	})

	var initialize sdkmcp.InitializeRequest
	initialize.Params.ProtocolVersion = sdkmcp.LATEST_PROTOCOL_VERSION
	initialize.Params.ClientInfo = sdkmcp.Implementation{Name: "agh-e2e", Version: "1.0.0"}
	if _, err := client.Initialize(ctx, initialize); err != nil {
		t.Fatalf("MCP client.Initialize() error = %v", err)
	}
	return client
}

func callMCPServeToolJSON[T any](
	t *testing.T,
	ctx context.Context,
	client *mcpclient.Client,
	toolName string,
	arguments map[string]any,
) T {
	t.Helper()
	var request sdkmcp.CallToolRequest
	request.Params.Name = toolName
	request.Params.Arguments = arguments
	result, err := client.CallTool(ctx, request)
	if err != nil {
		t.Fatalf("CallTool(%q) error = %v", toolName, err)
	}
	if result == nil || result.IsError {
		t.Fatalf("CallTool(%q) result = %#v, want success", toolName, result)
	}
	payload, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("json.Marshal(CallTool(%q)) error = %v", toolName, err)
	}
	var decoded T
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(CallTool(%q)) error = %v; payload=%s", toolName, err, payload)
	}
	return decoded
}

func mcpServeSessionListContains(sessions []struct {
	ID string `json:"id"`
}, id string) bool {
	for _, session := range sessions {
		if session.ID == id {
			return true
		}
	}
	return false
}
