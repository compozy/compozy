package ingest

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/attachment"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
	"github.com/compozy/compozy/engine/pdftext"
)

type staticResolved struct {
	path string
	mime string
}

func (s *staticResolved) AsURL() (string, bool)      { return "", false }
func (s *staticResolved) AsFilePath() (string, bool) { return s.path, true }
func (s *staticResolved) Open() (io.ReadCloser, error) {
	return os.Open(s.path)
}
func (s *staticResolved) MIME() string { return s.mime }
func (s *staticResolved) Cleanup()     {}

var _ attachment.Resolved = (*staticResolved)(nil)

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
	extractor, err := pdftext.New(pdftext.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { extractor.Close() })

	fixture := filepath.Join("..", "..", "..", "test", "fixtures", "pdf", "indexing_optimization.pdf")
	t.Run("Should produce readable chunks from PDF source", func(t *testing.T) {
		originalDownload := downloadToTemp
		originalExtractor := pdfExtractor
		t.Cleanup(func() {
			downloadToTemp = originalDownload
			pdfExtractor = originalExtractor
		})

		downloadToTemp = func(_ context.Context, _ string, _ int64) (attachment.Resolved, int64, error) {
			info, err := os.Stat(fixture)
			if err != nil {
				return nil, 0, err
			}
			return &staticResolved{path: fixture, mime: "application/pdf"}, info.Size(), nil
		}
		pdfExtractor = func(ctx context.Context, _ string) (pdftext.Result, error) {
			return extractor.ExtractFile(ctx, fixture, 0)
		}

		binding := testPipelinePDFBinding()
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
	})
}

// testPipelinePDFBinding returns the resolved binding used by the PDF pipeline test.
func testPipelinePDFBinding() *knowledge.ResolvedBinding {
	return &knowledge.ResolvedBinding{
		ID: "kb_pdf",
		KnowledgeBase: knowledge.BaseConfig{
			ID:       "kb_pdf",
			Embedder: "embedder",
			VectorDB: "vector",
			Sources: []knowledge.SourceConfig{
				{Type: knowledge.SourceTypeURL, URLs: []string{"http://example.test/doc.pdf"}},
			},
			Chunking: knowledge.ChunkingConfig{
				Strategy: knowledge.ChunkStrategyRecursiveTextSplitter,
				Size:     512,
			},
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
}
