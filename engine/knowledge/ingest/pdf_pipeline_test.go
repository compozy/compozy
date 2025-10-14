package ingest

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
	"github.com/compozy/compozy/engine/pdftext"
)

type testEmbedder struct{}

func (testEmbedder) EmbedDocuments(_ context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for i := range texts {
		vectors[i] = []float32{float32(len(texts[i]))}
	}
	return vectors, nil
}

func (testEmbedder) EmbedQuery(context.Context, string) ([]float32, error) {
	return []float32{1}, nil
}

type captureStore struct {
	records []vectordb.Record
}

func (c *captureStore) Upsert(_ context.Context, recs []vectordb.Record) error {
	c.records = append(c.records, recs...)
	return nil
}

func (c *captureStore) Search(context.Context, []float32, vectordb.SearchOptions) ([]vectordb.Match, error) {
	return nil, nil
}

func (c *captureStore) Delete(context.Context, vectordb.Filter) error { return nil }

func (c *captureStore) Close(context.Context) error { return nil }

func TestPipelinePDFSourceProducesReadableChunks(t *testing.T) {
	originalFetcher := pdfFetcher
	defer func() { pdfFetcher = originalFetcher }()

	extractor, err := pdftext.New(pdftext.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { extractor.Close() })

	fixture := filepath.Join("..", "..", "..", "test", "fixtures", "pdf", "indexing_optimization.pdf")
	pdfFetcher = func(ctx context.Context, _ string) (pdftext.Result, error) {
		return extractor.ExtractFile(ctx, fixture, 0)
	}

	binding := &knowledge.ResolvedBinding{
		ID: "kb_pdf",
		KnowledgeBase: knowledge.BaseConfig{
			ID:       "kb_pdf",
			Embedder: "embedder",
			VectorDB: "vector",
			Sources: []knowledge.SourceConfig{
				{Type: knowledge.SourceTypePDFURL, URLs: []string{"http://example.test/doc.pdf"}},
			},
			Chunking: knowledge.ChunkingConfig{Strategy: knowledge.ChunkStrategyRecursiveTextSplitter, Size: 512},
		},
		Embedder: knowledge.EmbedderConfig{
			ID:       "embedder",
			Provider: "mock",
			Model:    "mock",
			Config:   knowledge.EmbedderRuntimeConfig{Dimension: 1, BatchSize: 8},
		},
		Vector: knowledge.VectorDBConfig{
			ID:     "vector",
			Type:   knowledge.VectorDBTypeFilesystem,
			Config: knowledge.VectorDBConnConfig{Dimension: 1},
		},
	}

	embed := &testEmbedder{}
	store := &captureStore{}

	pipe, err := NewPipeline(binding, embed, store, Options{})
	require.NoError(t, err)
	_, err = pipe.Run(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, store.records)

	var combined strings.Builder
	for _, rec := range store.records {
		combined.WriteString(rec.Text)
	}
	require.Contains(t, combined.String(), "Indexing Optimization")
}
