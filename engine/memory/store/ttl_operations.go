package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TTLOperations handles TTL and expiration-related operations for Redis memory store
type TTLOperations struct {
	keyManager *KeyManager
	client     any // Will be cast to cache.RedisInterface when needed
}

// NewTTLOperations creates a new TTL operations handler
func NewTTLOperations(keyManager *KeyManager, client any) *TTLOperations {
	return &TTLOperations{
		keyManager: keyManager,
		client:     client,
	}
}

// SetExpiration sets the TTL for a given key
func (t *TTLOperations) SetExpiration(ctx context.Context, key string, ttl time.Duration) error {
	fKey := t.keyManager.FullKey(key)
	if ttl <= 0 { // Redis EXPIRE with 0 or negative deletes the key or removes TTL. Be specific.
		// To remove TTL, use PERSIST. For this method, assume positive TTL means set, non-positive means error or no-op.
		return errors.New("TTL must be a positive duration to set expiration")
	}

	// Type assert to get the client
	client, ok := t.client.(interface {
		Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	})
	if !ok {
		return fmt.Errorf("invalid client type for TTL operations")
	}

	_, err := client.Expire(ctx, fKey, ttl).Result()
	if err != nil {
		return fmt.Errorf("failed to set expiration for key %s: %w", fKey, err)
	}
	return nil
}

// GetKeyTTL returns the remaining time-to-live for a given key
func (t *TTLOperations) GetKeyTTL(ctx context.Context, key string) (time.Duration, error) {
	fKey := t.keyManager.FullKey(key)

	// Type assert to get the client
	client, ok := t.client.(interface {
		TTL(ctx context.Context, key string) *redis.DurationCmd
	})
	if !ok {
		return 0, fmt.Errorf("invalid client type for TTL operations")
	}

	ttl, err := client.TTL(ctx, fKey).Result()
	if err == redis.Nil {
		return -2 * time.Second, nil // Key does not exist, consistent with redis.TTL behavior for non-existent keys
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL for key %s: %w", fKey, err)
	}
	// Redis returns -2 for non-existent keys and -1 for keys without expiry
	// The TTL command returns time.Duration, but miniredis might return it differently
	// Normalize special values
	if ttl == -2*time.Nanosecond {
		return -2 * time.Second, nil
	}
	if ttl == -1*time.Nanosecond {
		return -1 * time.Second, nil
	}
	return ttl, nil
}

// SetLastFlushed sets the last flushed timestamp for a key
func (t *TTLOperations) SetLastFlushed(ctx context.Context, key string, timestamp time.Time) error {
	lastFlushedKey := t.keyManager.LastFlushedKey(key)

	// Type assert to get the client
	client, ok := t.client.(interface {
		Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	})
	if !ok {
		return fmt.Errorf("invalid client type for TTL operations")
	}

	_, err := client.Set(ctx, lastFlushedKey, timestamp.Unix(), 0).Result()
	if err != nil {
		return fmt.Errorf("failed to set last flushed timestamp for key %s: %w", key, err)
	}
	return nil
}

// GetLastFlushed gets the last flushed timestamp for a key
func (t *TTLOperations) GetLastFlushed(ctx context.Context, key string) (time.Time, error) {
	lastFlushedKey := t.keyManager.LastFlushedKey(key)

	// Type assert to get the client
	client, ok := t.client.(interface {
		Get(ctx context.Context, key string) *redis.StringCmd
	})
	if !ok {
		return time.Time{}, fmt.Errorf("invalid client type for TTL operations")
	}

	result, err := client.Get(ctx, lastFlushedKey).Result()
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

	// Type assert to get the client
	client, ok := t.client.(interface {
		Get(ctx context.Context, key string) *redis.StringCmd
	})
	if !ok {
		return time.Time{}, fmt.Errorf("invalid client type for TTL operations")
	}

	result, err := client.Get(ctx, expirationKey).Result()
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
