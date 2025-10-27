package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	t.Run("Should create file database at specified path", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "store.db")

		ctx := t.Context()
		store, err := NewStore(ctx, &Config{Path: dbPath})
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, store.Close(ctx))
		})

		info, err := os.Stat(dbPath)
		require.NoError(t, err)
		assert.False(t, info.IsDir())
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("Should create in memory database for memory path", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		store, err := NewStore(ctx, &Config{Path: ":memory:"})
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, store.Close(ctx))
		})

		_, err = store.DB().ExecContext(ctx, "CREATE TABLE memory_check (id INTEGER PRIMARY KEY, value TEXT)")
		require.NoError(t, err)
	})

	t.Run("Should enable foreign keys on connection", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		store, err := NewStore(ctx, &Config{Path: filepath.Join(t.TempDir(), "fk.db")})
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, store.Close(ctx))
		})

		var fkEnabled int
		err = store.DB().QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fkEnabled)
		require.NoError(t, err)
		assert.Equal(t, 1, fkEnabled)
	})

	t.Run("Should handle concurrent connections", func(t *testing.T) {
		ctx := t.Context()
		store, err := NewStore(ctx, &Config{Path: filepath.Join(t.TempDir(), "concurrency.db")})
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, store.Close(ctx))
		})

		_, err = store.DB().
			ExecContext(ctx, "CREATE TABLE IF NOT EXISTS concurrency_test (id INTEGER PRIMARY KEY AUTOINCREMENT, value TEXT)")
		require.NoError(t, err)

		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for i := 0; i < 8; i++ {
			value := i
			wg.Go(func() {
				localCtx, cancel := context.WithTimeout(ctx, time.Second)
				defer cancel()
				_, execErr := store.DB().ExecContext(localCtx, "INSERT INTO concurrency_test (value) VALUES (?)", value)
				if execErr != nil {
					errs <- execErr
				}
			})
		}
		wg.Wait()
		close(errs)
		for execErr := range errs {
			require.NoError(t, execErr)
		}

		var count int
		err = store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM concurrency_test").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 8, count)
	})

	t.Run("Should return error for invalid path", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		_, err := NewStore(ctx, &Config{Path: t.TempDir()})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is a directory")
	})

	t.Run("Should close cleanly", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		store, err := NewStore(ctx, &Config{Path: filepath.Join(t.TempDir(), "close.db")})
		require.NoError(t, err)
		require.NoError(t, store.Close(ctx))
	})

	t.Run("Should perform health check successfully", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		store, err := NewStore(ctx, &Config{Path: filepath.Join(t.TempDir(), "health.db")})
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, store.Close(ctx))
		})

		require.NoError(t, store.HealthCheck(ctx))
	})
}
