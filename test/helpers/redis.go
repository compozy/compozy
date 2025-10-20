package helpers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// RedisHelper provides Redis setup and teardown for integration tests
type RedisHelper struct {
	client    *redis.Client
	miniredis *miniredis.Miniredis
	keyPrefix string
}

// NewRedisHelper creates a new Redis helper using miniredis
func NewRedisHelper(t *testing.T) *RedisHelper {
	// Start miniredis server
	s, err := miniredis.Run()
	require.NoError(t, err, "Failed to start miniredis")
	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:         s.Addr(),
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	// Verify connection
	err = client.Ping(t.Context()).Err()
	require.NoError(t, err, "Failed to ping Redis")
	// Generate unique key prefix for test isolation
	keyPrefix := fmt.Sprintf("test:%s:%d:", t.Name(), time.Now().UnixNano())
	return &RedisHelper{
		client:    client,
		miniredis: s,
		keyPrefix: keyPrefix,
	}
}

// GetClient returns the Redis client
func (h *RedisHelper) GetClient() *redis.Client {
	return h.client
}

// GetAddr returns the Redis server address for configuration
func (h *RedisHelper) GetAddr() string {
	return h.miniredis.Addr()
}

// GetKeyPrefix returns the unique key prefix for this test
func (h *RedisHelper) GetKeyPrefix() string {
	return h.keyPrefix
}

// Key generates a namespaced key for test isolation
func (h *RedisHelper) Key(key string) string {
	return h.keyPrefix + key
}

// isPrefixed checks if a key is already prefixed to avoid double prefixing
func (h *RedisHelper) isPrefixed(key string) bool {
	return len(key) > len(h.keyPrefix) && key[:len(h.keyPrefix)] == h.keyPrefix
}

// ensureKey ensures a key has the proper prefix without double prefixing
func (h *RedisHelper) ensureKey(key string) string {
	if h.isPrefixed(key) {
		return key
	}
	return h.Key(key)
}

// Set sets a value with the test namespace
func (h *RedisHelper) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	return h.client.Set(ctx, h.Key(key), value, expiration).Err()
}

// Get gets a value with the test namespace
func (h *RedisHelper) Get(ctx context.Context, key string) (string, error) {
	return h.client.Get(ctx, h.Key(key)).Result()
}

// Delete deletes keys with the test namespace
func (h *RedisHelper) Delete(ctx context.Context, keys ...string) error {
	namespacedKeys := make([]string, len(keys))
	for i, key := range keys {
		namespacedKeys[i] = h.Key(key)
	}
	return h.client.Del(ctx, namespacedKeys...).Err()
}

// FlushNamespace deletes all keys with the test namespace prefix
func (h *RedisHelper) FlushNamespace(ctx context.Context) error {
	pattern := h.keyPrefix + "*"
	iter := h.client.Scan(ctx, 0, pattern, 0).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(keys) > 0 {
		return h.client.Del(ctx, keys...).Err()
	}
	return nil
}

// Cleanup cleans up Redis resources
func (h *RedisHelper) Cleanup(t *testing.T) {
	t.Helper()
	// Clean up namespace
	if err := h.FlushNamespace(t.Context()); err != nil {
		t.Logf("Failed to flush namespace: %v", err)
	}
	// Close client
	if err := h.client.Close(); err != nil {
		t.Logf("Failed to close Redis client: %v", err)
	}
	// Stop miniredis
	h.miniredis.Close()
}

// FastForward advances miniredis time for testing TTL
func (h *RedisHelper) FastForward(duration time.Duration) {
	h.miniredis.FastForward(duration)
}

// SetTime sets the miniredis server time
func (h *RedisHelper) SetTime(t time.Time) {
	h.miniredis.SetTime(t)
}

// AssertKeyExists asserts that a key exists (accepts both prefixed and unprefixed keys)
func (h *RedisHelper) AssertKeyExists(ctx context.Context, t *testing.T, key string) {
	exists, err := h.client.Exists(ctx, h.ensureKey(key)).Result()
	require.NoError(t, err, "Failed to check key existence")
	require.Equal(t, int64(1), exists, "Key %s does not exist", key)
}

// AssertKeyNotExists asserts that a key does not exist (accepts both prefixed and unprefixed keys)
func (h *RedisHelper) AssertKeyNotExists(ctx context.Context, t *testing.T, key string) {
	exists, err := h.client.Exists(ctx, h.ensureKey(key)).Result()
	require.NoError(t, err, "Failed to check key existence")
	require.Equal(t, int64(0), exists, "Key %s exists but should not", key)
}

// AssertTTL asserts that a key has the expected TTL (accepts both prefixed and unprefixed keys)
// Tolerance is within 1 second
func (h *RedisHelper) AssertTTL(ctx context.Context, t *testing.T, key string, expectedTTL time.Duration) {
	ttl, err := h.client.TTL(ctx, h.ensureKey(key)).Result()
	require.NoError(t, err, "Failed to get TTL")
	// Allow 1 second tolerance for TTL
	tolerance := time.Second
	require.InDelta(t, expectedTTL.Seconds(), ttl.Seconds(), tolerance.Seconds(),
		"TTL mismatch for key %s: expected %v, got %v", key, expectedTTL, ttl)
}
