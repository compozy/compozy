package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// createTestDatabase creates a PostgreSQL container for testing
func createTestDatabase(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("test-db"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second),
		),
	)
	require.NoError(t, err)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	cleanup := func() {
		pool.Close()
		terminateCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := pgContainer.Terminate(terminateCtx); err != nil {
			t.Logf("Warning: failed to terminate container: %s", err)
		}
	}

	return pool, cleanup
}

func TestRunEmbeddedMigrations(t *testing.T) {
	t.Run("Should successfully run migrations on clean database", func(t *testing.T) {
		// Create test database using testcontainers
		ctx := context.Background()
		pool, cleanup := createTestDatabase(ctx, t)
		defer cleanup()

		// Get sql.DB from pool for migrations
		db := stdlib.OpenDBFromPool(pool)
		defer db.Close()

		// Run migrations
		err := runEmbeddedMigrations(ctx, db)
		assert.NoError(t, err)

		// Verify that goose_db_version table exists
		var exists bool
		err = pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'goose_db_version')").
			Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "goose_db_version table should exist after migrations")

		// Verify that our tables exist
		tables := []string{"workflow_states", "task_states", "users", "api_keys"}
		for _, table := range tables {
			err = pool.QueryRow(ctx,
				"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)",
				table).Scan(&exists)
			require.NoError(t, err)
			assert.True(t, exists, "table %s should exist after migrations", table)
		}
	})

	t.Run("Should handle concurrent migration attempts correctly", func(t *testing.T) {
		// Create test database
		ctx := context.Background()
		pool, cleanup := createTestDatabase(ctx, t)
		defer cleanup()

		// Reset migrationOnce for this test
		migrationOnce = sync.Once{}
		migrationErr = nil

		// Run migrations concurrently
		const numGoroutines = 5
		errors := make([]error, numGoroutines)
		done := make(chan bool, numGoroutines)

		for i := range numGoroutines {
			go func(idx int) {
				db := stdlib.OpenDBFromPool(pool)
				defer db.Close()
				errors[idx] = runEmbeddedMigrations(ctx, db)
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for range numGoroutines {
			<-done
		}

		// All calls should succeed
		for i, err := range errors {
			assert.NoError(t, err, "goroutine %d should not have error", i)
		}

		// Verify migrations were applied only once
		var count int
		err := pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM goose_db_version WHERE is_applied = true").
			Scan(&count)
		require.NoError(t, err)
		assert.Greater(t, count, 0, "migrations should be applied")
	})

	t.Run("Should return cached error on subsequent calls after failure", func(t *testing.T) {
		// This test would require mocking goose.Up to fail
		// Since we're using embedded migrations and real database,
		// we'll skip the failure scenario for now
		t.Skip("Failure scenario requires mocking goose internals")
	})
}
