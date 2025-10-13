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
)

func TestDeleteParseInput(t *testing.T) {
	uc := &Delete{}
	t.Run("rejects nil input", func(t *testing.T) {
		_, _, err := uc.parseInput(nil)
		require.ErrorIs(t, err, ErrInvalidInput)
	})
	t.Run("rejects blank project", func(t *testing.T) {
		_, _, err := uc.parseInput(&DeleteInput{Project: " "})
		require.ErrorIs(t, err, ErrProjectMissing)
	})
	t.Run("rejects blank id", func(t *testing.T) {
		_, _, err := uc.parseInput(&DeleteInput{Project: "proj", ID: ""})
		require.ErrorIs(t, err, ErrIDMissing)
	})
	t.Run("returns trimmed identifiers", func(t *testing.T) {
		project, id, err := uc.parseInput(&DeleteInput{Project: " proj ", ID: " kb "})
		require.NoError(t, err)
		assert.Equal(t, "proj", project)
		assert.Equal(t, "kb", id)
	})
}

func TestDeleteCleanupVectors(t *testing.T) {
	ctx := newContext(t)
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
	uc := &Delete{store: store}

	t.Run("successfully deletes records via filesystem store", func(t *testing.T) {
		err := uc.cleanupVectors(ctx, "proj", "kb", vecCfg)
		require.NoError(t, err)
	})

	t.Run("fails when vector config invalid", func(t *testing.T) {
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

func TestDeleteExecuteReturnsConflict(t *testing.T) {
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
	projectCfg := &project.Config{Knowledge: []core.KnowledgeBinding{{ID: "kb"}}}
	putResource(
		t,
		store,
		resources.ResourceKey{Project: "proj", Type: resources.ResourceProject, ID: "proj"},
		projectCfg,
	)
	deleteUC := NewDelete(store)
	err := deleteUC.Execute(ctx, &DeleteInput{Project: "proj", ID: "kb"})
	require.Error(t, err)
	assert.IsType(t, resourceutil.ConflictError{}, err)
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
	store := &failingStore{getErr: errors.New("boom")}
	deleteUC := &Delete{store: store}
	_, err := deleteUC.collectConflicts(newContext(t), "proj", "kb")
	require.Error(t, err)
}
