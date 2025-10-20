package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/compozy/compozy/engine/task"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// TestConfigStore implements services.ConfigStore for testing using miniredis
type TestConfigStore struct {
	mu        sync.RWMutex
	client    *redis.Client
	miniredis *miniredis.Miniredis
	ttl       time.Duration
	baseCtx   context.Context
}

// NewTestConfigStore creates a new test config store using miniredis
func NewTestConfigStore(t *testing.T) *TestConfigStore {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
		DB:   0,
	})
	ctx := t.Context()
	err := client.Ping(ctx).Err()
	require.NoError(t, err, "Failed to ping miniredis")
	return &TestConfigStore{
		client:    client,
		miniredis: mr,
		ttl:       10 * time.Minute,
		baseCtx:   context.WithoutCancel(ctx),
	}
}

// NewTestConfigStoreWithTTL creates a test config store with custom TTL
func NewTestConfigStoreWithTTL(t *testing.T, ttl time.Duration) *TestConfigStore {
	store := NewTestConfigStore(t)
	store.ttl = ttl
	return store
}

func (s *TestConfigStore) Save(ctx context.Context, key string, config *task.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Serialize config to JSON
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	// Store in Redis with TTL
	err = s.client.Set(ctx, s.configKey(key), data, s.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

func (s *TestConfigStore) Get(ctx context.Context, key string) (*task.Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Get from Redis
	data, err := s.client.Get(ctx, s.configKey(key)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("config not found for taskExecID %s", key)
		}
		return nil, fmt.Errorf("failed to get config: %w", err)
	}
	// Deserialize config
	var config task.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	// Extend TTL on read (similar to the actual Redis store)
	s.client.Expire(ctx, s.configKey(key), s.ttl)
	return &config, nil
}

func (s *TestConfigStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.client.Del(ctx, s.configKey(key)).Err()
	if err != nil {
		return fmt.Errorf("failed to delete config: %w", err)
	}
	return nil
}

func (s *TestConfigStore) SaveMetadata(ctx context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.client.Set(ctx, s.metadataKey(key), data, s.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}
	return nil
}

func (s *TestConfigStore) GetMetadata(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := s.client.Get(ctx, s.metadataKey(key)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("metadata not found for key %s", key)
		}
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}
	// Extend TTL on read
	s.client.Expire(ctx, s.metadataKey(key), s.ttl)
	return data, nil
}

func (s *TestConfigStore) DeleteMetadata(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.client.Del(ctx, s.metadataKey(key)).Err()
	if err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}
	return nil
}

func (s *TestConfigStore) Close() error {
	// Close Redis client
	if err := s.client.Close(); err != nil {
		return fmt.Errorf("failed to close Redis client: %w", err)
	}
	// Close miniredis
	s.miniredis.Close()
	return nil
}

// configKey generates the Redis key for a config
func (s *TestConfigStore) configKey(key string) string {
	return fmt.Sprintf("test:config:%s", key)
}

// metadataKey generates the Redis key for metadata
func (s *TestConfigStore) metadataKey(key string) string {
	return fmt.Sprintf("test:metadata:%s", key)
}

// FastForward advances miniredis time for testing TTL
func (s *TestConfigStore) FastForward(duration time.Duration) {
	s.miniredis.FastForward(duration)
}

// Flush clears all data in the test store
func (s *TestConfigStore) Flush() error {
	return s.client.FlushDB(s.baseCtx).Err()
}
