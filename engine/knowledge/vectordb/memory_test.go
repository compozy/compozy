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

	t.Run("ShouldFailUpsertWhenDimensionMismatch", func(t *testing.T) {
		mismatchStore := newMemoryStore(&Config{Dimension: 4})
		err := mismatchStore.Upsert(ctx, []Record{{ID: "bad", Embedding: []float32{1, 1, 1}}})
		require.Error(t, err)
	})

	t.Run("ShouldFailSearchWhenQueryDimensionMismatch", func(t *testing.T) {
		otherStore := newMemoryStore(&Config{Dimension: 2})
		record := Record{ID: "c", Embedding: []float32{1, 0}}
		require.NoError(t, otherStore.Upsert(ctx, []Record{record}))
		_, err := otherStore.Search(ctx, []float32{1, 0, 0}, SearchOptions{TopK: 1})
		require.Error(t, err)
	})

	t.Run("ShouldRespectTopKWhenExceedingAvailableRecords", func(t *testing.T) {
		limitedStore := newMemoryStore(&Config{Dimension: 2})
		records := []Record{
			{ID: "d", Text: "delta", Embedding: []float32{1, 0}},
			{ID: "e", Text: "echo", Embedding: []float32{0, 1}},
		}
		require.NoError(t, limitedStore.Upsert(ctx, records))
		matches, err := limitedStore.Search(ctx, []float32{1, 0}, SearchOptions{TopK: 10})
		require.NoError(t, err)
		require.Len(t, matches, 2)
	})
}
