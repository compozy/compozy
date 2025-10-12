package knowledge_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appconfig "github.com/compozy/compozy/pkg/config"

	"github.com/compozy/compozy/engine/knowledge"
)

const (
	testDimension = 1536
)

func intPtr(v int) *int {
	return &v
}

func TestDefinitions_Validate(t *testing.T) {
	defaults := knowledge.DefaultDefaults()
	t.Run("Should return error when knowledge base references missing embedder", func(t *testing.T) {
		defs := knowledge.Definitions{
			VectorDBs: []knowledge.VectorDBConfig{createVectorDBConfig("pgvector_main")},
			KnowledgeBases: []knowledge.BaseConfig{
				{
					ID:       "support_docs",
					Embedder: "missing_embedder",
					VectorDB: "pgvector_main",
					Sources: []knowledge.SourceConfig{
						{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
					},
				},
			},
		}
		defs.Normalize()
		err := defs.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, `knowledge_base "support_docs" references unknown embedder "missing_embedder"`)
	})

	t.Run("Should reject invalid chunking configuration", func(t *testing.T) {
		defs := knowledge.Definitions{
			Embedders: []knowledge.EmbedderConfig{createEmbedderConfig("openai_embedder")},
			VectorDBs: []knowledge.VectorDBConfig{createVectorDBConfig("pgvector_main")},
			KnowledgeBases: []knowledge.BaseConfig{
				{
					ID:       "support_docs",
					Embedder: "openai_embedder",
					VectorDB: "pgvector_main",
					Sources: []knowledge.SourceConfig{
						{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
					},
					Chunking: knowledge.ChunkingConfig{
						Strategy: knowledge.ChunkStrategyRecursiveTextSplitter,
						Size:     48,
						Overlap:  intPtr(64),
					},
				},
			},
		}
		defs.Normalize()
		err := defs.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "chunking.size must be in [")
		assert.ErrorContains(t, err, "chunking.overlap must be < chunking.size")
	})

	t.Run("Should normalize defaults for embedder and knowledge base", func(t *testing.T) {
		defs := knowledge.Definitions{
			Embedders: []knowledge.EmbedderConfig{
				{
					ID:       "openai_embedder",
					Provider: "openai",
					Model:    "text-embedding-3-small",
					APIKey:   "{{ .env.OPENAI_API_KEY }}",
					Config: knowledge.EmbedderRuntimeConfig{
						Dimension: testDimension,
					},
				},
			},
			VectorDBs: []knowledge.VectorDBConfig{createVectorDBConfig("pgvector_main")},
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
		defs.Normalize()
		require.NoError(t, defs.Validate())
		embedder := defs.Embedders[0]
		require.NotNil(t, embedder.Config.StripNewLines)
		assert.True(t, *embedder.Config.StripNewLines)
		assert.Equal(t, defaults.EmbedderBatchSize, embedder.Config.BatchSize)
		kb := defs.KnowledgeBases[0]
		assert.Equal(t, knowledge.ChunkStrategyRecursiveTextSplitter, kb.Chunking.Strategy)
		assert.Equal(t, defaults.ChunkSize, kb.Chunking.Size)
		assert.Equal(t, defaults.ChunkOverlap, kb.Chunking.OverlapValue())
		assert.Equal(t, defaults.RetrievalTopK, kb.Retrieval.TopK)
		require.NotNil(t, kb.Preprocess.Deduplicate)
		assert.True(t, *kb.Preprocess.Deduplicate)
	})

	t.Run("Should reject unsupported source type", func(t *testing.T) {
		defs := knowledge.Definitions{
			Embedders: []knowledge.EmbedderConfig{createEmbedderConfig("openai_embedder")},
			VectorDBs: []knowledge.VectorDBConfig{createVectorDBConfig("pgvector_main")},
			KnowledgeBases: []knowledge.BaseConfig{
				{
					ID:       "support_docs",
					Embedder: "openai_embedder",
					VectorDB: "pgvector_main",
					Sources: []knowledge.SourceConfig{
						{Type: "unsupported"},
					},
				},
			},
		}
		defs.Normalize()
		err := defs.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "source type \"unsupported\" is not supported")
	})

	t.Run("Should reject plaintext secrets", func(t *testing.T) {
		defs := knowledge.Definitions{
			Embedders: []knowledge.EmbedderConfig{
				{
					ID:       "openai_embedder",
					Provider: "openai",
					Model:    "text-embedding-3-small",
					APIKey:   "sk-test",
					Config: knowledge.EmbedderRuntimeConfig{
						Dimension: testDimension,
						BatchSize: defaults.EmbedderBatchSize,
					},
				},
			},
			VectorDBs: []knowledge.VectorDBConfig{
				{
					ID:   "pgvector_main",
					Type: knowledge.VectorDBTypePGVector,
					Config: knowledge.VectorDBConnConfig{
						DSN:       "postgres://example",
						Dimension: testDimension,
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
		defs.Normalize()
		err := defs.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "api_key must use env or secret interpolation")
		assert.ErrorContains(t, err, "dsn must use env or secret interpolation")
	})
}

func TestDefaultsFromConfig(t *testing.T) {
	defaultsBaseline := knowledge.DefaultDefaults()
	cfg := appconfig.Default()
	cfg.Knowledge.EmbedderBatchSize = 1024
	cfg.Knowledge.ChunkSize = 640
	cfg.Knowledge.ChunkOverlap = 32
	cfg.Knowledge.RetrievalTopK = 12
	cfg.Knowledge.RetrievalMinScore = 0.35
	defaults := knowledge.DefaultsFromConfig(cfg)
	assert.Equal(t, 1024, defaults.EmbedderBatchSize)
	assert.Equal(t, 640, defaults.ChunkSize)
	assert.Equal(t, 32, defaults.ChunkOverlap)
	assert.Equal(t, 12, defaults.RetrievalTopK)
	assert.InDelta(t, 0.35, defaults.RetrievalMinScore, 0.0001)

	cfg.Knowledge.EmbedderBatchSize = 0
	cfg.Knowledge.ChunkSize = 20
	cfg.Knowledge.ChunkOverlap = -1
	cfg.Knowledge.RetrievalTopK = 100
	cfg.Knowledge.RetrievalMinScore = 2
	defaults = knowledge.DefaultsFromConfig(cfg)
	assert.Equal(t, defaultsBaseline.EmbedderBatchSize, defaults.EmbedderBatchSize)
	assert.Equal(t, defaultsBaseline.ChunkSize, defaults.ChunkSize)
	assert.Equal(t, defaultsBaseline.ChunkOverlap, defaults.ChunkOverlap)
	assert.Equal(t, defaultsBaseline.RetrievalTopK, defaults.RetrievalTopK)
	assert.InDelta(t, defaultsBaseline.RetrievalMinScore, defaults.RetrievalMinScore, 0.0001)
}

func TestDefaultDefaultsMatchesConfig(t *testing.T) {
	defaults := knowledge.DefaultDefaults()
	configDefaults := knowledge.DefaultsFromConfig(appconfig.Default())
	assert.Equal(t, configDefaults, defaults)
}

func TestDefinitionsNormalizeWithDefaults(t *testing.T) {
	custom := knowledge.Defaults{
		EmbedderBatchSize: 2048,
		ChunkSize:         512,
		ChunkOverlap:      48,
		RetrievalTopK:     8,
		RetrievalMinScore: 0.45,
	}
	defs := knowledge.Definitions{
		Embedders: []knowledge.EmbedderConfig{
			{
				ID:       "openai_embedder",
				Provider: "openai",
				Model:    "text-embedding-3-small",
				APIKey:   "{{ .env.OPENAI_API_KEY }}",
				Config: knowledge.EmbedderRuntimeConfig{
					Dimension: testDimension,
				},
			},
		},
		VectorDBs: []knowledge.VectorDBConfig{createVectorDBConfig("pgvector_main")},
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
	defs.NormalizeWithDefaults(custom)
	require.NoError(t, defs.Validate())
	assert.Equal(t, 2048, defs.Embedders[0].Config.BatchSize)
	assert.Equal(t, 512, defs.KnowledgeBases[0].Chunking.Size)
	assert.Equal(t, 48, defs.KnowledgeBases[0].Chunking.OverlapValue())
	assert.Equal(t, 8, defs.KnowledgeBases[0].Retrieval.TopK)
	assert.InDelta(t, 0.45, defs.KnowledgeBases[0].Retrieval.MinScoreValue(), 0.0001)
}

func TestKnowledgeBaseIngestMode(t *testing.T) {
	t.Run("Should default ingest to manual during normalization", func(t *testing.T) {
		defs := knowledge.Definitions{
			Embedders: []knowledge.EmbedderConfig{createEmbedderConfig("default_embedder")},
			VectorDBs: []knowledge.VectorDBConfig{createVectorDBConfig("pgvector_main")},
			KnowledgeBases: []knowledge.BaseConfig{
				{
					ID:       "kb_manual_default",
					Embedder: "default_embedder",
					VectorDB: "pgvector_main",
					Sources: []knowledge.SourceConfig{
						{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
					},
				},
			},
		}
		defs.Normalize()
		require.NoError(t, defs.Validate())
		assert.Equal(t, knowledge.IngestManual, defs.KnowledgeBases[0].Ingest)
	})
	t.Run("Should accept on_start ingest mode", func(t *testing.T) {
		defs := knowledge.Definitions{
			Embedders: []knowledge.EmbedderConfig{createEmbedderConfig("default_embedder")},
			VectorDBs: []knowledge.VectorDBConfig{createVectorDBConfig("pgvector_main")},
			KnowledgeBases: []knowledge.BaseConfig{
				{
					ID:       "kb_on_start",
					Embedder: "default_embedder",
					VectorDB: "pgvector_main",
					Ingest:   knowledge.IngestOnStart,
					Sources: []knowledge.SourceConfig{
						{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
					},
				},
			},
		}
		defs.Normalize()
		require.NoError(t, defs.Validate())
		assert.Equal(t, knowledge.IngestOnStart, defs.KnowledgeBases[0].Ingest)
	})
	t.Run("Should reject unsupported ingest mode", func(t *testing.T) {
		defs := knowledge.Definitions{
			Embedders: []knowledge.EmbedderConfig{createEmbedderConfig("default_embedder")},
			VectorDBs: []knowledge.VectorDBConfig{createVectorDBConfig("pgvector_main")},
			KnowledgeBases: []knowledge.BaseConfig{
				{
					ID:       "kb_invalid_mode",
					Embedder: "default_embedder",
					VectorDB: "pgvector_main",
					Ingest:   "nightly",
					Sources: []knowledge.SourceConfig{
						{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
					},
				},
			},
		}
		defs.Normalize()
		err := defs.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "ingest must be one of")
	})
}

func createEmbedderConfig(id string) knowledge.EmbedderConfig {
	defaults := knowledge.DefaultDefaults()
	return knowledge.EmbedderConfig{
		ID:       id,
		Provider: "openai",
		Model:    "text-embedding-3-small",
		APIKey:   "{{ .env.OPENAI_API_KEY }}",
		Config: knowledge.EmbedderRuntimeConfig{
			Dimension: testDimension,
			BatchSize: defaults.EmbedderBatchSize,
		},
	}
}

func createVectorDBConfig(id string) knowledge.VectorDBConfig {
	return knowledge.VectorDBConfig{
		ID:   id,
		Type: knowledge.VectorDBTypePGVector,
		Config: knowledge.VectorDBConnConfig{
			DSN:       "{{ .secrets.PGVECTOR_DSN }}",
			Table:     "knowledge_chunks",
			Dimension: testDimension,
		},
	}
}
