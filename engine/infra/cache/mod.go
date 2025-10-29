package cache

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	modeStandalone  = "standalone"
	modeDistributed = "distributed"
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
	// embedded holds the standalone miniredis server when running in standalone
	// mode. It remains nil when using an external (distributed) Redis backend.
	embedded *MiniredisStandalone
}

// SetupCache creates a mode-aware cache backend using configuration from context.
// It returns a unified Cache object plus a cleanup function safe to call multiple times.
//
// Modes (resolved via cfg.EffectiveRedisMode()):
//   - "standalone": starts an embedded miniredis and connects a client to it
//   - "distributed" (default): connects to external Redis using provided settings
func SetupCache(ctx context.Context) (*Cache, func(), error) {
	log := logger.FromContext(ctx)
	appCfg := config.FromContext(ctx)
	if appCfg == nil {
		return nil, nil, fmt.Errorf("missing configuration in context")
	}

	cacheCfg := FromAppConfig(appCfg)
	mode := appCfg.EffectiveRedisMode()
	log.Info("Initializing cache backend", "mode", mode)

	switch mode {
	case modeStandalone:
		return setupStandaloneCache(ctx, cacheCfg)

	case modeDistributed:
		return setupDistributedCache(ctx, cacheCfg)

	default:
		return nil, nil, fmt.Errorf("unsupported redis mode: %s", mode)
	}
}

// setupStandaloneCache creates embedded miniredis backend and wraps it with Redis facade.
func setupStandaloneCache(ctx context.Context, cacheCfg *Config) (*Cache, func(), error) {
	log := logger.FromContext(ctx)
	mr, err := NewMiniredisStandalone(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("create miniredis standalone: %w", err)
	}
	r := &Redis{client: mr.Client(), config: cacheCfg, ctx: ctx}
	lm, err := NewRedisLockManager(r)
	if err != nil {
		_ = mr.Close(ctx)
		return nil, nil, err
	}
	ns, err := NewRedisNotificationSystem(r, cacheCfg)
	if err != nil {
		_ = mr.Close(ctx)
		return nil, nil, err
	}
	c := &Cache{Redis: r, LockManager: lm, Notification: ns, embedded: mr}
	cleanup := func() {
		_ = c.Notification.Close()
		_ = c.Redis.Close()
		_ = mr.Close(ctx)
	}
	log.Info("Standalone cache initialized")
	return c, cleanup, nil
}

// setupDistributedCache creates external Redis backend using application configuration.
func setupDistributedCache(ctx context.Context, cacheCfg *Config) (*Cache, func(), error) {
	log := logger.FromContext(ctx)
	r, err := NewRedis(ctx, cacheCfg)
	if err != nil {
		return nil, nil, err
	}
	lm, err := NewRedisLockManager(r)
	if err != nil {
		_ = r.Close()
		return nil, nil, err
	}
	ns, err := NewRedisNotificationSystem(r, cacheCfg)
	if err != nil {
		_ = r.Close()
		return nil, nil, err
	}
	c := &Cache{Redis: r, LockManager: lm, Notification: ns}
	cleanup := func() { _ = c.Close() }
	log.Info("Distributed cache initialized")
	return c, cleanup, nil
}

// Close gracefully shuts down the cache
func (c *Cache) Close() error {
	if c.Notification != nil {
		if err := c.Notification.Close(); err != nil {
			return fmt.Errorf("failed to close notification system: %w", err)
		}
	}
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
