package resources

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestMemoryStore_ListWithValues_AndPage(t *testing.T) {
	t.Run("Should return deep-copied values and paginate", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		st := NewMemoryResourceStore()
		for i := range 10 {
			id := "id" + string(rune('a'+i))
			_, _ = st.Put(
				ctx,
				ResourceKey{Project: "p", Type: ResourceSchema, ID: id},
				map[string]any{"id": id, "n": i},
			)
		}
		items, err := st.ListWithValues(ctx, "p", ResourceSchema)
		require.NoError(t, err)
		require.Len(t, items, 10)
		m := items[0].Value.(map[string]any)
		m["id"] = "changed"
		items2, err := st.ListWithValues(ctx, "p", ResourceSchema)
		require.NoError(t, err)
		require.NotEqual(t, "changed", items2[0].Value.(map[string]any)["id"])
		page, total, err := st.ListWithValuesPage(ctx, "p", ResourceSchema, 3, 4)
		require.NoError(t, err)
		require.Equal(t, 10, total)
		require.Len(t, page, 4)
	})
}

func TestMemoryStore_ContextAndCloseErrors(t *testing.T) {
	t.Run("Should error on context canceled", func(t *testing.T) {
		st := NewMemoryResourceStore()
		cctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := st.List(cctx, "p", ResourceTool)
		require.Error(t, err)
	})
	t.Run("Should error after store Close", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		st := NewMemoryResourceStore()
		require.NoError(t, st.Close())
		_, err := st.Put(ctx, ResourceKey{Project: "p", Type: ResourceAgent, ID: "a"}, map[string]any{"id": "a"})
		require.Error(t, err)
		_, _, err = st.Get(ctx, ResourceKey{Project: "p", Type: ResourceAgent, ID: "a"})
		require.Error(t, err)
		require.Error(t, st.Delete(ctx, ResourceKey{Project: "p", Type: ResourceAgent, ID: "a"}))
		_, err = st.ListWithValues(ctx, "p", ResourceAgent)
		require.Error(t, err)
		_, _, err = st.ListWithValuesPage(ctx, "p", ResourceAgent, 0, 1)
		require.Error(t, err)
		_, werr := st.Watch(ctx, "p", ResourceAgent)
		require.Error(t, werr)
	})
	t.Run("Should close watcher on cancel even when busy", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		st := NewMemoryResourceStore()
		wctx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
		defer cancel()
		ch, err := st.Watch(wctx, "p", ResourceAgent)
		require.NoError(t, err)
		<-wctx.Done()
		_, ok := <-ch
		require.False(t, ok)
	})
}

func TestMemoryStore_Watch_PrimeDropOnFullBuffer(t *testing.T) {
	t.Run("Should drop prime events when buffer full without blocking", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		st := NewMemoryResourceStore()
		for i := range 100 {
			_, _ = st.Put(
				ctx,
				ResourceKey{
					Project: "p",
					Type:    ResourceWorkflow,
					ID:      "w" + string(rune('a'+(i%26))) + string(rune('a'+((i/26)%26))),
				},
				map[string]any{"id": i},
			)
		}
		wctx, cancel := context.WithCancel(ctx)
		ch, err := st.Watch(wctx, "p", ResourceWorkflow)
		require.NoError(t, err)
		_ = ch
		cancel()
	})
}
