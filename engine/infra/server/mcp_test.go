package server

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	"github.com/stretchr/testify/assert"
)

func TestShouldEmbedMCPProxy(t *testing.T) {
	t.Run("ShouldEmbedStandaloneEvenWhenProxyURLIsConfigured", func(t *testing.T) {
		cfg := config.Default()
		cfg.MCPProxy.Mode = modeStandalone
		cfg.LLM.ProxyURL = "http://localhost:6001"
		assert.True(t, shouldEmbedMCPProxy(cfg))
	})
	t.Run("ShouldNotEmbedWhenModeIsExternal", func(t *testing.T) {
		cfg := config.Default()
		cfg.MCPProxy.Mode = ""
		assert.False(t, shouldEmbedMCPProxy(cfg))
	})
}

func TestServerAfterMCPReady(t *testing.T) {
	t.Run("ShouldOverrideProxyURLInStandaloneMode", func(t *testing.T) {
		cfg := config.Default()
		cfg.MCPProxy.Mode = modeStandalone
		cfg.LLM.ProxyURL = "http://localhost:6001"
		srv := &Server{}
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		srv.ctx = ctx
		baseURL := "http://127.0.0.1:6123"
		srv.afterMCPReady(ctx, cfg, baseURL, "memory")
		assert.True(t, srv.isMCPReady())
		assert.Equal(t, baseURL, cfg.LLM.ProxyURL)
	})
}

func TestClientManagerConfigFromApp(t *testing.T) {
	t.Run("ShouldReturnDefaultsWhenConfigNil", func(t *testing.T) {
		cmCfg := clientManagerConfigFromApp(nil)
		assert.Equal(t, mcpproxy.DefaultClientManagerConfig().DefaultConnectTimeout, cmCfg.DefaultConnectTimeout)
		assert.Equal(t, mcpproxy.DefaultClientManagerConfig().DefaultRequestTimeout, cmCfg.DefaultRequestTimeout)
	})
	t.Run("ShouldUseReadinessTimeoutWhenGreaterThanClientTimeout", func(t *testing.T) {
		cfg := config.Default()
		cfg.LLM.MCPClientTimeout = 30 * time.Second
		cfg.LLM.MCPReadinessTimeout = 70 * time.Second
		cmCfg := clientManagerConfigFromApp(cfg)
		assert.Equal(t, 70*time.Second, cmCfg.DefaultConnectTimeout)
		assert.Equal(t, 70*time.Second, cmCfg.DefaultRequestTimeout)
	})
	t.Run("ShouldHonorClientTimeoutWhenGreaterThanReadiness", func(t *testing.T) {
		cfg := config.Default()
		cfg.LLM.MCPClientTimeout = 80 * time.Second
		cfg.LLM.MCPReadinessTimeout = 20 * time.Second
		cmCfg := clientManagerConfigFromApp(cfg)
		assert.Equal(t, 80*time.Second, cmCfg.DefaultConnectTimeout)
		assert.Equal(t, 80*time.Second, cmCfg.DefaultRequestTimeout)
	})
	t.Run("ShouldNotReduceTimeoutBelowPackageDefault", func(t *testing.T) {
		cfg := config.Default()
		cfg.LLM.MCPClientTimeout = 2 * time.Second
		cfg.LLM.MCPReadinessTimeout = 3 * time.Second
		cmCfg := clientManagerConfigFromApp(cfg)
		assert.Equal(t, mcpproxy.DefaultClientManagerConfig().DefaultConnectTimeout, cmCfg.DefaultConnectTimeout)
		assert.Equal(t, mcpproxy.DefaultClientManagerConfig().DefaultRequestTimeout, cmCfg.DefaultRequestTimeout)
	})
}
