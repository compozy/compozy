package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/model"
	execpkg "github.com/compozy/compozy/internal/core/run/exec"
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
	if !strings.Contains(result.Error, "max depth") {
		t.Fatalf("expected deterministic max-depth error, got %#v", result)
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
