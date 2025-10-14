package workflow

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/knowledge"
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

func ctxWithBG() context.Context { return context.Background() }

func TestWorkflow_IndexToResourceStore_AndCompile(t *testing.T) {
	t.Run("Should index workflow-scoped resources and resolve selectors on compile", func(t *testing.T) {
		ctx := logger.ContextWithLogger(ctxWithBG(), logger.NewForTests())
		store := resources.NewMemoryResourceStore()
		proj := &project.Config{Name: "demo"}
		wf := &Config{
			ID:     "wf1",
			Agents: []agent.Config{{ID: "writer"}},
			Tools:  []tool.Config{{ID: "fmt", Description: "format"}},
			Tasks: []task.Config{
				{BaseConfig: task.BaseConfig{ID: "t1", Type: task.TaskTypeBasic, Agent: &agent.Config{ID: "writer"}}},
			},
		}
		require.NoError(t, wf.IndexToResourceStore(ctx, proj.Name, store))
		_, _, err := store.Get(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceAgent, ID: "writer"})
		require.NoError(t, err)
		compiled, err := wf.Compile(ctx, proj, store)
		require.NoError(t, err)
		require.NotNil(t, compiled.Tasks[0].Agent)
		require.Equal(t, "writer", compiled.Tasks[0].Agent.ID)
	})
}

func TestWorkflow_IndexToResourceStore_ErrorsAndSchemas(t *testing.T) {
	t.Run("Should fail on missing prerequisites", func(t *testing.T) {
		var nilWF *Config
		err := nilWF.IndexToResourceStore(
			logger.ContextWithLogger(ctxWithBG(), logger.NewForTests()),
			"proj",
			resources.NewMemoryResourceStore(),
		)
		require.Error(t, err)
		wf := &Config{ID: ""}
		err = wf.IndexToResourceStore(
			logger.ContextWithLogger(ctxWithBG(), logger.NewForTests()),
			"",
			resources.NewMemoryResourceStore(),
		)
		require.Error(t, err)
		err = wf.IndexToResourceStore(logger.ContextWithLogger(ctxWithBG(), logger.NewForTests()), "proj", nil)
		require.Error(t, err)
		wf.ID = "ok"
		err = wf.IndexToResourceStore(
			logger.ContextWithLogger(ctxWithBG(), logger.NewForTests()),
			"",
			resources.NewMemoryResourceStore(),
		)
		require.Error(t, err)
	})
	t.Run("Should index agents/tools/mcps and only schemas with id", func(t *testing.T) {
		ctx := logger.ContextWithLogger(ctxWithBG(), logger.NewForTests())
		store := resources.NewMemoryResourceStore()
		wf := &Config{
			ID:   "wf-x",
			MCPs: []mcp.Config{{ID: "kb", URL: "http://example/mcp"}},
			Schemas: []schema.Schema{
				{"id": "s1", "type": "object"},
				{"type": "object"},
			},
		}
		err := wf.IndexToResourceStore(ctx, "demo", store)
		require.NoError(t, err)
		_, _, err = store.Get(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceMCP, ID: "kb"})
		require.NoError(t, err)
		_, _, err = store.Get(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceSchema, ID: "s1"})
		require.NoError(t, err)
		_, _, err = store.Get(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceSchema, ID: ""})
		assert.Error(t, err)
	})
	t.Run("Should propagate error when context is canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(logger.ContextWithLogger(ctxWithBG(), logger.NewForTests()))
		cancel()
		store := resources.NewMemoryResourceStore()
		wf := &Config{ID: "wf-y"}
		err := wf.IndexToResourceStore(ctx, "demo", store)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})
}

func TestSchemaID_Helper(t *testing.T) {
	t.Run("Should extract id string and ignore invalid", func(t *testing.T) {
		s1 := schema.Schema{"id": "abc"}
		assert.Equal(t, "abc", schemaID(&s1))
		s2 := schema.Schema{"id": 123}
		assert.Equal(t, "", schemaID(&s2))
		assert.Equal(t, "", schemaID(nil))
	})
}

func TestWorkflow_IndexToResourceStore_KnowledgeBases(t *testing.T) {
	ctx := logger.ContextWithLogger(ctxWithBG(), logger.NewForTests())
	store := resources.NewMemoryResourceStore()
	wf := &Config{
		ID: "wf-kb",
		KnowledgeBases: []knowledge.BaseConfig{
			{
				ID:       "kb_manual_default",
				Embedder: "embedder",
				VectorDB: "vector",
			},
			{
				ID:       "kb_on_start",
				Embedder: "embedder",
				VectorDB: "vector",
				Ingest:   knowledge.IngestOnStart,
			},
		},
	}
	require.NoError(t, wf.IndexToResourceStore(ctx, "demo", store))
	val, _, err := store.Get(
		ctx,
		resources.ResourceKey{Project: "demo", Type: resources.ResourceKnowledgeBase, ID: "kb_manual_default"},
	)
	require.NoError(t, err)
	cfg, ok := val.(*knowledge.BaseConfig)
	require.True(t, ok)
	assert.Equal(t, knowledge.IngestManual, cfg.Ingest)
	val, _, err = store.Get(
		ctx,
		resources.ResourceKey{Project: "demo", Type: resources.ResourceKnowledgeBase, ID: "kb_on_start"},
	)
	require.NoError(t, err)
	cfg, ok = val.(*knowledge.BaseConfig)
	require.True(t, ok)
	assert.Equal(t, knowledge.IngestOnStart, cfg.Ingest)
}
