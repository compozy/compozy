package knowledge_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
)

func floatPtr(v float64) *float64 {
	return &v
}

func TestResolver_WorkflowPrecedence(t *testing.T) {
	t.Run("ShouldPreferWorkflowScopedKnowledgeBase", func(t *testing.T) {
		defaults := knowledge.DefaultDefaults()
		defs := knowledge.Definitions{
			Embedders: []knowledge.EmbedderConfig{
				{
					ID:       "openai_embedder",
					Provider: "openai",
					Model:    "text-embedding-3-small",
					APIKey:   "{{ .env.OPENAI_API_KEY }}",
					Config: knowledge.EmbedderRuntimeConfig{
						Dimension: defaults.ChunkSize,
						BatchSize: defaults.EmbedderBatchSize,
					},
				},
			},
			VectorDBs: []knowledge.VectorDBConfig{
				{
					ID:   "pgvector_main",
					Type: knowledge.VectorDBTypePGVector,
					Config: knowledge.VectorDBConnConfig{
						DSN:       "{{ .secrets.PGVECTOR_DSN }}",
						Dimension: defaults.ChunkSize,
					},
				},
			},
			KnowledgeBases: []knowledge.BaseConfig{
				{
					ID:       "support_docs",
					Embedder: "openai_embedder",
					VectorDB: "pgvector_main",
					Sources: []knowledge.SourceConfig{
						{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/project/**/*.md"},
					},
				},
			},
		}
		resolver, err := knowledge.NewResolver(defs, knowledge.DefaultDefaults())
		require.NoError(t, err)
		input := knowledge.ResolveInput{
			WorkflowKnowledgeBases: []knowledge.BaseConfig{
				{
					ID:       "wf_support_docs",
					Embedder: "openai_embedder",
					VectorDB: "pgvector_main",
					Sources: []knowledge.SourceConfig{
						{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/workflow/**/*.md"},
					},
					Retrieval: knowledge.RetrievalConfig{TopK: 4, MinScore: floatPtr(0.2)},
				},
			},
			ProjectBinding:  []core.KnowledgeBinding{{ID: "support_docs"}},
			WorkflowBinding: []core.KnowledgeBinding{{ID: "wf_support_docs"}},
		}
		result, err := resolver.Resolve(&input)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "wf_support_docs", result.ID)
		assert.Equal(t, 4, result.Retrieval.TopK)
		assert.InDelta(t, 0.2, result.Retrieval.MinScoreValue(), 0.0001)
		assert.Equal(t, "docs/workflow/**/*.md", result.KnowledgeBase.Sources[0].Path)
	})
}

func TestResolver_InlineOverride(t *testing.T) {
	t.Run("ShouldOverrideRetrieverConfigInline", func(t *testing.T) {
		defaults := knowledge.DefaultDefaults()
		defs := knowledge.Definitions{
			Embedders: []knowledge.EmbedderConfig{
				{
					ID:       "openai_embedder",
					Provider: "openai",
					Model:    "text-embedding-3-small",
					APIKey:   "{{ .env.OPENAI_API_KEY }}",
					Config: knowledge.EmbedderRuntimeConfig{
						Dimension: 1024,
						BatchSize: defaults.EmbedderBatchSize,
					},
				},
			},
			VectorDBs: []knowledge.VectorDBConfig{
				{
					ID:   "pgvector_main",
					Type: knowledge.VectorDBTypePGVector,
					Config: knowledge.VectorDBConnConfig{
						DSN:       "{{ .secrets.PGVECTOR_DSN }}",
						Dimension: 1024,
					},
				},
			},
			KnowledgeBases: []knowledge.BaseConfig{
				{
					ID:       "support_docs",
					Embedder: "openai_embedder",
					VectorDB: "pgvector_main",
					Sources: []knowledge.SourceConfig{
						{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
					},
					Retrieval: knowledge.RetrievalConfig{
						TopK:     6,
						MinScore: floatPtr(0.15),
						InjectAs: "default_inject",
					},
				},
			},
		}
		resolver, err := knowledge.NewResolver(defs, knowledge.DefaultDefaults())
		require.NoError(t, err)
		topK := 2
		minScore := 0.5
		maxTokens := 800
		input := knowledge.ResolveInput{
			InlineBinding: []core.KnowledgeBinding{
				{
					ID:        "support_docs",
					TopK:      &topK,
					MinScore:  &minScore,
					MaxTokens: &maxTokens,
					InjectAs:  "task_context",
					Fallback:  "No knowledge available",
				},
			},
		}
		result, err := resolver.Resolve(&input)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "support_docs", result.ID)
		assert.Equal(t, 2, result.Retrieval.TopK)
		assert.InDelta(t, 0.5, result.Retrieval.MinScoreValue(), 0.0001)
		assert.Equal(t, 800, result.Retrieval.MaxTokens)
		assert.Equal(t, "task_context", result.Retrieval.InjectAs)
		assert.Equal(t, "No knowledge available", result.Retrieval.Fallback)
	})
}

func TestResolver_AppliesCustomDefaults(t *testing.T) {
	t.Run("ShouldApplyDefaultsToResolvedBinding", func(t *testing.T) {
		defaults := knowledge.DefaultDefaults()
		defs := knowledge.Definitions{
			Embedders: []knowledge.EmbedderConfig{
				{
					ID:       "openai_embedder",
					Provider: "openai",
					Model:    "text-embedding-3-small",
					APIKey:   "{{ .env.OPENAI_API_KEY }}",
					Config: knowledge.EmbedderRuntimeConfig{
						Dimension: defaults.ChunkSize,
					},
				},
			},
			VectorDBs: []knowledge.VectorDBConfig{
				{
					ID:   "pgvector_main",
					Type: knowledge.VectorDBTypePGVector,
					Config: knowledge.VectorDBConnConfig{
						DSN:       "{{ .secrets.PGVECTOR_DSN }}",
						Dimension: defaults.ChunkSize,
					},
				},
			},
			KnowledgeBases: []knowledge.BaseConfig{
				{
					ID:       "support_docs",
					Embedder: "openai_embedder",
					VectorDB: "pgvector_main",
					Sources: []knowledge.SourceConfig{
						{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
					},
				},
			},
		}
		customDefaults := knowledge.Defaults{
			EmbedderBatchSize: 256,
			ChunkSize:         600,
			ChunkOverlap:      24,
			RetrievalTopK:     9,
			RetrievalMinScore: 0.4,
		}
		resolver, err := knowledge.NewResolver(defs, customDefaults)
		require.NoError(t, err)
		result, err := resolver.Resolve(
			&knowledge.ResolveInput{ProjectBinding: []core.KnowledgeBinding{{ID: "support_docs"}}},
		)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 9, result.Retrieval.TopK)
		assert.InDelta(t, 0.4, result.Retrieval.MinScoreValue(), 0.0001)
		assert.Equal(t, 24, result.KnowledgeBase.Chunking.OverlapValue())
		assert.Equal(t, 600, result.KnowledgeBase.Chunking.Size)
		assert.Equal(t, 256, result.Embedder.Config.BatchSize)
	})
}

func TestResolver_EmptyFilterOverrideClearsBase(t *testing.T) {
	t.Run("ShouldClearFiltersWhenInlineBindingIsEmpty", func(t *testing.T) {
		defaults := knowledge.DefaultDefaults()
		defs := knowledge.Definitions{
			Embedders: []knowledge.EmbedderConfig{
				{
					ID:       "openai_embedder",
					Provider: "openai",
					Model:    "text-embedding-3-small",
					APIKey:   "{{ .env.OPENAI_API_KEY }}",
					Config: knowledge.EmbedderRuntimeConfig{
						Dimension: defaults.ChunkSize,
						BatchSize: defaults.EmbedderBatchSize,
					},
				},
			},
			VectorDBs: []knowledge.VectorDBConfig{
				{
					ID:   "pgvector_main",
					Type: knowledge.VectorDBTypePGVector,
					Config: knowledge.VectorDBConnConfig{
						DSN:       "{{ .secrets.PGVECTOR_DSN }}",
						Dimension: defaults.ChunkSize,
					},
				},
			},
			KnowledgeBases: []knowledge.BaseConfig{
				{
					ID:       "support_docs",
					Embedder: "openai_embedder",
					VectorDB: "pgvector_main",
					Sources:  []knowledge.SourceConfig{{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"}},
					Retrieval: knowledge.RetrievalConfig{
						TopK:     6,
						MinScore: floatPtr(0.1),
						Filters:  map[string]string{"scope": "project"},
					},
				},
			},
		}
		resolver, err := knowledge.NewResolver(defs, knowledge.DefaultDefaults())
		require.NoError(t, err)
		input := knowledge.ResolveInput{
			ProjectBinding: []core.KnowledgeBinding{{ID: "support_docs"}},
			InlineBinding:  []core.KnowledgeBinding{{ID: "support_docs", Filters: map[string]string{}}},
		}
		resolved, err := resolver.Resolve(&input)
		require.NoError(t, err)
		require.NotNil(t, resolved)
		assert.Len(t, resolved.Retrieval.Filters, 0)
	})
}
