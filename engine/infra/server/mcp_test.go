package server

import (
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldEmbedMCPProxy(t *testing.T) {
	t.Run("ShouldEmbedProxyEvenWhenProxyURLIsConfigured", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		mgr := config.NewManager(ctx, config.NewService())
		_, err := mgr.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
		require.NoError(t, err)
		ctx = config.ContextWithManager(ctx, mgr)
		t.Cleanup(func() { _ = mgr.Close(ctx) })
		c := config.FromContext(ctx)
		require.NotNil(t, c)
		c.MCPProxy.Mode = config.ModeMemory
		c.LLM.ProxyURL = "http://localhost:6001"
		assert.True(t, shouldEmbedMCPProxy(ctx))
	})
	t.Run("ShouldNotEmbedWhenModeIsExternal", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		mgr := config.NewManager(ctx, config.NewService())
		_, err := mgr.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
		require.NoError(t, err)
		ctx = config.ContextWithManager(ctx, mgr)
		t.Cleanup(func() { _ = mgr.Close(ctx) })
		c := config.FromContext(ctx)
		require.NotNil(t, c)
		c.MCPProxy.Mode = config.ModeDistributed
		assert.False(t, shouldEmbedMCPProxy(ctx))
	})
}

func TestServerAfterMCPReady(t *testing.T) {
	t.Run("ShouldOverrideProxyURLInEmbeddedMode", func(t *testing.T) {
		cfg := config.Default()
		cfg.MCPProxy.Mode = config.ModeMemory
		cfg.LLM.ProxyURL = "http://localhost:6001"
		srv := &Server{}
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
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
