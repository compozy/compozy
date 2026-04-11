package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/model"
	execpkg "github.com/compozy/compozy/internal/core/run/exec"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestRunAgentReturnsStructuredFailureForMissingAgent(t *testing.T) {
	t.Parallel()

	engine := NewEngine()
	result := engine.RunAgent(context.Background(), HostContext{
		BaseRuntime: reusableagents.NestedBaseRuntime{
			WorkspaceRoot: t.TempDir(),
			IDE:           model.IDECodex,
		},
	}, RunAgentRequest{
		Name:  "missing-agent",
		Input: "do the work",
	})

	if result.Success {
		t.Fatalf("expected structured failure, got %#v", result)
	}
	if result.Name != "missing-agent" {
		t.Fatalf("unexpected failure result name: %#v", result)
	}
	if strings.TrimSpace(result.Error) == "" {
		t.Fatalf("expected failure error message, got %#v", result)
	}
}

func TestRunAgentCapsChildAccessAndKeepsChildMCPIsolation(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	writeAgentFixture(
		t,
		workspaceRoot,
		"parent",
		strings.Join([]string{
			"---",
			"title: Parent",
			"description: Parent agent",
			"ide: codex",
			"access_mode: full",
			"---",
			"",
			"Parent prompt.",
			"",
		}, "\n"),
		`{"mcpServers":{"github":{"command":"/tmp/github-mcp","args":["--serve"]}}}`,
	)
	writeAgentFixture(
		t,
		workspaceRoot,
		"child",
		strings.Join([]string{
			"---",
			"title: Child",
			"description: Child agent",
			"ide: codex",
			"access_mode: full",
			"---",
			"",
			"Child prompt.",
			"",
		}, "\n"),
		`{"mcpServers":{"filesystem":{"command":"/tmp/fs-mcp","args":["--serve"],"env":{"ROOT":"/tmp/workspace"}}}}`,
	)

	engine := NewEngine(WithPromptExecutor(
		func(
			_ context.Context,
			cfg *model.RuntimeConfig,
			prompt string,
			agentExecution *reusableagents.ExecutionContext,
			buildMCPServers execpkg.SessionMCPBuilder,
		) (execpkg.PreparedPromptResult, error) {
			if cfg.AccessMode != model.AccessModeDefault {
				t.Fatalf("expected capped child access mode, got %q", cfg.AccessMode)
			}
			if prompt != "delegate this" {
				t.Fatalf("unexpected child prompt: %q", prompt)
			}
			if agentExecution == nil || agentExecution.Agent.Name != "child" {
				t.Fatalf("unexpected child execution context: %#v", agentExecution)
			}

			servers, err := buildMCPServers("run-child-1")
			if err != nil {
				t.Fatalf("build child MCP servers: %v", err)
			}
			if len(servers) != 2 {
				t.Fatalf("expected reserved plus child-local MCP server, got %#v", servers)
			}
			gotNames := []string{servers[0].Stdio.Name, servers[1].Stdio.Name}
			wantNames := []string{reusableagents.ReservedMCPServerName, "filesystem"}
			if strings.Join(gotNames, ",") != strings.Join(wantNames, ",") {
				t.Fatalf("unexpected child MCP server names: got %v want %v", gotNames, wantNames)
			}

			payload := servers[0].Stdio.Env[reusableagents.RunAgentContextEnvVar]
			var runtimeContext reusableagents.ReservedServerRuntimeContext
			if err := json.Unmarshal([]byte(payload), &runtimeContext); err != nil {
				t.Fatalf("decode reserved context: %v", err)
			}
			if runtimeContext.Nested.Depth != 1 || runtimeContext.Nested.MaxDepth != 2 {
				t.Fatalf("unexpected nested context: %#v", runtimeContext.Nested)
			}
			if runtimeContext.Nested.ParentRunID != "run-child-1" {
				t.Fatalf("unexpected child run id in reserved context: %#v", runtimeContext.Nested)
			}
			if runtimeContext.Nested.ParentAgentName != "child" {
				t.Fatalf("unexpected child parent agent name: %#v", runtimeContext.Nested)
			}
			if runtimeContext.Nested.ParentAccessMode != model.AccessModeDefault {
				t.Fatalf("unexpected capped access mode in reserved context: %#v", runtimeContext.Nested)
			}
			if got, want := runtimeContext.Nested.AgentPath, []string{"parent", "child"}; !slices.Equal(got, want) {
				t.Fatalf("unexpected child agent path: got %v want %v", got, want)
			}
			return execpkg.PreparedPromptResult{
				RunID:  "run-child-1",
				Output: "child complete",
			}, nil
		},
	))

	result := engine.RunAgent(context.Background(), HostContext{
		BaseRuntime: reusableagents.NestedBaseRuntime{
			WorkspaceRoot: workspaceRoot,
			IDE:           model.IDECodex,
			AccessMode:    model.AccessModeFull,
		},
		Nested: reusableagents.NestedExecutionContext{
			Depth:            0,
			MaxDepth:         2,
			ParentRunID:      "run-parent-1",
			ParentAgentName:  "parent",
			ParentAccessMode: model.AccessModeDefault,
		},
	}, RunAgentRequest{
		Name:  "child",
		Input: "delegate this",
	})

	if !result.Success {
		t.Fatalf("expected successful child result, got %#v", result)
	}
	if result.Name != "child" || result.Source != string(reusableagents.ScopeWorkspace) {
		t.Fatalf("unexpected child identity in result: %#v", result)
	}
	if result.Output != "child complete" || result.RunID != "run-child-1" {
		t.Fatalf("unexpected child result payload: %#v", result)
	}
}

func TestRunAgentBlocksWhenMaxDepthReached(t *testing.T) {
	t.Parallel()

	engine := NewEngine(WithPromptExecutor(
		func(
			context.Context,
			*model.RuntimeConfig,
			string,
			*reusableagents.ExecutionContext,
			execpkg.SessionMCPBuilder,
		) (execpkg.PreparedPromptResult, error) {
			t.Fatal("prompt executor should not run when depth is blocked")
			return execpkg.PreparedPromptResult{}, nil
		},
	))

	result := engine.RunAgent(context.Background(), HostContext{
		BaseRuntime: reusableagents.NestedBaseRuntime{
			WorkspaceRoot: t.TempDir(),
			IDE:           model.IDECodex,
		},
		Nested: reusableagents.NestedExecutionContext{
			Depth:            2,
			MaxDepth:         2,
			ParentAccessMode: model.AccessModeFull,
		},
	}, RunAgentRequest{
		Name:  "child",
		Input: "delegate this",
	})

	if result.Success {
		t.Fatalf("expected max-depth block result, got %#v", result)
	}
	if !result.Blocked || result.BlockedReason != kinds.ReusableAgentBlockedReasonDepthLimit {
		t.Fatalf("expected depth-limit blocked result, got %#v", result)
	}
	if !strings.Contains(result.Error, "max depth") {
		t.Fatalf("expected deterministic max-depth error, got %#v", result)
	}
}

func TestRunAgentBlocksCyclesWithStructuredReason(t *testing.T) {
	t.Parallel()

	engine := NewEngine(WithPromptExecutor(
		func(
			context.Context,
			*model.RuntimeConfig,
			string,
			*reusableagents.ExecutionContext,
			execpkg.SessionMCPBuilder,
		) (execpkg.PreparedPromptResult, error) {
			t.Fatal("prompt executor should not run for cyclic nested execution")
			return execpkg.PreparedPromptResult{}, nil
		},
	))

	result := engine.RunAgent(context.Background(), HostContext{
		BaseRuntime: reusableagents.NestedBaseRuntime{
			WorkspaceRoot: t.TempDir(),
			IDE:           model.IDECodex,
		},
		Nested: reusableagents.NestedExecutionContext{
			Depth:            1,
			MaxDepth:         3,
			ParentAgentName:  "child",
			ParentAccessMode: model.AccessModeFull,
			AgentPath:        []string{"parent", "child"},
		},
	}, RunAgentRequest{
		Name:  "parent",
		Input: "delegate back to parent",
	})

	if result.Success {
		t.Fatalf("expected cycle-detected block result, got %#v", result)
	}
	if !result.Blocked || result.BlockedReason != kinds.ReusableAgentBlockedReasonCycleDetected {
		t.Fatalf("expected cycle-detected blocked reason, got %#v", result)
	}
	if !strings.Contains(result.Error, "parent -> child -> parent") {
		t.Fatalf("expected cycle path in error, got %#v", result)
	}
}

func TestRunAgentClassifiesInvalidMCPFailures(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	writeAgentFixture(
		t,
		workspaceRoot,
		"child",
		strings.Join([]string{
			"---",
			"title: Child",
			"description: Child agent",
			"ide: codex",
			"---",
			"",
			"Child prompt.",
			"",
		}, "\n"),
		`{"mcpServers":{"filesystem":{"command":"/tmp/fs-mcp","args":["--serve"],"env":{"ROOT":"${MISSING_AGENT_ROOT}"}}}}`,
	)

	engine := NewEngine()
	result := engine.RunAgent(context.Background(), HostContext{
		BaseRuntime: reusableagents.NestedBaseRuntime{
			WorkspaceRoot: workspaceRoot,
			IDE:           model.IDECodex,
		},
		Nested: reusableagents.NestedExecutionContext{
			ParentAgentName:  "parent",
			ParentAccessMode: model.AccessModeFull,
		},
	}, RunAgentRequest{
		Name:  "child",
		Input: "delegate this",
	})

	if result.Success {
		t.Fatalf("expected invalid-mcp block result, got %#v", result)
	}
	if !result.Blocked || result.BlockedReason != kinds.ReusableAgentBlockedReasonInvalidMCP {
		t.Fatalf("expected invalid-mcp blocked reason, got %#v", result)
	}
	if !strings.Contains(result.Error, "MISSING_AGENT_ROOT") {
		t.Fatalf("expected actionable missing env detail, got %#v", result)
	}
}

func TestRunAgentHonorsWorkspaceOverrideDuringNestedResolution(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeAgentFixture(
		t,
		workspaceRoot,
		"helper",
		strings.Join([]string{
			"---",
			"title: Workspace Helper",
			"description: Workspace helper",
			"ide: codex",
			"---",
			"",
			"Workspace prompt.",
			"",
		}, "\n"),
		"",
	)

	globalDir := filepath.Join(homeDir, model.WorkflowRootDirName, "agents", "helper")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("mkdir global agent dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(globalDir, "AGENT.md"),
		[]byte(strings.Join([]string{
			"---",
			"title: Global Helper",
			"description: Global helper",
			"ide: codex",
			"---",
			"",
			"Global prompt.",
			"",
		}, "\n")),
		0o600,
	); err != nil {
		t.Fatalf("write global AGENT.md: %v", err)
	}

	engine := NewEngine(WithPromptExecutor(
		func(
			_ context.Context,
			_ *model.RuntimeConfig,
			prompt string,
			agentExecution *reusableagents.ExecutionContext,
			_ execpkg.SessionMCPBuilder,
		) (execpkg.PreparedPromptResult, error) {
			if prompt != "delegate this" {
				t.Fatalf("unexpected nested prompt: %q", prompt)
			}
			if agentExecution == nil {
				t.Fatal("expected nested agent execution context")
			}
			if agentExecution.Agent.Source.Scope != reusableagents.ScopeWorkspace {
				t.Fatalf("expected workspace override source, got %#v", agentExecution.Agent.Source)
			}
			if agentExecution.Agent.Prompt != "Workspace prompt.\n" &&
				agentExecution.Agent.Prompt != "Workspace prompt." {
				t.Fatalf("expected workspace prompt body, got %q", agentExecution.Agent.Prompt)
			}
			return execpkg.PreparedPromptResult{RunID: "run-helper", Output: "workspace helper"}, nil
		},
	))

	result := engine.RunAgent(context.Background(), HostContext{
		BaseRuntime: reusableagents.NestedBaseRuntime{
			WorkspaceRoot: workspaceRoot,
			IDE:           model.IDECodex,
		},
		Nested: reusableagents.NestedExecutionContext{
			ParentAgentName:  "parent",
			ParentAccessMode: model.AccessModeFull,
		},
	}, RunAgentRequest{
		Name:  "helper",
		Input: "delegate this",
	})

	if !result.Success {
		t.Fatalf("expected workspace override nested success, got %#v", result)
	}
}

func TestCapAccessModePreventsEscalation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		child  string
		parent string
		want   string
	}{
		{
			name:   "full parent keeps full child",
			child:  model.AccessModeFull,
			parent: model.AccessModeFull,
			want:   model.AccessModeFull,
		},
		{
			name:   "default child stays default under full parent",
			child:  model.AccessModeDefault,
			parent: model.AccessModeFull,
			want:   model.AccessModeDefault,
		},
		{
			name:   "default parent caps full child",
			child:  model.AccessModeFull,
			parent: model.AccessModeDefault,
			want:   model.AccessModeDefault,
		},
		{name: "blank parent caps child", child: model.AccessModeFull, parent: "", want: model.AccessModeDefault},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := capAccessMode(tt.child, tt.parent); got != tt.want {
				t.Fatalf("capAccessMode(%q, %q) = %q, want %q", tt.child, tt.parent, got, tt.want)
			}
		})
	}
}

func writeAgentFixture(t *testing.T, workspaceRoot, name, agentMarkdown, mcpJSON string) {
	t.Helper()

	agentDir := filepath.Join(workspaceRoot, model.WorkflowRootDirName, "agents", name)
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir agent dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte(agentMarkdown), 0o600); err != nil {
		t.Fatalf("write AGENT.md: %v", err)
	}
	if strings.TrimSpace(mcpJSON) != "" {
		if err := os.WriteFile(filepath.Join(agentDir, "mcp.json"), []byte(mcpJSON), 0o600); err != nil {
			t.Fatalf("write mcp.json: %v", err)
		}
	}
}
