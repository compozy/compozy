package uc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/tplengine"
	testhelpers "github.com/compozy/compozy/test/helpers"
)

func TestGetExecute(t *testing.T) {
	ctx := testhelpers.NewTestContext(t)
	store := resources.NewMemoryResourceStore()
	base := &knowledge.BaseConfig{
		ID:       "kb",
		Embedder: "embed",
		VectorDB: "vec",
		Sources: []knowledge.SourceConfig{
			{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
		},
	}
	embedderCfg := &knowledge.EmbedderConfig{
		ID:       "embed",
		Provider: "openai",
		Model:    "text-embedding-3-small",
		Config: knowledge.EmbedderRuntimeConfig{
			Dimension: 1536,
			BatchSize: 1,
		},
	}
	vectorCfg := &knowledge.VectorDBConfig{
		ID:   "vec",
		Type: knowledge.VectorDBTypeFilesystem,
		Config: knowledge.VectorDBConnConfig{
			Path:      filepath.Join(t.TempDir(), "vec.store"),
			Dimension: 1536,
		},
	}
	stubKnowledgeTriple(ctx, t, store, "proj", base, embedderCfg, vectorCfg)
	getUC := NewGet(store)

	t.Run("returns stored knowledge base", func(t *testing.T) {
		out, err := getUC.Execute(ctx, &GetInput{Project: "proj", ID: "kb"})
		require.NoError(t, err)
		assert.Equal(t, "kb", out.KnowledgeBase.ID)
		assert.NotEmpty(t, out.ETag)
	})

	t.Run("translates store not found error", func(t *testing.T) {
		out, err := getUC.Execute(ctx, &GetInput{Project: "proj", ID: "missing"})
		require.ErrorIs(t, err, ErrNotFound)
		assert.Nil(t, out)
	})

	t.Run("rejects invalid input", func(t *testing.T) {
		out, err := getUC.Execute(ctx, &GetInput{Project: " ", ID: ""})
		require.ErrorIs(t, err, ErrProjectMissing)
		assert.Nil(t, out)
	})
}

func TestListExecute(t *testing.T) {
	ctx := testhelpers.NewTestContext(t)
	store := resources.NewMemoryResourceStore()
	baseA := &knowledge.BaseConfig{
		ID:       "alpha",
		Embedder: "embed",
		VectorDB: "vec",
		Sources: []knowledge.SourceConfig{
			{Type: knowledge.SourceTypeMarkdownGlob, Path: "alpha/**/*.md"},
		},
	}
	baseB := &knowledge.BaseConfig{
		ID:       "beta",
		Embedder: "embed",
		VectorDB: "vec",
		Sources: []knowledge.SourceConfig{
			{Type: knowledge.SourceTypeMarkdownGlob, Path: "beta/**/*.md"},
		},
	}
	embedderCfg := &knowledge.EmbedderConfig{
		ID:       "embed",
		Provider: "openai",
		Model:    "text-embedding-3-small",
		Config: knowledge.EmbedderRuntimeConfig{
			Dimension: 8,
			BatchSize: 1,
		},
	}
	vectorCfg := &knowledge.VectorDBConfig{
		ID:   "vec",
		Type: knowledge.VectorDBTypeFilesystem,
		Config: knowledge.VectorDBConnConfig{
			Path:      filepath.Join(t.TempDir(), "vec.store"),
			Dimension: 8,
		},
	}
	stubKnowledgeTriple(ctx, t, store, "proj", baseA, embedderCfg, vectorCfg)
	stubKnowledgeTriple(ctx, t, store, "proj", baseB, embedderCfg, vectorCfg)
	listUC := NewList(store)

	out, err := listUC.Execute(ctx, &ListInput{Project: "proj", Prefix: "a", Limit: 1})
	require.NoError(t, err)
	require.Len(t, out.Items, 1)
	assert.Equal(t, "alpha", out.Items[0]["id"])
	assert.Equal(t, 1, out.Total)
}

func TestLoadKnowledgeTriple(t *testing.T) {
	ctx := testhelpers.NewTestContext(t)
	store := resources.NewMemoryResourceStore()
	base := &knowledge.BaseConfig{
		ID:       "kb",
		Embedder: "embed",
		VectorDB: "vec",
		Sources: []knowledge.SourceConfig{
			{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
		},
	}
	embedderCfg := &knowledge.EmbedderConfig{
		ID:       "embed",
		Provider: "openai",
		Model:    "text-embedding-3-small",
		Config: knowledge.EmbedderRuntimeConfig{
			Dimension: 4,
			BatchSize: 1,
		},
	}
	vectorCfg := &knowledge.VectorDBConfig{
		ID:   "vec",
		Type: knowledge.VectorDBTypeFilesystem,
		Config: knowledge.VectorDBConnConfig{
			Path:      filepath.Join(t.TempDir(), "vec.store"),
			Dimension: 4,
		},
	}
	stubKnowledgeTriple(ctx, t, store, "proj", base, embedderCfg, vectorCfg)

	triple, err := loadKnowledgeTriple(ctx, store, "proj", "kb")
	require.NoError(t, err)
	assert.Equal(t, "kb", triple.base.ID)
	assert.Equal(t, "embed", triple.embedder.ID)
	assert.Equal(t, "vec", triple.vector.ID)

	_, err = loadKnowledgeTriple(ctx, store, "proj", "missing")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestNormalizeKnowledgeTriple(t *testing.T) {
	ctx := testhelpers.NewTestContext(t)
	base, embedder, vector, err := normalizeKnowledgeTriple(
		ctx,
		nil,
	)
	_ = base
	_ = embedder
	_ = vector
	require.Error(t, err)

	badBinding := &knowledgeTriple{
		base: &knowledge.BaseConfig{
			ID:       "kb",
			Embedder: "embed",
			VectorDB: "vec",
			Sources: []knowledge.SourceConfig{
				{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
			},
		},
		embedder: &knowledge.EmbedderConfig{
			ID:       "embed",
			Provider: "openai",
			Model:    "text-embedding-3-small",
			Config: knowledge.EmbedderRuntimeConfig{
				Dimension: 0,
				BatchSize: 1,
			},
			APIKey: "{{ .env.OPENAI_API_KEY }}",
		},
		vector: &knowledge.VectorDBConfig{
			ID:   "vec",
			Type: knowledge.VectorDBTypeFilesystem,
			Config: knowledge.VectorDBConnConfig{
				Path:      filepath.Join(t.TempDir(), "vec.store"),
				Dimension: 4,
			},
		},
	}
	base, embedder, vector, err = normalizeKnowledgeTriple(
		ctx,
		badBinding,
	)
	_ = base
	_ = embedder
	_ = vector
	require.Error(t, err)
}

func TestRenderKnowledgeValue(t *testing.T) {
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	templateCtx := map[string]any{"env": map[string]any{"SUFFIX": "Docs"}}
	value := knowledge.BaseConfig{
		ID:          "kb",
		Embedder:    "embed",
		VectorDB:    "vec",
		Description: "Knowledge {{ .env.SUFFIX }}",
		Sources: []knowledge.SourceConfig{
			{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
		},
	}
	resolved, err := renderKnowledgeValue(engine, templateCtx, value)
	require.NoError(t, err)
	assert.Equal(t, "Knowledge Docs", resolved.Description)

	_, err = renderKnowledgeValue(engine, templateCtx, map[string]any{"invalid": make(chan int)})
	require.Error(t, err)
}

func TestCaptureEnvironment(t *testing.T) {
	t.Setenv("KNOWLEDGE_TEST_ENV", "value")
	env := captureEnvironment()
	require.Equal(t, os.Getenv("KNOWLEDGE_TEST_ENV"), env["KNOWLEDGE_TEST_ENV"])
}
