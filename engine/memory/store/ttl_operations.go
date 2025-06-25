package store

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient defines the Redis operations needed for TTL management
type RedisClient interface {
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	TTL(ctx context.Context, key string) *redis.DurationCmd
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
}

// TTLOperations handles TTL and expiration-related operations for Redis memory store
type TTLOperations struct {
	keyManager *KeyManager
	client     RedisClient
}

// NewTTLOperations creates a new TTL operations handler
func NewTTLOperations(keyManager *KeyManager, client RedisClient) *TTLOperations {
	return &TTLOperations{
		keyManager: keyManager,
		client:     client,
	}
}

// SetExpiration sets the TTL for a given key
func (t *TTLOperations) SetExpiration(ctx context.Context, key string, ttl time.Duration) error {
	fKey := t.keyManager.FullKey(key)
	metaKey := t.keyManager.MetadataKey(key)
	if ttl <= 0 { // Redis EXPIRE with 0 or negative deletes the key or removes TTL. Be specific.
		// Redis EXPIRE with 0 deletes the key immediately
		// For TTL removal, use a separate method or make this behavior explicit
		return fmt.Errorf("TTL must be positive to set expiration, got %v", ttl)
	}

	// Set TTL on both the main key and metadata key
	_, err := t.client.Expire(ctx, fKey, ttl).Result()
	if err != nil {
		return fmt.Errorf("failed to set expiration for key %s: %w", fKey, err)
	}

	// Also set TTL on metadata key
	_, err = t.client.Expire(ctx, metaKey, ttl).Result()
	if err != nil {
		// Metadata key TTL failure should be logged but not fatal
		return fmt.Errorf("failed to set expiration for metadata key %s: %w", metaKey, err)
	}

	return nil
}

// GetKeyTTL returns the remaining time-to-live for a given key
func (t *TTLOperations) GetKeyTTL(ctx context.Context, key string) (time.Duration, error) {
	fKey := t.keyManager.FullKey(key)

	ttl, err := t.client.TTL(ctx, fKey).Result()
	if err == redis.Nil {
		return -2 * time.Second, nil // Key does not exist, consistent with redis.TTL behavior for non-existent keys
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL for key %s: %w", fKey, err)
	}
	// Normalize Redis TTL special values to standard durations
	switch ttl {
	case -2 * time.Nanosecond:
		return -2 * time.Second, nil // Key does not exist
	case -1 * time.Nanosecond:
		return -1 * time.Second, nil // Key exists but has no expiry
	}
	return ttl, nil
}

// SetLastFlushed sets the last flushed timestamp for a key
func (t *TTLOperations) SetLastFlushed(ctx context.Context, key string, timestamp time.Time) error {
	lastFlushedKey := t.keyManager.LastFlushedKey(key)

	_, err := t.client.Set(ctx, lastFlushedKey, timestamp.Unix(), 0).Result()
	if err != nil {
		return fmt.Errorf("failed to set last flushed timestamp for key %s: %w", key, err)
	}
	return nil
}

// GetLastFlushed gets the last flushed timestamp for a key
func (t *TTLOperations) GetLastFlushed(ctx context.Context, key string) (time.Time, error) {
	lastFlushedKey := t.keyManager.LastFlushedKey(key)

	result, err := t.client.Get(ctx, lastFlushedKey).Result()
	if err == redis.Nil {
		return time.Time{}, nil // Not found
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get last flushed timestamp for key %s: %w", key, err)
	}

	var timestamp int64
	if _, err := fmt.Sscanf(result, "%d", &timestamp); err != nil {
		return time.Time{}, fmt.Errorf("failed to parse last flushed timestamp for key %s: %w", key, err)
	}

	return time.Unix(timestamp, 0), nil
}

// GetExpiration gets the expiration timestamp for a key
func (t *TTLOperations) GetExpiration(ctx context.Context, key string) (time.Time, error) {
	expirationKey := t.keyManager.ExpirationKey(key)

	result, err := t.client.Get(ctx, expirationKey).Result()
	if err == redis.Nil {
		return time.Time{}, nil // Not found
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get expiration timestamp for key %s: %w", key, err)
	}

	var timestamp int64
	if _, err := fmt.Sscanf(result, "%d", &timestamp); err != nil {
		return time.Time{}, fmt.Errorf("failed to parse expiration timestamp for key %s: %w", key, err)
	}

	return time.Unix(timestamp, 0), nil
}
