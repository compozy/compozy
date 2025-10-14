package uc

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/ingest"
	"github.com/compozy/compozy/engine/resources"
	testhelpers "github.com/compozy/compozy/test/helpers"
)

func TestValidateIngestInput(t *testing.T) {
	cases := []struct {
		name    string
		input   *IngestInput
		wantErr error
		project string
		kbID    string
	}{
		{name: "Should return ErrInvalidInput when input is nil", input: nil, wantErr: ErrInvalidInput},
		{
			name:    "Should return ErrProjectMissing when project blank",
			input:   &IngestInput{Project: " ", ID: "kb"},
			wantErr: ErrProjectMissing,
		},
		{
			name:    "Should return ErrIDMissing when id blank",
			input:   &IngestInput{Project: "proj", ID: " "},
			wantErr: ErrIDMissing,
		},
		{
			name:    "Should trim and return identifiers",
			input:   &IngestInput{Project: " proj ", ID: " kb "},
			project: "proj",
			kbID:    "kb",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			project, kbID, err := validateIngestInput(tc.input)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.project, project)
			assert.Equal(t, tc.kbID, kbID)
		})
	}
}

func TestNewResolvedBinding(t *testing.T) {
	t.Run("Should build binding with expected identifiers", func(t *testing.T) {
		kb := &knowledge.BaseConfig{ID: "kb", Embedder: "embed", VectorDB: "vec"}
		emb := &knowledge.EmbedderConfig{ID: "embed"}
		vec := &knowledge.VectorDBConfig{ID: "vec"}
		binding := newResolvedBinding(kb, emb, vec)
		assert.Equal(t, "kb", binding.ID)
		assert.Equal(t, kb.ID, binding.KnowledgeBase.ID)
		assert.Equal(t, emb.ID, binding.Embedder.ID)
		assert.Equal(t, vec.ID, binding.Vector.ID)
	})
}

func TestInitIngestAdapters(t *testing.T) {
	t.Run("Should return error when embedder config invalid", func(t *testing.T) {
		ctx := testhelpers.NewTestContext(t)
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
		adapter, store, closer, err := initIngestAdapters(ctx, "proj", emb, vec)
		require.Error(t, err)
		assert.Nil(t, adapter)
		assert.Nil(t, store)
		assert.Nil(t, closer)
	})
}

func TestValidateQueryInput(t *testing.T) {
	cases := []struct {
		name    string
		input   *QueryInput
		wantErr error
		project string
		kbID    string
		query   string
	}{
		{name: "Should return ErrInvalidInput when input nil", input: nil, wantErr: ErrInvalidInput},
		{
			name:    "Should return ErrProjectMissing when project blank",
			input:   &QueryInput{Project: " ", ID: "kb"},
			wantErr: ErrProjectMissing,
		},
		{
			name:    "Should return ErrIDMissing when id blank",
			input:   &QueryInput{Project: "proj", ID: " ", Query: "text"},
			wantErr: ErrIDMissing,
		},
		{
			name:    "Should return error when query missing",
			input:   &QueryInput{Project: "proj", ID: "kb", Query: " "},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "Should trim and return normalized values",
			input:   &QueryInput{Project: " proj ", ID: " kb ", Query: " question "},
			project: "proj",
			kbID:    "kb",
			query:   "question",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			project, kbID, query, err := validateQueryInput(tc.input)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.project, project)
			assert.Equal(t, tc.kbID, kbID)
			assert.Equal(t, tc.query, query)
		})
	}
}

func TestMergeRetrieval(t *testing.T) {
	t.Run("Should override topK and merge filters without mutating base", func(t *testing.T) {
		minScore := 0.8
		baseFilters := map[string]string{"existing": "value"}
		base := &knowledge.RetrievalConfig{TopK: 3, MinScore: &minScore, Filters: baseFilters}
		filters := map[string]string{"tag": "alpha"}
		out := mergeRetrieval(base, 10, nil, filters)
		assert.Equal(t, 10, out.TopK)
		assert.Equal(t, "value", base.Filters["existing"])
		assert.Equal(t, "alpha", out.Filters["tag"])
		_, present := out.Filters["existing"]
		assert.False(t, present)
		out.Filters["tag"] = "changed"
		assert.Equal(t, "alpha", filters["tag"])
	})
}

func TestQueryPrepareQuery(t *testing.T) {
	t.Run("Should return ErrNotFound when knowledge triple missing", func(t *testing.T) {
		ctx := testhelpers.NewTestContext(t)
		store := resources.NewMemoryResourceStore()
		queryUC := NewQuery(store)
		out, adapter, vecStore, err := queryUC.prepareQuery(ctx, "proj", "kb", &QueryInput{TopK: 5})
		assert.ErrorIs(t, err, ErrNotFound)
		assert.Nil(t, out)
		assert.Nil(t, adapter)
		assert.Nil(t, vecStore)
	})

	t.Run("Should return error when embedder dimension is invalid", func(t *testing.T) {
		ctx := testhelpers.NewTestContext(t)
		store := resources.NewMemoryResourceStore()
		base := createKnowledgeBase("kb", "embed", "vec", "docs/**/*.md")
		emb := &knowledge.EmbedderConfig{
			ID:       "embed",
			Provider: "openai",
			Model:    "text-embedding-3-small",
			Config:   knowledge.EmbedderRuntimeConfig{Dimension: 0, BatchSize: 1},
		}
		vec := createVectorDBConfig(t, "vec", 16)
		stubKnowledgeTriple(ctx, t, store, "proj", base, emb, vec)
		queryUC := NewQuery(store)
		out, adapter, vecStore, err := queryUC.prepareQuery(ctx, "proj", "kb", &QueryInput{Query: "text"})
		assert.ErrorContains(t, err, "dimension")
		assert.Nil(t, out)
		assert.Nil(t, adapter)
		assert.Nil(t, vecStore)
	})
}

func TestIngestExecute(t *testing.T) {
	t.Run("Should return ErrNotFound when knowledge missing", func(t *testing.T) {
		ctx := testhelpers.NewTestContext(t)
		store := resources.NewMemoryResourceStore()
		ingestUC := NewIngest(store)
		out, err := ingestUC.Execute(ctx, &IngestInput{Project: "proj", ID: "kb"})
		require.ErrorIs(t, err, ErrNotFound)
		assert.Nil(t, out)
	})

	t.Run("Should return error when normalization fails", func(t *testing.T) {
		ctx := testhelpers.NewTestContext(t)
		store := resources.NewMemoryResourceStore()
		ingestUC := NewIngest(store)
		base := createKnowledgeBase("kb", "embed", "vec", "docs/**/*.md")
		emb := createEmbedderConfig("embed", 8)
		vec := &knowledge.VectorDBConfig{
			ID:   "vec",
			Type: knowledge.VectorDBTypeFilesystem,
			Config: knowledge.VectorDBConnConfig{
				Dimension: 4,
			},
		}
		stubKnowledgeTriple(ctx, t, store, "proj", base, emb, vec)
		out, err := ingestUC.Execute(ctx, &IngestInput{Project: "proj", ID: "kb"})
		require.Error(t, err)
		assert.Nil(t, out)
	})
}

func TestIngestLoadNormalizedTriple(t *testing.T) {
	t.Run("Should load normalized triple from store", func(t *testing.T) {
		ctx := testhelpers.NewTestContext(t)
		store := resources.NewMemoryResourceStore()
		ingestUC := NewIngest(store)
		base := createKnowledgeBase("kb", "embed", "vec", "docs/**/*.md")
		emb := createEmbedderConfig("embed", 16)
		vec := createVectorDBConfig(t, "vec", 16)
		stubKnowledgeTriple(ctx, t, store, "proj", base, emb, vec)
		kbCfg, embedderCfg, vectorCfg, err := ingestUC.loadNormalizedTriple(ctx, "proj", "kb")
		require.NoError(t, err)
		assert.Equal(t, "kb", kbCfg.ID)
		assert.Equal(t, "embed", embedderCfg.ID)
		assert.Equal(t, "vec", vectorCfg.ID)
	})
}

func TestLogIngestResult(t *testing.T) {
	t.Run("Should log ingestion summary", func(t *testing.T) {
		ctx := testhelpers.NewTestContext(t)
		logIngestResult(ctx, "kb", &ingest.Result{Documents: 1, Chunks: 2, Persisted: 1})
	})
}

func TestQueryExecuteErrors(t *testing.T) {
	t.Run("Should return ErrInvalidInput when request nil", func(t *testing.T) {
		ctx := testhelpers.NewTestContext(t)
		store := resources.NewMemoryResourceStore()
		queryUC := NewQuery(store)
		out, err := queryUC.Execute(ctx, nil)
		require.ErrorIs(t, err, ErrInvalidInput)
		assert.Nil(t, out)
	})

	t.Run("Should return error when knowledge missing", func(t *testing.T) {
		ctx := testhelpers.NewTestContext(t)
		store := resources.NewMemoryResourceStore()
		queryUC := NewQuery(store)
		out, err := queryUC.Execute(ctx, &QueryInput{Project: "proj", ID: "kb", Query: "question"})
		require.Error(t, err)
		assert.Nil(t, out)
	})
}

func TestQueryExecutePrepareError(t *testing.T) {
	t.Run("Should bubble up preparation errors", func(t *testing.T) {
		ctx := testhelpers.NewTestContext(t)
		store := resources.NewMemoryResourceStore()
		base := createKnowledgeBase("kb", "embed", "vec", "docs/**/*.md")
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
		stubKnowledgeTriple(ctx, t, store, "proj", base, emb, vec)
		queryUC := NewQuery(store)
		out, err := queryUC.Execute(ctx, &QueryInput{Project: "proj", ID: "kb", Query: "hello"})
		require.Error(t, err)
		assert.Nil(t, out)
	})
}

func createKnowledgeBase(id, embedderID, vectorID, sourceGlob string) *knowledge.BaseConfig {
	return &knowledge.BaseConfig{
		ID:       id,
		Embedder: embedderID,
		VectorDB: vectorID,
		Sources:  []knowledge.SourceConfig{{Type: knowledge.SourceTypeMarkdownGlob, Path: sourceGlob}},
	}
}

func createEmbedderConfig(id string, dimension int) *knowledge.EmbedderConfig {
	return &knowledge.EmbedderConfig{
		ID:       id,
		Provider: "openai",
		Model:    "text-embedding-3-small",
		Config: knowledge.EmbedderRuntimeConfig{
			Dimension: dimension,
			BatchSize: 1,
		},
	}
}

func createVectorDBConfig(t *testing.T, id string, dimension int) *knowledge.VectorDBConfig {
	t.Helper()
	return &knowledge.VectorDBConfig{
		ID:   id,
		Type: knowledge.VectorDBTypeFilesystem,
		Config: knowledge.VectorDBConnConfig{
			Path:      filepath.Join(t.TempDir(), id+".store"),
			Dimension: dimension,
		},
	}
}
