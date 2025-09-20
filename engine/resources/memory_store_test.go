package resources

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const shortWait = 500 * time.Millisecond

func ctxWithLogger() context.Context {
	return logger.ContextWithLogger(context.Background(), logger.NewForTests())
}

func TestMemoryStore_PutGetDeepCopy(t *testing.T) {
	t.Run("Should store deep copy and preserve original on Get", func(t *testing.T) {
		ctx := ctxWithLogger()
		st := NewMemoryResourceStore()
		key := ResourceKey{Project: "p1", Type: ResourceAgent, ID: "writer"}
		orig := map[string]any{"id": "writer", "cfg": map[string]any{"a": 1.0}}
		et1, err := st.Put(ctx, key, orig)
		require.NoError(t, err)
		assert.NotEmpty(t, et1)
		orig["id"] = "mutated"
		got, et2, err := st.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, et1, et2)
		m := got.(map[string]any)
		assert.Equal(t, "writer", m["id"].(string))
		m["id"] = "changed"
		got2, et3, err := st.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, et1, et3)
		m2 := got2.(map[string]any)
		assert.Equal(t, "writer", m2["id"].(string))
	})
}

func TestMemoryStore_ListAndDelete(t *testing.T) {
	t.Run("Should list by project/type and delete idempotently", func(t *testing.T) {
		ctx := ctxWithLogger()
		st := NewMemoryResourceStore()
		a := ResourceKey{Project: "p1", Type: ResourceTool, ID: "browser"}
		b := ResourceKey{Project: "p1", Type: ResourceTool, ID: "search"}
		c := ResourceKey{Project: "p2", Type: ResourceTool, ID: "x"}
		_, _ = st.Put(ctx, a, map[string]any{"id": "browser"})
		_, _ = st.Put(ctx, b, map[string]any{"id": "search"})
		_, _ = st.Put(ctx, c, map[string]any{"id": "x"})
		keys, err := st.List(ctx, "p1", ResourceTool)
		require.NoError(t, err)
		assert.Len(t, keys, 2)
		require.NoError(t, st.Delete(ctx, a))
		require.NoError(t, st.Delete(ctx, a))
	})
}

func TestMemoryStore_Watch_PrimeAndEvents(t *testing.T) {
	t.Run("Should prime and receive put/delete events", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctxWithLogger())
		defer cancel()
		st := NewMemoryResourceStore()
		key := ResourceKey{Project: "p1", Type: ResourceModel, ID: "gpt"}
		_, _ = st.Put(ctx, key, map[string]any{"id": "gpt"})
		ch, err := st.Watch(ctx, "p1", ResourceModel)
		require.NoError(t, err)
		select {
		case e := <-ch:
			require.Equal(t, EventPut, e.Type)
			require.Equal(t, key, e.Key)
			assert.NotEmpty(t, e.ETag)
		case <-time.After(shortWait):
			t.Fatalf("timeout waiting prime event")
		}
		_, _ = st.Put(ctx, key, map[string]any{"id": "gpt", "v": 2.0})
		select {
		case e := <-ch:
			require.Equal(t, EventPut, e.Type)
			require.Equal(t, key, e.Key)
		case <-time.After(shortWait):
			t.Fatalf("timeout waiting put event")
		}
		require.NoError(t, st.Delete(ctx, key))
		select {
		case e := <-ch:
			require.Equal(t, EventDelete, e.Type)
			require.Equal(t, key, e.Key)
		case <-time.After(shortWait):
			t.Fatalf("timeout waiting delete event")
		}
	})
}

func TestMemoryStore_Watch_StopOnCancel(t *testing.T) {
	t.Run("Should close channel on context cancel", func(t *testing.T) {
		st := NewMemoryResourceStore()
		ctx, cancel := context.WithCancel(ctxWithLogger())
		ch, err := st.Watch(ctx, "p1", ResourceSchema)
		require.NoError(t, err)
		cancel()
		select {
		case _, ok := <-ch:
			assert.False(t, ok)
		case <-time.After(shortWait):
			t.Fatalf("timeout waiting channel close")
		}
	})
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	t.Run("Should support concurrent put/get/delete with events", func(t *testing.T) {
		ctx := ctxWithLogger()
		st := NewMemoryResourceStore()
		project := "p1"
		typ := ResourceTask
		var wg sync.WaitGroup
		ch, err := st.Watch(ctx, project, typ)
		require.NoError(t, err)
		wg.Add(4)
		for i := 0; i < 4; i++ {
			go func() {
				defer wg.Done()
				for j := range 200 {
					k := ResourceKey{Project: project, Type: typ, ID: "k"}
					switch j % 3 {
					case 0:
						_, _ = st.Put(ctx, k, map[string]any{"n": j})
					case 1:
						_, _, _ = st.Get(ctx, k)
					default:
						_ = st.Delete(ctx, k)
					}
				}
			}()
		}
		done := make(chan struct{})
		go func() {
			defer close(done)
			count := 0
			deadline := time.After(2 * time.Second)
			for {
				select {
				case <-ch:
					count++
					if count > 0 {
						return
					}
				case <-deadline:
					return
				}
			}
		}()
		wg.Wait()
		<-done
	})
}
