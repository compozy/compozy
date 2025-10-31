package cache

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// testContext builds a context carrying a test logger and config manager.
func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx := t.Context()
	ctx = logger.ContextWithLogger(ctx, logger.NewForTests())

	manager := config.NewManager(ctx, config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	t.Cleanup(func() { _ = manager.Close(ctx) })
	ctx = config.ContextWithManager(ctx, manager)
	return ctx
}

// adapterCase describes one backend under test implementing RedisInterface.
type adapterCase struct {
	name  string
	build func(ctx context.Context, t *testing.T) (cache.RedisInterface, func())
}

// contractBackends returns the two backends under test:
// - standalone: embedded miniredis via MiniredisStandalone
// - external: go-redis client talking to a standalone miniredis server
func contractBackends(t *testing.T) []adapterCase {
	t.Helper()
	return []adapterCase{
		{
			name: "standalone",
			build: func(ctx context.Context, t *testing.T) (cache.RedisInterface, func()) {
				mr, err := cache.NewMiniredisStandalone(ctx)
				if err != nil {
					t.Fatalf("standalone setup failed: %v", err)
				}
				// The embedded client already satisfies cache.RedisInterface (redis.Client implements it)
				client := mr.Client()
				cleanup := func() { _ = mr.Close(ctx) }
				return client, cleanup
			},
		},
		{
			name: "external",
			build: func(ctx context.Context, t *testing.T) (cache.RedisInterface, func()) {
				// Start a separate miniredis instance to emulate an external Redis endpoint.
				s := miniredis.RunT(t)
				client := redis.NewClient(&redis.Options{Addr: s.Addr()})
				if err := client.Ping(ctx).Err(); err != nil {
					t.Fatalf("external ping failed: %v", err)
				}
				cleanup := func() { _ = client.Close(); s.Close() }
				return client, cleanup
			},
		},
	}
}
