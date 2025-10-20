package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
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
	Pipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error)
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
	ctx    context.Context
}

const fallbackRedisPingTimeout time.Duration = 10 * time.Second

// NewRedis creates a new Redis client with the provided configuration.
func NewRedis(ctx context.Context, cfg *Config) (*Redis, error) {
	log := logger.FromContext(ctx).With("component", "infra_redis")
	ctx = logger.ContextWithLogger(ctx, log)
	if cfg == nil {
		return nil, fmt.Errorf("redis config is required")
	}
	client, err := buildRedisClient(cfg)
	if err != nil {
		return nil, err
	}
	timeout := cfg.PingTimeout
	if timeout <= 0 {
		timeout = fallbackRedisPingTimeout
	}
	if err := pingRedis(ctx, client, timeout); err != nil {
		client.Close()
		return nil, err
	}
	logRedisConnection(ctx, cfg)
	return &Redis{
		client: client,
		config: cfg,
		ctx:    ctx,
	}, nil
}

// buildRedisClient configures the Redis client from the provided config.
func buildRedisClient(cfg *Config) (redis.UniversalClient, error) {
	if cfg.URL != "" {
		opt, err := redis.ParseURL(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("parsing Redis URL: %w", err)
		}
		applyConfigToOptions(opt, cfg)
		return redis.NewClient(opt), nil
	}
	opt := &redis.Options{
		Addr:     net.JoinHostPort(cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	}
	applyConfigToOptions(opt, cfg)
	return redis.NewClient(opt), nil
}

// pingRedis validates connectivity within the configured timeout.
func pingRedis(ctx context.Context, client redis.UniversalClient, timeout time.Duration) error {
	pingCtx, pingCancel := context.WithTimeout(ctx, timeout)
	defer pingCancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		return fmt.Errorf("pinging Redis server (timeout=%s): %w", timeout, err)
	}
	return nil
}

// logRedisConnection emits a diagnostic message after successful connection.
func logRedisConnection(ctx context.Context, cfg *Config) {
	logger.FromContext(ctx).With(
		"cache_driver", "redis",
		"host", cfg.Host,
		"port", cfg.Port,
		"db", cfg.DB,
		"pool_size", cfg.PoolSize,
		"tls_enabled", cfg.TLSEnabled,
	).Info("Redis connection established")
}

// Close shuts down the Redis connection.
func (r *Redis) Close() error {
	var err error
	r.once.Do(func() {
		err = r.client.Close()
		if err != nil {
			logger.FromContext(r.ctx).Error("Redis connection close failed", "error", err)
		} else {
			logger.FromContext(r.ctx).Debug("Redis connection closed")
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

// Pipelined executes pipelined commands with the provided callback.
func (r *Redis) Pipelined(
	ctx context.Context,
	fn func(redis.Pipeliner) error,
) ([]redis.Cmder, error) {
	return r.client.Pipelined(ctx, fn)
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
	if err := r.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	testKey := "health_check_test"
	testValue := "test_value"
	if err := r.Set(ctx, testKey, testValue, 10*time.Second).Err(); err != nil {
		return fmt.Errorf("set operation failed: %w", err)
	}
	result, err := r.Get(ctx, testKey).Result()
	if err != nil {
		return fmt.Errorf("get operation failed: %w", err)
	}
	if result != testValue {
		return fmt.Errorf("get result mismatch: expected %s, got %s", testValue, result)
	}
	if err := r.Del(ctx, testKey).Err(); err != nil {
		log.Debug("failed to clean up test key", "key", testKey, "error", err)
	}
	return nil
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
	if cfg.MinIdleConns > 0 {
		opt.MinIdleConns = cfg.MinIdleConns
	} else {
		opt.MinIdleConns = max(1, cfg.MaxIdleConns/2)
	}
	if cfg.TLSEnabled {
		if cfg.TLSConfig != nil {
			opt.TLSConfig = cfg.TLSConfig
		} else {
			opt.TLSConfig = &tls.Config{
				ServerName: cfg.Host,
				MinVersion: tls.VersionTLS12,
			}
		}
	}
}
