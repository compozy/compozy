package uc

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	resourceutil "github.com/compozy/compozy/engine/resourceutil"
	testhelpers "github.com/compozy/compozy/test/helpers"
)

func TestDeleteParseInput(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	uc := NewDelete(store)
	t.Run("Should return error for nil input", func(t *testing.T) {
		_, _, err := uc.parseInput(nil)
		require.ErrorIs(t, err, ErrInvalidInput)
	})
	t.Run("Should return error for blank project", func(t *testing.T) {
		_, _, err := uc.parseInput(&DeleteInput{Project: " "})
		require.ErrorIs(t, err, ErrProjectMissing)
	})
	t.Run("Should return error for blank ID", func(t *testing.T) {
		_, _, err := uc.parseInput(&DeleteInput{Project: "proj", ID: ""})
		require.ErrorIs(t, err, ErrIDMissing)
	})
	t.Run("Should return trimmed identifiers", func(t *testing.T) {
		project, id, err := uc.parseInput(&DeleteInput{Project: " proj ", ID: " kb "})
		require.NoError(t, err)
		assert.Equal(t, "proj", project)
		assert.Equal(t, "kb", id)
	})
}

func TestDeleteCleanupVectors(t *testing.T) {
	ctx := testhelpers.NewTestContext(t)
	store := resources.NewMemoryResourceStore()
	vectorDir := t.TempDir()
	vecCfg := &knowledge.VectorDBConfig{
		ID:   "vec",
		Type: knowledge.VectorDBTypeFilesystem,
		Config: knowledge.VectorDBConnConfig{
			Path:      filepath.Join(vectorDir, "vec.store"),
			Dimension: 3,
		},
	}
	uc := NewDelete(store)

	t.Run("Should successfully delete records via filesystem store", func(t *testing.T) {
		err := uc.cleanupVectors(ctx, "proj", "kb", vecCfg)
		require.NoError(t, err)
	})

	t.Run("Should fail when vector config is invalid", func(t *testing.T) {
		badVec := &knowledge.VectorDBConfig{
			ID:   "vec",
			Type: knowledge.VectorDBTypeFilesystem,
			Config: knowledge.VectorDBConnConfig{
				Path:      filepath.Join(vectorDir, "invalid.store"),
				Dimension: 0,
			},
		}
		err := uc.cleanupVectors(ctx, "proj", "kb", badVec)
		require.Error(t, err)
	})
}

func TestDeleteExecute(t *testing.T) {
	t.Run("Should return conflict error when knowledge is referenced", func(t *testing.T) {
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
		stubKnowledgeTriple(ctx, t, store, "proj", base, emb, vec)
		projectCfg := &project.Config{Knowledge: []core.KnowledgeBinding{{ID: "kb"}}}
		putResource(
			ctx,
			t,
			store,
			resources.ResourceKey{Project: "proj", Type: resources.ResourceProject, ID: "proj"},
			projectCfg,
		)
		deleteUC := NewDelete(store)
		err := deleteUC.Execute(ctx, &DeleteInput{Project: "proj", ID: "kb"})
		require.Error(t, err)
		var conflict resourceutil.ConflictError
		assert.ErrorAs(t, err, &conflict)
	})
}

type failingStore struct {
	resources.ResourceStore
	getErr error
}

// Get injects failures for conflict collection tests.
func (s *failingStore) Get(context.Context, resources.ResourceKey) (any, resources.ETag, error) {
	return nil, "", s.getErr
}

func TestCollectConflictsPropagatesErrors(t *testing.T) {
	t.Run("Should propagate store errors during conflict collection", func(t *testing.T) {
		ctx := testhelpers.NewTestContext(t)
		store := &failingStore{getErr: errors.New("boom")}
		deleteUC := NewDelete(store)
		_, err := deleteUC.collectConflicts(ctx, "proj", "kb")
		require.Error(t, err)
	})
}
