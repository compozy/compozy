//go:build integration
// +build integration

package compozy

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/worker/embedded"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDistributedIntegrationLifecycle(t *testing.T) {
	t.Run("Should run distributed integration lifecycle", func(t *testing.T) {
		ctx := lifecycleTestContext(t)
		cfg := appconfig.FromContext(ctx)
		require.NotNil(t, cfg)
		cfg.Mode = string(ModeDistributed)
		mr := miniredis.NewMiniRedis()
		require.NoError(t, mr.Start())
		t.Cleanup(mr.Close)
		cfg.Redis.URL = "redis://" + mr.Addr()
		cfg.Redis.Mode = string(ModeDistributed)
		temporalPort := allocateTemporalFrontendPort(ctx, t)
		temporalCfg := &embedded.Config{
			DatabaseFile: filepath.Join(t.TempDir(), "temporal.db"),
			FrontendPort: temporalPort,
			BindIP:       "127.0.0.1",
			Namespace:    cfg.Temporal.Namespace,
			ClusterName:  "integration-distributed",
			EnableUI:     false,
			RequireUI:    false,
			// Align UI port offset with standalone defaults.
			UIPort:       temporalPort + 1000,
			LogLevel:     cfg.Temporal.Standalone.LogLevel,
			StartTimeout: cfg.Temporal.Standalone.StartTimeout,
		}
		if strings.TrimSpace(temporalCfg.LogLevel) == "" {
			temporalCfg.LogLevel = "warn"
		}
		if temporalCfg.StartTimeout <= 0 {
			temporalCfg.StartTimeout = 30 * time.Second
		}
		temporalServer, err := embedded.NewServer(ctx, temporalCfg)
		require.NoError(t, err)
		require.NoError(t, temporalServer.Start(ctx))
		t.Cleanup(func() {
			shutdownTimeout := cfg.Server.Timeouts.WorkerShutdown
			if shutdownTimeout <= 0 {
				shutdownTimeout = temporalCfg.StartTimeout
			}
			shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
			defer cancel()
			require.NoError(t, temporalServer.Stop(shutdownCtx))
		})
		cfg.Temporal.HostPort = temporalServer.FrontendAddress()
		engine, err := New(
			ctx,
			WithMode(ModeDistributed),
			WithWorkflow(&engineworkflow.Config{ID: "integration-distributed"}),
		)
		require.NoError(t, err)
		require.NoError(t, engine.Start(ctx))
		t.Cleanup(func() {
			require.NoError(t, engine.Stop(ctx))
			engine.Wait()
		})
		assert.True(t, engine.IsStarted())
		server := engine.Server()
		require.NotNil(t, server)
		resp, err := http.Get(fmt.Sprintf("http://%s", server.Addr))
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		store := engine.ResourceStore()
		require.NotNil(t, store)
		_, ok := store.(*resources.RedisResourceStore)
		assert.True(t, ok)
	})
}
