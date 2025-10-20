package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/redis/go-redis/v9"
)

const (
	// DefaultTTL is the default time-to-live for task configurations (24 hours)
	DefaultTTL = 24 * time.Hour
	// MetadataKeyPrefix is used to namespace metadata keys to avoid collisions
	MetadataKeyPrefix = "metadata:"
	// ConfigKeyPrefix is used to namespace config keys
	ConfigKeyPrefix = "config:"
)

// redisConfigStore implements ConfigStore using Redis for persistence
type redisConfigStore struct {
	redis *cache.Redis
	ttl   time.Duration
}

// NewRedisConfigStore creates a new Redis-backed config store
func NewRedisConfigStore(redis *cache.Redis, ttl time.Duration) ConfigStore {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &redisConfigStore{
		redis: redis,
		ttl:   ttl,
	}
}

// Save persists a task configuration with the given taskExecID as key
func (s *redisConfigStore) Save(ctx context.Context, taskExecID string, config *task.Config) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	if taskExecID == "" {
		return fmt.Errorf("taskExecID cannot be empty")
	}
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	// Marshal config to JSON
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config for taskExecID %s: %w", taskExecID, err)
	}
	// Save to Redis with TTL
	key := ConfigKeyPrefix + taskExecID
	if err := s.redis.Set(ctx, key, data, s.ttl).Err(); err != nil {
		return fmt.Errorf("failed to save config for taskExecID %s: %w", taskExecID, err)
	}
	log.With("task_exec_id", taskExecID, "ttl", s.ttl).Debug("Task config saved to Redis")
	return nil
}

// Get retrieves a task configuration by taskExecID and atomically extends TTL
func (s *redisConfigStore) Get(ctx context.Context, taskExecID string) (*task.Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	if taskExecID == "" {
		return nil, fmt.Errorf("taskExecID cannot be empty")
	}
	key := ConfigKeyPrefix + taskExecID
	data, err := s.redis.GetEx(ctx, key, s.ttl).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("config not found for taskExecID %s", taskExecID)
		}
		return nil, fmt.Errorf("failed to get config for taskExecID %s: %w", taskExecID, err)
	}
	// Unmarshal JSON to config
	var config task.Config
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config for taskExecID %s: %w", taskExecID, err)
	}
	log.With("task_exec_id", taskExecID, "ttl_extended", s.ttl).
		Debug("Task config retrieved from Redis with TTL extended")
	return &config, nil
}

// Delete removes a task configuration by taskExecID
func (s *redisConfigStore) Delete(ctx context.Context, taskExecID string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	if taskExecID == "" {
		return fmt.Errorf("taskExecID cannot be empty")
	}
	key := ConfigKeyPrefix + taskExecID
	deleted, err := s.redis.Del(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to delete config for taskExecID %s: %w", taskExecID, err)
	}
	if deleted > 0 {
		log.With("task_exec_id", taskExecID).Debug("Task config deleted from Redis")
	} else {
		log.With("task_exec_id", taskExecID).Debug("Task config not found for deletion")
	}
	return nil
}

// SaveMetadata persists arbitrary metadata with the given key
func (s *redisConfigStore) SaveMetadata(ctx context.Context, key string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if data == nil {
		return fmt.Errorf("data cannot be nil")
	}
	// Save to Redis with metadata prefix to avoid collisions
	prefixedKey := MetadataKeyPrefix + key
	if err := s.redis.Set(ctx, prefixedKey, data, s.ttl).Err(); err != nil {
		return fmt.Errorf("failed to save metadata for key %s: %w", key, err)
	}
	log.With("metadata_key", key, "ttl", s.ttl).Debug("Metadata saved to Redis")
	return nil
}

// GetMetadata retrieves metadata by key and atomically extends TTL
func (s *redisConfigStore) GetMetadata(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}
	prefixedKey := MetadataKeyPrefix + key
	data, err := s.redis.GetEx(ctx, prefixedKey, s.ttl).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("metadata not found for key %s", key)
		}
		return nil, fmt.Errorf("failed to get metadata for key %s: %w", key, err)
	}
	log.With("metadata_key", key, "ttl_extended", s.ttl).Debug("Metadata retrieved from Redis with TTL extended")
	return []byte(data), nil
}

// DeleteMetadata removes metadata by key
func (s *redisConfigStore) DeleteMetadata(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}
	log := logger.FromContext(ctx)
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	prefixedKey := MetadataKeyPrefix + key
	deleted, err := s.redis.Del(ctx, prefixedKey).Result()
	if err != nil {
		return fmt.Errorf("failed to delete metadata for key %s: %w", key, err)
	}
	if deleted > 0 {
		log.With("metadata_key", key).Debug("Metadata deleted from Redis")
	} else {
		log.With("metadata_key", key).Debug("Metadata not found for deletion")
	}
	return nil
}

// Close closes the underlying Redis connection and releases resources
func (s *redisConfigStore) Close() error {
	if s.redis != nil {
		return s.redis.Close()
	}
	return nil
}

// HealthCheck performs a health check on the Redis store
func (s *redisConfigStore) HealthCheck(ctx context.Context) error {
	return s.redis.HealthCheck(ctx)
}

// GetKeys returns all keys matching the pattern (useful for debugging and migration)
func (s *redisConfigStore) GetKeys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	var cursor uint64
	for {
		batch, nextCursor, err := s.redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to scan keys with pattern %s: %w", pattern, err)
		}
		keys = append(keys, batch...)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return keys, nil
}

// GetAllConfigKeys returns all task configuration keys
func (s *redisConfigStore) GetAllConfigKeys(ctx context.Context) ([]string, error) {
	return s.GetKeys(ctx, ConfigKeyPrefix+"*")
}

// GetAllMetadataKeys returns all metadata keys
func (s *redisConfigStore) GetAllMetadataKeys(ctx context.Context) ([]string, error) {
	return s.GetKeys(ctx, MetadataKeyPrefix+"*")
}

// ExtendTTL extends the TTL of a task configuration
func (s *redisConfigStore) ExtendTTL(ctx context.Context, taskExecID string, ttl time.Duration) error {
	log := logger.FromContext(ctx)
	if taskExecID == "" {
		return fmt.Errorf("taskExecID cannot be empty")
	}
	key := ConfigKeyPrefix + taskExecID
	if err := s.redis.Expire(ctx, key, ttl).Err(); err != nil {
		return fmt.Errorf("failed to extend TTL for taskExecID %s: %w", taskExecID, err)
	}
	log.With("task_exec_id", taskExecID, "new_ttl", ttl).Debug("Task config TTL extended")
	return nil
}

// GetTTL returns the remaining TTL of a task configuration
func (s *redisConfigStore) GetTTL(ctx context.Context, taskExecID string) (time.Duration, error) {
	if taskExecID == "" {
		return 0, fmt.Errorf("taskExecID cannot be empty")
	}
	key := ConfigKeyPrefix + taskExecID
	ttl, err := s.redis.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL for taskExecID %s: %w", taskExecID, err)
	}
	return ttl, nil
}
