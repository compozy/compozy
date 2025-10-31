package helpers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	redis "github.com/redis/go-redis/v9"
)

// ResourceStoreTestEnv encapsulates a Redis-backed resource store environment
// running in embedded (miniredis-backed) mode for integration tests.
type ResourceStoreTestEnv struct {
	Cache   *cache.Cache
	Store   resources.ResourceStore
	Cleanup func()
}

// SetupEmbeddedResourceStore creates a RedisResourceStore backed by the
// embedded Redis using the mode-aware cache factory. It assumes
// the provided context comes from t.Context().
func SetupEmbeddedResourceStore(ctx context.Context, t *testing.T) *ResourceStoreTestEnv {
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

	// Force embedded mode (memory) for Redis so SetupCache spins up MiniredisEmbedded.
	cfg.Mode = config.ModeMemory
	cfg.Redis.Mode = config.ModeMemory

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
// embedded miniredis instance created via the mode-aware factory.
// It exposes convenience helpers that use native go-redis Pub/Sub types.
type StreamingTestEnv struct {
	Cache   *cache.Cache
	Client  redis.UniversalClient
	subs    []*redis.PubSub
	Cleanup func()
}

// SetupEmbeddedStreaming creates a StreamingTestEnv using embedded miniredis.
// It enforces that logger and configuration are present in the context and
// forces the Redis mode to embedded to exercise the Miniredis backend.
func SetupEmbeddedStreaming(ctx context.Context, t *testing.T) *StreamingTestEnv {
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
	// Force embedded mode explicitly
	cfg.Mode = config.ModeMemory
	cfg.Redis.Mode = config.ModeMemory

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

// PersistenceTestEnv encapsulates an embedded miniredis instance, a go-redis
// client bound to it, and a SnapshotManager configured for persistence.
type PersistenceTestEnv struct {
	Server          *cache.MiniredisEmbedded // optional when using MiniredisEmbedded
	Mini            *miniredis.Miniredis
	Client          *redis.Client
	SnapshotManager *cache.SnapshotManager
	Cleanup         func(context.Context)
}

// SetupEmbeddedWithPersistence creates a miniredis instance, attaches a
// SnapshotManager backed by BadgerDB at dataDir, and returns a go-redis client
// connected to the server. Context must come from t.Context().
func SetupEmbeddedWithPersistence(
	ctx context.Context,
	t *testing.T,
	persistCfg config.RedisPersistenceConfig,
) *PersistenceTestEnv {
	t.Helper()
	if logger.FromContext(ctx) == nil {
		ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
	}
	mgr := config.NewManager(ctx, config.NewService())
	_, err := mgr.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
	require.NoError(t, err)
	active := mgr.Get()
	require.NotNil(t, active)
	active.Mode = config.ModePersistent
	active.Redis.Mode = config.ModePersistent
	active.Redis.Standalone.Persistence = persistCfg
	ctx = config.ContextWithManager(ctx, mgr)
	t.Cleanup(func() { _ = mgr.Close(ctx) })

	mr := miniredis.NewMiniRedis()
	require.NoError(t, mr.Start())
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	require.NoError(t, client.Ping(ctx).Err())

	sm, err := cache.NewSnapshotManager(ctx, mr, persistCfg)
	require.NoError(t, err)

	env := &PersistenceTestEnv{
		Mini:            mr,
		Client:          client,
		SnapshotManager: sm,
	}
	env.Cleanup = func(_ context.Context) {
		if env.SnapshotManager != nil {
			env.SnapshotManager.Stop()
		}
		if env.Client != nil {
			_ = env.Client.Close()
		}
		if env.Mini != nil {
			env.Mini.Close()
		}
	}
	t.Cleanup(func() { env.Cleanup(ctx) })
	return env
}

// SetupEmbeddedWithPeriodicSnapshots is a convenience around
// SetupEmbeddedWithPersistence that configures a custom snapshot interval and
// starts periodic snapshots.
func SetupEmbeddedWithPeriodicSnapshots(
	ctx context.Context,
	t *testing.T,
	dataDir string,
	interval time.Duration,
) *PersistenceTestEnv {
	t.Helper()
	cfg := config.RedisPersistenceConfig{
		Enabled:            true,
		DataDir:            dataDir,
		SnapshotInterval:   interval,
		SnapshotOnShutdown: true,
		RestoreOnStartup:   false,
	}
	env := SetupEmbeddedWithPersistence(ctx, t, cfg)
	env.SnapshotManager.StartPeriodicSnapshots(ctx)
	return env
}

// SetupMiniredisEmbeddedWithConfig creates a MiniredisEmbedded instance that
// uses the configured persistence settings (RestoreOnStartup/SnapshotOnShutdown).
func SetupMiniredisEmbeddedWithConfig(
	ctx context.Context,
	t *testing.T,
	persistCfg config.RedisPersistenceConfig,
) *PersistenceTestEnv {
	t.Helper()
	if logger.FromContext(ctx) == nil {
		ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
	}
	mgr := config.NewManager(ctx, config.NewService())
	_, err := mgr.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
	require.NoError(t, err)
	active := mgr.Get()
	require.NotNil(t, active)
	active.Mode = config.ModePersistent
	active.Redis.Mode = config.ModePersistent
	active.Redis.Standalone.Persistence = persistCfg
	ctx = config.ContextWithManager(ctx, mgr)
	t.Cleanup(func() { _ = mgr.Close(ctx) })

	// Retry a few times to avoid transient Badger directory lock contention on CI/macOS.
	var (
		mr      *cache.MiniredisEmbedded
		openErr error
	)
	for i := 0; i < 5; i++ {
		mr, openErr = cache.NewMiniredisEmbedded(ctx)
		if openErr == nil || !strings.Contains(openErr.Error(), "Cannot acquire directory lock") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.NoError(t, openErr)
	env := &PersistenceTestEnv{
		Server: mr,
		Client: mr.Client(),
	}
	env.Cleanup = func(c context.Context) { _ = mr.Close(c) }
	t.Cleanup(func() { env.Cleanup(ctx) })
	return env
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
