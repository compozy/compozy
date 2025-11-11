package cache

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func setupMiniredis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	s := miniredis.NewMiniRedis()
	require.NoError(t, s.Start())
	return s
}

func testPersistenceConfig(dir string) config.RedisPersistenceConfig {
	return config.RedisPersistenceConfig{
		Enabled:            true,
		DataDir:            dir,
		SnapshotInterval:   0,
		SnapshotOnShutdown: true,
		RestoreOnStartup:   true,
	}
}

func newSnapshotTestContext(t *testing.T, cfg *config.Config) *config.Manager {
	t.Helper()
	ctx := t.Context()
	ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
	m := config.NewManager(ctx, config.NewService())
	_, err := m.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
	require.NoError(t, err)
	if cfg != nil {
		// Mutate the active config returned by the manager in place so
		// config.FromContext(ctx) observes our test overrides.
		active := m.Get()
		require.NotNil(t, active)
		active.Redis = cfg.Redis
	}
	t.Cleanup(func() { _ = m.Close(ctx) })
	return m
}

func TestSnapshotManager_SnapshotAndRestore(t *testing.T) {
	t.Run("snapshot then restore", func(t *testing.T) {
		ctx := t.Context()
		ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
		tempDir := t.TempDir()

		mr := setupMiniredis(t)
		defer mr.Close()
		mr.Set("key1", "value1")
		mr.Set("key2", "value2")

		cfg := testPersistenceConfig(tempDir)
		m := newSnapshotTestContext(t, &config.Config{
			Redis: config.RedisConfig{
				Standalone: config.EmbeddedRedisConfig{Persistence: cfg},
			},
		})
		ctx = config.ContextWithManager(ctx, m)

		sm, err := NewSnapshotManager(ctx, mr, cfg)
		require.NoError(t, err)
		defer sm.Stop()

		require.NoError(t, sm.Snapshot(ctx))
		// Close first manager to release Badger directory lock before restore.
		sm.Stop()

		// Create new miniredis and restore
		mr2 := setupMiniredis(t)
		defer mr2.Close()
		sm2, err := NewSnapshotManager(ctx, mr2, cfg)
		require.NoError(t, err)
		defer sm2.Stop()
		require.NoError(t, sm2.Restore(ctx))

		v1, _ := mr2.Get("key1")
		v2, _ := mr2.Get("key2")
		assert.Equal(t, "value1", v1)
		assert.Equal(t, "value2", v2)

		metrics := sm.GetSnapshotMetrics()
		assert.GreaterOrEqual(t, metrics.SnapshotsTaken, int64(1))
		assert.Equal(t, int64(0), metrics.SnapshotFailures)
		assert.Greater(t, metrics.LastSizeBytes, int64(0))
	})
}

func TestSnapshotManager_Periodic(t *testing.T) {
	t.Run("takes periodic snapshots", func(t *testing.T) {
		ctx := t.Context()
		ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
		tempDir := t.TempDir()

		mr := setupMiniredis(t)
		defer mr.Close()
		cfg := testPersistenceConfig(tempDir)
		cfg.SnapshotInterval = 500 * time.Millisecond

		m := newSnapshotTestContext(t, &config.Config{
			Redis: config.RedisConfig{Standalone: config.EmbeddedRedisConfig{Persistence: cfg}},
		})
		ctx = config.ContextWithManager(ctx, m)

		sm, err := NewSnapshotManager(ctx, mr, cfg)
		require.NoError(t, err)
		defer sm.Stop()
		sm.StartPeriodicSnapshots(ctx)

		mr.Set("k", "v1")
		mr.Set("k", "v2")
		require.Eventually(t, func() bool {
			m := sm.GetSnapshotMetrics()
			return m.SnapshotsTaken >= 2
		}, 3*time.Second, 100*time.Millisecond)
	})
}

func TestMiniredisEmbedded_GracefulShutdownSnapshot(t *testing.T) {
	t.Run("snapshot on shutdown + restore on startup", func(t *testing.T) {
		base := t.Context()
		base = logger.ContextWithLogger(base, logger.NewForTests())
		dataDir := filepath.Join(t.TempDir(), "data")
		cfg := &config.Config{Redis: config.RedisConfig{Standalone: config.EmbeddedRedisConfig{
			Persistence: config.RedisPersistenceConfig{
				Enabled:            true,
				DataDir:            dataDir,
				SnapshotInterval:   10 * time.Second, // long, rely on shutdown snapshot
				SnapshotOnShutdown: true,
				RestoreOnStartup:   true,
			},
		}}}
		m := newSnapshotTestContext(t, cfg)
		ctx := config.ContextWithManager(base, m)

		mr, err := NewMiniredisEmbedded(ctx)
		require.NoError(t, err)
		require.NoError(t, mr.Client().Set(ctx, "persist-key", "persist-val", 0).Err())
		require.NoError(t, mr.Close(ctx))

		// New instance should restore the key
		mr2, err := NewMiniredisEmbedded(ctx)
		require.NoError(t, err)
		defer mr2.Close(ctx)
		val, err := mr2.Client().Get(ctx, "persist-key").Result()
		require.NoError(t, err)
		assert.Equal(t, "persist-val", val)
	})
}
