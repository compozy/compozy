package uc

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type conflictStore struct {
	resources.ResourceStore
	triggerConflict bool
}

func (s *conflictStore) PutIfMatch(
	ctx context.Context,
	key resources.ResourceKey,
	value any,
	expected resources.ETag,
) (resources.ETag, error) {
	if expected == "" && s.triggerConflict {
		return "", resources.ErrETagMismatch
	}
	return s.ResourceStore.PutIfMatch(ctx, key, value, expected)
}

func TestStoreKnowledgeBase_ConcurrentCreate(t *testing.T) {
	t.Run("Should return ErrAlreadyExists when resource appears between get and put", func(t *testing.T) {
		ctx := context.Background()
		store := &conflictStore{
			ResourceStore:   resources.NewMemoryResourceStore(),
			triggerConflict: true,
		}
		uc := &Upsert{store: store}
		key := resources.ResourceKey{Project: "proj", Type: resources.ResourceKnowledgeBase, ID: "support"}
		cfg := &knowledge.BaseConfig{ID: "support"}

		etag, created, err := uc.storeKnowledgeBase(ctx, key, cfg, "")

		require.ErrorIs(t, err, ErrAlreadyExists)
		assert.False(t, created)
		assert.Empty(t, etag)
	})

	t.Run("Should create knowledge base when store is empty", func(t *testing.T) {
		ctx := context.Background()
		store := resources.NewMemoryResourceStore()
		uc := &Upsert{store: store}
		key := resources.ResourceKey{Project: "proj", Type: resources.ResourceKnowledgeBase, ID: "support"}
		cfg := &knowledge.BaseConfig{ID: "support"}

		etag, created, err := uc.storeKnowledgeBase(ctx, key, cfg, "")

		require.NoError(t, err)
		assert.True(t, created)
		assert.NotEmpty(t, etag)
	})
}
