//go:build integration
// +build integration

package compozy

import (
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/compozy/compozy/engine/resources"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDistributedIntegrationLifecycle(t *testing.T) {
	ctx := lifecycleTestContext(t)
	cfg := appconfig.FromContext(ctx)
	require.NotNil(t, cfg)
	cfg.Mode = string(ModeDistributed)
	mr := miniredis.NewMiniRedis()
	require.NoError(t, mr.Start())
	defer mr.Close()
	cfg.Redis.URL = "redis://" + mr.Addr()
	cfg.Redis.Mode = string(ModeDistributed)
	temporalListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer temporalListener.Close()
	cfg.Temporal.HostPort = temporalListener.Addr().String()
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
}
