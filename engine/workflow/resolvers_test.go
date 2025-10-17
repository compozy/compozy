package workflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
)

func TestResolveAgent(t *testing.T) {
	t.Run("Should resolve selector when stored value is a map", func(t *testing.T) {
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		store := resources.NewMemoryResourceStore()
		agentMap := map[string]any{
			"resource": "agent",
			"id":       "analyzer",
			"model": map[string]any{
				"provider": "openai",
				"model":    "gpt-4o-mini",
			},
			"instructions": "review code",
		}
		key := resources.ResourceKey{Project: "code-reviewer", Type: resources.ResourceAgent, ID: "analyzer"}
		_, err := store.Put(ctx, key, agentMap)
		require.NoError(t, err)
		proj := &project.Config{Name: "code-reviewer"}
		selector := &agent.Config{ID: "analyzer"}
		resolved, err := resolveAgent(ctx, proj, store, selector)
		require.NoError(t, err)
		require.NotNil(t, resolved)
		assert.Equal(t, "analyzer", resolved.ID)
		assert.Equal(t, "agent", resolved.Resource)
	})

	t.Run("Should decode agent with string model reference from store", func(t *testing.T) {
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		store := resources.NewMemoryResourceStore()
		agentMap := map[string]any{
			"resource":     "agent",
			"id":           "doc_comment",
			"model":        "groq:openai/gpt-oss-120b",
			"instructions": "Add documentation",
		}
		key := resources.ResourceKey{Project: "code-reviewer", Type: resources.ResourceAgent, ID: "doc_comment"}
		_, err := store.Put(ctx, key, agentMap)
		require.NoError(t, err)
		proj := &project.Config{Name: "code-reviewer"}
		selector := &agent.Config{ID: "doc_comment"}
		resolved, err := resolveAgent(ctx, proj, store, selector)
		require.NoError(t, err)
		require.NotNil(t, resolved)
		assert.Equal(t, "doc_comment", resolved.ID)
		assert.True(t, resolved.Model.HasRef(), "model should be a reference")
		assert.Equal(t, "groq:openai/gpt-oss-120b", resolved.Model.Ref)
	})
}

func TestResolveTool(t *testing.T) {
	t.Run("Should resolve selector when stored value is a map", func(t *testing.T) {
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		store := resources.NewMemoryResourceStore()
		toolMap := map[string]any{
			"resource":    "tool",
			"id":          "list_files",
			"description": "List files in a directory",
		}
		key := resources.ResourceKey{Project: "code-reviewer", Type: resources.ResourceTool, ID: "list_files"}
		_, err := store.Put(ctx, key, toolMap)
		require.NoError(t, err)
		proj := &project.Config{Name: "code-reviewer"}
		selector := &tool.Config{ID: "list_files"}
		resolved, err := resolveTool(ctx, proj, store, selector)
		require.NoError(t, err)
		require.NotNil(t, resolved)
		assert.Equal(t, "list_files", resolved.ID)
		assert.Equal(t, "tool", resolved.Resource)
		assert.Equal(t, "List files in a directory", resolved.Description)
	})

	t.Run("Should surface type mismatch when stored value is incompatible", func(t *testing.T) {
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		store := resources.NewMemoryResourceStore()
		key := resources.ResourceKey{Project: "code-reviewer", Type: resources.ResourceTool, ID: "broken"}
		_, err := store.Put(ctx, key, 123)
		require.NoError(t, err)
		proj := &project.Config{Name: "code-reviewer"}
		selector := &tool.Config{ID: "broken"}
		_, err = resolveTool(ctx, proj, store, selector)
		require.Error(t, err)
		var tmErr *TypeMismatchError
		assert.ErrorAs(t, err, &tmErr)
	})
}

func TestResolveMCPs(t *testing.T) {
	t.Run("Should populate MCP defaults when resolving from map", func(t *testing.T) {
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		store := resources.NewMemoryResourceStore()
		mcpMap := map[string]any{
			"id": "fs",
		}
		key := resources.ResourceKey{Project: "code-reviewer", Type: resources.ResourceMCP, ID: "fs"}
		_, err := store.Put(ctx, key, mcpMap)
		require.NoError(t, err)
		agentCfg := &agent.Config{
			ID: "analyzer",
			LLMProperties: agent.LLMProperties{
				MCPs: []mcp.Config{{ID: "fs"}},
			},
			Model: agent.Model{},
		}
		proj := &project.Config{Name: "code-reviewer"}
		require.NoError(t, resolveMCPs(ctx, proj, store, agentCfg))
		require.Len(t, agentCfg.MCPs, 1)
		resolved := agentCfg.MCPs[0]
		assert.Equal(t, "fs", resolved.ID)
		assert.Equal(t, "fs", resolved.Resource)
		assert.NotEmpty(t, resolved.Transport)
	})

	t.Run("Should surface type mismatch when MCP entry incompatible", func(t *testing.T) {
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		store := resources.NewMemoryResourceStore()
		key := resources.ResourceKey{Project: "code-reviewer", Type: resources.ResourceMCP, ID: "oops"}
		_, err := store.Put(ctx, key, 123)
		require.NoError(t, err)
		agentCfg := &agent.Config{
			ID: "analyzer",
			LLMProperties: agent.LLMProperties{
				MCPs: []mcp.Config{{ID: "oops"}},
			},
		}
		proj := &project.Config{Name: "code-reviewer"}
		err = resolveMCPs(ctx, proj, store, agentCfg)
		require.Error(t, err)
		var tmErr *TypeMismatchError
		assert.ErrorAs(t, err, &tmErr)
	})
}

func TestApplyAgentModelSelector(t *testing.T) {
	t.Run("Should resolve provider config when stored value is a map", func(t *testing.T) {
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		store := resources.NewMemoryResourceStore()
		modelMap := map[string]any{
			"provider": "openai",
			"model":    "o4-mini",
		}
		key := resources.ResourceKey{Project: "code-reviewer", Type: resources.ResourceModel, ID: "openai:o4-mini"}
		_, err := store.Put(ctx, key, modelMap)
		require.NoError(t, err)
		agentCfg := &agent.Config{Model: agent.Model{}}
		err = applyAgentModelSelector(ctx, &project.Config{Name: "code-reviewer"}, store, agentCfg, "openai:o4-mini")
		require.NoError(t, err)
		assert.Equal(t, core.ProviderName("openai"), agentCfg.Model.Config.Provider)
		assert.Equal(t, "o4-mini", agentCfg.Model.Config.Model)
	})

	t.Run("Should return type mismatch on incompatible stored provider config", func(t *testing.T) {
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		store := resources.NewMemoryResourceStore()
		key := resources.ResourceKey{Project: "code-reviewer", Type: resources.ResourceModel, ID: "broken"}
		_, err := store.Put(ctx, key, 123)
		require.NoError(t, err)
		agentCfg := &agent.Config{Model: agent.Model{}}
		err = applyAgentModelSelector(ctx, &project.Config{Name: "code-reviewer"}, store, agentCfg, "broken")
		require.Error(t, err)
		var tmErr *TypeMismatchError
		assert.ErrorAs(t, err, &tmErr)
	})
}

func TestModelConfigFromStoreNormalizesAllShapes(t *testing.T) {
	t.Run("Should normalize pointer inputs", func(t *testing.T) {
		original := &core.ProviderConfig{
			Provider:     core.ProviderName(" openai "),
			Model:        " gpt-4o ",
			APIKey:       " sk-123 ",
			APIURL:       " https://api.openai.com/v1 ",
			Organization: " org-123 ",
		}
		got, err := modelConfigFromStore(original)
		require.NoError(t, err)
		require.Same(t, original, got)
		assert.Equal(t, core.ProviderOpenAI, got.Provider)
		assert.Equal(t, "gpt-4o", got.Model)
		assert.Equal(t, "sk-123", got.APIKey)
		assert.Equal(t, "https://api.openai.com/v1", got.APIURL)
		assert.Equal(t, "org-123", got.Organization)
	})

	t.Run("Should normalize value inputs", func(t *testing.T) {
		value := core.ProviderConfig{
			Provider:     core.ProviderName(" openrouter "),
			Model:        " claude-3 ",
			APIKey:       " key ",
			APIURL:       " https://proxy ",
			Organization: " team ",
		}
		got, err := modelConfigFromStore(value)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, core.ProviderName("openrouter"), got.Provider)
		assert.Equal(t, "claude-3", got.Model)
		assert.Equal(t, "key", got.APIKey)
		assert.Equal(t, "https://proxy", got.APIURL)
		assert.Equal(t, "team", got.Organization)
	})
}

func TestFetchSchemaStoresMap(t *testing.T) {
	ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
	store := resources.NewMemoryResourceStore()
	schemaMap := map[string]any{
		"id":   "list_files_input",
		"type": "object",
		"properties": map[string]any{
			"dir": map[string]any{"type": "string"},
		},
	}
	key := resources.ResourceKey{Project: "code-reviewer", Type: resources.ResourceSchema, ID: "list_files_input"}
	_, err := store.Put(ctx, key, schemaMap)
	require.NoError(t, err)
	res, err := fetchSchema(ctx, &project.Config{Name: "code-reviewer"}, store, "list_files_input")
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "list_files_input", schema.GetID(res))
}
