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
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return nil, fmt.Errorf("configuration missing from context; attach a manager with config.ContextWithManager")
	}
	if !shouldEmbedMCPProxy(cfg) {
		return func() {}, nil
	}
	server, driver, baseURL, err := s.launchMCPServer(ctx, cfg)
	if err != nil {
		return nil, err
	}
	cmCfg := clientManagerConfigFromApp(cfg)
	total := mcpProbeTimeout(cfg)
	client := &http.Client{Timeout: cmCfg.DefaultRequestTimeout}
	poll, reqTimeout := readinessTimings(cfg, 500*time.Millisecond, cmCfg.DefaultRequestTimeout)
	if !s.awaitMCPProxyReady(ctx, client, baseURL, total, reqTimeout, poll) {
		if stopErr := s.shutdownMCPServer(ctx, cfg, server); stopErr != nil {
			logger.FromContext(ctx).Warn("Failed to stop embedded MCP proxy after readiness failure", "error", stopErr)
		}
		return nil, fmt.Errorf("embedded MCP proxy failed readiness within timeout: %s", baseURL)
	}
	s.afterMCPReady(ctx, cfg, baseURL, driver)
	s.mcpProxy = server
	return func() {
		if err := s.shutdownMCPServer(ctx, cfg, server); err != nil {
			logger.FromContext(ctx).Warn("Failed to stop embedded MCP proxy", "error", err)
		}
		s.setMCPReady(false)
		s.onReadinessMaybeChanged("mcp_stopped")
	}, nil
}

func (s *Server) awaitMCPProxyReady(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	total time.Duration,
	requestTimeout time.Duration,
	pollInterval time.Duration,
) bool {
	deadline := time.Now().Add(total)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		if probeMCPProxy(ctx, client, baseURL, requestTimeout) {
			return true
		}
		time.Sleep(pollInterval)
	}
	return false
}

// readinessTimings determines poll and request timeouts for readiness probing.
func readinessTimings(
	cfg *config.Config,
	fallbackPoll time.Duration,
	requestTimeout time.Duration,
) (time.Duration, time.Duration) {
	poll := fallbackPoll
	if cfg != nil && cfg.LLM.MCPReadinessPollInterval > 0 {
		poll = cfg.LLM.MCPReadinessPollInterval
	}
	if poll <= 0 {
		poll = 200 * time.Millisecond
	}
	timeout := requestTimeout
	if cfg != nil && cfg.LLM.MCPClientTimeout > timeout {
		timeout = cfg.LLM.MCPClientTimeout
	}
	if timeout <= 0 {
		timeout = mcpproxy.DefaultRequestTimeout
	}
	return poll, timeout
}

// probeMCPProxy checks MCP readiness endpoint once.
func probeMCPProxy(ctx context.Context, client *http.Client, baseURL string, timeout time.Duration) bool {
	rctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodGet, baseURL+"/healthz", http.NoBody)
	if err != nil {
		logger.FromContext(ctx).Error("failed to create MCP readiness request", "error", err, "url", baseURL+"/healthz")
		return false
	}
	resp, err := client.Do(req)
	if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close()
		return true
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
	return false
}

// launchMCPServer starts the embedded MCP proxy and waits for binding.
func (s *Server) launchMCPServer(ctx context.Context, cfg *config.Config) (*mcpproxy.Server, string, string, error) {
	host, portStr := normalizeMCPHostAndPort(cfg)
	initialBase := initialMCPBaseURL(host, portStr, cfg.MCPProxy.BaseURL)
	server, driver, err := s.newMCPProxyServer(ctx, cfg, host, portStr, initialBase, cfg.MCPProxy.ShutdownTimeout)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to initialize MCP proxy: %w", err)
	}
	go s.runMCPServer(ctx, server)
	if err := s.waitForMCPBind(ctx, cfg, server); err != nil {
		if stopErr := s.shutdownMCPServer(ctx, cfg, server); stopErr != nil {
			logger.FromContext(ctx).Warn("Failed to stop embedded MCP proxy after bind timeout", "error", stopErr)
		}
		return nil, "", "", err
	}
	baseURL := server.BaseURL()
	s.mcpBaseURL = baseURL
	return server, driver, baseURL, nil
}

// runMCPServer starts the MCP server and logs terminal errors.
func (s *Server) runMCPServer(ctx context.Context, server *mcpproxy.Server) {
	if err := server.Start(ctx); err != nil {
		logger.FromContext(ctx).Error("Embedded MCP proxy exited with error", "error", err)
	}
}

// waitForMCPBind waits for the MCP server to bind within the configured timeout.
func (s *Server) waitForMCPBind(ctx context.Context, cfg *config.Config, server *mcpproxy.Server) error {
	bctx, cancel := context.WithTimeout(ctx, mcpProbeTimeout(cfg))
	defer cancel()
	select {
	case <-server.Bound():
		return nil
	case <-bctx.Done():
		return fmt.Errorf("embedded MCP proxy did not bind within timeout")
	}
}

// shutdownMCPServer stops the MCP server gracefully.
func (s *Server) shutdownMCPServer(ctx context.Context, cfg *config.Config, server *mcpproxy.Server) error {
	if server == nil {
		return nil
	}
	ctx2, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.MCPProxy.ShutdownTimeout)
	defer cancel()
	return server.Stop(ctx2)
}

func (s *Server) afterMCPReady(ctx context.Context, cfg *config.Config, baseURL, driver string) {
	s.setMCPReady(true)
	s.onReadinessMaybeChanged("mcp_ready")
	log := logger.FromContext(ctx)
	if cfg.MCPProxy.Mode == modeStandalone {
		if cfg.LLM.ProxyURL != baseURL {
			cfg.LLM.ProxyURL = baseURL
			log.Info("Set LLM proxy URL from embedded MCP proxy", "proxy_url", baseURL)
		}
	} else if cfg.LLM.ProxyURL == "" {
		cfg.LLM.ProxyURL = baseURL
		log.Info("Set LLM proxy URL from embedded MCP proxy", "proxy_url", baseURL)
	}
	log.Info(
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
	cmCfg := clientManagerConfigFromApp(cfg)
	logger.FromContext(ctx).Debug(
		"Configured MCP client manager timeouts",
		"connect_timeout", cmCfg.DefaultConnectTimeout,
		"request_timeout", cmCfg.DefaultRequestTimeout,
	)
	cm := mcpproxy.NewMCPClientManager(ctx, storage, cmCfg)
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
	return true
}

func clientManagerConfigFromApp(cfg *config.Config) *mcpproxy.ClientManagerConfig {
	cmCfg := mcpproxy.DefaultClientManagerConfig()
	if cfg == nil {
		return cmCfg
	}
	timeout := max(cfg.LLM.MCPReadinessTimeout, cfg.LLM.MCPClientTimeout)
	if timeout <= 0 {
		return cmCfg
	}
	if timeout < mcpproxy.DefaultConnectTimeout {
		timeout = mcpproxy.DefaultConnectTimeout
	}
	cmCfg.DefaultConnectTimeout = timeout
	if cmCfg.DefaultRequestTimeout <= 0 || timeout > cmCfg.DefaultRequestTimeout {
		cmCfg.DefaultRequestTimeout = timeout
	}
	return cmCfg
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
