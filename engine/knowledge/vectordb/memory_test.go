package vectordb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStore(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore(&Config{Dimension: 4})

	t.Run("ShouldUpsertAndSearchByCosine", func(t *testing.T) {
		records := []Record{
			{ID: "a", Text: "alpha", Embedding: []float32{1, 0, 0, 0}, Metadata: map[string]any{"kind": "one"}},
			{ID: "b", Text: "bravo", Embedding: []float32{0, 1, 0, 0}, Metadata: map[string]any{"kind": "two"}},
		}
		require.NoError(t, store.Upsert(ctx, records))
		matches, err := store.Search(ctx, []float32{1, 0, 0, 0}, SearchOptions{TopK: 1})
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, "a", matches[0].ID)
	})

	t.Run("ShouldFilterByMetadata", func(t *testing.T) {
		matches, err := store.Search(
			ctx,
			[]float32{0, 1, 0, 0},
			SearchOptions{TopK: 2, Filters: map[string]string{"kind": "two"}},
		)
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, "b", matches[0].ID)
	})

	t.Run("ShouldDeleteByID", func(t *testing.T) {
		require.NoError(t, store.Delete(ctx, Filter{IDs: []string{"a"}}))
		matches, err := store.Search(ctx, []float32{1, 0, 0, 0}, SearchOptions{TopK: 2, MinScore: 0.1})
		require.NoError(t, err)
		require.Len(t, matches, 0)
	})
}
