package uc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/resources"
	testhelpers "github.com/compozy/compozy/test/helpers"
)

func TestValidateUpsertInput(t *testing.T) {
	t.Run("Should return error when input is nil", func(t *testing.T) {
		_, _, err := validateUpsertInput(nil)
		require.ErrorIs(t, err, ErrInvalidInput)
	})
	t.Run("Should return error when project is missing", func(t *testing.T) {
		_, _, err := validateUpsertInput(&UpsertInput{Project: " ", ID: "kb"})
		require.ErrorIs(t, err, ErrProjectMissing)
	})
	t.Run("Should return error when ID is missing", func(t *testing.T) {
		_, _, err := validateUpsertInput(&UpsertInput{Project: "proj", ID: " "})
		require.ErrorIs(t, err, ErrIDMissing)
	})
	t.Run("Should trim and return valid identifiers", func(t *testing.T) {
		project, id, err := validateUpsertInput(&UpsertInput{Project: " proj ", ID: " kb "})
		require.NoError(t, err)
		assert.Equal(t, "proj", project)
		assert.Equal(t, "kb", id)
	})
}

func TestUpsertExecuteCreatesAndUpdates(t *testing.T) {
	ctx := testhelpers.NewTestContext(t)
	store := resources.NewMemoryResourceStore()
	upsert := NewUpsert(store)
	emb := &knowledge.EmbedderConfig{
		ID:       "embed",
		Provider: "openai",
		Model:    "text-embedding-3-small",
		Config:   knowledge.EmbedderRuntimeConfig{Dimension: 16, BatchSize: 1},
	}
	vec := &knowledge.VectorDBConfig{
		ID:     "vec",
		Type:   knowledge.VectorDBTypeFilesystem,
		Config: knowledge.VectorDBConnConfig{Path: "ignored", Dimension: 16},
	}
	putResource(
		ctx,
		t,
		store,
		resources.ResourceKey{Project: "proj", Type: resources.ResourceEmbedder, ID: "embed"},
		emb,
	)
	putResource(ctx, t, store, resources.ResourceKey{Project: "proj", Type: resources.ResourceVectorDB, ID: "vec"}, vec)
	body := map[string]any{
		"id":        "kb",
		"embedder":  "embed",
		"vector_db": "vec",
		"sources": []map[string]any{
			{"type": string(knowledge.SourceTypeMarkdownGlob), "path": "docs/**/*.md"},
		},
	}
	var createdResult *UpsertOutput
	t.Run("Should create knowledge base", func(t *testing.T) {
		out, err := upsert.Execute(ctx, &UpsertInput{Project: "proj", ID: "kb", Body: body})
		require.NoError(t, err)
		assert.True(t, out.Created)
		assert.Equal(t, "kb", out.KnowledgeBase["id"])
		createdResult = out
	})
	t.Run("Should update knowledge base when If-Match matches", func(t *testing.T) {
		require.NotNil(t, createdResult)
		updated, err := upsert.Execute(
			ctx,
			&UpsertInput{Project: "proj", ID: "kb", Body: body, IfMatch: string(createdResult.ETag)},
		)
		require.NoError(t, err)
		assert.False(t, updated.Created)
	})
}

func TestUpsertNormalizeConfig(t *testing.T) {
	ctx := testhelpers.NewTestContext(t)
	store := resources.NewMemoryResourceStore()
	upsert := NewUpsert(store)
	emb := &knowledge.EmbedderConfig{
		ID:       "embed",
		Provider: "openai",
		Model:    "text-embedding-3-small",
		Config: knowledge.EmbedderRuntimeConfig{
			Dimension: 16,
			BatchSize: 1,
		},
	}
	vec := &knowledge.VectorDBConfig{
		ID:   "vec",
		Type: knowledge.VectorDBTypeFilesystem,
		Config: knowledge.VectorDBConnConfig{
			Path:      "ignored",
			Dimension: 16,
		},
	}
	putResource(
		ctx,
		t,
		store,
		resources.ResourceKey{Project: "proj", Type: resources.ResourceEmbedder, ID: "embed"},
		emb,
	)
	putResource(ctx, t, store, resources.ResourceKey{Project: "proj", Type: resources.ResourceVectorDB, ID: "vec"}, vec)
	t.Run("Should normalize config with valid resources", func(t *testing.T) {
		body := map[string]any{
			"id":        "kb",
			"embedder":  "embed",
			"vector_db": "vec",
			"sources": []map[string]any{
				{"type": string(knowledge.SourceTypeMarkdownGlob), "path": "docs/**/*.md"},
			},
		}
		cfg, err := upsert.normalizeConfig(ctx, "proj", "kb", body)
		require.NoError(t, err)
		assert.Equal(t, "kb", cfg.ID)
	})
	t.Run("Should return error when embedder is missing", func(t *testing.T) {
		_, err := upsert.normalizeConfig(ctx, "proj", "kb", map[string]any{"embedder": "missing"})
		require.Error(t, err)
	})
}

// errorStore injects failures for Get operations.
type errorStore struct {
	resources.ResourceStore
	getErr error
}

// Get returns the configured error for testing error propagation.
func (s *errorStore) Get(context.Context, resources.ResourceKey) (any, resources.ETag, error) {
	return nil, "", s.getErr
}

func TestStoreKnowledgeBase_WithIfMatch(t *testing.T) {
	t.Run("Should return ETag mismatch when If-Match differs", func(t *testing.T) {
		ctx := testhelpers.NewTestContext(t)
		store := resources.NewMemoryResourceStore()
		upsert := &Upsert{store: store}
		key := resources.ResourceKey{Project: "proj", Type: resources.ResourceKnowledgeBase, ID: "kb"}
		cfg := &knowledge.BaseConfig{ID: "kb"}
		_, err := store.Put(ctx, key, cfg)
		require.NoError(t, err)
		etag, created, err := upsert.storeKnowledgeBase(ctx, key, cfg, "mismatch")
		require.ErrorIs(t, err, ErrETagMismatch)
		assert.False(t, created)
		assert.Empty(t, etag)
	})
}

func TestStoreKnowledgeBase_StaleIfMatch(t *testing.T) {
	t.Run("Should return stale If-Match error when tag missing", func(t *testing.T) {
		ctx := testhelpers.NewTestContext(t)
		store := resources.NewMemoryResourceStore()
		upsert := &Upsert{store: store}
		key := resources.ResourceKey{Project: "proj", Type: resources.ResourceKnowledgeBase, ID: "kb"}
		cfg := &knowledge.BaseConfig{ID: "kb"}
		etag, created, err := upsert.storeKnowledgeBase(ctx, key, cfg, "etag")
		require.ErrorIs(t, err, ErrStaleIfMatch)
		assert.False(t, created)
		assert.Empty(t, etag)
	})
}

func TestStoreKnowledgeBase_InspectError(t *testing.T) {
	t.Run("Should propagate inspect errors with context", func(t *testing.T) {
		ctx := testhelpers.NewTestContext(t)
		errStore := &errorStore{getErr: errors.New("boom")}
		upsert := &Upsert{store: errStore}
		key := resources.ResourceKey{Project: "proj", Type: resources.ResourceKnowledgeBase, ID: "kb"}
		cfg := &knowledge.BaseConfig{ID: "kb"}
		etag, created, err := upsert.storeKnowledgeBase(ctx, key, cfg, "")
		require.ErrorContains(t, err, "inspect knowledge base")
		assert.False(t, created)
		assert.Empty(t, etag)
	})
}
