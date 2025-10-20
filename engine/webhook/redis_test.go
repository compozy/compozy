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

type goRedisAdapter struct {
	c redis.UniversalClient
}

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
		defer client.Close()
		svc := NewRedisService(&goRedisAdapter{c: client})
		ctx := t.Context()
		err := svc.CheckAndSet(ctx, "k1", time.Minute)
		require.NoError(t, err)
		err = svc.CheckAndSet(ctx, "k1", time.Minute)
		assert.ErrorIs(t, err, ErrDuplicate)
	})

	t.Run("Should allow reuse after TTL expiration", func(t *testing.T) {
		mr := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		defer client.Close()
		svc := NewRedisService(&goRedisAdapter{c: client})
		ctx := t.Context()
		err := svc.CheckAndSet(ctx, "k2", time.Second)
		require.NoError(t, err)
		mr.FastForward(2 * time.Second)
		err = svc.CheckAndSet(ctx, "k2", time.Second)
		require.NoError(t, err)
	})

	t.Run("Should propagate client errors", func(t *testing.T) {
		svc := NewRedisService(badClient{})
		err := svc.CheckAndSet(t.Context(), "k3", time.Minute)
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

	t.Run("Should trim spaces from header value", func(t *testing.T) {
		h := make(http.Header)
		h.Set(HeaderIdempotencyKey, "  hdr-456  ")
		got, err := DeriveKey(h, []byte(`{"id":"body-456"}`), "id")
		require.NoError(t, err)
		assert.Equal(t, "hdr-456", got)
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

	t.Run("Should support nested JSON fields with dot notation", func(t *testing.T) {
		h := make(http.Header)
		got, err := DeriveKey(h, []byte(`{"data":{"id":"nested-123"}}`), "data.id")
		require.NoError(t, err)
		assert.Equal(t, "nested-123", got)
	})

	t.Run("Should support deeply nested JSON fields", func(t *testing.T) {
		h := make(http.Header)
		got, err := DeriveKey(h, []byte(`{"data":{"user":{"profile":{"id":"deep-456"}}}}`), "data.user.profile.id")
		require.NoError(t, err)
		assert.Equal(t, "deep-456", got)
	})

	t.Run("Should return ErrKeyNotFound when nested path doesn't exist", func(t *testing.T) {
		h := make(http.Header)
		_, err := DeriveKey(h, []byte(`{"data":{"user":{"name":"john"}}}`), "data.user.id")
		assert.ErrorIs(t, err, ErrKeyNotFound)
	})

	t.Run("Should return ErrKeyNotFound when intermediate path is not an object", func(t *testing.T) {
		h := make(http.Header)
		_, err := DeriveKey(h, []byte(`{"data":{"user":"not-an-object"}}`), "data.user.id")
		assert.ErrorIs(t, err, ErrKeyNotFound)
	})

	t.Run("Should coerce non-string nested JSON values", func(t *testing.T) {
		h := make(http.Header)
		got, err := DeriveKey(h, []byte(`{"data":{"user":{"id":789}}}`), "data.user.id")
		require.NoError(t, err)
		assert.Equal(t, "789", got)
	})

	t.Run("Should handle nested boolean values", func(t *testing.T) {
		h := make(http.Header)
		got, err := DeriveKey(h, []byte(`{"data":{"active":true}}`), "data.active")
		require.NoError(t, err)
		assert.Equal(t, "true", got)
	})

	t.Run("Should handle nested null values", func(t *testing.T) {
		h := make(http.Header)
		got, err := DeriveKey(h, []byte(`{"data":{"value":null}}`), "data.value")
		require.NoError(t, err)
		assert.Equal(t, "<nil>", got)
	})

	t.Run("Should return ErrKeyNotFound for empty nested string values", func(t *testing.T) {
		h := make(http.Header)
		_, err := DeriveKey(h, []byte(`{"data":{"id":""}}`), "data.id")
		assert.ErrorIs(t, err, ErrKeyNotFound)
	})

	t.Run("Should return ErrKeyNotFound for whitespace-only nested string values", func(t *testing.T) {
		h := make(http.Header)
		_, err := DeriveKey(h, []byte(`{"data":{"id":"   "}}`), "data.id")
		assert.ErrorIs(t, err, ErrKeyNotFound)
	})

	t.Run("Should prefer header over nested field", func(t *testing.T) {
		h := make(http.Header)
		h.Set(HeaderIdempotencyKey, "header-key")
		got, err := DeriveKey(h, []byte(`{"data":{"id":"nested-key"}}`), "data.id")
		require.NoError(t, err)
		assert.Equal(t, "header-key", got)
	})

	t.Run("Should handle nested arrays (return as string)", func(t *testing.T) {
		h := make(http.Header)
		got, err := DeriveKey(h, []byte(`{"data":{"items":[1,2,3]}}`), "data.items")
		require.NoError(t, err)
		assert.Equal(t, "[1 2 3]", got)
	})

	t.Run("Should return ErrKeyNotFound when root is not an object", func(t *testing.T) {
		h := make(http.Header)
		_, err := DeriveKey(h, []byte(`"not-an-object"`), "data.id")
		assert.ErrorIs(t, err, ErrKeyNotFound)
	})
}
