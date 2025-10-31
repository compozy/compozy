package compozy

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/worker/embedded"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func (e *Engine) bootstrapStandalone(ctx context.Context, cfg *appconfig.Config) (*modeRuntimeState, error) {
	state := &modeRuntimeState{}
	log := logger.FromContext(ctx)
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}
	mergeStandaloneRedisConfig(cfg, e.standaloneRedis)
	embeddedCfg := mergeStandaloneTemporalConfig(cfg, e.standaloneTemporal)
	server, err := embedded.NewServer(ctx, embeddedCfg)
	if err != nil {
		return nil, fmt.Errorf("prepare embedded temporal server: %w", err)
	}
	if err := server.Start(ctx); err != nil {
		state.addCleanup(func(cleanupCtx context.Context) error {
			return stopEmbeddedTemporal(cleanupCtx, server, embeddedCfg.StartTimeout, cfg)
		})
		state.cleanupOnError(context.WithoutCancel(ctx))
		return nil, fmt.Errorf("start embedded temporal server: %w", err)
	}
	cfg.Temporal.HostPort = server.FrontendAddress()
	state.addCleanup(func(cleanupCtx context.Context) error {
		return stopEmbeddedTemporal(cleanupCtx, server, embeddedCfg.StartTimeout, cfg)
	})
	if log != nil {
		log.Info("embedded temporal server started",
			"frontend_addr", server.FrontendAddress(),
			"namespace", embeddedCfg.Namespace,
			"cluster", embeddedCfg.ClusterName,
		)
	}
	mini, err := cache.NewMiniredisStandalone(ctx)
	if err != nil {
		state.cleanupOnError(context.WithoutCancel(ctx))
		return nil, fmt.Errorf("start embedded redis: %w", err)
	}
	state.addCleanup(func(cleanupCtx context.Context) error {
		return mini.Close(context.WithoutCancel(cleanupCtx))
	})
	redisClient := mini.Client()
	addr := redisClient.Options().Addr
	updateRedisConfig(cfg, addr)
	useRedisStore := e.shouldUseStandaloneRedis(cfg)
	if log != nil {
		storeKind := "memory"
		if useRedisStore {
			storeKind = "redis"
		}
		log.Info("embedded redis ready", "addr", addr, "resource_store", storeKind)
	}
	state.resourceStore = selectStandaloneStore(redisClient, useRedisStore)
	return state, nil
}

func mergeStandaloneTemporalConfig(cfg *appconfig.Config, override *StandaloneTemporalConfig) *embedded.Config {
	base := cfg.Temporal.Standalone
	if override != nil {
		if override.DatabaseFile != "" {
			base.DatabaseFile = override.DatabaseFile
		}
		if override.FrontendPort != 0 {
			base.FrontendPort = override.FrontendPort
		}
		if override.BindIP != "" {
			base.BindIP = override.BindIP
		}
		if override.Namespace != "" {
			base.Namespace = override.Namespace
		}
		if override.ClusterName != "" {
			base.ClusterName = override.ClusterName
		}
		if override.UIPort != 0 {
			base.UIPort = override.UIPort
		}
		if override.LogLevel != "" {
			base.LogLevel = override.LogLevel
		}
		if override.StartTimeout > 0 {
			base.StartTimeout = override.StartTimeout
		}
		base.EnableUI = override.EnableUI
	}
	cfg.Temporal.Standalone = base
	return &embedded.Config{
		DatabaseFile: base.DatabaseFile,
		FrontendPort: base.FrontendPort,
		BindIP:       base.BindIP,
		Namespace:    base.Namespace,
		ClusterName:  base.ClusterName,
		EnableUI:     base.EnableUI,
		RequireUI:    base.RequireUI,
		UIPort:       base.UIPort,
		LogLevel:     base.LogLevel,
		StartTimeout: base.StartTimeout,
	}
}

func mergeStandaloneRedisConfig(cfg *appconfig.Config, override *StandaloneRedisConfig) {
	if cfg == nil || override == nil {
		return
	}
	if override.Persistence {
		cfg.Redis.Standalone.Persistence.Enabled = true
	}
	if override.PersistenceDir != "" {
		cfg.Redis.Standalone.Persistence.DataDir = override.PersistenceDir
	}
	if override.SnapshotInterval > 0 {
		cfg.Redis.Standalone.Persistence.SnapshotInterval = override.SnapshotInterval
	}
}

func stopEmbeddedTemporal(
	ctx context.Context,
	server *embedded.Server,
	startup time.Duration,
	cfg *appconfig.Config,
) error {
	if server == nil {
		return nil
	}
	shutdown := time.Duration(0)
	if cfg != nil {
		shutdown = cfg.Server.Timeouts.WorkerShutdown
	}
	if shutdown <= 0 {
		shutdown = startup
	}
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdown)
	defer cancel()
	return server.Stop(stopCtx)
}

func selectStandaloneStore(client cache.RedisInterface, useRedis bool) resources.ResourceStore {
	if useRedis && client != nil {
		return resources.NewRedisResourceStore(client)
	}
	return resources.NewMemoryResourceStore()
}

func (e *Engine) shouldUseStandaloneRedis(cfg *appconfig.Config) bool {
	if e != nil && e.standaloneRedis != nil {
		return true
	}
	if cfg == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(cfg.Redis.Mode), string(ModeStandalone))
}

func updateRedisConfig(cfg *appconfig.Config, addr string) {
	if cfg == nil || addr == "" {
		return
	}
	cfg.Redis.URL = fmt.Sprintf("redis://%s", addr)
	host, port := parseRedisHostPort(addr)
	if host != "" {
		cfg.Redis.Host = host
	}
	if port != "" {
		cfg.Redis.Port = port
	}
}

func parseRedisHostPort(addr string) (string, string) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, ""
	}
	return host, port
}
