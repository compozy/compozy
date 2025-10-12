package project

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProject_IndexToResourceStore(t *testing.T) {
	t.Run("Should index core and knowledge resources", func(t *testing.T) {
		ctx := context.Background()
		store := resources.NewMemoryResourceStore()
		p := &Config{
			Name:    "demo",
			Tools:   []tool.Config{{ID: "fmt", Description: "format"}},
			Schemas: []schema.Schema{{"id": "input_schema", "type": "object"}},
			Embedders: []knowledge.EmbedderConfig{{
				ID:       "default_embedder",
				Provider: "openai",
				Model:    "text-embedding-3-small",
				Config: knowledge.EmbedderRuntimeConfig{
					Dimension: 1536,
					BatchSize: 32,
				},
			}},
			VectorDBs: []knowledge.VectorDBConfig{{
				ID:   "pgvector",
				Type: knowledge.VectorDBTypePGVector,
				Config: knowledge.VectorDBConnConfig{
					DSN:       "{{ env \"PGVECTOR_DSN\" }}",
					Table:     "vectors",
					Dimension: 1536,
				},
			}},
			KnowledgeBases: []knowledge.BaseConfig{{
				ID:       "support_docs",
				Embedder: "default_embedder",
				VectorDB: "pgvector",
				Sources: []knowledge.SourceConfig{{
					Type: knowledge.SourceTypeMarkdownGlob,
					Path: "docs/**/*.md",
				}},
			}},
		}
		require.NoError(t, p.IndexToResourceStore(ctx, store))
		v, _, err := store.Get(ctx, resources.ResourceKey{Project: "demo", Type: resources.ResourceTool, ID: "fmt"})
		require.NoError(t, err)
		require.NotNil(t, v)
		v2, _, err := store.Get(
			ctx,
			resources.ResourceKey{Project: "demo", Type: resources.ResourceSchema, ID: "input_schema"},
		)
		require.NoError(t, err)
		require.NotNil(t, v2)
		embedderKey := resources.ResourceKey{Project: "demo", Type: resources.ResourceEmbedder, ID: "default_embedder"}
		embedderVal, _, err := store.Get(ctx, embedderKey)
		require.NoError(t, err)
		require.NotNil(t, embedderVal)
		vectorKey := resources.ResourceKey{Project: "demo", Type: resources.ResourceVectorDB, ID: "pgvector"}
		vectorVal, _, err := store.Get(ctx, vectorKey)
		require.NoError(t, err)
		require.NotNil(t, vectorVal)
		kbKey := resources.ResourceKey{Project: "demo", Type: resources.ResourceKnowledgeBase, ID: "support_docs"}
		kbVal, _, err := store.Get(ctx, kbKey)
		require.NoError(t, err)
		require.NotNil(t, kbVal)
		kbCfg, ok := kbVal.(*knowledge.BaseConfig)
		require.True(t, ok)
		assert.Equal(t, knowledge.IngestManual, kbCfg.Ingest)
	})
}

func TestProject_IndexToResourceStore_WritesMeta(t *testing.T) {
	t.Run("Should write YAML meta source", func(t *testing.T) {
		ctx := context.Background()
		store := resources.NewMemoryResourceStore()
		p := &Config{Name: "demo", Tools: []tool.Config{{ID: "fmt"}}}
		require.NoError(t, p.IndexToResourceStore(ctx, store))
		metaKey := resources.ResourceKey{Project: "demo", Type: resources.ResourceMeta, ID: "demo:tool:fmt"}
		v, _, err := store.Get(ctx, metaKey)
		require.NoError(t, err)
		m, ok := v.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "yaml", m["source"])
	})
}
