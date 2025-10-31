package agent

import (
	"context"
	"errors"
	"testing"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/tool"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
)

func TestNew(t *testing.T) {
	t.Run("Should create agent with minimal configuration", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test-agent",
			WithInstructions("You are a helpful assistant"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected config, got nil")
		}
		if cfg.ID != "test-agent" {
			t.Errorf("expected ID 'test-agent', got '%s'", cfg.ID)
		}
		if cfg.Instructions != "You are a helpful assistant" {
			t.Errorf("expected instructions, got '%s'", cfg.Instructions)
		}
		if cfg.Resource != string(core.ConfigAgent) {
			t.Errorf("expected resource 'agent', got '%s'", cfg.Resource)
		}
	})
	t.Run("Should trim whitespace from ID and instructions", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "  test-agent  ",
			WithInstructions("  You are helpful  "),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ID != "test-agent" {
			t.Errorf("expected trimmed ID 'test-agent', got '%s'", cfg.ID)
		}
		if cfg.Instructions != "You are helpful" {
			t.Errorf("expected trimmed instructions, got '%s'", cfg.Instructions)
		}
	})
	t.Run("Should fail when context is nil", func(t *testing.T) {
		var nilCtx context.Context
		_, err := New(nilCtx, "test-agent",
			WithInstructions("Test"),
		)
		if err == nil {
			t.Fatal("expected error for nil context")
		}
		if err.Error() != "context is required" {
			t.Errorf("unexpected error message: %v", err)
		}
	})
	t.Run("Should fail when ID is empty", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "",
			WithInstructions("Test"),
		)
		if err == nil {
			t.Fatal("expected error for empty ID")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when instructions are empty", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test-agent")
		if err == nil {
			t.Fatal("expected error for empty instructions")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should create agent with all options", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "full-agent",
			WithInstructions("Complex agent"),
			WithTools([]tool.Config{{ID: "tool1"}}),
			WithMCPs([]mcp.Config{{ID: "mcp1"}}),
			WithMaxIterations(10),
			WithMemory([]core.MemoryReference{{ID: "mem1"}}),
			WithKnowledge([]core.KnowledgeBinding{{ID: "kb1"}}),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Tools) != 1 {
			t.Errorf("expected 1 tool, got %d", len(cfg.Tools))
		}
		if len(cfg.MCPs) != 1 {
			t.Errorf("expected 1 MCP, got %d", len(cfg.MCPs))
		}
		if cfg.MaxIterations != 10 {
			t.Errorf("expected max iterations 10, got %d", cfg.MaxIterations)
		}
		if len(cfg.Memory) != 1 {
			t.Errorf("expected 1 memory ref, got %d", len(cfg.Memory))
		}
		if len(cfg.Knowledge) != 1 {
			t.Errorf("expected 1 knowledge binding, got %d", len(cfg.Knowledge))
		}
	})
	t.Run("Should fail with multiple knowledge bindings", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test-agent",
			WithInstructions("Test"),
			WithKnowledge([]core.KnowledgeBinding{
				{ID: "kb1"},
				{ID: "kb2"},
			}),
		)
		if err == nil {
			t.Fatal("expected error for multiple knowledge bindings")
		}
	})
	t.Run("Should fail with empty knowledge binding ID", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test-agent",
			WithInstructions("Test"),
			WithKnowledge([]core.KnowledgeBinding{{ID: ""}}),
		)
		if err == nil {
			t.Fatal("expected error for empty knowledge binding ID")
		}
	})
	t.Run("Should fail with empty memory reference ID", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "test-agent",
			WithInstructions("Test"),
			WithMemory([]core.MemoryReference{{ID: ""}}),
		)
		if err == nil {
			t.Fatal("expected error for empty memory reference ID")
		}
	})
	t.Run("Should create deep copy of configuration", func(t *testing.T) {
		ctx := context.Background()
		cfg1, err := New(ctx, "test-agent",
			WithInstructions("Test"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cfg2 := &engineagent.Config{}
		*cfg2 = *cfg1
		cfg2.Instructions = "Modified"
		if cfg1.Instructions == "Modified" {
			t.Error("configuration was not deep copied")
		}
	})
}

func TestWithActions(t *testing.T) {
	t.Run("Should set actions", func(t *testing.T) {
		ctx := context.Background()
		action := &engineagent.ActionConfig{ID: "action1"}
		cfg, err := New(ctx, "test",
			WithInstructions("Test"),
			WithActions([]*engineagent.ActionConfig{action}),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Actions) != 1 {
			t.Errorf("expected 1 action, got %d", len(cfg.Actions))
		}
		if cfg.Actions[0].ID != "action1" {
			t.Errorf("expected action ID 'action1', got '%s'", cfg.Actions[0].ID)
		}
	})
}

func TestWithModel(t *testing.T) {
	t.Run("Should set model with ref", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test",
			WithInstructions("Test"),
			WithModel(engineagent.Model{Ref: "gpt-4"}),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.Model.HasRef() {
			t.Error("expected model to have ref")
		}
		if cfg.Model.Ref != "gpt-4" {
			t.Errorf("expected model ref 'gpt-4', got '%s'", cfg.Model.Ref)
		}
	})
	t.Run("Should set model with config", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "test",
			WithInstructions("Test"),
			WithModel(engineagent.Model{
				Config: core.ProviderConfig{
					Provider: core.ProviderOpenAI,
					Model:    "gpt-4",
				},
			}),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.Model.HasConfig() {
			t.Error("expected model to have config")
		}
		if cfg.Model.Config.Provider != core.ProviderOpenAI {
			t.Errorf("expected provider openai, got '%s'", cfg.Model.Config.Provider)
		}
	})
}
