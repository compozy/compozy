package helpers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// ResourceStoreTestEnv encapsulates a Redis-backed resource store environment
// running in standalone (embedded miniredis) mode for integration tests.
type ResourceStoreTestEnv struct {
	Cache   *cache.Cache
	Store   resources.ResourceStore
	Cleanup func()
}

// SetupStandaloneResourceStore creates a RedisResourceStore backed by the
// standalone (embedded) Redis using the mode-aware cache factory. It assumes
// the provided context comes from t.Context().
func SetupStandaloneResourceStore(ctx context.Context, t *testing.T) *ResourceStoreTestEnv {
	t.Helper()
	// Ensure logger and config are present in context for all code paths.
	if logger.FromContext(ctx) == nil {
		ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
	}
	cfg := config.FromContext(ctx)
	if cfg == nil {
		// Fall back to a test manager if not already present.
		mgr := config.NewManager(ctx, config.NewService())
		_, err := mgr.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
		require.NoError(t, err)
		ctx = config.ContextWithManager(ctx, mgr)
		t.Cleanup(func() { _ = mgr.Close(ctx) })
		cfg = config.FromContext(ctx)
	}

	// Force standalone mode for Redis so SetupCache spins up MiniredisStandalone.
	cfg.Mode = "standalone"
	cfg.Redis.Mode = "standalone"

	c, cleanup, err := cache.SetupCache(ctx)
	require.NoError(t, err)

	// Small reconcile interval to make watch-driven tests snappy.
	store := resources.NewRedisResourceStore(c.Redis, resources.WithReconcileInterval(100*time.Millisecond))

	t.Cleanup(func() {
		_ = store.Close()
		cleanup()
	})

	return &ResourceStoreTestEnv{
		Cache: c,
		Store: store,
		Cleanup: func() {
			_ = store.Close()
			cleanup()
		},
	}
}
