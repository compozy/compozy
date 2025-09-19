package workflow

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
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
			Model: agent.Model{
				Config: core.ProviderConfig{Provider: core.ProviderAnthropic, Model: "claude-3-haiku"},
			},
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
		assert.Equal(t, core.ProviderAnthropic, got.Model.Config.Provider)
		assert.Equal(t, "claude-3-haiku", got.Model.Config.Model)
		// Ensure deep copy (mutate compiled copy should not affect store value)
		got.Model.Config.Model = "modified"
		v, _, err := store.Get(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceAgent, ID: "writer"})
		require.NoError(t, err)
		original := v.(*agent.Config)
		assert.Equal(t, "claude-3-haiku", original.Model.Config.Model)
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
		assert.Contains(t, err.Error(), "exactly one of agent or tool is required")
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
		assert.Contains(t, err.Error(), "exactly one of agent or tool is required")
	})
}

func TestCompile_SuggestNearestAgentIDs(t *testing.T) {
	t.Run("Should suggest nearest agent IDs on not found", func(t *testing.T) {
		ctx := withCtx(t)
		store := resources.NewMemoryResourceStore()
		proj := &project.Config{Name: "demo"}
		a1 := &agent.Config{
			ID:           "writer",
			Instructions: "You are a writer.",
			Model: agent.Model{
				Config: core.ProviderConfig{Provider: core.ProviderAnthropic, Model: "claude-3-haiku"},
			},
		}
		a2 := &agent.Config{
			ID:           "writer_pro",
			Instructions: "Pro writer.",
			Model: agent.Model{
				Config: core.ProviderConfig{Provider: core.ProviderAnthropic, Model: "claude-3-sonnet"},
			},
		}
		_, err := store.Put(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceAgent, ID: a1.ID}, a1)
		require.NoError(t, err)
		_, err = store.Put(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceAgent, ID: a2.ID}, a2)
		require.NoError(t, err)
		wf := &Config{
			ID: "wf_suggest",
			Tasks: []task.Config{
				{BaseConfig: task.BaseConfig{ID: "t1", Type: task.TaskTypeBasic, Agent: &agent.Config{ID: "writr"}}},
			},
		}
		_, err = wf.Compile(ctx, proj, store)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Did you mean")
		require.Contains(t, err.Error(), "writer")
	})
}

func TestSchemaIDLinking_FromMap(t *testing.T) {
	ctx := withCtx(t)
	store := resources.NewMemoryResourceStore()

	// Project with a reusable schema indexed by ID
	proj := &project.Config{
		Name: "proj1",
		Schemas: []schema.Schema{
			{
				"id":   "my_schema",
				"type": "object",
				"properties": map[string]any{
					"x": map[string]any{"type": "string"},
				},
				"required": []any{"x"},
			},
		},
	}
	require.NoError(t, proj.IndexToResourceStore(ctx, store))

	// Tool decoded from map with input: "my_schema"
	var tl tool.Config
	require.NoError(t, tl.FromMap(map[string]any{
		"id":          "t1",
		"description": "test tool",
		"input":       "my_schema",
	}))

	// Agent action decoded from map with input: "my_schema"
	var act agent.ActionConfig
	require.NoError(t, act.FromMap(map[string]any{
		"id":        "act",
		"prompt":    "Do something",
		"json_mode": true,
		"input":     "my_schema",
	}))
	ag := agent.Config{
		ID:           "a1",
		Instructions: "You are helpful",
		Model:        agent.Model{Config: core.ProviderConfig{Provider: core.ProviderOpenAI, Model: "gpt-4o-mini"}},
		Actions:      []*agent.ActionConfig{&act},
	}

	// Task decoded from map with input: "my_schema" and tool selector
	var tTool task.Config
	require.NoError(t, tTool.FromMap(map[string]any{
		"id":    "t_tool",
		"type":  string(task.TaskTypeBasic),
		"tool":  map[string]any{"id": "t1"},
		"input": "my_schema",
	}))

	// Second task using the agent to exercise agent action schema linking
	tAgent := task.Config{
		BaseConfig: task.BaseConfig{ID: "t_agent", Type: task.TaskTypeBasic, Agent: &agent.Config{ID: "a1"}},
		BasicTask:  task.BasicTask{Action: "act"},
	}

	wf := &Config{
		ID:     "wf_link",
		Tools:  []tool.Config{tl},
		Agents: []agent.Config{ag},
		Tasks:  []task.Config{tTool, tAgent},
	}

	// Index workflow-scoped resources (agents/tools) so selectors can resolve
	require.NoError(t, wf.IndexToResourceStore(ctx, proj.Name, store))

	compiled, err := wf.Compile(ctx, proj, store)
	require.NoError(t, err)
	require.Len(t, compiled.Tasks, 2)

	// Task-level input schema should be linked (no longer a ref)
	ts := compiled.Tasks[0]
	require.NotNil(t, ts.InputSchema)
	isRef, _ := ts.InputSchema.IsRef()
	assert.False(t, isRef)
	assert.Equal(t, "object", (*ts.InputSchema)["type"]) // materialized schema

	// Tool input schema should be linked on the resolved tool
	require.NotNil(t, ts.Tool)
	require.NotNil(t, ts.Tool.InputSchema)
	isRefTool, _ := ts.Tool.InputSchema.IsRef()
	assert.False(t, isRefTool)
	assert.Equal(t, "object", (*ts.Tool.InputSchema)["type"])

	// Agent action input schema should be linked on the resolved agent
	ta := compiled.Tasks[1]
	require.NotNil(t, ta.Agent)
	require.NotEmpty(t, ta.Agent.Actions)
	require.NotNil(t, ta.Agent.Actions[0].InputSchema)
	isRefAct, _ := ta.Agent.Actions[0].InputSchema.IsRef()
	assert.False(t, isRefAct)
	assert.Equal(t, "object", (*ta.Agent.Actions[0].InputSchema)["type"])
}

func TestCompile_TaskModelPrecedence(t *testing.T) {
	t.Run("task.model overrides agent default and global", func(t *testing.T) {
		ctx := withCtx(t)
		store := resources.NewMemoryResourceStore()
		proj := &project.Config{Name: "demo"}
		defaultAgent := &agent.Config{
			ID:           "writer",
			Instructions: "You are a writer.",
			Model:        agent.Model{Config: core.ProviderConfig{Provider: core.ProviderOpenAI, Model: "gpt-4o"}},
		}
		_, err := store.Put(
			ctx,
			resources.ResourceKey{Project: "demo", Type: resources.ResourceAgent, ID: defaultAgent.ID},
			defaultAgent,
		)
		require.NoError(t, err)
		modelFast := &core.ProviderConfig{Provider: core.ProviderAnthropic, Model: "claude-3-5-haiku"}
		_, err = store.Put(
			ctx,
			resources.ResourceKey{Project: "demo", Type: resources.ResourceModel, ID: "fast"},
			modelFast,
		)
		require.NoError(t, err)
		wf := &Config{
			ID: "wf_model_prec",
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:    "t1",
						Type:  task.TaskTypeBasic,
						Agent: &agent.Config{ID: "writer", Model: agent.Model{Ref: "fast"}},
					},
				},
			},
		}
		compiled, err := wf.Compile(ctx, proj, store)
		require.NoError(t, err)
		got := compiled.Tasks[0].Agent
		require.NotNil(t, got)
		assert.Equal(t, core.ProviderAnthropic, got.Model.Config.Provider)
		assert.Equal(t, "claude-3-5-haiku", got.Model.Config.Model)
	})
}

func TestCompile_SuggestNearestToolIDs(t *testing.T) {
	t.Run("Should suggest nearest tool IDs on not found", func(t *testing.T) {
		ctx := withCtx(t)
		store := resources.NewMemoryResourceStore()
		proj := &project.Config{Name: "demo"}
		tl1 := &tool.Config{ID: "format", Description: "format code"}
		_, err := store.Put(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceTool, ID: tl1.ID}, tl1)
		require.NoError(t, err)
		wf := &Config{
			ID: "wf_tool_suggest",
			Tasks: []task.Config{
				{BaseConfig: task.BaseConfig{ID: "t1", Type: task.TaskTypeBasic, Tool: &tool.Config{ID: "frmat"}}},
			},
		}
		_, err = wf.Compile(ctx, proj, store)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Did you mean")
		require.Contains(t, err.Error(), "format")
	})
}

func TestCompile_SuggestNearestModelIDs(t *testing.T) {
	t.Run("Should suggest nearest model IDs when task.model not found", func(t *testing.T) {
		ctx := withCtx(t)
		store := resources.NewMemoryResourceStore()
		proj := &project.Config{Name: "demo"}
		a := &agent.Config{
			ID:           "writer",
			Instructions: "You are a writer.",
			Model:        agent.Model{Config: core.ProviderConfig{Provider: core.ProviderOpenAI, Model: "gpt-4o"}},
		}
		_, err := store.Put(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceAgent, ID: a.ID}, a)
		require.NoError(t, err)
		model := &core.ProviderConfig{Provider: core.ProviderAnthropic, Model: "claude-3-5-haiku"}
		_, err = store.Put(
			ctx,
			resources.ResourceKey{Project: "demo", Type: resources.ResourceModel, ID: "fast"},
			model,
		)
		require.NoError(t, err)
		wf := &Config{
			ID: "wf_model_suggest",
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:    "t1",
						Type:  task.TaskTypeBasic,
						Agent: &agent.Config{ID: "writer", Model: agent.Model{Ref: "fas"}},
					},
				},
			},
		}
		_, err = wf.Compile(ctx, proj, store)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Did you mean")
		require.Contains(t, err.Error(), "fast")
	})
}

func TestCompile_ResolveMCPSelector(t *testing.T) {
	t.Run("Should resolve MCP by ID and deepcopy into agent", func(t *testing.T) {
		ctx := withCtx(t)
		store := resources.NewMemoryResourceStore()
		proj := &project.Config{Name: "demo"}

		// MCP proxy is required by mcp.Config.Validate
		require.NoError(t, os.Setenv("MCP_PROXY_URL", "http://localhost:4000/proxy"))
		defer os.Unsetenv("MCP_PROXY_URL")

		// Put MCP into store
		mc := &mcp.Config{ID: "srv", URL: "http://localhost:3000/mcp"}
		_, err := store.Put(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceMCP, ID: mc.ID}, mc)
		require.NoError(t, err)

		// Inline agent that references MCP selector by ID
		wf := &Config{
			ID: "wf_mcp_1",
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "t1",
						Type: task.TaskTypeBasic,
						Agent: &agent.Config{
							ID:           "inline-agent",
							Instructions: "You are helpful.",
							Model: agent.Model{
								Config: core.ProviderConfig{Provider: core.ProviderOpenAI, Model: "gpt-4o-mini"},
							},
							LLMProperties: agent.LLMProperties{
								MCPs: []mcp.Config{{ID: "srv"}},
							},
						},
					},
				},
			},
		}

		compiled, err := wf.Compile(ctx, proj, store)
		require.NoError(t, err)
		got := compiled.Tasks[0].Agent
		require.NotNil(t, got)
		require.Len(t, got.MCPs, 1)
		// Ensure deep copy: mutate compiled copy should not affect store value
		got.MCPs[0].URL = "http://modified:9999"
		v, _, err := store.Get(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceMCP, ID: "srv"})
		require.NoError(t, err)
		original := v.(*mcp.Config)
		assert.Equal(t, "http://localhost:3000/mcp", original.URL)
		// Defaults applied
		assert.NotEmpty(t, got.MCPs[0].Proto)
		assert.NotEmpty(t, got.MCPs[0].Transport)
	})

	t.Run("Should suggest nearest MCP IDs on not found", func(t *testing.T) {
		ctx := withCtx(t)
		store := resources.NewMemoryResourceStore()
		proj := &project.Config{Name: "demo"}
		require.NoError(t, os.Setenv("MCP_PROXY_URL", "http://localhost:4000/proxy"))
		defer os.Unsetenv("MCP_PROXY_URL")

		// Seed store with some MCPs for suggestions
		m1 := &mcp.Config{ID: "server", URL: "http://localhost:3001/mcp"}
		m2 := &mcp.Config{ID: "srv-alpha", URL: "http://localhost:3002/mcp"}
		_, err := store.Put(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceMCP, ID: m1.ID}, m1)
		require.NoError(t, err)
		_, err = store.Put(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceMCP, ID: m2.ID}, m2)
		require.NoError(t, err)

		wf := &Config{
			ID: "wf_mcp_suggest",
			Tasks: []task.Config{{
				BaseConfig: task.BaseConfig{
					ID:   "t1",
					Type: task.TaskTypeBasic,
					Agent: &agent.Config{
						ID:           "inline-agent",
						Instructions: "Helper",
						Model: agent.Model{
							Config: core.ProviderConfig{Provider: core.ProviderOpenAI, Model: "gpt-4o-mini"},
						},
						LLMProperties: agent.LLMProperties{
							MCPs: []mcp.Config{{ID: "svr"}},
						},
					},
				},
			}},
		}

		_, err = wf.Compile(ctx, proj, store)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Did you mean")
		// Expect one of the known IDs in the suggestion list
		require.True(t, strings.Contains(err.Error(), "server") || strings.Contains(err.Error(), "srv-alpha"))
	})
}
