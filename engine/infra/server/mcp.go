package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
)

func (s *Server) setupMCPProxy(ctx context.Context) (func(), error) {
	log := logger.FromContext(ctx)
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return nil, fmt.Errorf("configuration missing from context; attach a manager with config.ContextWithManager")
	}
	if cfg.MCPProxy.Mode != modeStandalone {
		return func() {}, nil
	}
	host, portStr := normalizeMCPHostAndPort(cfg)
	initialBase := initialMCPBaseURL(host, portStr, cfg.MCPProxy.BaseURL)
	server, driver, err := s.newMCPProxyServer(ctx, cfg, host, portStr, initialBase, cfg.MCPProxy.ShutdownTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MCP proxy: %w", err)
	}
	go func() {
		if err := server.Start(ctx); err != nil {
			logger.FromContext(ctx).Error("Embedded MCP proxy exited with error", "error", err)
		}
	}()
	total := mcpProbeTimeout(cfg)
	client := &http.Client{Timeout: mcpHealthRequestTimeout}
	bctx, bcancel := context.WithTimeout(ctx, total)
	select {
	case <-server.Bound():
	case <-bctx.Done():
		bcancel()
		ctx2, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.MCPProxy.ShutdownTimeout)
		if stopErr := server.Stop(ctx2); stopErr != nil {
			logger.FromContext(ctx).Warn("Failed to stop embedded MCP proxy after bind timeout", "error", stopErr)
		}
		cancel()
		return nil, fmt.Errorf("embedded MCP proxy did not bind within timeout")
	}
	bcancel()
	baseURL := server.BaseURL()
	s.mcpBaseURL = baseURL
	ready := s.awaitMCPProxyReady(ctx, client, baseURL, total)
	if !ready {
		ctx2, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.MCPProxy.ShutdownTimeout)
		if stopErr := server.Stop(ctx2); stopErr != nil {
			logger.FromContext(ctx).Warn("Failed to stop embedded MCP proxy after readiness failure", "error", stopErr)
		}
		cancel()
		return nil, fmt.Errorf("embedded MCP proxy failed readiness within timeout: %s", baseURL)
	}
	s.afterMCPReady(ctx, cfg, baseURL, driver)
	cleanup := func() {
		ctx2, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.MCPProxy.ShutdownTimeout)
		defer cancel()
		if err := server.Stop(ctx2); err != nil {
			log.Warn("Failed to stop embedded MCP proxy", "error", err)
		}
		s.setMCPReady(false)
		s.onReadinessMaybeChanged("mcp_stopped")
	}
	s.mcpProxy = server
	return cleanup, nil
}

func (s *Server) awaitMCPProxyReady(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	total time.Duration,
) bool {
	deadline := time.Now().Add(total)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		rctx, cancel := context.WithTimeout(ctx, mcpHealthRequestTimeout)
		req, reqErr := http.NewRequestWithContext(rctx, http.MethodGet, baseURL+"/healthz", http.NoBody)
		if reqErr != nil {
			cancel()
			time.Sleep(mcpHealthPollInterval)
			continue
		}
		resp, err := client.Do(req)
		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			cancel()
			return true
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		cancel()
		time.Sleep(mcpHealthPollInterval)
	}
	return false
}

func (s *Server) afterMCPReady(ctx context.Context, cfg *config.Config, baseURL, driver string) {
	s.setMCPReady(true)
	s.onReadinessMaybeChanged("mcp_ready")
	if cfg.LLM.ProxyURL == "" {
		cfg.LLM.ProxyURL = baseURL
		logger.FromContext(ctx).Info("Set LLM proxy URL from embedded MCP proxy", "proxy_url", baseURL)
	}
	logger.FromContext(ctx).Info(
		"Embedded MCP proxy started",
		"mode", cfg.MCPProxy.Mode,
		"mcp_storage_driver", driver,
		"base_url", baseURL,
	)
}

func normalizeMCPHostAndPort(cfg *config.Config) (string, string) {
	host := cfg.MCPProxy.Host
	if host == "" {
		host = "127.0.0.1"
	}
	if cfg.MCPProxy.Port <= 0 {
		return host, "0"
	}
	return host, fmt.Sprintf("%d", cfg.MCPProxy.Port)
}

func initialMCPBaseURL(host, portStr, cfgBase string) string {
	if cfgBase != "" {
		return cfgBase
	}
	bhost := host
	if host == "0.0.0.0" || host == "::" {
		bhost = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%s", bhost, portStr)
}

func mcpProbeTimeout(cfg *config.Config) time.Duration {
	if cfg.Worker.MCPProxyHealthCheckTimeout <= 0 {
		return 10 * time.Second
	}
	return cfg.Worker.MCPProxyHealthCheckTimeout
}

func (s *Server) newMCPProxyServer(
	ctx context.Context,
	cfg *config.Config,
	host string,
	port string,
	baseURL string,
	shutdown time.Duration,
) (*mcpproxy.Server, string, error) {
	storageCfg := storageConfigForMCP(cfg)
	storage, err := mcpproxy.NewStorage(ctx, storageCfg)
	if err != nil {
		return nil, "", fmt.Errorf("failed to initialize MCP storage: %w", err)
	}
	cm := mcpproxy.NewMCPClientManager(storage, nil)
	mcfg := &mcpproxy.Config{
		Host:               host,
		Port:               port,
		BaseURL:            baseURL,
		ShutdownTimeout:    shutdown,
		UseOSSignalHandler: false,
	}
	server := mcpproxy.NewServer(mcfg, storage, cm)
	return server, string(storageCfg.Type), nil
}

func shouldEmbedMCPProxy(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if cfg.MCPProxy.Mode != modeStandalone {
		return false
	}
	if cfg.LLM.ProxyURL != "" {
		return false
	}
	return true
}

func storageConfigForMCP(cfg *config.Config) *mcpproxy.StorageConfig {
	if cfg == nil {
		return mcpproxy.DefaultStorageConfig()
	}
	if !isRedisConfigured(cfg) {
		return &mcpproxy.StorageConfig{Type: mcpproxy.StorageTypeMemory}
	}
	app := cfg.Redis
	redisCfg := &mcpproxy.RedisConfig{
		URL:             app.URL,
		Addr:            redisAddr(app.Host, app.Port),
		Password:        app.Password,
		DB:              app.DB,
		PoolSize:        app.PoolSize,
		MinIdleConns:    app.MinIdleConns,
		MaxRetries:      app.MaxRetries,
		DialTimeout:     app.DialTimeout,
		ReadTimeout:     app.ReadTimeout,
		WriteTimeout:    app.WriteTimeout,
		PoolTimeout:     app.PoolTimeout,
		MinRetryBackoff: app.MinRetryBackoff,
		MaxRetryBackoff: app.MaxRetryBackoff,
		TLSEnabled:      app.TLSEnabled,
		TLSConfig:       app.TLSConfig,
	}
	return &mcpproxy.StorageConfig{Type: mcpproxy.StorageTypeRedis, Redis: redisCfg}
}

func redisAddr(host, port string) string {
	if host == "" || port == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s", host, port)
}
