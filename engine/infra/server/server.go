package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/metric"
)

const (
	statusNotReady              = "not_ready"
	statusReady                 = "ready"
	monitoringInitTimeout       = 500 * time.Millisecond
	monitoringShutdownTimeout   = 5 * time.Second
	dbShutdownTimeout           = 30 * time.Second
	workerShutdownTimeout       = 30 * time.Second
	serverShutdownTimeout       = 5 * time.Second
	scheduleRetryMaxDuration    = 5 * time.Minute
	scheduleRetryBaseDelay      = 1 * time.Second
	scheduleRetryMaxDelay       = 30 * time.Second
	httpReadTimeout             = 15 * time.Second
	httpWriteTimeout            = 15 * time.Second
	httpIdleTimeout             = 60 * time.Second
	modeStandalone              = "standalone"
	mcpHealthPollInterval       = 200 * time.Millisecond
	mcpHealthRequestTimeout     = 500 * time.Millisecond
	temporalReachabilityTimeout = 1500 * time.Millisecond
	serverStartProbeDelay       = 100 * time.Millisecond
	hostAny                     = "0.0.0.0"
	hostLoopback                = "127.0.0.1"
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
	redisClient           *redis.Client
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
