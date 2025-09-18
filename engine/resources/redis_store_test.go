package resources

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/compozy/compozy/engine/infra/cache"
	appcfg "github.com/compozy/compozy/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rshort = 500 * time.Millisecond

func newTestRedisClient(t *testing.T) (cache.RedisInterface, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	ctx := context.TODO()
	cfg := &cache.Config{RedisConfig: &appcfg.RedisConfig{URL: "redis://" + mr.Addr(), PingTimeout: time.Second}}
	r, err := cache.NewRedis(ctx, cfg)
	require.NoError(t, err)
	// Close the client when test cleanup runs; miniredis will be closed by caller
	t.Cleanup(func() { _ = r.Close() })
	// sanity ping
	require.NoError(t, r.Ping(ctx).Err())
	return r, mr
}

func TestRedisStore_PutGetDeepCopy(t *testing.T) {
	t.Run("Should store and return deep copies with deterministic ETags", func(t *testing.T) {
		ctx := context.TODO()
		c, mr := newTestRedisClient(t)
		defer mr.Close()
		st := NewRedisResourceStore(c, WithReconcileInterval(50*time.Millisecond))
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

func TestRedisStore_ListAndDelete(t *testing.T) {
	t.Run("Should list by project/type and delete idempotently", func(t *testing.T) {
		ctx := context.TODO()
		c, mr := newTestRedisClient(t)
		defer mr.Close()
		st := NewRedisResourceStore(c)
		a := ResourceKey{Project: "p1", Type: ResourceTool, ID: "browser"}
		b := ResourceKey{Project: "p1", Type: ResourceTool, ID: "search"}
		ckey := ResourceKey{Project: "p2", Type: ResourceTool, ID: "x"}
		_, _ = st.Put(ctx, a, map[string]any{"id": "browser"})
		_, _ = st.Put(ctx, b, map[string]any{"id": "search"})
		_, _ = st.Put(ctx, ckey, map[string]any{"id": "x"})
		keys, err := st.List(ctx, "p1", ResourceTool)
		require.NoError(t, err)
		assert.Len(t, keys, 2)
		require.NoError(t, st.Delete(ctx, a))
		require.NoError(t, st.Delete(ctx, a))
	})
}

func TestRedisStore_Watch_PrimeAndEvents(t *testing.T) {
	t.Run("Should prime and receive put/delete events via PubSub", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()
		c, mr := newTestRedisClient(t)
		defer mr.Close()
		st := NewRedisResourceStore(c, WithReconcileInterval(200*time.Millisecond))
		key := ResourceKey{Project: "p1", Type: ResourceModel, ID: "gpt"}
		_, _ = st.Put(ctx, key, map[string]any{"id": "gpt"})
		ch, err := st.Watch(ctx, "p1", ResourceModel)
		require.NoError(t, err)
		select {
		case e := <-ch:
			require.Equal(t, EventPut, e.Type)
			require.Equal(t, key, e.Key)
			assert.NotEmpty(t, e.ETag)
		case <-time.After(rshort):
			t.Fatalf("timeout waiting prime event")
		}
		_, _ = st.Put(ctx, key, map[string]any{"id": "gpt", "v": 2.0})
		select {
		case e := <-ch:
			require.Equal(t, EventPut, e.Type)
			require.Equal(t, key, e.Key)
		case <-time.After(rshort):
			t.Fatalf("timeout waiting put event")
		}
		require.NoError(t, st.Delete(ctx, key))
		select {
		case e := <-ch:
			require.Equal(t, EventDelete, e.Type)
			require.Equal(t, key, e.Key)
		case <-time.After(rshort):
			t.Fatalf("timeout waiting delete event")
		}
	})
}

func TestRedisStore_Watch_StopOnCancel(t *testing.T) {
	t.Run("Should close channel on context cancel", func(t *testing.T) {
		base := context.TODO()
		c, mr := newTestRedisClient(t)
		defer mr.Close()
		st := NewRedisResourceStore(c)
		ctx, cancel := context.WithCancel(base)
		ch, err := st.Watch(ctx, "p1", ResourceSchema)
		require.NoError(t, err)
		cancel()
		select {
		case _, ok := <-ch:
			assert.False(t, ok)
		case <-time.After(rshort):
			t.Fatalf("timeout waiting channel close")
		}
	})
}
