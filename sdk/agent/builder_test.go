package agent

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	sdkknowledge "github.com/compozy/compozy/sdk/knowledge"
	sdkmemory "github.com/compozy/compozy/sdk/memory"
)

func TestNewAgentBuilderTrimsID(t *testing.T) {
	t.Parallel()
	builder := New("  agent-alpha  ")
	require.NotNil(t, builder)
	require.Equal(t, "agent-alpha", builder.config.ID)
	require.Equal(t, string(core.ConfigAgent), builder.config.Resource)
}

func TestWithModelSetsInlineConfig(t *testing.T) {
	t.Parallel()
	builder := New("agent").WithModel("OpenAI", "gpt-4o").WithInstructions("do things")
	builder.AddAction(&engineagent.ActionConfig{ID: "act", Prompt: "prompt"})
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Equal(t, "openai", string(cfg.Model.Config.Provider))
	require.Equal(t, "gpt-4o", cfg.Model.Config.Model)
	require.False(t, cfg.Model.HasRef())
}

func TestWithModelRefSetsReference(t *testing.T) {
	t.Parallel()
	builder := New("agent").WithModelRef("model-ref").WithInstructions("run it")
	builder.AddAction(&engineagent.ActionConfig{ID: "act", Prompt: "prompt"})
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Equal(t, "model-ref", cfg.Model.Ref)
	require.True(t, cfg.Model.HasRef())
}

func TestWithInstructionsRejectsEmpty(t *testing.T) {
	t.Parallel()
	builder := New("agent").WithModelRef("model")
	builder.WithInstructions("   ")
	builder.AddAction(&engineagent.ActionConfig{ID: "act", Prompt: "prompt"})
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.NotEmpty(t, buildErr.Error())
}

func TestWithKnowledgeStoresBinding(t *testing.T) {
	t.Parallel()
	binding := &sdkknowledge.BindingConfig{ID: "kb-alpha"}
	builder := New("agent").WithModelRef("model").WithInstructions("ready").WithKnowledge(binding)
	builder.AddAction(&engineagent.ActionConfig{ID: "act", Prompt: "prompt"})
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Len(t, cfg.Knowledge, 1)
	require.Equal(t, "kb-alpha", cfg.Knowledge[0].ID)
}

func TestWithMemoryAppendsReference(t *testing.T) {
	t.Parallel()
	mem := &sdkmemory.ReferenceConfig{ID: "session", Mode: "read-write"}
	builder := New("agent").WithModelRef("model").WithInstructions("ready").WithMemory(mem)
	builder.AddAction(&engineagent.ActionConfig{ID: "act", Prompt: "prompt"})
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Len(t, cfg.Memory, 1)
	require.Equal(t, "session", cfg.Memory[0].ID)
	require.Equal(t, "read-write", cfg.Memory[0].Mode)
}

func TestAddActionClonesConfiguration(t *testing.T) {
	t.Parallel()
	action := &engineagent.ActionConfig{ID: "step", Prompt: "do"}
	builder := New("agent").WithModelRef("model").WithInstructions("ok").AddAction(action)
	action.Prompt = "changed"
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Len(t, cfg.Actions, 1)
	require.Equal(t, "do", cfg.Actions[0].Prompt)
}

func TestAddToolAppendsToolReference(t *testing.T) {
	t.Parallel()
	builder := New("agent").WithModelRef("model").WithInstructions("ok").AddTool("fs-reader")
	builder.AddAction(&engineagent.ActionConfig{ID: "act", Prompt: "prompt"})
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Len(t, cfg.Tools, 1)
	require.Equal(t, "fs-reader", cfg.Tools[0].ID)
}

func TestAddMCPAccumulatesReferences(t *testing.T) {
	t.Parallel()
	builder := New("agent").WithModelRef("model").WithInstructions("ok").AddMCP("filesystem")
	builder.AddMCP("github")
	builder.AddAction(&engineagent.ActionConfig{ID: "act", Prompt: "prompt"})
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Len(t, cfg.MCPs, 2)
	require.Equal(t, "filesystem", cfg.MCPs[0].ID)
	require.Equal(t, "github", cfg.MCPs[1].ID)
}

func TestBuildFailsWhenIDMissing(t *testing.T) {
	t.Parallel()
	builder := New("   ").WithModelRef("model").WithInstructions("ok")
	builder.AddAction(&engineagent.ActionConfig{ID: "act", Prompt: "prompt"})
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.True(t, errors.Is(err, buildErr))
}

func TestBuildFailsWhenModelMissing(t *testing.T) {
	t.Parallel()
	builder := New("agent").WithInstructions("ok")
	builder.AddAction(&engineagent.ActionConfig{ID: "act", Prompt: "prompt"})
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)
}

func TestBuildSucceedsWithValidConfig(t *testing.T) {
	t.Parallel()
	action := &engineagent.ActionConfig{ID: "act", Prompt: "prompt"}
	builder := New("agent").WithModelRef("model").WithInstructions("do work").AddAction(action)
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "agent", cfg.ID)
	require.Len(t, cfg.Actions, 1)
}
