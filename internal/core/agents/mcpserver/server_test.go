package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var stdioSwapMu sync.Mutex

func TestLoadHostContextFromEnvParsesReservedPayload(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(reusableagents.ReservedServerRuntimeContext{
		BaseRuntime: reusableagents.NestedBaseRuntime{
			WorkspaceRoot:   "/tmp/workspace",
			IDE:             model.IDECodex,
			Model:           "gpt-5.4",
			ReasoningEffort: "high",
			AccessMode:      model.AccessModeFull,
		},
		Nested: reusableagents.NestedExecutionContext{
			Depth:            1,
			MaxDepth:         3,
			ParentRunID:      "run-123",
			ParentAgentName:  "planner",
			ParentAccessMode: model.AccessModeDefault,
		},
	})
	if err != nil {
		t.Fatalf("marshal reserved payload: %v", err)
	}

	host, err := loadHostContextFromEnv(func(key string) (string, bool) {
		if key != reusableagents.RunAgentContextEnvVar {
			return "", false
		}
		return string(raw), true
	})
	if err != nil {
		t.Fatalf("load host context: %v", err)
	}
	if host.BaseRuntime.WorkspaceRoot != "/tmp/workspace" || host.BaseRuntime.Model != "gpt-5.4" {
		t.Fatalf("unexpected base runtime: %#v", host.BaseRuntime)
	}
	if host.Nested.ParentRunID != "run-123" || host.Nested.ParentAgentName != "planner" {
		t.Fatalf("unexpected nested context: %#v", host.Nested)
	}
}

func TestLoadHostContextFromEnvTreatsMissingPayloadAsEmpty(t *testing.T) {
	t.Parallel()

	host, err := loadHostContextFromEnv(func(string) (string, bool) {
		return "", false
	})
	if err != nil {
		t.Fatalf("load empty host context: %v", err)
	}
	if host.BaseRuntime.WorkspaceRoot != "" || host.BaseRuntime.IDE != "" || host.Nested.Depth != 0 ||
		host.Nested.MaxDepth != 0 {
		t.Fatalf("expected zero host context, got %#v", host)
	}
}

func TestRunAgentToolMarksStructuredFailuresAsToolErrors(t *testing.T) {
	t.Parallel()

	server := NewServer()
	handler := server.runAgentTool(HostContext{})

	result, output, err := handler(context.Background(), nil, RunAgentRequest{
		Name:  "missing-agent",
		Input: "delegate this",
	})
	if err != nil {
		t.Fatalf("run agent tool: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected tool error result, got %#v", result)
	}
	if output.Success {
		t.Fatalf("expected structured failure output, got %#v", output)
	}
	if output.Name != "missing-agent" {
		t.Fatalf("unexpected structured output: %#v", output)
	}
}

func TestNewServerAppliesOptionsAndDefaults(t *testing.T) {
	t.Parallel()

	customEngine := NewEngine()
	customImpl := &mcp.Implementation{Name: "custom", Version: "1.2.3"}

	server := NewServer(WithEngine(customEngine), WithImplementation(customImpl))
	if server.engineOrDefault() != customEngine {
		t.Fatalf("expected custom engine, got %#v", server.engine)
	}
	if server.impl() != customImpl {
		t.Fatalf("expected custom implementation, got %#v", server.implementation)
	}

	defaultServer := NewServer()
	if defaultServer.engineOrDefault() == nil {
		t.Fatal("expected default engine")
	}
	if defaultServer.impl().Name != reusableagents.ReservedMCPServerName {
		t.Fatalf("unexpected default implementation: %#v", defaultServer.impl())
	}
}

func TestLoadHostContextFromEnvUsesProcessEnvironment(t *testing.T) {
	raw, err := json.Marshal(reusableagents.ReservedServerRuntimeContext{
		BaseRuntime: reusableagents.NestedBaseRuntime{WorkspaceRoot: "/tmp/workspace", IDE: model.IDECodex},
		Nested:      reusableagents.NestedExecutionContext{Depth: 1, MaxDepth: 3},
	})
	if err != nil {
		t.Fatalf("marshal reserved payload: %v", err)
	}

	t.Setenv(reusableagents.RunAgentContextEnvVar, string(raw))

	host, err := LoadHostContextFromEnv()
	if err != nil {
		t.Fatalf("load host context from process env: %v", err)
	}
	if host.BaseRuntime.WorkspaceRoot != "/tmp/workspace" || host.Nested.Depth != 1 {
		t.Fatalf("unexpected host context from env: %#v", host)
	}
}

func TestServeStdioReturnsWhenContextIsCanceled(t *testing.T) {
	stdioSwapMu.Lock()
	defer stdioSwapMu.Unlock()

	stdinRead, stdinWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	defer stdinRead.Close()
	defer stdinWrite.Close()
	defer stdoutRead.Close()
	defer stdoutWrite.Close()

	originalStdin := os.Stdin
	originalStdout := os.Stdout
	os.Stdin = stdinRead
	os.Stdout = stdoutWrite
	defer func() {
		os.Stdin = originalStdin
		os.Stdout = originalStdout
	}()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeStdio(ctx, HostContext{})
	}()

	cancel()

	select {
	case serveErr := <-errCh:
		if serveErr != nil && !errors.Is(serveErr, context.Canceled) {
			t.Fatalf("expected canceled stdio server to exit cleanly, got %v", serveErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ServeStdio did not exit after context cancellation")
	}
}
