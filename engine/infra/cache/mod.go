package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	modeMemory                = config.ModeMemory
	modePersistent            = config.ModePersistent
	modeDistributed           = config.ModeDistributed
	defaultPersistenceDataDir = "./.compozy/redis"
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
	// embedded holds the embedded miniredis server when running in memory or
	// persistent modes. It remains nil when using an external (distributed) Redis
	// backend.
	embedded *MiniredisStandalone
}

// SetupCache creates a mode-aware cache backend using configuration from context.
// It returns a unified Cache object plus a cleanup function safe to call multiple times.
//
// Modes (resolved via cfg.EffectiveRedisMode()):
//   - "memory": starts an embedded miniredis without persistence
//   - "persistent": starts an embedded miniredis with persistence enabled
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
	case modeMemory:
		return setupMemoryCache(ctx, cacheCfg)

	case modePersistent:
		return setupPersistentCache(ctx, cacheCfg)

	case modeDistributed:
		log.Info("Cache in distributed mode (external Redis)")
		return setupDistributedCache(ctx, cacheCfg)

	default:
		return nil, nil, fmt.Errorf("unsupported redis mode: %s", mode)
	}
}

func setupMemoryCache(ctx context.Context, cacheCfg *Config) (*Cache, func(), error) {
	redisCfg := cacheCfg.RedisConfig
	if redisCfg == nil {
		return nil, nil, fmt.Errorf("missing redis configuration for memory mode")
	}
	persistence := &redisCfg.Standalone.Persistence
	previouslyEnabled := persistence.Enabled
	persistence.Enabled = false
	log := logger.FromContext(ctx)
	log.Info("Cache in memory mode",
		"persistence_enabled", persistence.Enabled,
		"previously_enabled", previouslyEnabled,
	)
	return setupStandaloneCache(ctx, cacheCfg, modeMemory)
}

func setupPersistentCache(ctx context.Context, cacheCfg *Config) (*Cache, func(), error) {
	redisCfg := cacheCfg.RedisConfig
	if redisCfg == nil {
		return nil, nil, fmt.Errorf("missing redis configuration for persistent mode")
	}
	persistence := &redisCfg.Standalone.Persistence
	if !persistence.Enabled {
		persistence.Enabled = true
	}
	defaultedDataDir := false
	if persistence.DataDir == "" {
		persistence.DataDir = defaultPersistenceDataDir
		defaultedDataDir = true
	}
	log := logger.FromContext(ctx)
	log.Info("Cache in persistent mode",
		"persistence_enabled", persistence.Enabled,
		"data_dir", persistence.DataDir,
		"default_data_dir", defaultedDataDir,
	)
	return setupStandaloneCache(ctx, cacheCfg, modePersistent)
}

// setupStandaloneCache creates embedded miniredis backend and wraps it with Redis facade.
func setupStandaloneCache(ctx context.Context, cacheCfg *Config, mode string) (*Cache, func(), error) {
	log := logger.FromContext(ctx)
	mr, err := NewMiniredisStandalone(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("create miniredis standalone: %w", err)
	}
	r := NewRedisFromClient(ctx, mr.Client(), cacheCfg)
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
		_ = c.Close(context.WithoutCancel(ctx))
	}
	persistenceEnabled := false
	if cacheCfg != nil && cacheCfg.RedisConfig != nil {
		persistenceEnabled = cacheCfg.Standalone.Persistence.Enabled
	}
	log.Info("Embedded cache initialized",
		"mode", mode,
		"persistence_enabled", persistenceEnabled,
	)
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
	cleanup := func() { _ = c.Close(context.WithoutCancel(ctx)) }
	log.Info("Distributed cache initialized")
	return c, cleanup, nil
}

// Close gracefully shuts down the cache and any embedded runtime components.
func (c *Cache) Close(ctx context.Context) error {
	var errs []error
	if c.Notification != nil {
		if err := c.Notification.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close notification system: %w", err))
		}
	}
	if c.Redis != nil {
		if err := c.Redis.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close redis client: %w", err))
		}
	}
	if c.embedded != nil {
		if err := c.embedded.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("close embedded redis: %w", err))
		}
	}
	return errors.Join(errs...)
}

// HealthCheck performs a health check on all cache components
func (c *Cache) HealthCheck(ctx context.Context) error {
	if c.Redis != nil {
		return c.Redis.HealthCheck(ctx)
	}
	return nil
}
