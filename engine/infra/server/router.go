package server

import (
	"fmt"
	"strings"
	"time"

	authuc "github.com/compozy/compozy/engine/auth/uc"
	rediscache "github.com/compozy/compozy/engine/infra/redis"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	corsmiddleware "github.com/compozy/compozy/engine/infra/server/middleware/cors"
	lgmiddleware "github.com/compozy/compozy/engine/infra/server/middleware/logger"
	"github.com/compozy/compozy/engine/infra/server/middleware/ratelimit"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/version"
	"github.com/gin-gonic/gin"
)

func convertRateLimitConfig(cfg *config.Config) *ratelimit.Config {
	return &ratelimit.Config{
		GlobalRate: ratelimit.RateConfig{
			Limit:  cfg.RateLimit.GlobalRate.Limit,
			Period: cfg.RateLimit.GlobalRate.Period,
		},
		APIKeyRate: ratelimit.RateConfig{
			Limit:  cfg.RateLimit.APIKeyRate.Limit,
			Period: cfg.RateLimit.APIKeyRate.Period,
		},
		Prefix:   cfg.RateLimit.Prefix,
		MaxRetry: cfg.RateLimit.MaxRetry,
		ExcludedPaths: []string{
			"/health",                // legacy/unversioned
			routes.HealthVersioned(), // versioned API health
			"/healthz",               // k8s liveness probe
			"/readyz",                // k8s readiness probe
			"/mcp-proxy/health",      // MCP readiness probe
			"/metrics",               // Prometheus
			"/swagger",               // Swagger UI (OpenAPI v3)
			"/swagger/index.html",    // Swagger UI entry
			"/docs",                  // Docs UI (OpenAPI v3) - legacy
			"/docs/index.html",       // Docs UI entry - legacy
			"/openapi.json",          // OpenAPI 3 spec
		},
	}
}

func (s *Server) buildAuthRepo(cfg *config.Config, base authuc.Repository) (authuc.Repository, string) {
	repo := base
	driver := driverNone
	const cacheDriverRedis = "redis"
	ttl := cfg.Cache.TTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	redisClient := s.RedisClient()
	if redisClient != nil {
		repo = rediscache.NewCachedRepository(repo, redisClient, ttl)
		driver = cacheDriverRedis
		return repo, driver
	}
	return repo, driver
}

func (s *Server) buildRouter(state *appstate.State) error {
	r := gin.New()
	r.Use(gin.Recovery())
	cfg := config.FromContext(s.ctx)
	if cfg.RateLimit.GlobalRate.Limit > 0 {
		log := logger.FromContext(s.ctx)
		rateLimitConfig := convertRateLimitConfig(cfg)
		redisClient := s.RedisClient()
		var manager *ratelimit.Manager
		var err error
		if s.monitoring != nil && s.monitoring.IsInitialized() {
			manager, err = ratelimit.NewManagerWithMetrics(s.ctx, rateLimitConfig, redisClient, s.monitoring.Meter())
		} else {
			manager, err = ratelimit.NewManager(rateLimitConfig, redisClient)
		}
		if err != nil {
			log.Error("Failed to initialize rate limiting", "error", err)
		} else {
			r.Use(manager.Middleware())
			driver := "memory"
			if redisClient != nil {
				driver = "redis"
			}
			log.Info("rate limiter initialized",
				"driver", driver,
				"global_limit", cfg.RateLimit.GlobalRate.Limit,
				"global_period", cfg.RateLimit.GlobalRate.Period)
		}
	}
	if s.monitoring != nil && s.monitoring.IsInitialized() {
		r.Use(s.monitoring.GinMiddleware(s.ctx))
	}
	r.Use(lgmiddleware.Middleware(s.ctx))
	if cfg.Server.CORSEnabled {
		r.Use(corsmiddleware.Middleware(cfg.Server.CORS))
	}
	r.Use(appstate.StateMiddleware(state))
	r.Use(router.ErrorHandler())
	if s.monitoring != nil && s.monitoring.IsInitialized() {
		monitoringPath := state.ProjectConfig.MonitoringConfig.Path
		r.GET(monitoringPath, gin.WrapH(s.monitoring.ExporterHandler()))
	}
	if err := RegisterRoutes(s.ctx, r, state, s); err != nil {
		return err
	}
	s.router = r
	return nil
}

func (s *Server) logStartupBanner() {
	log := logger.FromContext(s.ctx)
	cfg := config.FromContext(s.ctx)
	host := s.serverConfig.Host
	port := s.serverConfig.Port
	if cfg != nil {
		host = cfg.Server.Host
		port = cfg.Server.Port
	}
	fh := friendlyHost(host)
	httpURL := fmt.Sprintf("http://%s:%d", fh, port)
	apiURL := fmt.Sprintf("%s%s", httpURL, routes.Base())
	docsURL := fmt.Sprintf("%s/swagger/index.html", httpURL)
	openapiJSON := fmt.Sprintf("%s/openapi.json", httpURL)
	hooksURL := fmt.Sprintf("%s%s", httpURL, routes.Hooks())
	mcp := s.mcpBaseURLValue()
	temporalHP := ""
	if cfg := config.FromContext(s.ctx); cfg != nil {
		temporalHP = cfg.Temporal.HostPort
	}
	ver := version.Get().Version
	lines := []string{
		fmt.Sprintf("Compozy %s", ver),
		fmt.Sprintf("  API           > %s", apiURL),
		fmt.Sprintf("  Health        > %s%s/health", httpURL, routes.Base()),
		fmt.Sprintf("  Readyz        > %s/readyz", httpURL),
		fmt.Sprintf("  Docs          > %s", docsURL),
		fmt.Sprintf("  OpenAPI JSON  > %s", openapiJSON),
		fmt.Sprintf("  Webhooks      > %s", hooksURL),
	}
	if mcp != "" {
		lines = append(lines,
			fmt.Sprintf("  MCP Proxy     > %s", mcp),
			fmt.Sprintf("  MCP Admin     > %s/admin/mcps", mcp),
		)
	}
	if temporalHP != "" {
		lines = append(lines, fmt.Sprintf("  Temporal gRPC > %s", temporalHP))
	}
	banner := "\n" + strings.Join(lines, "\n")
	log.Info(banner)
}

func friendlyHost(h string) string {
	if h == hostAny || h == "::" || h == "" {
		return hostLoopback
	}
	return h
}
