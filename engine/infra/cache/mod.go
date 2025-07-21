package cache

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/pkg/config"
)

// Config represents the cache-specific configuration
// This combines Redis connection settings with cache behavior settings
type Config struct {
	*config.CacheConfig
	*config.RedisConfig
}

// FromAppConfig creates a cache Config from the centralized app configuration
func FromAppConfig(appConfig *config.Config) *Config {
	redisConfig := &appConfig.Redis
	cacheConfig := &appConfig.Cache

	return &Config{
		RedisConfig: redisConfig,
		CacheConfig: cacheConfig,
	}
}

type Cache struct {
	Redis        *Redis
	LockManager  LockManager
	Notification NotificationSystem
}

// SetupCache creates a new Cache instance with Redis backend
func SetupCache(ctx context.Context, config *Config) (*Cache, error) {
	if config == nil {
		return nil, fmt.Errorf("cache config cannot be nil")
	}

	redis, err := NewRedis(ctx, config)
	if err != nil {
		return nil, err
	}

	// Create lock manager for distributed locking
	lockManager, err := NewRedisLockManager(redis)
	if err != nil {
		return nil, err
	}

	// Create notification system for pub/sub
	notification, err := NewRedisNotificationSystem(redis, config)
	if err != nil {
		return nil, err
	}

	return &Cache{
		Redis:        redis,
		LockManager:  lockManager,
		Notification: notification,
	}, nil
}

// Close gracefully shuts down the cache
func (c *Cache) Close() error {
	// Close notification system first to stop subscriptions
	if c.Notification != nil {
		if err := c.Notification.Close(); err != nil {
			return fmt.Errorf("failed to close notification system: %w", err)
		}
	}

	// Close Redis connection
	if c.Redis != nil {
		if err := c.Redis.Close(); err != nil {
			return fmt.Errorf("failed to close Redis: %w", err)
		}
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
