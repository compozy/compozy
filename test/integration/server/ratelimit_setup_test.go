package server

import (
	"context"
	"os"
	"testing"

	"github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/pkg/config"
	"github.com/stretchr/testify/require"
)

// TestSetupRedisClient_SkipsWhenRedisMissing ensures server continues when Redis is not configured.
func TestSetupRedisClient_SkipsWhenRedisMissing(t *testing.T) {
	t.Run("Should skip Redis client when configuration missing", func(t *testing.T) {
		t.Setenv("REDIS_URL", "")
		t.Setenv("REDIS_HOST", "")
		t.Setenv("REDIS_PORT", "")
		_ = os.Unsetenv("REDIS_PASSWORD")
		mgr := config.NewManager(config.NewService())
		ctx := config.ContextWithManager(context.Background(), mgr)
		_, err := mgr.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
		require.NoError(t, err)
		srv, err := server.NewServer(ctx, ".", "", "")
		require.NoError(t, err)
		cfg := config.FromContext(ctx)
		client, cleanup, err := srv.SetupRedisClient(cfg)
		require.NoError(t, err)
		require.Nil(t, client)
		require.Nil(t, cleanup)
	})
}
