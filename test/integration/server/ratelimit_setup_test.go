package server

import (
	"testing"

	"github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/pkg/config"
	"github.com/stretchr/testify/require"
)

// TestRedisClient_NilWhenNotInitialized ensures RedisClient() returns nil when cache is not initialized.
func TestRedisClient_NilWhenNotInitialized(t *testing.T) {
	t.Run("Should return nil when cache instance not initialized", func(t *testing.T) {
		mgr := config.NewManager(t.Context(), config.NewService())
		ctx := config.ContextWithManager(t.Context(), mgr)
		_, err := mgr.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
		require.NoError(t, err)
		srv, err := server.NewServer(ctx, ".", "", "")
		require.NoError(t, err)
		client := srv.RedisClient()
		require.Nil(t, client, "RedisClient should return nil when cache not initialized")
	})
}
