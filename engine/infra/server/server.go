package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/metric"
)

const (
	statusNotReady = "not_ready"
	statusReady    = "ready"
	hostAny        = "0.0.0.0"
	hostLoopback   = "127.0.0.1"
	driverPostgres = "postgres"
	driverSQLite   = "sqlite"
	driverNone     = "none"
)

type MCPProxy interface {
	Start(context.Context) error
	Stop(context.Context) error
}

type Server struct {
	serverConfig          *config.ServerConfig
	cwd                   string
	configFile            string
	envFilePath           string
	router                *gin.Engine
	monitoring            *monitoring.Service
	cacheInstance         *cache.Cache
	ctx                   context.Context
	cancel                context.CancelFunc
	httpServer            *http.Server
	shutdownChan          chan struct{}
	reconciliationState   *reconciliationStatus
	readinessMu           sync.RWMutex
	temporalReady         bool
	workerReady           bool
	mcpReady              bool
	mcpBaseURL            string
	mcpProxy              MCPProxy
	readyGauge            metric.Int64ObservableGauge
	readyTransitionsTotal metric.Int64Counter
	readyCallback         metric.Registration
	lastReady             bool
	shutdownOnce          sync.Once
	storeDriverLabel      string
	cacheDriverLabel      string
	authRepoDriverLabel   string
	authCacheDriverLabel  string
	cleanupMu             sync.Mutex
	extraCleanups         []func()
}

func NewServer(ctx context.Context, cwd, configFile, envFilePath string) (*Server, error) {
	serverCtx, cancel := context.WithCancel(ctx)
	cfg := config.FromContext(serverCtx)
	if cfg == nil {
		cancel()
		return nil, fmt.Errorf("configuration missing from context; attach a manager with config.ContextWithManager")
	}
	log := logger.FromContext(serverCtx)
	mode := strings.TrimSpace(cfg.Mode)
	if mode == "" {
		mode = config.ModeMemory
	}
	log.Info("Resolved server runtime configuration",
		"mode", mode,
		"temporal_mode", cfg.EffectiveTemporalMode(),
		"redis_mode", cfg.EffectiveRedisMode(),
		"mcp_proxy_mode", cfg.EffectiveMCPProxyMode(),
		"database_driver", cfg.EffectiveDatabaseDriver(),
	)
	return &Server{
		serverConfig:        &cfg.Server,
		cwd:                 cwd,
		configFile:          configFile,
		envFilePath:         envFilePath,
		ctx:                 serverCtx,
		cancel:              cancel,
		shutdownChan:        make(chan struct{}, 1),
		reconciliationState: &reconciliationStatus{},
		lastReady:           false,
	}, nil
}

// RedisClient returns the Redis client from the cache instance.
// Returns nil if cache is not initialized (safe for rate limiting fallback).
func (s *Server) RedisClient() *redis.Client {
	if s.cacheInstance == nil || s.cacheInstance.Redis == nil {
		return nil
	}
	client := s.cacheInstance.Redis.Client()
	if c, ok := client.(*redis.Client); ok {
		return c
	}
	return nil
}
