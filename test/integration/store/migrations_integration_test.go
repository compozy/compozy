package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/test/helpers"
)

// setupTestWithConfig creates a test database and initializes config with the given settings
func setupTestWithConfig(t *testing.T, autoMigrate bool) (context.Context, *pgxpool.Pool, func()) {
	ctx := context.Background()

	// Create test container without migrations
	pool, cleanup := helpers.CreateTestContainerDatabaseWithoutMigrations(ctx, t)

	// Get connection string from pool config
	connStr := pool.Config().ConnString()

	// Initialize config
	err := config.Initialize(ctx, nil, config.NewDefaultProvider())
	require.NoError(t, err)

	// Set up configuration
	cfg := config.Get()
	cfg.Database.AutoMigrate = autoMigrate
	cfg.Database.ConnString = connStr

	return ctx, pool, cleanup
}

func TestMigrations_Integration(t *testing.T) {
	t.Run("Should run migrations automatically when AutoMigrate is enabled", func(t *testing.T) {
		ctx, pool, cleanup := setupTestWithConfig(t, true)
		defer cleanup()

		// Create store which should trigger migrations
		testStore, err := store.SetupStoreWithConfig(ctx)
		require.NoError(t, err)
		require.NotNil(t, testStore)

		// Verify migrations were applied
		var exists bool
		err = pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'workflow_states')").
			Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "workflow_states table should exist after automatic migrations")

		err = pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'task_states')").
			Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "task_states table should exist after automatic migrations")
	})

	// Skip concurrent test for now - config.Initialize uses sync.Once
	// which prevents proper testing of concurrent initialization
	t.Run("Should handle concurrent store initialization with migrations", func(t *testing.T) {
		t.Skip("Skipping concurrent test - config.Initialize uses sync.Once")
	})

	t.Run("Should skip migrations when AutoMigrate is disabled", func(t *testing.T) {
		ctx, pool, cleanup := setupTestWithConfig(t, false)
		defer cleanup()

		// Create store - should NOT trigger migrations
		testStore, err := store.SetupStoreWithConfig(ctx)
		require.NoError(t, err)
		require.NotNil(t, testStore)

		// Verify migrations were NOT applied
		var exists bool
		err = pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'goose_db_version')").
			Scan(&exists)
		require.NoError(t, err)
		assert.False(t, exists, "goose_db_version table should not exist when AutoMigrate is disabled")
	})
}
