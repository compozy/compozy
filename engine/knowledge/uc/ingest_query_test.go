package uc

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/ingest"
	"github.com/compozy/compozy/engine/resources"
)

func TestValidateIngestInput(t *testing.T) {
	_, _, err := validateIngestInput(nil)
	require.ErrorIs(t, err, ErrInvalidInput)
	_, _, err = validateIngestInput(&IngestInput{Project: " ", ID: "kb"})
	require.ErrorIs(t, err, ErrProjectMissing)
	_, _, err = validateIngestInput(&IngestInput{Project: "proj", ID: " "})
	require.ErrorIs(t, err, ErrIDMissing)
	project, kb, err := validateIngestInput(&IngestInput{Project: " proj ", ID: " kb "})
	require.NoError(t, err)
	assert.Equal(t, "proj", project)
	assert.Equal(t, "kb", kb)
}

func TestNewResolvedBinding(t *testing.T) {
	kb := &knowledge.BaseConfig{ID: "kb", Embedder: "embed", VectorDB: "vec"}
	emb := &knowledge.EmbedderConfig{ID: "embed"}
	vec := &knowledge.VectorDBConfig{ID: "vec"}
	binding := newResolvedBinding(kb, emb, vec)
	assert.Equal(t, "kb", binding.ID)
	assert.Equal(t, kb.ID, binding.KnowledgeBase.ID)
	assert.Equal(t, emb.ID, binding.Embedder.ID)
	assert.Equal(t, vec.ID, binding.Vector.ID)
}

func TestInitIngestAdapters(t *testing.T) {
	ctx := newContext(t)
	emb := &knowledge.EmbedderConfig{
		ID:       "embed",
		Provider: "openai",
		Model:    "text-embedding-3-small",
		Config: knowledge.EmbedderRuntimeConfig{
			Dimension: 0,
			BatchSize: 1,
		},
	}
	vec := &knowledge.VectorDBConfig{
		ID:   "vec",
		Type: knowledge.VectorDBTypeFilesystem,
		Config: knowledge.VectorDBConnConfig{
			Path:      filepath.Join(t.TempDir(), "vec.store"),
			Dimension: 16,
		},
	}
	adapter, store, closer, err := initIngestAdapters(ctx, "proj", emb, vec)
	require.Error(t, err)
	assert.Nil(t, adapter)
	assert.Nil(t, store)
	assert.Nil(t, closer)
}

func validateQueryInputErrorOnly(input *QueryInput) error {
	project, kb, query, err := validateQueryInput(
		input,
	)
	_ = project
	_ = kb
	_ = query
	return err
}

func TestValidateQueryInput(t *testing.T) {
	err := validateQueryInputErrorOnly(nil)
	require.ErrorIs(t, err, ErrInvalidInput)
	err = validateQueryInputErrorOnly(&QueryInput{Project: " ", ID: "kb"})
	require.ErrorIs(t, err, ErrProjectMissing)
	err = validateQueryInputErrorOnly(&QueryInput{Project: "proj", ID: " ", Query: "text"})
	require.ErrorIs(t, err, ErrIDMissing)
	err = validateQueryInputErrorOnly(&QueryInput{Project: "proj", ID: "kb", Query: " "})
	assert.ErrorContains(t, err, "query text is required")
	project, kb, query, err := validateQueryInput(&QueryInput{Project: " proj ", ID: " kb ", Query: " question "})
	require.NoError(t, err)
	assert.Equal(t, "proj", project)
	assert.Equal(t, "kb", kb)
	assert.Equal(t, "question", query)
}

func TestMergeRetrieval(t *testing.T) {
	minScore := 0.8
	baseFilters := map[string]string{"existing": "value"}
	base := &knowledge.RetrievalConfig{
		TopK:     3,
		MinScore: &minScore,
		Filters:  baseFilters,
	}
	filters := map[string]string{"tag": "alpha"}
	out := mergeRetrieval(base, 10, nil, filters)
	assert.Equal(t, 10, out.TopK)
	assert.Equal(t, "value", base.Filters["existing"])
	assert.Equal(t, "alpha", out.Filters["tag"])
	_, present := out.Filters["existing"]
	assert.False(t, present)
	out.Filters["tag"] = "changed"
	assert.Equal(t, "alpha", filters["tag"])
}

func TestQueryPrepareQueryPropagatesErrors(t *testing.T) {
	ctx := newContext(t)
	store := resources.NewMemoryResourceStore()
	queryUC := NewQuery(store)
	out, adapter, vec, err := queryUC.prepareQuery(ctx, "proj", "kb", &QueryInput{TopK: 5})
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Nil(t, adapter)
	assert.Nil(t, vec)
}

func TestQueryPrepareQueryEmbedderValidationFail(t *testing.T) {
	ctx := newContext(t)
	store := resources.NewMemoryResourceStore()
	base := &knowledge.BaseConfig{
		ID:       "kb",
		Embedder: "embed",
		VectorDB: "vec",
		Sources: []knowledge.SourceConfig{
			{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
		},
	}
	emb := &knowledge.EmbedderConfig{
		ID:       "embed",
		Provider: "openai",
		Model:    "text-embedding-3-small",
		Config:   knowledge.EmbedderRuntimeConfig{Dimension: 0, BatchSize: 1},
	}
	vec := &knowledge.VectorDBConfig{
		ID:   "vec",
		Type: knowledge.VectorDBTypeFilesystem,
		Config: knowledge.VectorDBConnConfig{
			Path:      filepath.Join(t.TempDir(), "vec.store"),
			Dimension: 16,
		},
	}
	stubKnowledgeTriple(t, store, "proj", base, emb, vec)
	queryUC := NewQuery(store)
	out, adapter, storeVec, err := queryUC.prepareQuery(ctx, "proj", "kb", &QueryInput{Query: "text"})
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Nil(t, adapter)
	assert.Nil(t, storeVec)
}

func TestIngestExecuteMissingKnowledge(t *testing.T) {
	ctx := newContext(t)
	store := resources.NewMemoryResourceStore()
	ingestUC := NewIngest(store)
	out, err := ingestUC.Execute(ctx, &IngestInput{Project: "proj", ID: "kb"})
	require.ErrorIs(t, err, ErrNotFound)
	assert.Nil(t, out)
}

func TestIngestExecuteNormalizationError(t *testing.T) {
	ctx := newContext(t)
	store := resources.NewMemoryResourceStore()
	ingestUC := NewIngest(store)
	base := &knowledge.BaseConfig{
		ID:       "kb",
		Embedder: "embed",
		VectorDB: "vec",
		Sources: []knowledge.SourceConfig{
			{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
		},
	}
	emb := &knowledge.EmbedderConfig{
		ID:       "embed",
		Provider: "openai",
		Model:    "text-embedding-3-small",
		Config:   knowledge.EmbedderRuntimeConfig{Dimension: 8, BatchSize: 1},
	}
	vec := &knowledge.VectorDBConfig{
		ID:     "vec",
		Type:   knowledge.VectorDBTypeFilesystem,
		Config: knowledge.VectorDBConnConfig{Dimension: 4},
	}
	stubKnowledgeTriple(t, store, "proj", base, emb, vec)
	out, err := ingestUC.Execute(ctx, &IngestInput{Project: "proj", ID: "kb"})
	require.Error(t, err)
	assert.Nil(t, out)
}

func TestIngestLoadNormalizedTriple(t *testing.T) {
	ctx := newContext(t)
	store := resources.NewMemoryResourceStore()
	ingestUC := NewIngest(store)
	base := &knowledge.BaseConfig{
		ID:       "kb",
		Embedder: "embed",
		VectorDB: "vec",
		Sources: []knowledge.SourceConfig{
			{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
		},
	}
	emb := &knowledge.EmbedderConfig{
		ID:       "embed",
		Provider: "openai",
		Model:    "text-embedding-3-small",
		Config:   knowledge.EmbedderRuntimeConfig{Dimension: 16, BatchSize: 1},
	}
	vec := &knowledge.VectorDBConfig{
		ID:   "vec",
		Type: knowledge.VectorDBTypeFilesystem,
		Config: knowledge.VectorDBConnConfig{
			Path:      filepath.Join(t.TempDir(), "vec.store"),
			Dimension: 16,
		},
	}
	stubKnowledgeTriple(t, store, "proj", base, emb, vec)
	kb, embedderCfg, vectorCfg, err := ingestUC.loadNormalizedTriple(ctx, "proj", "kb")
	require.NoError(t, err)
	assert.Equal(t, "kb", kb.ID)
	assert.Equal(t, "embed", embedderCfg.ID)
	assert.Equal(t, "vec", vectorCfg.ID)
}

func TestLogIngestResult(t *testing.T) {
	ctx := newContext(t)
	logIngestResult(ctx, "kb", &ingest.Result{Documents: 1, Chunks: 2, Persisted: 1})
}

func TestQueryExecuteErrors(t *testing.T) {
	ctx := newContext(t)
	store := resources.NewMemoryResourceStore()
	queryUC := NewQuery(store)
	out, err := queryUC.Execute(ctx, nil)
	require.ErrorIs(t, err, ErrInvalidInput)
	assert.Nil(t, out)
	out, err = queryUC.Execute(ctx, &QueryInput{Project: "proj", ID: "kb", Query: "q"})
	require.Error(t, err)
	assert.Nil(t, out)
}

func TestQueryExecutePrepareError(t *testing.T) {
	ctx := newContext(t)
	store := resources.NewMemoryResourceStore()
	base := &knowledge.BaseConfig{
		ID:       "kb",
		Embedder: "embed",
		VectorDB: "vec",
		Sources: []knowledge.SourceConfig{
			{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
		},
	}
	emb := &knowledge.EmbedderConfig{
		ID:       "embed",
		Provider: "openai",
		Model:    "text-embedding-3-small",
		Config:   knowledge.EmbedderRuntimeConfig{Dimension: 0, BatchSize: 1},
	}
	vec := &knowledge.VectorDBConfig{
		ID:     "vec",
		Type:   knowledge.VectorDBTypeFilesystem,
		Config: knowledge.VectorDBConnConfig{Dimension: 4},
	}
	stubKnowledgeTriple(t, store, "proj", base, emb, vec)
	queryUC := NewQuery(store)
	out, err := queryUC.Execute(ctx, &QueryInput{Project: "proj", ID: "kb", Query: "hello"})
	require.Error(t, err)
	assert.Nil(t, out)
}
