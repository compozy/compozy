package agents

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

func TestBuildSessionMCPServersReturnsNilWithoutExecution(t *testing.T) {
	t.Parallel()

	servers, err := BuildSessionMCPServers(nil, SessionMCPContext{})
	if err != nil {
		t.Fatalf("build session MCP servers: %v", err)
	}
	if servers != nil {
		t.Fatalf("expected nil MCP servers for nil execution, got %#v", servers)
	}
}

func TestBuildSessionMCPServersPrependsReservedServerAndSerializesHostContext(t *testing.T) {
	t.Parallel()

	execution := &ExecutionContext{
		Agent: ResolvedAgent{
			Name: "planner",
			MCP: &MCPConfig{
				Servers: []MCPServer{{
					Name:    "filesystem",
					Command: "/tmp/fs-mcp",
					Args:    []string{"--serve"},
					Env: map[string]string{
						"ROOT": "/tmp/workspace",
					},
				}},
			},
			Source: Source{Scope: ScopeWorkspace},
		},
		BaseRuntime: NestedBaseRuntime{
			WorkspaceRoot: "/tmp/workspace",
			IDE:           model.IDECodex,
			Model:         "gpt-5.4",
			AccessMode:    model.AccessModeFull,
		},
	}

	servers, err := BuildSessionMCPServers(execution, SessionMCPContext{
		RunID:                "run-parent",
		EffectiveAccessMode:  model.AccessModeDefault,
		NestedDepth:          1,
		MaxNestedDepth:       4,
		ReservedServerBinary: "/tmp/compozy-test",
	})
	if err != nil {
		t.Fatalf("build session MCP servers: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("expected reserved plus one local MCP server, got %#v", servers)
	}

	reserved := servers[0].Stdio
	if reserved == nil {
		t.Fatalf("expected reserved stdio MCP server, got %#v", servers[0])
	}
	if reserved.Name != ReservedMCPServerName {
		t.Fatalf("unexpected reserved server name: %q", reserved.Name)
	}
	if reserved.Command != "/tmp/compozy-test" {
		t.Fatalf("unexpected reserved server command: %q", reserved.Command)
	}
	if len(reserved.Args) != 3 {
		t.Fatalf("unexpected reserved server args: %#v", reserved.Args)
	}

	payload := reserved.Env[RunAgentContextEnvVar]
	if payload == "" {
		t.Fatal("expected reserved MCP env payload")
	}
	var runtimeContext ReservedServerRuntimeContext
	if err := json.Unmarshal([]byte(payload), &runtimeContext); err != nil {
		t.Fatalf("decode reserved server context: %v", err)
	}
	if runtimeContext.BaseRuntime.WorkspaceRoot != "/tmp/workspace" {
		t.Fatalf("unexpected base runtime workspace root: %#v", runtimeContext.BaseRuntime)
	}
	if runtimeContext.Nested.ParentRunID != "run-parent" {
		t.Fatalf("unexpected nested parent run id: %#v", runtimeContext.Nested)
	}
	if runtimeContext.Nested.ParentAgentName != "planner" {
		t.Fatalf("unexpected nested parent agent: %#v", runtimeContext.Nested)
	}
	if runtimeContext.Nested.ParentAccessMode != model.AccessModeDefault {
		t.Fatalf("unexpected nested access mode: %#v", runtimeContext.Nested)
	}
	if runtimeContext.Nested.Depth != 1 || runtimeContext.Nested.MaxDepth != 4 {
		t.Fatalf("unexpected nested depth context: %#v", runtimeContext.Nested)
	}
	if got, want := runtimeContext.Nested.AgentPath, []string{"planner"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("unexpected nested agent path: got %v want %v", got, want)
	}

	local := servers[1].Stdio
	if local == nil || local.Name != "filesystem" {
		t.Fatalf("unexpected local MCP server: %#v", servers[1])
	}
}

func TestBuildSessionMCPServersUsesDefaultReservedCommandAndNestedDefaults(t *testing.T) {
	t.Parallel()

	execution := &ExecutionContext{
		Agent: ResolvedAgent{
			Name: "planner",
		},
	}

	servers, err := BuildSessionMCPServers(execution, SessionMCPContext{})
	if err != nil {
		t.Fatalf("build session MCP servers: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected only reserved MCP server, got %#v", servers)
	}

	reserved := servers[0].Stdio
	if reserved == nil {
		t.Fatalf("expected reserved stdio MCP server, got %#v", servers[0])
	}
	if reserved.Name != ReservedMCPServerName {
		t.Fatalf("unexpected reserved server name: %q", reserved.Name)
	}
	if reserved.Command == "" {
		t.Fatal("expected reserved server command to default from the current executable")
	}
	if len(reserved.Args) != 3 {
		t.Fatalf("unexpected reserved server args: %#v", reserved.Args)
	}

	var runtimeContext ReservedServerRuntimeContext
	if err := json.Unmarshal([]byte(reserved.Env[RunAgentContextEnvVar]), &runtimeContext); err != nil {
		t.Fatalf("decode reserved server context: %v", err)
	}
	if runtimeContext.Nested.ParentAgentName != "planner" {
		t.Fatalf("expected reserved server to default parent agent name, got %#v", runtimeContext.Nested)
	}
	if runtimeContext.Nested.ParentAccessMode != model.AccessModeDefault {
		t.Fatalf("expected reserved server to default access mode, got %#v", runtimeContext.Nested)
	}
	if runtimeContext.Nested.MaxDepth != DefaultMaxNestedDepth {
		t.Fatalf("expected reserved server to default max depth, got %#v", runtimeContext.Nested)
	}
	if got, want := runtimeContext.Nested.AgentPath, []string{"planner"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("expected reserved server to default agent path, got %#v", runtimeContext.Nested)
	}
}

func TestBuildSessionMCPServersRejectsReservedNameCollision(t *testing.T) {
	t.Parallel()

	execution := &ExecutionContext{
		Agent: ResolvedAgent{
			Name: "planner",
			MCP: &MCPConfig{
				Servers: []MCPServer{{
					Name:    ReservedMCPServerName,
					Command: "/tmp/bad",
				}},
			},
		},
	}

	_, err := BuildSessionMCPServers(execution, SessionMCPContext{
		RunID:                "run-parent",
		EffectiveAccessMode:  model.AccessModeDefault,
		ReservedServerBinary: "/tmp/compozy-test",
	})
	if err == nil {
		t.Fatal("expected reserved-name collision error")
	}
	if !errors.Is(err, ErrReservedMCPServerName) {
		t.Fatalf("expected reserved server error, got %v", err)
	}
}

func TestNestedBaseRuntimeRuntimeConfigClonesFields(t *testing.T) {
	t.Parallel()

	base := NestedBaseRuntime{
		WorkspaceRoot:          "/tmp/workspace",
		IDE:                    model.IDECodex,
		Model:                  "gpt-5.4",
		AddDirs:                []string{"/tmp/extra"},
		ReasoningEffort:        "high",
		AccessMode:             model.AccessModeFull,
		ExplicitRuntime:        model.ExplicitRuntimeFlags{Model: true},
		Timeout:                2 * time.Minute,
		MaxRetries:             3,
		RetryBackoffMultiplier: 1.5,
	}

	cfg := base.RuntimeConfig()
	if cfg.WorkspaceRoot != base.WorkspaceRoot ||
		cfg.IDE != base.IDE ||
		cfg.Model != base.Model ||
		cfg.ReasoningEffort != base.ReasoningEffort ||
		cfg.AccessMode != base.AccessMode {
		t.Fatalf("unexpected runtime config materialization: %#v", cfg)
	}
	if cfg.Mode != model.ExecutionModeExec || cfg.OutputFormat != model.OutputFormatText {
		t.Fatalf("expected nested runtime to materialize exec/text defaults, got %#v", cfg)
	}
	cfg.AddDirs[0] = "/tmp/changed"
	if base.AddDirs[0] != "/tmp/extra" {
		t.Fatalf("expected add_dirs clone, got %#v", base.AddDirs)
	}
}

func TestCaptureNestedBaseRuntimeHandlesNilAndClonesAddDirs(t *testing.T) {
	t.Parallel()

	if got := captureNestedBaseRuntime(nil); got.WorkspaceRoot != "" || got.IDE != "" || len(got.AddDirs) != 0 {
		t.Fatalf("expected zero nested runtime for nil config, got %#v", got)
	}

	cfg := &model.RuntimeConfig{
		WorkspaceRoot:          "/tmp/workspace",
		IDE:                    model.IDECodex,
		Model:                  "gpt-5.4",
		AddDirs:                []string{"/tmp/extra"},
		ReasoningEffort:        "medium",
		AccessMode:             model.AccessModeDefault,
		ExplicitRuntime:        model.ExplicitRuntimeFlags{AccessMode: true},
		Timeout:                time.Minute,
		MaxRetries:             2,
		RetryBackoffMultiplier: 1.25,
	}

	got := captureNestedBaseRuntime(cfg)
	cfg.AddDirs[0] = "/tmp/changed"
	if got.AddDirs[0] != "/tmp/extra" {
		t.Fatalf("expected nested base runtime to clone add_dirs, got %#v", got)
	}
}
