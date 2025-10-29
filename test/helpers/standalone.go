package helpers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	redis "github.com/redis/go-redis/v9"
)

const testModeStandalone = "standalone"

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
	cfg.Mode = testModeStandalone
	cfg.Redis.Mode = testModeStandalone

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

// StreamingTestEnv encapsulates Redis Pub/Sub testing utilities backed by the
// standalone (embedded) miniredis instance created via the mode-aware factory.
// It exposes convenience helpers that use native go-redis Pub/Sub types.
type StreamingTestEnv struct {
	Cache   *cache.Cache
	Client  redis.UniversalClient
	subs    []*redis.PubSub
	Cleanup func()
}

// SetupStandaloneStreaming creates a StreamingTestEnv using embedded miniredis.
// It enforces that logger and configuration are present in the context and
// forces the Redis mode to "standalone" to exercise the Miniredis backend.
func SetupStandaloneStreaming(ctx context.Context, t *testing.T) *StreamingTestEnv {
	t.Helper()
	if logger.FromContext(ctx) == nil {
		ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
	}
	cfg := config.FromContext(ctx)
	if cfg == nil {
		mgr := config.NewManager(ctx, config.NewService())
		_, err := mgr.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
		require.NoError(t, err)
		ctx = config.ContextWithManager(ctx, mgr)
		t.Cleanup(func() { _ = mgr.Close(ctx) })
		cfg = config.FromContext(ctx)
	}
	// Force standalone mode explicitly
	cfg.Mode = "standalone"
	cfg.Redis.Mode = "standalone"

	c, cleanup, err := cache.SetupCache(ctx)
	require.NoError(t, err)

	env := &StreamingTestEnv{
		Cache:  c,
		Client: c.Redis.Client(),
	}
	env.Cleanup = func() {
		// Close all active subscriptions first to avoid goroutine leaks
		for _, s := range env.subs {
			_ = s.Close()
		}
		cleanup()
	}
	t.Cleanup(env.Cleanup)
	return env
}

// Publish sends a string payload to a channel using native go-redis Pub/Sub.
func (e *StreamingTestEnv) Publish(ctx context.Context, channel, payload string) error {
	if e == nil || e.Client == nil {
		return errors.New("streaming env not initialized")
	}
	return e.Client.Publish(ctx, channel, payload).Err()
}

// Subscribe subscribes to a single channel and forwards payloads to out.
// It uses native go-redis PubSub and confirms the subscription with Receive.
func (e *StreamingTestEnv) Subscribe(ctx context.Context, channel string, out chan<- string) error {
	if channel == "" {
		return errors.New("channel cannot be empty")
	}
	ps := e.Client.Subscribe(ctx, channel)
	if _, err := ps.Receive(ctx); err != nil {
		_ = ps.Close()
		return err
	}
	e.subs = append(e.subs, ps)
	go func(ch <-chan *redis.Message) {
		for msg := range ch {
			if msg == nil {
				continue
			}
			select {
			case out <- msg.Payload:
			case <-ctx.Done():
				return
			}
		}
	}(ps.Channel())
	return nil
}

// SubscribePattern subscribes to a pattern and forwards payloads to out.
func (e *StreamingTestEnv) SubscribePattern(ctx context.Context, pattern string, out chan<- string) error {
	if pattern == "" {
		return errors.New("pattern cannot be empty")
	}
	ps := e.Client.PSubscribe(ctx, pattern)
	if _, err := ps.Receive(ctx); err != nil {
		_ = ps.Close()
		return err
	}
	e.subs = append(e.subs, ps)
	go func(ch <-chan *redis.Message) {
		for msg := range ch {
			if msg == nil {
				continue
			}
			select {
			case out <- msg.Payload:
			case <-ctx.Done():
				return
			}
		}
	}(ps.Channel())
	return nil
}

// SubscribeRaw exposes the native PubSub for lifecycle tests.
func (e *StreamingTestEnv) SubscribeRaw(ctx context.Context, channel string) *redis.PubSub {
	ps := e.Client.Subscribe(ctx, channel)
	// Best-effort confirm; lifecycle tests can also call ReceiveMessage.
	if _, err := ps.ReceiveTimeout(ctx, 2*time.Second); err != nil {
		_ = ps.Close()
	}
	e.subs = append(e.subs, ps)
	return ps
}
