package uc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/embedder"
	"github.com/compozy/compozy/engine/knowledge/ingest"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
)

// stubEmbeddingAdapter provides deterministic embeddings for pipeline tests.
type stubEmbeddingAdapter struct {
	dimension int
}

func (s *stubEmbeddingAdapter) EmbedDocuments(_ context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for i := range texts {
		vec := make([]float32, s.dimension)
		for j := range vec {
			vec[j] = float32(i + j)
		}
		vectors[i] = vec
	}
	return vectors, nil
}

func (s *stubEmbeddingAdapter) EmbedQuery(_ context.Context, _ string) ([]float32, error) {
	return make([]float32, s.dimension), nil
}

// stubVectorStore records upsert operations for assertions.
type stubVectorStore struct {
	upserts []vectordb.Record
}

func (s *stubVectorStore) Upsert(_ context.Context, records []vectordb.Record) error {
	s.upserts = append(s.upserts, records...)
	return nil
}

func (s *stubVectorStore) Search(context.Context, []float32, vectordb.SearchOptions) ([]vectordb.Match, error) {
	return nil, nil
}

func (s *stubVectorStore) Delete(context.Context, vectordb.Filter) error {
	return nil
}

func (s *stubVectorStore) Close(context.Context) error {
	return nil
}

func TestRunIngestPipeline(t *testing.T) {
	ctx := newContext(t)
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "docs", "readme.md"), []byte("# Title\ncontent"), 0o600))
	cwd, err := core.CWDFromPath(dir)
	require.NoError(t, err)

	dimension := 4
	adapter, err := embedder.Wrap(&embedder.Config{
		ID:        "embed",
		Provider:  embedder.ProviderOpenAI,
		Model:     "stub",
		Dimension: dimension,
		BatchSize: 2,
	}, &stubEmbeddingAdapter{dimension: dimension})
	require.NoError(t, err)

	vectorStore := &stubVectorStore{}
	binding := &knowledge.ResolvedBinding{
		ID: "kb",
		KnowledgeBase: knowledge.BaseConfig{
			ID:       "kb",
			Embedder: "embed",
			VectorDB: "vec",
			Sources: []knowledge.SourceConfig{
				{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
			},
			Chunking: knowledge.ChunkingConfig{
				Strategy: knowledge.ChunkStrategyRecursiveTextSplitter,
				Size:     256,
			},
		},
		Embedder: knowledge.EmbedderConfig{
			ID: "embed",
			Config: knowledge.EmbedderRuntimeConfig{
				Dimension: dimension,
				BatchSize: 2,
			},
		},
		Vector: knowledge.VectorDBConfig{
			ID: "vec",
			Config: knowledge.VectorDBConnConfig{
				Dimension: dimension,
			},
		},
	}

	result, err := runIngestPipeline(ctx, binding, adapter, vectorStore, ingest.Options{
		CWD:      cwd,
		Strategy: ingest.StrategyUpsert,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Greater(t, result.Documents, 0)
	assert.NotEmpty(t, vectorStore.upserts)
}
