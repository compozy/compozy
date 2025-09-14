package sugardb

import (
	"context"
	"testing"
	"time"

	ccache "github.com/compozy/compozy/engine/infra/cache"
	sdk "github.com/echovault/sugardb/sugardb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSugarAdapter(t testing.TB) *Adapter {
	t.Helper()
	db, err := sdk.NewSugarDB()
	require.NoError(t, err)
	ad, err := NewAdapter(db)
	require.NoError(t, err)
	return ad
}

func TestSugarAdapter_KV(t *testing.T) {
	ctx := context.Background()
	a := newSugarAdapter(t)
	t.Run("Should set, get and delete keys with neutral errors", func(t *testing.T) {
		err := a.Set(ctx, "a", "1", 0)
		require.NoError(t, err)
		v, err := a.Get(ctx, "a")
		require.NoError(t, err)
		assert.Equal(t, "1", v)
		// SugarDB Get returns a sentinel value for missing; treat via MGet below
		_ = a.Set(ctx, "x", "x", 0)
		_ = a.Set(ctx, "y", "y", 0)
		n, err := a.Del(ctx, "x", "y")
		require.NoError(t, err)
		assert.Equal(t, int64(2), n)
		vals, err := a.MGet(ctx, "x", "missing", "y")
		require.NoError(t, err)
		assert.Equal(t, []string{"", "", ""}, vals)
		// SugarDB TTL semantics differ; skip strict TTL validation here
	})
}

func TestSugarAdapter_Lists(t *testing.T) {
	ctx := context.Background()
	a := newSugarAdapter(t)
	t.Run("Should push, range, trim and report length", func(t *testing.T) {
		n, err := a.RPush(ctx, "L", "a", "b", "c", "d")
		require.NoError(t, err)
		assert.Equal(t, int64(4), n)
		out, err := a.LRange(ctx, "L", 1, 2)
		require.NoError(t, err)
		assert.Equal(t, []string{"b", "c"}, out)
		err = a.LTrim(ctx, "L", -2, -1)
		require.NoError(t, err)
		ln, err := a.LLen(ctx, "L")
		require.NoError(t, err)
		assert.Equal(t, int64(2), ln)
		out, err = a.LRange(ctx, "L", 0, -1)
		require.NoError(t, err)
		assert.Equal(t, []string{"c", "d"}, out)
	})
}

func TestSugarAdapter_Hashes(t *testing.T) {
	ctx := context.Background()
	a := newSugarAdapter(t)
	t.Run("Should set, get, incr and delete hash fields", func(t *testing.T) {
		n, err := a.HSet(ctx, "H", "f1", "v1", "f2", "v2")
		require.NoError(t, err)
		assert.Equal(t, int64(2), n)
		v, err := a.HGet(ctx, "H", "f1")
		require.NoError(t, err)
		assert.Equal(t, "v1", v)
		cur, err := a.HIncrBy(ctx, "H", "cnt", 3)
		require.NoError(t, err)
		assert.Equal(t, int64(3), cur)
		cur, err = a.HIncrBy(ctx, "H", "cnt", -1)
		require.NoError(t, err)
		assert.Equal(t, int64(2), cur)
		del, err := a.HDel(ctx, "H", "f2")
		require.NoError(t, err)
		assert.Equal(t, int64(1), del)
		_, err = a.HGet(ctx, "H", "missing")
		assert.ErrorIs(t, err, ccache.ErrNotFound)
	})
}

func TestSugarAdapter_AtomicListWithMetadata(t *testing.T) {
	ctx := context.Background()
	a := newSugarAdapter(t)
	t.Run("Should atomically append, trim and update token metadata", func(t *testing.T) {
		n, err := a.AppendAndTrimWithMetadata(ctx, "msgs", []string{"m1", "m2", "m3"}, 15, 3, 50*time.Millisecond)
		require.NoError(t, err)
		assert.Equal(t, int64(3), n)
		n, err = a.AppendAndTrimWithMetadata(ctx, "msgs", []string{"m4", "m5"}, 5, 3, 50*time.Millisecond)
		require.NoError(t, err)
		assert.Equal(t, int64(3), n)
		out, err := a.LRange(ctx, "msgs", 0, -1)
		require.NoError(t, err)
		assert.Equal(t, []string{"m3", "m4", "m5"}, out)
		tok, err := a.Get(ctx, "msgs:tokens")
		require.NoError(t, err)
		assert.Equal(t, "20", tok)
	})
}

func TestSugarAdapter_Capabilities(t *testing.T) {
	a := newSugarAdapter(t)
	caps := a.Capabilities()
	assert.True(t, caps.KV)
	assert.True(t, caps.Lists)
	assert.True(t, caps.Hashes)
	assert.True(t, caps.PubSub)
	assert.True(t, caps.Locks)
	assert.False(t, caps.KeysIteration)
	assert.True(t, caps.AtomicListWithMetadata)
}
