package embedded

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fixtures "github.com/compozy/compozy/test/fixtures/embedded"
	helpers "github.com/compozy/compozy/test/helpers"

	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func TestPersistence_FullCycle(t *testing.T) {
	t.Run("Should persist and restore data across full cycle", func(t *testing.T) {
		ctx := t.Context()
		ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
		dataDir := t.TempDir()

		dataset := map[string]string{
			"user:1": "alice",
			"user:2": "bob",
			"count":  "42",
		}

		// Phase 1: create data and snapshot
		{
			cfg := config.RedisPersistenceConfig{
				Enabled:            true,
				DataDir:            dataDir,
				SnapshotOnShutdown: true,
				RestoreOnStartup:   false,
			}
			env := helpers.SetupEmbeddedWithPersistence(ctx, t, cfg)
			for k, v := range dataset {
				require.NoError(t, env.Client.Set(ctx, k, v, 0).Err())
			}
			require.NoError(t, env.SnapshotManager.Snapshot(ctx))
			env.Cleanup(ctx)
		}

		// Phase 2: restore and verify
		{
			cfg := config.RedisPersistenceConfig{
				Enabled:            true,
				DataDir:            dataDir,
				SnapshotOnShutdown: false,
				RestoreOnStartup:   false,
			}
			env := helpers.SetupEmbeddedWithPersistence(ctx, t, cfg)
			defer env.Cleanup(ctx)
			require.NoError(t, env.SnapshotManager.Restore(ctx))
			for k, expected := range dataset {
				val, err := env.Client.Get(ctx, k).Result()
				require.NoError(t, err)
				assert.Equal(t, expected, val, "key %s mismatch", k)
			}
		}
	})

	t.Run("Should persist data across multiple restarts", func(t *testing.T) {
		ctx := t.Context()
		ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
		dataDir := t.TempDir()

		for cycle := 1; cycle <= 3; cycle++ {
			cfg := config.RedisPersistenceConfig{
				Enabled:            true,
				DataDir:            dataDir,
				SnapshotOnShutdown: true,
				RestoreOnStartup:   false,
			}
			env := helpers.SetupEmbeddedWithPersistence(ctx, t, cfg)
			// Restore any previous snapshot to simulate a true restart cycle
			_ = env.SnapshotManager.Restore(ctx)
			key := fmt.Sprintf("cycle:%d", cycle)
			require.NoError(t, env.Client.Set(ctx, key, fmt.Sprintf("data-%d", cycle), 0).Err())
			require.NoError(t, env.SnapshotManager.Snapshot(ctx))
			env.Cleanup(ctx)
		}

		cfg := config.RedisPersistenceConfig{Enabled: true, DataDir: dataDir}
		env := helpers.SetupEmbeddedWithPersistence(ctx, t, cfg)
		defer env.Cleanup(ctx)
		require.NoError(t, env.SnapshotManager.Restore(ctx))
		for cycle := 1; cycle <= 3; cycle++ {
			key := fmt.Sprintf("cycle:%d", cycle)
			val, err := env.Client.Get(ctx, key).Result()
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("data-%d", cycle), val)
		}
	})
}

func TestPersistence_FailureHandling(t *testing.T) {
	t.Run("Should handle snapshot failures gracefully", func(t *testing.T) {
		ctx := t.Context()
		ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
		dataDir := t.TempDir()

		cfg := config.RedisPersistenceConfig{Enabled: true, DataDir: dataDir}
		env := helpers.SetupEmbeddedWithPersistence(ctx, t, cfg)
		defer env.Cleanup(ctx)

		require.NoError(t, env.Client.Set(ctx, "key1", "value1", 0).Err())
		// Simulate BadgerDB error by closing the manager then calling Snapshot.
		env.SnapshotManager.Stop()
		err := env.SnapshotManager.Snapshot(ctx)
		assert.Error(t, err)

		// Create a fresh manager and verify next snapshot succeeds.
		mr := miniredis.NewMiniRedis()
		require.NoError(t, mr.Start())
		defer mr.Close()
		// Recreate manager over same data dir
		sm2, err2 := cache.NewSnapshotManager(ctx, mr, cfg)
		require.NoError(t, err2)
		defer sm2.Stop()
		require.NoError(t, sm2.Snapshot(ctx))
	})

	t.Run("Should detect and recover from corrupt snapshot", func(t *testing.T) {
		ctx := t.Context()
		ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
		dataDir := t.TempDir()

		// Phase 1: create a valid snapshot
		{
			cfg := config.RedisPersistenceConfig{Enabled: true, DataDir: dataDir}
			env := helpers.SetupEmbeddedWithPersistence(ctx, t, cfg)
			require.NoError(t, env.Client.Set(ctx, "key1", "value1", 0).Err())
			require.NoError(t, env.SnapshotManager.Snapshot(ctx))
			env.Cleanup(ctx)
		}

		// Corrupt a Badger file (truncate first file found)
		files, err := os.ReadDir(dataDir)
		require.NoError(t, err)
		if len(files) > 0 {
			fp := filepath.Join(dataDir, files[0].Name())
			require.NoError(t, os.Truncate(fp, 0))
		}

		// Phase 2: attempt to open DB and restore
		{
			mr := miniredis.NewMiniRedis()
			require.NoError(t, mr.Start())
			defer mr.Close()
			cfg := config.RedisPersistenceConfig{Enabled: true, DataDir: dataDir}

			// NewSnapshotManager may fail to open due to corruption, or restore may fail
			sm, openErr := cache.NewSnapshotManager(ctx, mr, cfg)
			if openErr != nil {
				assert.Error(t, openErr, "expected open to fail due to corruption")
			} else {
				defer sm.Stop()
				rerr := sm.Restore(ctx)
				assert.Error(t, rerr, "restore should fail with corrupt snapshot")
			}
		}

		// System should remain operational with empty state; create fresh data and snapshot
		// Use a fresh directory to recover from corruption.
		{
			freshDir := filepath.Join(t.TempDir(), "fresh")
			cfg := config.RedisPersistenceConfig{Enabled: true, DataDir: freshDir}
			env := helpers.SetupEmbeddedWithPersistence(ctx, t, cfg)
			defer env.Cleanup(ctx)
			_, err := env.Client.Get(ctx, "key1").Result()
			assert.Error(t, err)
			require.NoError(t, env.Client.Set(ctx, "key2", "value2", 0).Err())
			assert.NoError(t, env.SnapshotManager.Snapshot(ctx))
		}
	})
}

func TestPersistence_PeriodicSnapshots(t *testing.T) {
	t.Run("Should take periodic snapshots under load", func(t *testing.T) {
		ctx := t.Context()
		ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
		dataDir := t.TempDir()

		env := helpers.SetupEmbeddedWithPeriodicSnapshots(ctx, t, dataDir, 200*time.Millisecond)

		var writes atomic.Int64
		stop := make(chan struct{})
		go func() {
			for {
				select {
				case <-stop:
					return
				default:
					n := writes.Add(1)
					key := fmt.Sprintf("key:%d", n)
					_ = env.Client.Set(ctx, key, fmt.Sprintf("v:%d", n), 0).Err()
					time.Sleep(10 * time.Millisecond)
				}
			}
		}()

		time.Sleep(2 * time.Second)
		close(stop)
		require.NoError(t, env.SnapshotManager.Snapshot(ctx))
		final := writes.Load()
		t.Logf("wrote %d keys", final)

		// Restart and restore
		env.Cleanup(ctx)
		cfg := config.RedisPersistenceConfig{Enabled: true, DataDir: dataDir}
		env2 := helpers.SetupEmbeddedWithPersistence(ctx, t, cfg)
		defer env2.Cleanup(ctx)
		require.NoError(t, env2.SnapshotManager.Restore(ctx))

		// Verify existence of sample keys
		step := int64(1)
		if final > 50 {
			step = 25
		}
		for i := int64(1); i <= final; i += step {
			key := fmt.Sprintf("key:%d", i)
			_, err := env2.Client.Get(ctx, key).Result()
			assert.NoError(t, err, "key %s should exist", key)
		}
	})
}

func TestPersistence_GracefulShutdown_And_StartupRestore(t *testing.T) {
	t.Run("Should snapshot on graceful shutdown", func(t *testing.T) {
		ctx := t.Context()
		ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
		dataDir := filepath.Join(t.TempDir(), "data")

		// Phase 1: create data and simulate graceful shutdown snapshot
		{
			cfg := config.RedisPersistenceConfig{Enabled: true, DataDir: dataDir}
			env := helpers.SetupEmbeddedWithPersistence(ctx, t, cfg)
			for i := 0; i < 100; i++ {
				key := fmt.Sprintf("key:%d", i)
				require.NoError(t, env.Client.Set(ctx, key, fmt.Sprintf("value:%d", i), 0).Err())
			}
			// Simulate graceful shutdown snapshot
			require.NoError(t, env.SnapshotManager.Snapshot(ctx))
			env.Cleanup(ctx)
		}

		// Phase 2: restore and verify
		{
			cfg := config.RedisPersistenceConfig{Enabled: true, DataDir: dataDir}
			env := helpers.SetupEmbeddedWithPersistence(ctx, t, cfg)
			defer env.Cleanup(ctx)
			require.NoError(t, env.SnapshotManager.Restore(ctx))
			for i := 0; i < 100; i++ {
				key := fmt.Sprintf("key:%d", i)
				val, err := env.Client.Get(ctx, key).Result()
				require.NoError(t, err)
				assert.Equal(t, fmt.Sprintf("value:%d", i), val)
			}
		}
	})

	t.Run("Should restore snapshot on startup when configured", func(t *testing.T) {
		ctx := t.Context()
		ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
		dataDir := t.TempDir()

		// Create snapshot with raw manager
		dataset := fixtures.GenerateKVData("cold", 50)
		{
			cfg := config.RedisPersistenceConfig{Enabled: true, DataDir: dataDir}
			env := helpers.SetupEmbeddedWithPersistence(ctx, t, cfg)
			for _, kv := range dataset {
				require.NoError(t, env.Client.Set(ctx, kv.Key, kv.Value, 0).Err())
			}
			require.NoError(t, env.SnapshotManager.Snapshot(ctx))
			env.Cleanup(ctx)
		}

		// Simulate startup restore by creating a new manager and calling Restore
		{
			cfg := config.RedisPersistenceConfig{Enabled: true, DataDir: dataDir}
			env := helpers.SetupEmbeddedWithPersistence(ctx, t, cfg)
			defer env.Cleanup(ctx)
			require.NoError(t, env.SnapshotManager.Restore(ctx))
			m := fixtures.ToMap(dataset)
			for k, expected := range m {
				v, err := env.Client.Get(ctx, k).Result()
				require.NoError(t, err)
				assert.Equal(t, expected, v)
			}
		}
	})
}
