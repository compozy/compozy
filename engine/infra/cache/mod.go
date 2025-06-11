package cache

import (
	"context"
	"os"
	"strconv"
	"time"
)

type Cache struct {
	Redis        *Redis
	LockManager  LockManager
	Notification NotificationSystem
}

// SetupCache creates a new Cache instance with Redis backend
func SetupCache(ctx context.Context, config *Config) (*Cache, error) {
	if config == nil {
		config = buildConfigFromEnv()
	}

	redis, err := NewRedis(ctx, config)
	if err != nil {
		return nil, err
	}

	// Create lock manager for distributed locking
	lockManager := NewRedisLockManager(redis)

	// Create notification system for pub/sub
	notification := NewRedisNotificationSystem(redis)

	return &Cache{
		Redis:        redis,
		LockManager:  lockManager,
		Notification: notification,
	}, nil
}

// buildConfigFromEnv creates a Redis config from environment variables
func buildConfigFromEnv() *Config {
	config := &Config{
		URL:      os.Getenv("REDIS_URL"),
		Host:     getEnvOrDefault(os.Getenv("REDIS_HOST"), "localhost"),
		Port:     getEnvOrDefault(os.Getenv("REDIS_PORT"), "6379"),
		Password: os.Getenv("REDIS_PASSWORD"),
	}

	// Parse integer values with defaults
	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		if db, err := strconv.Atoi(dbStr); err == nil {
			config.DB = db
		}
	}

	if poolSizeStr := os.Getenv("REDIS_POOL_SIZE"); poolSizeStr != "" {
		if poolSize, err := strconv.Atoi(poolSizeStr); err == nil {
			config.PoolSize = poolSize
		}
	}

	// Parse duration values with defaults
	if timeoutStr := os.Getenv("REDIS_DIAL_TIMEOUT"); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			config.DialTimeout = timeout
		}
	}

	if timeoutStr := os.Getenv("REDIS_READ_TIMEOUT"); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			config.ReadTimeout = timeout
		}
	}

	if timeoutStr := os.Getenv("REDIS_WRITE_TIMEOUT"); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			config.WriteTimeout = timeout
		}
	}

	// Parse TLS configuration
	if tlsStr := os.Getenv("REDIS_TLS"); tlsStr != "" {
		if tls, err := strconv.ParseBool(tlsStr); err == nil {
			config.TLSEnabled = tls
		}
	}

	return config
}

// Close gracefully shuts down the cache
func (c *Cache) Close() error {
	// Close notification system first to stop subscriptions
	if c.Notification != nil {
		c.Notification.Close()
	}

	// Close Redis connection
	if c.Redis != nil {
		return c.Redis.Close()
	}
	return nil
}

// HealthCheck performs a health check on all cache components
func (c *Cache) HealthCheck(ctx context.Context) error {
	if c.Redis != nil {
		return c.Redis.HealthCheck(ctx)
	}
	return nil
}
