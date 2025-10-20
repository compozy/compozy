package resourceutil

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockResourceStore struct {
	mock.Mock
}

func (m *MockResourceStore) Get(
	ctx context.Context,
	key resources.ResourceKey,
) (any, resources.ETag, error) {
	args := m.Called(ctx, key)
	return args.Get(0), args.Get(1).(resources.ETag), args.Error(2)
}

func (m *MockResourceStore) Put(
	ctx context.Context,
	key resources.ResourceKey,
	value any,
) (resources.ETag, error) {
	args := m.Called(ctx, key, value)
	return args.Get(0).(resources.ETag), args.Error(1)
}

func (m *MockResourceStore) Delete(
	ctx context.Context,
	key resources.ResourceKey,
) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockResourceStore) List(
	ctx context.Context,
	project string,
	resourceType resources.ResourceType,
) ([]resources.ResourceKey, error) {
	args := m.Called(ctx, project, resourceType)
	return args.Get(0).([]resources.ResourceKey), args.Error(1)
}

func (m *MockResourceStore) ListWithValues(
	ctx context.Context,
	project string,
	resourceType resources.ResourceType,
) ([]resources.StoredItem, error) {
	args := m.Called(ctx, project, resourceType)
	return args.Get(0).([]resources.StoredItem), args.Error(1)
}

func (m *MockResourceStore) GetBulk(
	ctx context.Context,
	keys []resources.ResourceKey,
) (map[resources.ResourceKey]resources.StoredItem, error) {
	args := m.Called(ctx, keys)
	return args.Get(0).(map[resources.ResourceKey]resources.StoredItem), args.Error(1)
}

func (m *MockResourceStore) PutIfMatch(
	ctx context.Context,
	key resources.ResourceKey,
	value any,
	expectedETag resources.ETag,
) (resources.ETag, error) {
	args := m.Called(ctx, key, value, expectedETag)
	return args.Get(0).(resources.ETag), args.Error(1)
}

func (m *MockResourceStore) Watch(
	ctx context.Context,
	project string,
	resourceType resources.ResourceType,
) (<-chan resources.Event, error) {
	args := m.Called(ctx, project, resourceType)
	return args.Get(0).(<-chan resources.Event), args.Error(1)
}

func (m *MockResourceStore) ListWithValuesPage(
	ctx context.Context,
	project string,
	resourceType resources.ResourceType,
	offset, limit int,
) ([]resources.StoredItem, int, error) {
	args := m.Called(ctx, project, resourceType, offset, limit)
	return args.Get(0).([]resources.StoredItem), args.Int(1), args.Error(2)
}

func (m *MockResourceStore) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestWorkflowsReferencingAgent(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find workflows referencing agent", func(t *testing.T) {
		store := new(MockResourceStore)
		wf1 := &workflow.Config{
			ID: "wf1",
			Agents: []agent.Config{
				{ID: "agent1"},
			},
		}
		wf2 := &workflow.Config{
			ID: "wf2",
			Agents: []agent.Config{
				{ID: "agent2"},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceWorkflow).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "wf1"}, Value: wf1},
				{Key: resources.ResourceKey{ID: "wf2"}, Value: wf2},
			}, nil)
		refs, err := WorkflowsReferencingAgent(ctx, store, "project1", "agent1")
		require.NoError(t, err)
		assert.Equal(t, []string{"wf1"}, refs)
	})

	t.Run("Should return empty when no references", func(t *testing.T) {
		store := new(MockResourceStore)
		wf1 := &workflow.Config{
			ID: "wf1",
			Agents: []agent.Config{
				{ID: "agent2"},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceWorkflow).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "wf1"}, Value: wf1},
			}, nil)
		refs, err := WorkflowsReferencingAgent(ctx, store, "project1", "agent1")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})

	t.Run("Should handle store errors", func(t *testing.T) {
		store := new(MockResourceStore)
		storeErr := errors.New("store error")
		store.On("ListWithValues", ctx, "project1", resources.ResourceWorkflow).
			Return([]resources.StoredItem{}, storeErr)
		refs, err := WorkflowsReferencingAgent(ctx, store, "project1", "agent1")
		assert.ErrorIs(t, err, storeErr)
		assert.Nil(t, refs)
	})

	t.Run("Should handle whitespace in agent IDs", func(t *testing.T) {
		store := new(MockResourceStore)
		wf1 := &workflow.Config{
			ID: "wf1",
			Agents: []agent.Config{
				{ID: "  agent1  "},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceWorkflow).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "wf1"}, Value: wf1},
			}, nil)
		refs, err := WorkflowsReferencingAgent(ctx, store, "project1", "agent1")
		require.NoError(t, err)
		assert.Equal(t, []string{"wf1"}, refs)
	})
}

func TestWorkflowsReferencingTool(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find workflows referencing tool", func(t *testing.T) {
		store := new(MockResourceStore)
		wf1 := &workflow.Config{
			ID: "wf1",
			Tools: []tool.Config{
				{ID: "tool1"},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceWorkflow).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "wf1"}, Value: wf1},
			}, nil)
		refs, err := WorkflowsReferencingTool(ctx, store, "project1", "tool1")
		require.NoError(t, err)
		assert.Equal(t, []string{"wf1"}, refs)
	})

	t.Run("Should return empty when no tool references", func(t *testing.T) {
		store := new(MockResourceStore)
		wf1 := &workflow.Config{ID: "wf1", Tools: []tool.Config{}}
		store.On("ListWithValues", ctx, "project1", resources.ResourceWorkflow).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "wf1"}, Value: wf1},
			}, nil)
		refs, err := WorkflowsReferencingTool(ctx, store, "project1", "tool1")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})
}

func TestWorkflowsReferencingTask(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find workflows referencing task", func(t *testing.T) {
		store := new(MockResourceStore)
		wf1 := &workflow.Config{
			ID: "wf1",
			Tasks: []task.Config{
				{BaseConfig: task.BaseConfig{ID: "task1"}},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceWorkflow).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "wf1"}, Value: wf1},
			}, nil)
		refs, err := WorkflowsReferencingTask(ctx, store, "project1", "task1")
		require.NoError(t, err)
		assert.Equal(t, []string{"wf1"}, refs)
	})

	t.Run("Should handle multiple tasks in workflow", func(t *testing.T) {
		store := new(MockResourceStore)
		wf1 := &workflow.Config{
			ID: "wf1",
			Tasks: []task.Config{
				{BaseConfig: task.BaseConfig{ID: "task1"}},
				{BaseConfig: task.BaseConfig{ID: "task2"}},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceWorkflow).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "wf1"}, Value: wf1},
			}, nil)
		refs, err := WorkflowsReferencingTask(ctx, store, "project1", "task1")
		require.NoError(t, err)
		assert.Equal(t, []string{"wf1"}, refs)
	})
}

func TestWorkflowsReferencingMCP(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find workflows referencing MCP", func(t *testing.T) {
		store := new(MockResourceStore)
		wf1 := &workflow.Config{
			ID: "wf1",
			MCPs: []mcp.Config{
				{ID: "mcp1"},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceWorkflow).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "wf1"}, Value: wf1},
			}, nil)
		refs, err := WorkflowsReferencingMCP(ctx, store, "project1", "mcp1")
		require.NoError(t, err)
		assert.Equal(t, []string{"wf1"}, refs)
	})

	t.Run("Should return empty when MCP ID is empty", func(t *testing.T) {
		store := new(MockResourceStore)
		refs, err := WorkflowsReferencingMCP(ctx, store, "project1", "")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})

	t.Run("Should handle whitespace in MCP ID", func(t *testing.T) {
		store := new(MockResourceStore)
		refs, err := WorkflowsReferencingMCP(ctx, store, "project1", "  ")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})
}

func TestWorkflowsReferencingKnowledgeBase(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find workflows with knowledge at workflow level", func(t *testing.T) {
		store := new(MockResourceStore)
		wf1 := &workflow.Config{
			ID: "wf1",
			Knowledge: []core.KnowledgeBinding{
				{ID: "kb1"},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceWorkflow).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "wf1"}, Value: wf1},
			}, nil)
		refs, err := WorkflowsReferencingKnowledgeBase(ctx, store, "project1", "kb1")
		require.NoError(t, err)
		assert.Equal(t, []string{"wf1"}, refs)
	})

	t.Run("Should find workflows with knowledge at task level", func(t *testing.T) {
		store := new(MockResourceStore)
		wf1 := &workflow.Config{
			ID: "wf1",
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID: "task1",
						Knowledge: []core.KnowledgeBinding{
							{ID: "kb1"},
						},
					},
				},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceWorkflow).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "wf1"}, Value: wf1},
			}, nil)
		refs, err := WorkflowsReferencingKnowledgeBase(ctx, store, "project1", "kb1")
		require.NoError(t, err)
		assert.Equal(t, []string{"wf1"}, refs)
	})

	t.Run("Should find workflows with knowledge at agent level", func(t *testing.T) {
		store := new(MockResourceStore)
		wf1 := &workflow.Config{
			ID: "wf1",
			Agents: []agent.Config{
				{
					ID: "agent1",
					Knowledge: []core.KnowledgeBinding{
						{ID: "kb1"},
					},
				},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceWorkflow).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "wf1"}, Value: wf1},
			}, nil)
		refs, err := WorkflowsReferencingKnowledgeBase(ctx, store, "project1", "kb1")
		require.NoError(t, err)
		assert.Equal(t, []string{"wf1"}, refs)
	})

	t.Run("Should return empty when KB ID is empty", func(t *testing.T) {
		store := new(MockResourceStore)
		refs, err := WorkflowsReferencingKnowledgeBase(ctx, store, "project1", "")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})
}

func TestAgentsReferencingKnowledgeBase(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find agents referencing knowledge base", func(t *testing.T) {
		store := new(MockResourceStore)
		ag1 := &agent.Config{
			ID: "agent1",
			Knowledge: []core.KnowledgeBinding{
				{ID: "kb1"},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceAgent).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "agent1"}, Value: ag1},
			}, nil)
		refs, err := AgentsReferencingKnowledgeBase(ctx, store, "project1", "kb1")
		require.NoError(t, err)
		assert.Equal(t, []string{"agent1"}, refs)
	})

	t.Run("Should return empty when KB ID is empty", func(t *testing.T) {
		store := new(MockResourceStore)
		refs, err := AgentsReferencingKnowledgeBase(ctx, store, "project1", "")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})

	t.Run("Should handle agent decode errors", func(t *testing.T) {
		store := new(MockResourceStore)
		store.On("ListWithValues", ctx, "project1", resources.ResourceAgent).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "agent1"}, Value: "invalid"},
			}, nil)
		refs, err := AgentsReferencingKnowledgeBase(ctx, store, "project1", "kb1")
		assert.Error(t, err)
		assert.Nil(t, refs)
	})
}

func TestTasksReferencingKnowledgeBase(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find tasks referencing knowledge base", func(t *testing.T) {
		store := new(MockResourceStore)
		tk1 := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "task1",
				Knowledge: []core.KnowledgeBinding{
					{ID: "kb1"},
				},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceTask).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "task1"}, Value: tk1},
			}, nil)
		refs, err := TasksReferencingKnowledgeBase(ctx, store, "project1", "kb1")
		require.NoError(t, err)
		assert.Equal(t, []string{"task1"}, refs)
	})

	t.Run("Should return empty when KB ID is empty", func(t *testing.T) {
		store := new(MockResourceStore)
		refs, err := TasksReferencingKnowledgeBase(ctx, store, "project1", "")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})
}

func TestProjectReferencesKnowledgeBase(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should return true when project references knowledge base", func(t *testing.T) {
		store := new(MockResourceStore)
		proj := &project.Config{
			Name: "project1",
			Knowledge: []core.KnowledgeBinding{
				{ID: "kb1"},
			},
		}
		key := resources.ResourceKey{
			Project: "project1",
			Type:    resources.ResourceProject,
			ID:      "project1",
		}
		store.On("Get", ctx, key).Return(proj, resources.ETag(""), nil)
		hasRef, err := ProjectReferencesKnowledgeBase(ctx, store, "project1", "kb1")
		require.NoError(t, err)
		assert.True(t, hasRef)
	})

	t.Run("Should return false when project does not reference KB", func(t *testing.T) {
		store := new(MockResourceStore)
		proj := &project.Config{
			Name:      "project1",
			Knowledge: []core.KnowledgeBinding{},
		}
		key := resources.ResourceKey{
			Project: "project1",
			Type:    resources.ResourceProject,
			ID:      "project1",
		}
		store.On("Get", ctx, key).Return(proj, resources.ETag(""), nil)
		hasRef, err := ProjectReferencesKnowledgeBase(ctx, store, "project1", "kb1")
		require.NoError(t, err)
		assert.False(t, hasRef)
	})

	t.Run("Should return false when project not found", func(t *testing.T) {
		store := new(MockResourceStore)
		key := resources.ResourceKey{
			Project: "project1",
			Type:    resources.ResourceProject,
			ID:      "project1",
		}
		store.On("Get", ctx, key).Return(nil, resources.ETag(""), resources.ErrNotFound)
		hasRef, err := ProjectReferencesKnowledgeBase(ctx, store, "project1", "kb1")
		require.NoError(t, err)
		assert.False(t, hasRef)
	})

	t.Run("Should return empty when KB ID is empty", func(t *testing.T) {
		store := new(MockResourceStore)
		hasRef, err := ProjectReferencesKnowledgeBase(ctx, store, "project1", "")
		require.NoError(t, err)
		assert.False(t, hasRef)
	})
}

func TestWorkflowsReferencingSchema(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find workflows with input schema", func(t *testing.T) {
		store := new(MockResourceStore)
		inputSchema := schema.Schema{"__schema_ref__": "schema1"}
		wf1 := &workflow.Config{
			ID: "wf1",
			Opts: workflow.Opts{
				InputSchema: &inputSchema,
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceWorkflow).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "wf1"}, Value: wf1},
			}, nil)
		refs, err := WorkflowsReferencingSchema(ctx, store, "project1", "schema1")
		require.NoError(t, err)
		assert.Equal(t, []string{"wf1"}, refs)
	})

	t.Run("Should return empty when schema ID is empty", func(t *testing.T) {
		store := new(MockResourceStore)
		refs, err := WorkflowsReferencingSchema(ctx, store, "project1", "")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})
}

func TestAgentsReferencingSchema(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find agents with schema in actions", func(t *testing.T) {
		store := new(MockResourceStore)
		inputSchema := schema.Schema{"__schema_ref__": "schema1"}
		ag1 := &agent.Config{
			ID: "agent1",
			Actions: []*agent.ActionConfig{
				{InputSchema: &inputSchema},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceAgent).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "agent1"}, Value: ag1},
			}, nil)
		refs, err := AgentsReferencingSchema(ctx, store, "project1", "schema1")
		require.NoError(t, err)
		assert.Equal(t, []string{"agent1"}, refs)
	})

	t.Run("Should return empty when schema ID is empty", func(t *testing.T) {
		store := new(MockResourceStore)
		refs, err := AgentsReferencingSchema(ctx, store, "project1", "")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})
}

func TestTasksReferencingSchema(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find tasks with input schema", func(t *testing.T) {
		store := new(MockResourceStore)
		inputSchema := schema.Schema{"__schema_ref__": "schema1"}
		tk1 := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:          "task1",
				InputSchema: &inputSchema,
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceTask).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "task1"}, Value: tk1},
			}, nil)
		refs, err := TasksReferencingSchema(ctx, store, "project1", "schema1")
		require.NoError(t, err)
		assert.Equal(t, []string{"task1"}, refs)
	})

	t.Run("Should find tasks with output schema", func(t *testing.T) {
		store := new(MockResourceStore)
		outputSchema := schema.Schema{"__schema_ref__": "schema1"}
		tk1 := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:           "task1",
				OutputSchema: &outputSchema,
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceTask).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "task1"}, Value: tk1},
			}, nil)
		refs, err := TasksReferencingSchema(ctx, store, "project1", "schema1")
		require.NoError(t, err)
		assert.Equal(t, []string{"task1"}, refs)
	})

	t.Run("Should return empty when schema ID is empty", func(t *testing.T) {
		store := new(MockResourceStore)
		refs, err := TasksReferencingSchema(ctx, store, "project1", "")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})
}

func TestToolsReferencingSchema(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find tools with input schema", func(t *testing.T) {
		store := new(MockResourceStore)
		inputSchema := schema.Schema{"__schema_ref__": "schema1"}
		tl1 := &tool.Config{
			ID:          "tool1",
			InputSchema: &inputSchema,
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceTool).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "tool1"}, Value: tl1},
			}, nil)
		refs, err := ToolsReferencingSchema(ctx, store, "project1", "schema1")
		require.NoError(t, err)
		assert.Equal(t, []string{"tool1"}, refs)
	})

	t.Run("Should return empty when schema ID is empty", func(t *testing.T) {
		store := new(MockResourceStore)
		refs, err := ToolsReferencingSchema(ctx, store, "project1", "")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})
}

func TestAgentsReferencingModel(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find agents referencing model", func(t *testing.T) {
		store := new(MockResourceStore)
		ag1 := &agent.Config{
			ID:    "agent1",
			Model: agent.Model{Ref: "model1"},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceAgent).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "agent1"}, Value: ag1},
			}, nil)
		refs, err := AgentsReferencingModel(ctx, store, "project1", "model1")
		require.NoError(t, err)
		assert.Equal(t, []string{"agent1"}, refs)
	})

	t.Run("Should not find agents without model ref", func(t *testing.T) {
		store := new(MockResourceStore)
		ag1 := &agent.Config{
			ID:    "agent1",
			Model: agent.Model{},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceAgent).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "agent1"}, Value: ag1},
			}, nil)
		refs, err := AgentsReferencingModel(ctx, store, "project1", "model1")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})
}

func TestAgentsReferencingMCP(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find agents referencing MCP", func(t *testing.T) {
		store := new(MockResourceStore)
		ag1 := &agent.Config{
			ID: "agent1",
			LLMProperties: agent.LLMProperties{
				MCPs: []mcp.Config{
					{ID: "mcp1"},
				},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceAgent).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "agent1"}, Value: ag1},
			}, nil)
		refs, err := AgentsReferencingMCP(ctx, store, "project1", "mcp1")
		require.NoError(t, err)
		assert.Equal(t, []string{"agent1"}, refs)
	})

	t.Run("Should return empty when MCP ID is empty", func(t *testing.T) {
		store := new(MockResourceStore)
		refs, err := AgentsReferencingMCP(ctx, store, "project1", "")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})
}

func TestAgentsReferencingMemory(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find agents referencing memory", func(t *testing.T) {
		store := new(MockResourceStore)
		ag1 := &agent.Config{
			ID: "agent1",
			LLMProperties: agent.LLMProperties{
				Memory: []core.MemoryReference{
					{ID: "mem1"},
				},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceAgent).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "agent1"}, Value: ag1},
			}, nil)
		refs, err := AgentsReferencingMemory(ctx, store, "project1", "mem1")
		require.NoError(t, err)
		assert.Equal(t, []string{"agent1"}, refs)
	})

	t.Run("Should handle multiple memory bindings", func(t *testing.T) {
		store := new(MockResourceStore)
		ag1 := &agent.Config{
			ID: "agent1",
			LLMProperties: agent.LLMProperties{
				Memory: []core.MemoryReference{
					{ID: "mem1"},
					{ID: "mem2"},
				},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceAgent).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "agent1"}, Value: ag1},
			}, nil)
		refs, err := AgentsReferencingMemory(ctx, store, "project1", "mem1")
		require.NoError(t, err)
		assert.Equal(t, []string{"agent1"}, refs)
	})
}

func TestTasksReferencingToolResources(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find tasks referencing tool", func(t *testing.T) {
		store := new(MockResourceStore)
		tk1 := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task1",
				Tool: &tool.Config{ID: "tool1"},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceTask).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "task1"}, Value: tk1},
			}, nil)
		refs, err := TasksReferencingToolResources(ctx, store, "project1", "tool1")
		require.NoError(t, err)
		assert.Equal(t, []string{"task1"}, refs)
	})

	t.Run("Should not find tasks without tool reference", func(t *testing.T) {
		store := new(MockResourceStore)
		tk1 := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task1",
				Tool: nil,
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceTask).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "task1"}, Value: tk1},
			}, nil)
		refs, err := TasksReferencingToolResources(ctx, store, "project1", "tool1")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})
}

func TestTasksReferencingAgentResources(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find tasks referencing agent", func(t *testing.T) {
		store := new(MockResourceStore)
		tk1 := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:    "task1",
				Agent: &agent.Config{ID: "agent1"},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceTask).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "task1"}, Value: tk1},
			}, nil)
		refs, err := TasksReferencingAgentResources(ctx, store, "project1", "agent1")
		require.NoError(t, err)
		assert.Equal(t, []string{"task1"}, refs)
	})

	t.Run("Should not find tasks without agent reference", func(t *testing.T) {
		store := new(MockResourceStore)
		tk1 := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:    "task1",
				Agent: nil,
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceTask).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "task1"}, Value: tk1},
			}, nil)
		refs, err := TasksReferencingAgentResources(ctx, store, "project1", "agent1")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})
}

func TestTasksReferencingTaskResources(t *testing.T) {
	ctx := helpers.NewTestContext(t)

	t.Run("Should find tasks referencing via OnSuccess.Next", func(t *testing.T) {
		store := new(MockResourceStore)
		nextTask := "task2"
		tk1 := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "task1",
				OnSuccess: &core.SuccessTransition{
					Next: &nextTask,
				},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceTask).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "task1"}, Value: tk1},
			}, nil)
		refs, err := TasksReferencingTaskResources(ctx, store, "project1", "task2")
		require.NoError(t, err)
		assert.Equal(t, []string{"task1"}, refs)
	})

	t.Run("Should find tasks referencing via OnError.Next", func(t *testing.T) {
		store := new(MockResourceStore)
		nextTask := "task2"
		tk1 := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "task1",
				OnError: &core.ErrorTransition{
					Next: &nextTask,
				},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceTask).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "task1"}, Value: tk1},
			}, nil)
		refs, err := TasksReferencingTaskResources(ctx, store, "project1", "task2")
		require.NoError(t, err)
		assert.Equal(t, []string{"task1"}, refs)
	})

	t.Run("Should find tasks referencing via Routes", func(t *testing.T) {
		store := new(MockResourceStore)
		tk1 := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "task1",
			},
			RouterTask: task.RouterTask{
				Routes: map[string]any{
					"success": "task2",
					"error":   "task3",
				},
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceTask).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "task1"}, Value: tk1},
			}, nil)
		refs, err := TasksReferencingTaskResources(ctx, store, "project1", "task2")
		require.NoError(t, err)
		assert.Equal(t, []string{"task1"}, refs)
	})

	t.Run("Should not find tasks without task references", func(t *testing.T) {
		store := new(MockResourceStore)
		tk1 := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "task1",
			},
		}
		store.On("ListWithValues", ctx, "project1", resources.ResourceTask).
			Return([]resources.StoredItem{
				{Key: resources.ResourceKey{ID: "task1"}, Value: tk1},
			}, nil)
		refs, err := TasksReferencingTaskResources(ctx, store, "project1", "task2")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})
}

func TestDecodeStoredWorkflow(t *testing.T) {
	t.Run("Should decode workflow pointer", func(t *testing.T) {
		wf := &workflow.Config{ID: "wf1"}
		result, err := DecodeStoredWorkflow(wf, "wf1")
		require.NoError(t, err)
		assert.Equal(t, "wf1", result.ID)
	})

	t.Run("Should decode workflow value", func(t *testing.T) {
		wf := workflow.Config{ID: "wf1"}
		result, err := DecodeStoredWorkflow(wf, "wf1")
		require.NoError(t, err)
		assert.Equal(t, "wf1", result.ID)
	})

	t.Run("Should decode workflow from map", func(t *testing.T) {
		wfMap := map[string]any{
			"id": "wf1",
		}
		result, err := DecodeStoredWorkflow(wfMap, "wf1")
		require.NoError(t, err)
		assert.Equal(t, "wf1", result.ID)
	})

	t.Run("Should set ID when empty", func(t *testing.T) {
		wf := &workflow.Config{}
		result, err := DecodeStoredWorkflow(wf, "wf1")
		require.NoError(t, err)
		assert.Equal(t, "wf1", result.ID)
	})

	t.Run("Should return error for unsupported type", func(t *testing.T) {
		result, err := DecodeStoredWorkflow("invalid", "wf1")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "unsupported type")
	})
}

func TestSchemaRefMatches(t *testing.T) {
	t.Run("Should match schema with __schema_ref__", func(t *testing.T) {
		sc := schema.Schema{"__schema_ref__": "schema1"}
		assert.True(t, schemaRefMatches(&sc, "schema1"))
	})

	t.Run("Should match schema with id field", func(t *testing.T) {
		sc := schema.Schema{"id": "schema1"}
		assert.True(t, schemaRefMatches(&sc, "schema1"))
	})

	t.Run("Should not match different schema ID", func(t *testing.T) {
		sc := schema.Schema{"__schema_ref__": "schema2"}
		assert.False(t, schemaRefMatches(&sc, "schema1"))
	})

	t.Run("Should handle nil schema", func(t *testing.T) {
		assert.False(t, schemaRefMatches(nil, "schema1"))
	})

	t.Run("Should handle whitespace in schema ref", func(t *testing.T) {
		sc := schema.Schema{"__schema_ref__": "  schema1  "}
		assert.True(t, schemaRefMatches(&sc, "schema1"))
	})
}

func TestBindingListHasID(t *testing.T) {
	t.Run("Should find ID in binding list", func(t *testing.T) {
		bindings := []core.KnowledgeBinding{
			{ID: "kb1"},
			{ID: "kb2"},
		}
		assert.True(t, bindingListHasID(bindings, "kb1"))
	})

	t.Run("Should not find missing ID", func(t *testing.T) {
		bindings := []core.KnowledgeBinding{
			{ID: "kb1"},
		}
		assert.False(t, bindingListHasID(bindings, "kb2"))
	})

	t.Run("Should handle empty binding list", func(t *testing.T) {
		assert.False(t, bindingListHasID([]core.KnowledgeBinding{}, "kb1"))
	})

	t.Run("Should handle empty target ID", func(t *testing.T) {
		bindings := []core.KnowledgeBinding{
			{ID: "kb1"},
		}
		assert.False(t, bindingListHasID(bindings, ""))
	})

	t.Run("Should handle whitespace in IDs", func(t *testing.T) {
		bindings := []core.KnowledgeBinding{
			{ID: "  kb1  "},
		}
		assert.True(t, bindingListHasID(bindings, "kb1"))
	})
}
