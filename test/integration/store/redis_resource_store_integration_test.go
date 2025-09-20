package store

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/resources"
	appcfg "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisResourceStore_Integration_PutGetWatch(t *testing.T) {
	t.Run("Should Put/Get and receive Watch events against miniredis", func(t *testing.T) {
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		mr := miniredis.RunT(t)
		cfg := &cache.Config{RedisConfig: &appcfg.RedisConfig{URL: "redis://" + mr.Addr(), PingTimeout: time.Second}}
		client, err := cache.NewRedis(ctx, cfg)
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Close(); mr.Close() })
		st := resources.NewRedisResourceStore(client, resources.WithReconcileInterval(100*time.Millisecond))
		key := resources.ResourceKey{Project: "it", Type: resources.ResourceAgent, ID: "writer"}
		et, err := st.Put(ctx, key, map[string]any{"id": "writer"})
		require.NoError(t, err)
		assert.NotEmpty(t, et)
		ch, err := st.Watch(ctx, "it", resources.ResourceAgent)
		require.NoError(t, err)
		select {
		case e := <-ch:
			require.Equal(t, resources.EventPut, e.Type)
			require.Equal(t, key, e.Key)
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timeout waiting prime event")
		}
		_, et2, err := st.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, et, et2)
		require.NoError(t, st.Delete(ctx, key))
		select {
		case e := <-ch:
			require.Equal(t, resources.EventDelete, e.Type)
			require.Equal(t, key, e.Key)
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timeout waiting delete event")
		}
	})
}
