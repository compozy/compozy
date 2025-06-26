package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// RedisInterface defines the minimal interface needed by cache operations.
// This allows both real redis.Client and mock implementations to be used.
type RedisInterface interface {
	Ping(ctx context.Context) *redis.StatusCmd
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	GetEx(ctx context.Context, key string, expiration time.Duration) *redis.StringCmd
	MGet(ctx context.Context, keys ...string) *redis.SliceCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Exists(ctx context.Context, keys ...string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	TTL(ctx context.Context, key string) *redis.DurationCmd
	Keys(ctx context.Context, pattern string) *redis.StringSliceCmd
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
	Publish(ctx context.Context, channel string, message any) *redis.IntCmd
	Subscribe(ctx context.Context, channels ...string) *redis.PubSub
	PSubscribe(ctx context.Context, patterns ...string) *redis.PubSub
	Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd
	Pipeline() redis.Pipeliner
	// List operations
	LRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd
	LLen(ctx context.Context, key string) *redis.IntCmd
	LTrim(ctx context.Context, key string, start, stop int64) *redis.StatusCmd
	RPush(ctx context.Context, key string, values ...any) *redis.IntCmd
	// Hash operations
	HSet(ctx context.Context, key string, values ...any) *redis.IntCmd
	HGet(ctx context.Context, key, field string) *redis.StringCmd
	HIncrBy(ctx context.Context, key, field string, incr int64) *redis.IntCmd
	HDel(ctx context.Context, key string, fields ...string) *redis.IntCmd
	// Transaction operations
	TxPipeline() redis.Pipeliner
	Close() error
}

type Redis struct {
	client redis.UniversalClient
	config *Config
	once   sync.Once // guarantees idempotent, race-free Close
	log    logger.Logger
}

// NewRedis creates a new Redis client with the provided configuration.
func NewRedis(ctx context.Context, cfg *Config) (*Redis, error) {
	log := logger.FromContext(ctx)
	if cfg == nil {
		return nil, fmt.Errorf("redis config is required")
	}

	// Set defaults
	cfg = setConfigDefaults(cfg)

	var client redis.UniversalClient

	// Parse URL if provided, otherwise use individual parameters
	if cfg.URL != "" {
		opt, err := redis.ParseURL(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("parsing Redis URL: %w", err)
		}
		// Override with additional config
		applyConfigToOptions(opt, cfg)
		client = redis.NewClient(opt)
	} else {
		// Use individual parameters
		opt := &redis.Options{
			Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
			Password: cfg.Password,
			DB:       cfg.DB,
		}
		applyConfigToOptions(opt, cfg)
		client = redis.NewClient(opt)
	}

	// Test connection with configurable timeout
	pingCtx, pingCancel := context.WithTimeout(ctx, cfg.PingTimeout)
	defer pingCancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("pinging Redis server: %w", err)
	}

	log.With(
		"host", cfg.Host,
		"port", cfg.Port,
		"db", cfg.DB,
		"pool_size", cfg.PoolSize,
		"tls_enabled", cfg.TLSEnabled,
	).Info("Redis connection established")

	return &Redis{
		client: client,
		config: cfg,
		log:    log,
	}, nil
}

// Close shuts down the Redis connection.
func (r *Redis) Close() error {
	var err error
	r.once.Do(func() {
		err = r.client.Close()
		if err != nil {
			r.log.Error("Redis connection close failed", "error", err)
		} else {
			r.log.Debug("Redis connection closed")
		}
	})
	return err
}

// Client returns the underlying Redis client
func (r *Redis) Client() redis.UniversalClient {
	return r.client
}

// Ping checks if the Redis server is reachable
func (r *Redis) Ping(ctx context.Context) *redis.StatusCmd {
	return r.client.Ping(ctx)
}

// Set stores a key-value pair with optional expiration
func (r *Redis) Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	return r.client.Set(ctx, key, value, expiration)
}

// SetNX stores a key-value pair only if the key does not exist
func (r *Redis) SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd {
	return r.client.SetNX(ctx, key, value, expiration)
}

// Get retrieves a value by key
func (r *Redis) Get(ctx context.Context, key string) *redis.StringCmd {
	return r.client.Get(ctx, key)
}

// GetEx retrieves a value by key and atomically extends its TTL
func (r *Redis) GetEx(ctx context.Context, key string, expiration time.Duration) *redis.StringCmd {
	return r.client.GetEx(ctx, key, expiration)
}

// MGet retrieves multiple values by keys
func (r *Redis) MGet(ctx context.Context, keys ...string) *redis.SliceCmd {
	return r.client.MGet(ctx, keys...)
}

// Del deletes one or more keys
func (r *Redis) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return r.client.Del(ctx, keys...)
}

// Exists checks if keys exist
func (r *Redis) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	return r.client.Exists(ctx, keys...)
}

// Expire sets a timeout on a key
func (r *Redis) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	return r.client.Expire(ctx, key, expiration)
}

// TTL returns the remaining time to live of a key
func (r *Redis) TTL(ctx context.Context, key string) *redis.DurationCmd {
	return r.client.TTL(ctx, key)
}

// Keys returns all keys matching pattern
func (r *Redis) Keys(ctx context.Context, pattern string) *redis.StringSliceCmd {
	return r.client.Keys(ctx, pattern)
}

// Scan incrementally iterates over keys
func (r *Redis) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	return r.client.Scan(ctx, cursor, match, count)
}

// Publish sends a message to a channel
func (r *Redis) Publish(ctx context.Context, channel string, message any) *redis.IntCmd {
	return r.client.Publish(ctx, channel, message)
}

// Subscribe subscribes to channels
func (r *Redis) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return r.client.Subscribe(ctx, channels...)
}

// PSubscribe subscribes to channels matching patterns
func (r *Redis) PSubscribe(ctx context.Context, patterns ...string) *redis.PubSub {
	return r.client.PSubscribe(ctx, patterns...)
}

// Eval executes a Lua script
func (r *Redis) Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
	return r.client.Eval(ctx, script, keys, args...)
}

// Pipeline returns a pipeline for batching commands
func (r *Redis) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

// LRange returns a range of elements from a list
func (r *Redis) LRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd {
	return r.client.LRange(ctx, key, start, stop)
}

// LLen returns the length of a list
func (r *Redis) LLen(ctx context.Context, key string) *redis.IntCmd {
	return r.client.LLen(ctx, key)
}

// LTrim trims a list to the specified range
func (r *Redis) LTrim(ctx context.Context, key string, start, stop int64) *redis.StatusCmd {
	return r.client.LTrim(ctx, key, start, stop)
}

// RPush appends values to a list
func (r *Redis) RPush(ctx context.Context, key string, values ...any) *redis.IntCmd {
	return r.client.RPush(ctx, key, values...)
}

// HSet sets field in the hash stored at key to value
func (r *Redis) HSet(ctx context.Context, key string, values ...any) *redis.IntCmd {
	return r.client.HSet(ctx, key, values...)
}

// HGet returns the value associated with field in the hash stored at key
func (r *Redis) HGet(ctx context.Context, key, field string) *redis.StringCmd {
	return r.client.HGet(ctx, key, field)
}

// HIncrBy increments the number stored at field in the hash stored at key by increment
func (r *Redis) HIncrBy(ctx context.Context, key, field string, incr int64) *redis.IntCmd {
	return r.client.HIncrBy(ctx, key, field, incr)
}

// HDel removes the specified fields from the hash stored at key
func (r *Redis) HDel(ctx context.Context, key string, fields ...string) *redis.IntCmd {
	return r.client.HDel(ctx, key, fields...)
}

// TxPipeline returns a transactional pipeline
func (r *Redis) TxPipeline() redis.Pipeliner {
	return r.client.TxPipeline()
}

// HealthCheck performs a comprehensive health check
func (r *Redis) HealthCheck(ctx context.Context) error {
	log := logger.FromContext(ctx)
	// Test basic connectivity
	if err := r.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	// Test basic operations
	testKey := "health_check_test"
	testValue := "test_value"

	// Set a test value
	if err := r.Set(ctx, testKey, testValue, 10*time.Second).Err(); err != nil {
		return fmt.Errorf("set operation failed: %w", err)
	}

	// Get the test value
	result, err := r.Get(ctx, testKey).Result()
	if err != nil {
		return fmt.Errorf("get operation failed: %w", err)
	}

	if result != testValue {
		return fmt.Errorf("get result mismatch: expected %s, got %s", testValue, result)
	}

	// Clean up test key
	if err := r.Del(ctx, testKey).Err(); err != nil {
		log.Debug("failed to clean up test key", "key", testKey, "error", err)
	}

	return nil
}

// setConfigDefaults applies default values to configuration
func setConfigDefaults(cfg *Config) *Config {
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port == "" {
		cfg.Port = "6379"
	}
	if cfg.PoolSize == 0 {
		cfg.PoolSize = 10
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 2 * time.Second // Reduced from 5s for faster startup
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 3 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 3 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.MinRetryBackoff == 0 {
		cfg.MinRetryBackoff = 8 * time.Millisecond
	}
	if cfg.MaxRetryBackoff == 0 {
		cfg.MaxRetryBackoff = 512 * time.Millisecond
	}
	if cfg.PoolTimeout == 0 {
		cfg.PoolTimeout = 4 * time.Second
	}
	if cfg.PingTimeout == 0 {
		cfg.PingTimeout = 500 * time.Millisecond
	}
	return cfg
}

// applyConfigToOptions applies configuration to Redis options
func applyConfigToOptions(opt *redis.Options, cfg *Config) {
	opt.PoolSize = cfg.PoolSize
	opt.DialTimeout = cfg.DialTimeout
	opt.ReadTimeout = cfg.ReadTimeout
	opt.WriteTimeout = cfg.WriteTimeout
	opt.MaxRetries = cfg.MaxRetries
	opt.MinRetryBackoff = cfg.MinRetryBackoff
	opt.MaxRetryBackoff = cfg.MaxRetryBackoff
	opt.PoolTimeout = cfg.PoolTimeout

	// TLS Configuration
	if cfg.TLSEnabled {
		if cfg.TLSConfig != nil {
			opt.TLSConfig = cfg.TLSConfig
		} else {
			// Use default TLS config
			opt.TLSConfig = &tls.Config{
				ServerName: cfg.Host,
				MinVersion: tls.VersionTLS12,
			}
		}
	}
}

// getEnvOrDefault returns the environment variable value or the default value
func getEnvOrDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}
