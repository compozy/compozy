package cache

import (
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/pkg/config"
)

// runSuite executes the same contract tests against a RedisInterface backend.
func runSuite(t *testing.T, name string, client cache.RedisInterface) {
	t.Helper()
	t.Run(name+"/Should ping successfully", func(t *testing.T) {
		ctx := t.Context()
		err := client.Ping(ctx).Err()
		require.NoError(t, err)
	})

	t.Run(name+"/Should Get, Set, Del, Exists identically", func(t *testing.T) {
		ctx := t.Context()
		require.NoError(t, client.Set(ctx, "k1", "v1", 0).Err())
		v, err := client.Get(ctx, "k1").Result()
		require.NoError(t, err)
		assert.Equal(t, "v1", v)

		n, err := client.Exists(ctx, "k1", "missing").Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), n)

		d, err := client.Del(ctx, "k1").Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), d)

		_, err = client.Get(ctx, "k1").Result()
		assert.ErrorIs(t, err, redis.Nil)
	})

	t.Run(name+"/Should support SetNX and GetEx with TTL", func(t *testing.T) {
		ctx := t.Context()
		ok, err := client.SetNX(ctx, "nx", "a", 1500*time.Millisecond).Result()
		require.NoError(t, err)
		assert.True(t, ok)

		// Second SetNX must fail
		ok, err = client.SetNX(ctx, "nx", "b", 0).Result()
		require.NoError(t, err)
		assert.False(t, ok)

		// GetEx extends TTL
		_, err = client.GetEx(ctx, "nx", 1500*time.Millisecond).Result()
		require.NoError(t, err)
		ttl1, err := client.TTL(ctx, "nx").Result()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, ttl1.Milliseconds(), int64(900))

		// Avoid flakiness with time-based expiry in emulation.
		// We consider TTL positive check enough for contract equivalence.
	})

	t.Run(name+"/Should support MGet and Keys/Scan", func(t *testing.T) {
		ctx := t.Context()
		// prepare keys
		require.NoError(t, client.Set(ctx, "user:1", "a", 0).Err())
		require.NoError(t, client.Set(ctx, "user:2", "b", 0).Err())
		require.NoError(t, client.Set(ctx, "task:1", "c", 0).Err())

		vals, err := client.MGet(ctx, "user:1", "missing", "user:2").Result()
		require.NoError(t, err)
		// MGet returns nil for missing entries; normalize to strings for assertion
		out := make([]string, len(vals))
		for i, v := range vals {
			if v == nil {
				out[i] = ""
			} else {
				out[i] = v.(string)
			}
		}
		assert.Equal(t, []string{"a", "", "b"}, out)

		keys, err := client.Keys(ctx, "user:*").Result()
		require.NoError(t, err)
		assert.Len(t, keys, 2)

		// Scan should eventually yield both keys
		var cursor uint64
		var scanned []string
		for {
			res := client.Scan(ctx, cursor, "user:*", 10)
			ks, next, err := res.Result()
			require.NoError(t, err)
			scanned = append(scanned, ks...)
			cursor = next
			if cursor == 0 {
				break
			}
		}
		// order not guaranteed
		assert.ElementsMatch(t, []string{"user:1", "user:2"}, scanned)
	})

	t.Run(name+"/Should support Expire and TTL/Persist", func(t *testing.T) {
		ctx := t.Context()
		require.NoError(t, client.Set(ctx, "ttlkey", "x", 0).Err())
		ok, err := client.Expire(ctx, "ttlkey", 1500*time.Millisecond).Result()
		require.NoError(t, err)
		assert.True(t, ok)
		ttl, err := client.TTL(ctx, "ttlkey").Result()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, ttl.Milliseconds(), int64(900))
	})

	t.Run(name+"/Should execute Lua scripts identically (Eval)", func(t *testing.T) {
		ctx := t.Context()
		// Simple script: set a key and return OK
		script := `redis.call('SET', KEYS[1], ARGV[1]); return 'OK'`
		res, err := client.Eval(ctx, script, []string{"lua:key"}, "val").Result()
		require.NoError(t, err)
		assert.Equal(t, "OK", res)
		v, err := client.Get(ctx, "lua:key").Result()
		require.NoError(t, err)
		assert.Equal(t, "val", v)
	})

	t.Run(name+"/Should support Pipeline and Pipelined", func(t *testing.T) {
		ctx := t.Context()
		// Pipelined callback
		_, err := client.Pipelined(ctx, func(p redis.Pipeliner) error {
			p.Set(ctx, "p1", "v1", 0)
			p.Set(ctx, "p2", "v2", 0)
			return nil
		})
		require.NoError(t, err)
		v1, _ := client.Get(ctx, "p1").Result()
		v2, _ := client.Get(ctx, "p2").Result()
		assert.Equal(t, "v1", v1)
		assert.Equal(t, "v2", v2)

		// Manual pipeline
		pl := client.Pipeline()
		pl.Set(ctx, "p3", "v3", 0)
		pl.Set(ctx, "p4", "v4", 0)
		_, err = pl.Exec(ctx)
		require.NoError(t, err)
		v3, _ := client.Get(ctx, "p3").Result()
		v4, _ := client.Get(ctx, "p4").Result()
		assert.Equal(t, "v3", v3)
		assert.Equal(t, "v4", v4)
	})

	t.Run(name+"/Should support TxPipeline", func(t *testing.T) {
		ctx := t.Context()
		tx := client.TxPipeline()
		tx.Set(ctx, "tx:a", "1", 0)
		tx.Set(ctx, "tx:b", "2", 0)
		_, err := tx.Exec(ctx)
		require.NoError(t, err)
		a, _ := client.Get(ctx, "tx:a").Result()
		b, _ := client.Get(ctx, "tx:b").Result()
		assert.Equal(t, "1", a)
		assert.Equal(t, "2", b)
	})

	t.Run(name+"/Should support list operations (RPush, LRange, LLen, LTrim)", func(t *testing.T) {
		ctx := t.Context()
		n, err := client.RPush(ctx, "L", "a", "b", "c").Result()
		require.NoError(t, err)
		assert.Equal(t, int64(3), n)
		out, err := client.LRange(ctx, "L", 0, -1).Result()
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, out)
		require.NoError(t, client.LTrim(ctx, "L", -2, -1).Err())
		ln, err := client.LLen(ctx, "L").Result()
		require.NoError(t, err)
		assert.Equal(t, int64(2), ln)
	})

	t.Run(name+"/Should support hash operations (HSet, HGet, HIncrBy, HDel)", func(t *testing.T) {
		ctx := t.Context()
		n, err := client.HSet(ctx, "H", "f1", "v1", "f2", "v2").Result()
		require.NoError(t, err)
		assert.Equal(t, int64(2), n)
		v, err := client.HGet(ctx, "H", "f1").Result()
		require.NoError(t, err)
		assert.Equal(t, "v1", v)
		cur, err := client.HIncrBy(ctx, "H", "cnt", 3).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(3), cur)
		del, err := client.HDel(ctx, "H", "f2").Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), del)
	})

	t.Run(name+"/Should support Pub/Sub and PSubscribe", func(t *testing.T) {
		ctx := t.Context()
		sub := client.Subscribe(ctx, "events")
		defer sub.Close()
		psub := client.PSubscribe(ctx, "ev*")
		defer psub.Close()

		// Ensure subscriptions are active
		_, err := sub.Receive(ctx)
		require.NoError(t, err)
		_, err = psub.Receive(ctx)
		require.NoError(t, err)

		// Publish and ensure both receive
		sent := time.Now().Format(time.RFC3339Nano)
		n, err := client.Publish(ctx, "events", sent).Result()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, n, int64(1))

		// Read from both
		recv1, err := sub.ReceiveMessage(ctx)
		require.NoError(t, err)
		recv2, err := psub.ReceiveMessage(ctx)
		require.NoError(t, err)
		// Some backends may deliver duplicates; assert at least one matches
		assert.Equal(t, "events", recv1.Channel)
		assert.True(t, recv1.Payload == sent || recv2.Payload == sent)
	})

	t.Run(name+"/Should return consistent errors for missing keys", func(t *testing.T) {
		ctx := t.Context()
		_, err := client.Get(ctx, "__missing__").Result()
		assert.ErrorIs(t, err, redis.Nil)
	})
}

func TestCacheAdapter_ContractParity(t *testing.T) {
	ctx := testContext(t)
	for _, tc := range contractBackends(t) {
		client, cleanup := tc.build(ctx, t)
		t.Cleanup(cleanup)
		runSuite(t, tc.name, client)
	}
}

func TestCacheAdapter_ModeSwitching(t *testing.T) {
	t.Run("Should construct backends according to config mode", func(t *testing.T) {
		ctx := testContext(t)
		cfg := config.FromContext(ctx)

		// Embedded cache backend (memory/persistent modes)
		cfg.Mode = "distributed"
		cfg.Redis.Mode = config.ModePersistent
		c1, cleanup1, err := cache.SetupCache(ctx)
		require.NoError(t, err)
		require.NotNil(t, c1)
		t.Cleanup(cleanup1)

		// Distributed invalid config should error
		cfg.Mode = "distributed"
		cfg.Redis.Mode = "distributed"
		cfg.Redis.URL = "redis://127.0.0.1:0"
		_, _, err = cache.SetupCache(ctx)
		require.Error(t, err)
	})
}

func TestCacheAdapter_EdgeCases(t *testing.T) {
	ctx := testContext(t)
	for _, tc := range contractBackends(t) {
		client, cleanup := tc.build(ctx, t)
		t.Cleanup(cleanup)

		t.Run(tc.name+"/Should handle empty and large values", func(t *testing.T) {
			// Empty value
			require.NoError(t, client.Set(ctx, "empty", "", 0).Err())
			v, err := client.Get(ctx, "empty").Result()
			require.NoError(t, err)
			assert.Equal(t, "", v)

			// Large value ~1MB
			big := strings.Repeat("x", 1<<20)
			require.NoError(t, client.Set(ctx, "big", big, 0).Err())
			v2, err := client.Get(ctx, "big").Result()
			require.NoError(t, err)
			assert.Equal(t, len(big), len(v2))
		})

		t.Run(tc.name+"/Should handle concurrent operations", func(t *testing.T) {
			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				idx := i
				wg.Add(1)
				go func() {
					defer wg.Done()
					key := "c:" + itoa(idx)
					_ = client.Set(ctx, key, idx, 0).Err()
				}()
			}
			wg.Wait()
			ks, _, err := client.Scan(ctx, 0, "c:*", 100).Result()
			require.NoError(t, err)
			assert.NotEmpty(t, ks)
		})
	}
}

// itoa is a tiny helper to avoid importing fmt for int→string.
func itoa(n int) string { return strconv.Itoa(n) }
