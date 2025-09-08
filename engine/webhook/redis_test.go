package webhook

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type goRedisAdapter struct{ c redis.UniversalClient }

func (a *goRedisAdapter) SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error) {
	return a.c.SetNX(ctx, key, value, expiration).Result()
}

type badClient struct{}

var errBoom = errors.New("boom")

func (badClient) SetNX(context.Context, string, any, time.Duration) (bool, error) {
	return false, errBoom
}

func TestService_CheckAndSet(t *testing.T) {
	t.Run("Should set key on first call and detect duplicate within TTL", func(t *testing.T) {
		mr := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		svc := NewRedisClient(&goRedisAdapter{c: client})
		ctx := context.Background()
		err := svc.CheckAndSet(ctx, "k1", time.Minute)
		require.NoError(t, err)
		err = svc.CheckAndSet(ctx, "k1", time.Minute)
		assert.ErrorIs(t, err, ErrDuplicate)
	})

	t.Run("Should allow reuse after TTL expiration", func(t *testing.T) {
		mr := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		svc := NewRedisClient(&goRedisAdapter{c: client})
		ctx := context.Background()
		err := svc.CheckAndSet(ctx, "k2", time.Second)
		require.NoError(t, err)
		mr.FastForward(2 * time.Second)
		err = svc.CheckAndSet(ctx, "k2", time.Second)
		require.NoError(t, err)
	})

	t.Run("Should propagate client errors", func(t *testing.T) {
		svc := NewRedisClient(badClient{})
		err := svc.CheckAndSet(context.Background(), "k3", time.Minute)
		assert.ErrorIs(t, err, errBoom)
	})
}

func TestDeriveKey(t *testing.T) {
	t.Run("Should prefer header when present", func(t *testing.T) {
		h := make(http.Header)
		h.Set(HeaderIdempotencyKey, "hdr-123")
		got, err := DeriveKey(h, []byte(`{"id":"body-123"}`), "id")
		require.NoError(t, err)
		assert.Equal(t, "hdr-123", got)
	})

	t.Run("Should derive from JSON field when header missing", func(t *testing.T) {
		h := make(http.Header)
		got, err := DeriveKey(h, []byte(`{"id":"body-456"}`), "id")
		require.NoError(t, err)
		assert.Equal(t, "body-456", got)
	})

	t.Run("Should coerce non-string JSON values", func(t *testing.T) {
		h := make(http.Header)
		got, err := DeriveKey(h, []byte(`{"id":12345}`), "id")
		require.NoError(t, err)
		assert.Equal(t, "12345", got)
	})

	t.Run("Should return ErrKeyNotFound when key missing", func(t *testing.T) {
		h := make(http.Header)
		_, err := DeriveKey(h, []byte(`{"other":"x"}`), "id")
		assert.ErrorIs(t, err, ErrKeyNotFound)
	})

	t.Run("Should return error on invalid JSON", func(t *testing.T) {
		h := make(http.Header)
		_, err := DeriveKey(h, []byte(`not-json`), "id")
		assert.Error(t, err)
	})
}
