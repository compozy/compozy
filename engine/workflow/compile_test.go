package workflow

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withCtx(t *testing.T) context.Context {
	t.Helper()
	// Attach test logger to context to satisfy logging requirements
	return logger.ContextWithLogger(context.Background(), logger.NewForTests())
}

func TestCompile_ResolveAgentSelector(t *testing.T) {
	t.Run("Should resolve agent by ID and deepcopy into task", func(t *testing.T) {
		ctx := withCtx(t)
		store := resources.NewMemoryResourceStore()
		proj := &project.Config{Name: "demo"}
		// Put agent into store
		a := &agent.Config{
			ID:           "writer",
			Instructions: "You are a writer.",
			Config:       core.ProviderConfig{Provider: core.ProviderAnthropic, Model: "claude-3-haiku"},
		}
		_, err := store.Put(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceAgent, ID: "writer"}, a)
		require.NoError(t, err)
		wf := &Config{
			ID: "wf1",
			Tasks: []task.Config{
				{BaseConfig: task.BaseConfig{ID: "t1", Type: task.TaskTypeBasic, Agent: &agent.Config{ID: "writer"}}},
			},
		}
		compiled, err := wf.Compile(ctx, proj, store)
		require.NoError(t, err)
		require.Len(t, compiled.Tasks, 1)
		got := compiled.Tasks[0].Agent
		require.NotNil(t, got)
		assert.Equal(t, "writer", got.ID)
		assert.Equal(t, core.ProviderAnthropic, got.Config.Provider)
		assert.Equal(t, "claude-3-haiku", got.Config.Model)
		// Ensure deep copy (mutate compiled copy should not affect store value)
		got.Config.Model = "modified"
		v, _, err := store.Get(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceAgent, ID: "writer"})
		require.NoError(t, err)
		original := v.(*agent.Config)
		assert.Equal(t, "claude-3-haiku", original.Config.Model)
	})
}

func TestCompile_ResolveToolSelector(t *testing.T) {
	t.Run("Should resolve tool by ID and deepcopy into task", func(t *testing.T) {
		ctx := withCtx(t)
		store := resources.NewMemoryResourceStore()
		proj := &project.Config{Name: "demo"}
		tl := &tool.Config{ID: "fmt", Description: "format code"}
		_, err := store.Put(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceTool, ID: "fmt"}, tl)
		require.NoError(t, err)
		wf := &Config{
			ID: "wf2",
			Tasks: []task.Config{
				{BaseConfig: task.BaseConfig{ID: "t1", Type: task.TaskTypeBasic, Tool: &tool.Config{ID: "fmt"}}},
			},
		}
		compiled, err := wf.Compile(ctx, proj, store)
		require.NoError(t, err)
		got := compiled.Tasks[0].Tool
		require.NotNil(t, got)
		assert.Equal(t, "fmt", got.ID)
		assert.Equal(t, "format code", got.Description)
	})
}

func TestCompile_BasicSelectorValidation(t *testing.T) {
	t.Run("Should error when both agent and tool set on basic task", func(t *testing.T) {
		ctx := withCtx(t)
		store := resources.NewMemoryResourceStore()
		proj := &project.Config{Name: "demo"}
		wf := &Config{
			ID: "wf3",
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:    "t1",
						Type:  task.TaskTypeBasic,
						Agent: &agent.Config{ID: "a"},
						Tool:  &tool.Config{ID: "t"},
					},
				},
			},
		}
		_, err := wf.Compile(ctx, proj, store)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exactly one of agent/agent_ref or tool/tool_ref is required")
	})
	t.Run("Should error when neither agent nor tool set on basic task", func(t *testing.T) {
		ctx := withCtx(t)
		store := resources.NewMemoryResourceStore()
		proj := &project.Config{Name: "demo"}
		wf := &Config{
			ID:    "wf4",
			Tasks: []task.Config{{BaseConfig: task.BaseConfig{ID: "t1", Type: task.TaskTypeBasic}}},
		}
		_, err := wf.Compile(ctx, proj, store)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exactly one of agent/agent_ref or tool/tool_ref is required")
	})
}
