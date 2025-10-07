package ingest_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/ingest"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
)

type recordingEmbedder struct {
	failures int
	calls    [][]string
}

func (r *recordingEmbedder) EmbedDocuments(_ context.Context, texts []string) ([][]float32, error) {
	if r.failures > 0 {
		r.failures--
		return nil, errors.New("embed failed")
	}
	r.calls = append(r.calls, append([]string(nil), texts...))
	vectors := make([][]float32, len(texts))
	for i := range texts {
		vectors[i] = []float32{float32(len(texts[i]))}
	}
	return vectors, nil
}

func (r *recordingEmbedder) EmbedQuery(_ context.Context, _ string) ([]float32, error) {
	return []float32{1}, nil
}

type memoryStore struct {
	records     []vectordb.Record
	fail        int
	deleteCalls []vectordb.Filter
}

func (m *memoryStore) Upsert(_ context.Context, recs []vectordb.Record) error {
	if m.fail > 0 {
		m.fail--
		return errors.New("upsert failed")
	}
	for i := range recs {
		idx := -1
		for j := range m.records {
			if m.records[j].ID == recs[i].ID {
				idx = j
				break
			}
		}
		if idx >= 0 {
			m.records[idx] = recs[i]
			continue
		}
		m.records = append(m.records, recs[i])
	}
	return nil
}

func (m *memoryStore) Search(context.Context, []float32, vectordb.SearchOptions) ([]vectordb.Match, error) {
	return nil, nil
}

func (m *memoryStore) Delete(_ context.Context, filter vectordb.Filter) error {
	m.deleteCalls = append(m.deleteCalls, filter)
	if len(filter.IDs) == 0 && len(filter.Metadata) == 0 {
		return nil
	}
	next := make([]vectordb.Record, 0, len(m.records))
	for i := range m.records {
		rec := m.records[i]
		remove := false
		for _, id := range filter.IDs {
			if rec.ID == id {
				remove = true
				break
			}
		}
		if !remove && len(filter.Metadata) > 0 {
			match := true
			for key, val := range filter.Metadata {
				if fmt.Sprint(rec.Metadata[key]) != val {
					match = false
					break
				}
			}
			if match {
				remove = true
			}
		}
		if remove {
			continue
		}
		next = append(next, rec)
	}
	m.records = next
	return nil
}

func (m *memoryStore) Close(context.Context) error {
	return nil
}

func TestPipeline_ShouldBatchByLimit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "doc1.md"), "alpha beta gamma delta")
	writeFile(t, filepath.Join(dir, "doc2.md"), "epsilon zeta eta theta")
	cwd := cwdFromDir(t, dir)
	embed := &recordingEmbedder{}
	store := &memoryStore{}
	binding := resolvedBinding(1)
	pipe, err := ingest.NewPipeline(binding, embed, store, ingest.Options{CWD: cwd})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := pipe.Run(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.Documents)
	assert.Len(t, embed.calls, 2)
	for i := range embed.calls {
		assert.Len(t, embed.calls[i], 1)
	}
	assert.Len(t, store.records, 2)
}

func TestPipeline_ShouldPropagateProviderErrors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "doc.md"), "single document text content")
	cwd := cwdFromDir(t, dir)
	embed := &recordingEmbedder{failures: 5}
	store := &memoryStore{}
	binding := resolvedBinding(2)
	pipe, err := ingest.NewPipeline(binding, embed, store, ingest.Options{CWD: cwd})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = pipe.Run(ctx)
	require.Error(t, err)
	assert.Empty(t, store.records)
}

func TestPipeline_ShouldPersistInlinePayloadsAndReingestIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "first.md"), "alpha beta")
	writeFile(t, filepath.Join(dir, "second.md"), "gamma delta")
	cwd := cwdFromDir(t, dir)
	embed := &recordingEmbedder{}
	store := &memoryStore{}
	binding := resolvedBinding(3)
	pipe, err := ingest.NewPipeline(binding, embed, store, ingest.Options{CWD: cwd})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := pipe.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, result.Documents)
	require.Len(t, store.records, 2)
	firstIDs := collectIDs(store.records)
	secondEmbed := &recordingEmbedder{}
	pipe2, err := ingest.NewPipeline(binding, secondEmbed, store, ingest.Options{CWD: cwd})
	require.NoError(t, err)
	_, err = pipe2.Run(ctx)
	require.NoError(t, err)
	assert.Equal(t, firstIDs, collectIDs(store.records))
	for _, rec := range store.records {
		assert.NotEmpty(t, rec.Text)
		assert.NotEmpty(t, rec.Metadata["content_hash"])
		assert.Equal(t, binding.ID, rec.Metadata["knowledge_binding_id"])
		assert.Equal(t, binding.KnowledgeBase.ID, rec.Metadata["knowledge_base_id"])
	}
}

func TestPipeline_ShouldReplaceExistingRecords(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "new.md"), "fresh content")
	cwd := cwdFromDir(t, dir)
	binding := resolvedBinding(1)
	store := &memoryStore{
		records: []vectordb.Record{
			{
				ID:   "old-record",
				Text: "stale",
				Metadata: map[string]any{
					"knowledge_binding_id": binding.ID,
					"knowledge_base_id":    binding.KnowledgeBase.ID,
				},
			},
			{
				ID:   "other-record",
				Text: "keep",
				Metadata: map[string]any{
					"knowledge_binding_id": "other-binding",
					"knowledge_base_id":    binding.KnowledgeBase.ID,
				},
			},
		},
	}
	embed := &recordingEmbedder{}
	pipe, err := ingest.NewPipeline(binding, embed, store, ingest.Options{CWD: cwd, Strategy: ingest.StrategyReplace})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = pipe.Run(ctx)
	require.NoError(t, err)
	require.Len(t, store.deleteCalls, 1)
	deleteFilter := store.deleteCalls[0]
	assert.Equal(t, binding.ID, deleteFilter.Metadata["knowledge_binding_id"])
	assert.Equal(t, binding.KnowledgeBase.ID, deleteFilter.Metadata["knowledge_base_id"])
	ids := collectIDs(store.records)
	assert.Contains(t, ids, "other-record")
	assert.NotContains(t, ids, "old-record")
}

func TestPipeline_ShouldRejectLargeMarkdownFile(t *testing.T) {
	dir := t.TempDir()
	oversized := strings.Repeat("a", ingest.MaxMarkdownFileSizeBytes+1)
	writeFile(t, filepath.Join(dir, "large.md"), oversized)
	cwd := cwdFromDir(t, dir)
	binding := resolvedBinding(1)
	store := &memoryStore{}
	embed := &recordingEmbedder{}
	pipe, err := ingest.NewPipeline(binding, embed, store, ingest.Options{CWD: cwd})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = pipe.Run(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum size")
}

func resolvedBinding(batchSize int) *knowledge.ResolvedBinding {
	dedupe := true
	return &knowledge.ResolvedBinding{
		ID: "kb_binding",
		KnowledgeBase: knowledge.BaseConfig{
			ID:       "kb_support",
			Embedder: "embedder",
			VectorDB: "vector",
			Sources: []knowledge.SourceConfig{
				{Type: knowledge.SourceTypeMarkdownGlob, Path: "*.md"},
			},
			Chunking: knowledge.ChunkingConfig{
				Strategy: knowledge.ChunkStrategyRecursiveTextSplitter,
				Size:     128,
				Overlap:  16,
			},
			Preprocess: knowledge.PreprocessConfig{
				Deduplicate: &dedupe,
				RemoveHTML:  false,
			},
		},
		Embedder: knowledge.EmbedderConfig{
			ID:       "embedder",
			Provider: "openai",
			Model:    "text-embedding-3-small",
			Config: knowledge.EmbedderRuntimeConfig{
				Dimension: 4,
				BatchSize: batchSize,
			},
		},
		Vector: knowledge.VectorDBConfig{
			ID:   "vector",
			Type: knowledge.VectorDBTypeMemory,
			Config: knowledge.VectorDBConnConfig{
				Dimension: 4,
			},
		},
		Retrieval: knowledge.RetrievalConfig{
			TopK:     3,
			MinScore: 0.1,
		},
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(strings.TrimSpace(contents)), 0o644))
}

func cwdFromDir(t *testing.T, dir string) *core.PathCWD {
	t.Helper()
	cwd, err := core.CWDFromPath(dir)
	require.NoError(t, err)
	return cwd
}

func collectIDs(records []vectordb.Record) []string {
	out := make([]string, len(records))
	for i := range records {
		out[i] = records[i].ID
	}
	return out
}
