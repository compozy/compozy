package mcpproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// Storage defines the interface for MCP definition persistence
type Storage interface {
	SaveMCP(ctx context.Context, def *MCPDefinition) error
	LoadMCP(ctx context.Context, name string) (*MCPDefinition, error)
	DeleteMCP(ctx context.Context, name string) error
	ListMCPs(ctx context.Context) ([]*MCPDefinition, error)
	SaveStatus(ctx context.Context, status *MCPStatus) error
	LoadStatus(ctx context.Context, name string) (*MCPStatus, error)
	Close() error
}

// RedisStorage implements Storage using Redis
type RedisStorage struct {
	client *redis.Client
	prefix string
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DefaultRedisConfig returns a default Redis configuration
func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

// NewRedisStorage creates a new Redis-based storage instance
func NewRedisStorage(config *RedisConfig) (*RedisStorage, error) {
	if config == nil {
		config = DefaultRedisConfig()
	}

	client := redis.NewClient(&redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		MaxRetries:   config.MaxRetries,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	})

	// Test the connection to ensure Redis is accessible
	ctx, cancel := context.WithTimeout(context.Background(), AdminHealthCheckTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStorage{
		client: client,
		prefix: "mcp_proxy",
	}, nil
}

// Ping tests the Redis connection
func (r *RedisStorage) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close closes the Redis connection
func (r *RedisStorage) Close() error {
	return r.client.Close()
}

// SaveMCP saves an MCP definition to Redis
func (r *RedisStorage) SaveMCP(ctx context.Context, def *MCPDefinition) error {
	if def == nil {
		return fmt.Errorf("definition cannot be nil")
	}

	if err := def.Validate(); err != nil {
		return fmt.Errorf("invalid definition: %w", err)
	}

	def.SetDefaults()

	data, err := json.Marshal(def)
	if err != nil {
		return fmt.Errorf("failed to marshal definition: %w", err)
	}

	key := r.getMCPKey(def.Name)
	if err := r.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save definition to Redis: %w", err)
	}

	return nil
}

// LoadMCP loads an MCP definition from Redis
func (r *RedisStorage) LoadMCP(ctx context.Context, name string) (*MCPDefinition, error) {
	if name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}

	key := r.getMCPKey(name)
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("MCP definition '%s' not found", name)
		}
		return nil, fmt.Errorf("failed to load definition from Redis: %w", err)
	}

	var def MCPDefinition
	if err := json.Unmarshal([]byte(data), &def); err != nil {
		return nil, fmt.Errorf("failed to unmarshal definition: %w", err)
	}

	def.SetDefaults()

	return &def, nil
}

// DeleteMCP deletes an MCP definition from Redis
func (r *RedisStorage) DeleteMCP(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	key := r.getMCPKey(name)
	deleted, err := r.client.Del(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to delete definition from Redis: %w", err)
	}

	if deleted == 0 {
		return fmt.Errorf("MCP definition '%s' not found", name)
	}

	// Also delete the status if it exists
	statusKey := r.getStatusKey(name)
	if _, err := r.client.Del(ctx, statusKey).Result(); err != nil {
		logger.Warn("Failed to delete status during MCP deletion", "name", name, "error", err)
	}

	return nil
}

// ListMCPs lists all MCP definitions from Redis using SCAN for better performance
func (r *RedisStorage) ListMCPs(ctx context.Context) ([]*MCPDefinition, error) {
	pattern := r.getMCPKey("*")
	var definitions []*MCPDefinition
	var cursor uint64

	for {
		// Use SCAN instead of KEYS to avoid blocking Redis
		keys, nextCursor, err := r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to scan keys from Redis: %w", err)
		}

		if len(keys) > 0 {
			// Get values for this batch of keys
			values, err := r.client.MGet(ctx, keys...).Result()
			if err != nil {
				return nil, fmt.Errorf("failed to get values from Redis: %w", err)
			}

			// Process each value in this batch
			for i, value := range values {
				if value == nil {
					// Skip nil values (key was deleted between SCAN and MGET)
					continue
				}

				data, ok := value.(string)
				if !ok {
					logger.Warn("Unexpected value type for key", "key", keys[i])
					continue
				}

				var def MCPDefinition
				if err := json.Unmarshal([]byte(data), &def); err != nil {
					logger.Warn("Failed to unmarshal definition", "key", keys[i], "error", err)
					continue
				}

				def.SetDefaults()
				// create a copy to preserve unique address per iteration
				defCopy := def
				definitions = append(definitions, &defCopy)
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break // Scan complete
		}
	}

	return definitions, nil
}

// SaveStatus saves an MCP status to Redis
func (r *RedisStorage) SaveStatus(ctx context.Context, status *MCPStatus) error {
	if status == nil {
		return fmt.Errorf("status cannot be nil")
	}

	if status.Name == "" {
		return fmt.Errorf("status name cannot be empty")
	}

	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	key := r.getStatusKey(status.Name)
	if err := r.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save status to Redis: %w", err)
	}

	return nil
}

// LoadStatus loads an MCP status from Redis
func (r *RedisStorage) LoadStatus(ctx context.Context, name string) (*MCPStatus, error) {
	if name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}

	key := r.getStatusKey(name)
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Return default status if not found
			return NewMCPStatus(name), nil
		}
		return nil, fmt.Errorf("failed to load status from Redis: %w", err)
	}

	var status MCPStatus
	if err := json.Unmarshal([]byte(data), &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal status: %w", err)
	}

	return &status, nil
}

// getMCPKey returns the Redis key for an MCP definition
func (r *RedisStorage) getMCPKey(name string) string {
	return fmt.Sprintf("%s:mcps:%s", r.prefix, name)
}

// getStatusKey returns the Redis key for an MCP status
func (r *RedisStorage) getStatusKey(name string) string {
	return fmt.Sprintf("%s:status:%s", r.prefix, name)
}

// ExtractNameFromKey extracts the MCP name from a Redis key
func (r *RedisStorage) ExtractNameFromKey(key string) string {
	prefix := r.getMCPKey("")
	if after, ok := strings.CutPrefix(key, prefix); ok {
		return after
	}
	return ""
}

// Health returns the health status of the Redis connection
func (r *RedisStorage) Health(ctx context.Context) error {
	return r.Ping(ctx)
}

// Stats returns Redis connection statistics
func (r *RedisStorage) Stats() *redis.PoolStats {
	return r.client.PoolStats()
}
