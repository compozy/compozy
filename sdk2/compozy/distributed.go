package compozy

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/resources"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const defaultTemporalReachability = 5 * time.Second

func (e *Engine) bootstrapDistributed(ctx context.Context, cfg *appconfig.Config) (*modeRuntimeState, error) {
	state := &modeRuntimeState{}
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}
	if err := validateDistributedConfig(cfg); err != nil {
		return nil, err
	}
	cacheCfg := cache.FromAppConfig(cfg)
	redisClient, err := cache.NewRedis(ctx, cacheCfg)
	if err != nil {
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	state.addCleanup(func(cleanupCtx context.Context) error {
		return redisClient.Close()
	})
	state.resourceStore = resources.NewRedisResourceStore(redisClient)
	if err := ensureTemporalReachable(ctx, cfg); err != nil {
		state.cleanupOnError(context.WithoutCancel(ctx))
		return nil, err
	}
	log := logger.FromContext(ctx)
	if log != nil {
		log.Info("connected to distributed dependencies",
			"temporal_host", cfg.Temporal.HostPort,
			"redis", describeRedisEndpoint(cfg),
		)
	}
	return state, nil
}

func validateDistributedConfig(cfg *appconfig.Config) error {
	if strings.TrimSpace(cfg.Temporal.HostPort) == "" {
		return fmt.Errorf("temporal.host_port must be configured for distributed mode")
	}
	if hasRedisConnection(cfg) {
		return nil
	}
	return fmt.Errorf("redis connection details are required for distributed mode")
}

func ensureTemporalReachable(ctx context.Context, cfg *appconfig.Config) error {
	timeout := cfg.Server.Timeouts.TemporalReachability
	if timeout <= 0 {
		timeout = defaultTemporalReachability
	}
	dialer := &net.Dialer{Timeout: timeout}
	dialCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	conn, err := dialer.DialContext(dialCtx, "tcp", cfg.Temporal.HostPort)
	if err != nil {
		return fmt.Errorf("reach temporal at %s: %w", cfg.Temporal.HostPort, err)
	}
	_ = conn.Close()
	return nil
}

func hasRedisConnection(cfg *appconfig.Config) bool {
	if strings.TrimSpace(cfg.Redis.URL) != "" {
		return true
	}
	if strings.TrimSpace(cfg.Redis.Host) == "" {
		return false
	}
	return strings.TrimSpace(cfg.Redis.Port) != ""
}

func describeRedisEndpoint(cfg *appconfig.Config) string {
	if strings.TrimSpace(cfg.Redis.URL) != "" {
		return cfg.Redis.URL
	}
	if strings.TrimSpace(cfg.Redis.Host) == "" {
		return ""
	}
	return net.JoinHostPort(cfg.Redis.Host, cfg.Redis.Port)
}
