package uc

import (
	"testing"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeKnowledgeBase(t *testing.T) {
	t.Run("Should fill missing id from expected value", func(t *testing.T) {
		body := map[string]any{
			"id":        "   ",
			"embedder":  "embed",
			"vector_db": "vec",
			"sources": []map[string]any{
				{"type": string(knowledge.SourceTypeMarkdownGlob), "path": "docs/**/*.md"},
			},
		}
		cfg, err := decodeKnowledgeBase(body, "docs")
		require.NoError(t, err)
		assert.Equal(t, "docs", cfg.ID)
	})

	t.Run("Should reject mismatched identifiers", func(t *testing.T) {
		body := map[string]any{"id": "alpha", "embedder": "emb", "vector_db": "vec", "sources": []map[string]any{}}
		cfg, err := decodeKnowledgeBase(body, "beta")
		require.ErrorIs(t, err, ErrIDMismatch)
		assert.Nil(t, cfg)
	})
}

func TestDecodeStoredKnowledgeBase(t *testing.T) {
	t.Run("Should decode map input", func(t *testing.T) {
		val := map[string]any{
			"id":        "kb",
			"embedder":  "embed",
			"vector_db": "vec",
			"sources":   []map[string]any{},
		}
		cfg, err := decodeStoredKnowledgeBase(val, "kb")
		require.NoError(t, err)
		assert.Equal(t, "kb", cfg.ID)
	})

	t.Run("Should fail on unsupported type", func(t *testing.T) {
		cfg, err := decodeStoredKnowledgeBase(42, "kb")
		require.Nil(t, cfg)
		assert.ErrorContains(t, err, "unsupported type")
	})

	t.Run("Should reject nil pointer", func(t *testing.T) {
		cfg, err := decodeStoredKnowledgeBase((*knowledge.BaseConfig)(nil), "kb")
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("Should detect mismatched id", func(t *testing.T) {
		cfg, err := decodeStoredKnowledgeBase(&knowledge.BaseConfig{ID: "other"}, "kb")
		require.ErrorIs(t, err, ErrIDMismatch)
		assert.Nil(t, cfg)
	})
}

func TestDecodeStoredEmbedder(t *testing.T) {
	t.Run("Should reject nil pointer", func(t *testing.T) {
		cfg, err := decodeStoredEmbedder((*knowledge.EmbedderConfig)(nil), "emb")
		require.Nil(t, cfg)
		assert.ErrorContains(t, err, "nil value")
	})

	t.Run("Should coerce id when empty", func(t *testing.T) {
		cfg, err := decodeStoredEmbedder(&knowledge.EmbedderConfig{
			ID:       " ",
			Provider: "openai",
			Model:    "text-embedding-3-small",
			Config: knowledge.EmbedderRuntimeConfig{
				Dimension: 1536,
				BatchSize: 1,
			},
		}, "embed")
		require.NoError(t, err)
		assert.Equal(t, "embed", cfg.ID)
	})

	t.Run("Should decode map inputs", func(t *testing.T) {
		cfg, err := decodeStoredEmbedder(map[string]any{
			"id":       "map-embed",
			"provider": "openai",
			"model":    "text-embedding-3-small",
			"config": map[string]any{
				"dimension":  10,
				"batch_size": 1,
			},
		}, "map-embed")
		require.NoError(t, err)
		assert.Equal(t, "map-embed", cfg.ID)
	})
}

func TestDecodeStoredVectorDB(t *testing.T) {
	t.Run("Should reject mismatched ids", func(t *testing.T) {
		cfg, err := decodeStoredVectorDB(knowledge.VectorDBConfig{
			ID:   "other",
			Type: knowledge.VectorDBTypeFilesystem,
			Config: knowledge.VectorDBConnConfig{
				Path:      "somewhere",
				Dimension: 10,
			},
		}, "vec")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrIDMismatch)
		assert.Nil(t, cfg)
	})

	t.Run("Should accept matching struct values", func(t *testing.T) {
		cfg, err := decodeStoredVectorDB(knowledge.VectorDBConfig{
			ID:   "vec",
			Type: knowledge.VectorDBTypeFilesystem,
			Config: knowledge.VectorDBConnConfig{
				Path:      "somewhere",
				Dimension: 10,
			},
		}, "vec")
		require.NoError(t, err)
		assert.Equal(t, "vec", cfg.ID)
	})

	t.Run("Should decode map values", func(t *testing.T) {
		cfg, err := decodeStoredVectorDB(map[string]any{
			"id":   "vec",
			"type": string(knowledge.VectorDBTypeFilesystem),
			"config": map[string]any{
				"path":      "temp",
				"dimension": 2,
			},
		}, "vec")
		require.NoError(t, err)
		assert.Equal(t, "vec", cfg.ID)
	})
}

func TestEnsureStoredID(t *testing.T) {
	t.Run("Should inherit expected id when actual empty", func(t *testing.T) {
		id := ""
		err := ensureStoredID("component", &id, "expected")
		require.NoError(t, err)
		assert.Equal(t, "expected", id)
	})

	t.Run("Should error on mismatch", func(t *testing.T) {
		id := "actual"
		err := ensureStoredID("component", &id, "expected")
		require.ErrorIs(t, err, ErrIDMismatch)
		assert.Equal(t, "actual", id)
	})
}
