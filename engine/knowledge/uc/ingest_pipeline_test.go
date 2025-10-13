package uc

import (
	"context"
	"errors"
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
	testhelpers "github.com/compozy/compozy/test/helpers"
	"github.com/tmc/langchaingo/embeddings"
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
			vec[j] = float32((i + 1) * (j + 1))
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

func (s *stubVectorStore) Delete(context.Context, vectordb.Filter) error { return nil }

func (s *stubVectorStore) Close(context.Context) error { return nil }

type failingEmbeddingAdapter struct {
	dimension int
	err       error
}

func (f *failingEmbeddingAdapter) EmbedDocuments(context.Context, []string) ([][]float32, error) {
	return nil, f.err
}

func (f *failingEmbeddingAdapter) EmbedQuery(context.Context, string) ([]float32, error) {
	return make([]float32, f.dimension), nil
}

type failingVectorStore struct {
	err error
}

func (f *failingVectorStore) Upsert(context.Context, []vectordb.Record) error { return f.err }
func (f *failingVectorStore) Search(context.Context, []float32, vectordb.SearchOptions) ([]vectordb.Match, error) {
	return nil, nil
}
func (f *failingVectorStore) Delete(context.Context, vectordb.Filter) error { return nil }
func (f *failingVectorStore) Close(context.Context) error                   { return nil }

// TestRunIngestPipeline validates success and failure scenarios for the ingestion pipeline runner.
func TestRunIngestPipeline(t *testing.T) {
	t.Run("Should ingest documents and persist embeddings", func(t *testing.T) {
		ctx, binding, opts := setupIngestEnvironment(t)
		adapter := buildAdapter(t, &stubEmbeddingAdapter{dimension: 4})
		store := &stubVectorStore{}
		result, err := runIngestPipeline(ctx, binding, adapter, store, opts)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Greater(t, result.Documents, 0)
		assert.NotEmpty(t, store.upserts)
	})

	t.Run("Should return error when embedding adapter fails", func(t *testing.T) {
		errEmbed := errors.New("embedding failed")
		ctx, binding, opts := setupIngestEnvironment(t)
		adapter := buildAdapter(t, &failingEmbeddingAdapter{dimension: 4, err: errEmbed})
		store := &stubVectorStore{}
		result, err := runIngestPipeline(ctx, binding, adapter, store, opts)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "embedding failed")
	})

	t.Run("Should return error when vector store upsert fails", func(t *testing.T) {
		ctx, binding, opts := setupIngestEnvironment(t)
		adapter := buildAdapter(t, &stubEmbeddingAdapter{dimension: 4})
		store := &failingVectorStore{err: errors.New("upsert failed")}
		result, err := runIngestPipeline(ctx, binding, adapter, store, opts)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "upsert failed")
	})
}

func createTestBinding() *knowledge.ResolvedBinding {
	dimension := 4
	return &knowledge.ResolvedBinding{
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
}

func setupIngestEnvironment(t *testing.T) (context.Context, *knowledge.ResolvedBinding, ingest.Options) {
	t.Helper()
	ctx := testhelpers.NewTestContext(t)
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "docs", "readme.md"), []byte("# Title\ncontent"), 0o600))
	cwd, err := core.CWDFromPath(dir)
	require.NoError(t, err)
	return ctx, createTestBinding(), ingest.Options{CWD: cwd, Strategy: ingest.StrategyUpsert}
}

func buildAdapter(t *testing.T, impl embeddings.Embedder) *embedder.Adapter {
	t.Helper()
	adapter, err := embedder.Wrap(&embedder.Config{
		ID:        "embed",
		Provider:  embedder.ProviderOpenAI,
		Model:     "stub",
		Dimension: 4,
		BatchSize: 2,
	}, impl)
	require.NoError(t, err)
	return adapter
}
