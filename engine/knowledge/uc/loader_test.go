package uc

import (
	"testing"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeKnowledgeTripleResolvesTemplates(t *testing.T) {
	t.Run("Should resolve knowledge templates using environment variables", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "sk-test-key")
		t.Setenv("PGVECTOR_DSN", "postgres://user:pass@localhost:5432/db")
		t.Setenv("KB_DESC_SUFFIX", "Docs")

		triple := &knowledgeTriple{
			base: &knowledge.BaseConfig{
				ID:          "kb",
				Embedder:    "embed",
				VectorDB:    "pgvector",
				Description: "Knowledge {{ .env.KB_DESC_SUFFIX }}",
				Sources: []knowledge.SourceConfig{
					{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
				},
			},
			embedder: &knowledge.EmbedderConfig{
				ID:       "embed",
				Provider: "openai",
				Model:    "text-embedding-3-small",
				APIKey:   "{{ .env.OPENAI_API_KEY }}",
				Config: knowledge.EmbedderRuntimeConfig{
					Dimension: 1536,
					BatchSize: 32,
				},
			},
			vector: &knowledge.VectorDBConfig{
				ID:   "pgvector",
				Type: knowledge.VectorDBTypePGVector,
				Config: knowledge.VectorDBConnConfig{
					DSN:       "{{ .env.PGVECTOR_DSN }}",
					Dimension: 1536,
				},
			},
		}

		ctx := t.Context()
		kb, emb, vec, err := normalizeKnowledgeTriple(ctx, triple)
		require.NoError(t, err)

		assert.Equal(t, "sk-test-key", emb.APIKey)
		assert.Equal(t, "postgres://user:pass@localhost:5432/db", vec.Config.DSN)
		assert.Equal(t, "Knowledge Docs", kb.Description)
	})
}
