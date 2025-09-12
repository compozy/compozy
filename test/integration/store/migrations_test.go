package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/test/helpers"
)

// TestMigrationsOptimized_Integration uses shared database for better performance
func TestMigrationsOptimized_Integration(t *testing.T) {
	// Note: Shared container cleanup is handled by TestMain in operations_test.go

	t.Run("Should create all required tables when AutoMigrate is enabled", func(t *testing.T) {
		ctx, testStore, cleanup := helpers.SetupStoreTestWithSharedDB(t, true)
		defer cleanup()

		// Verify all expected tables were created by migrations
		expectedTables := []string{"workflow_states", "task_states", "users", "api_keys"}
		helpers.VerifyTablesExist(ctx, t, testStore.DB.Pool(), expectedTables)

		// Verify migration tracking table exists
		helpers.VerifyMigrationTableExists(ctx, t, testStore.DB.Pool(), true)
	})

	t.Run("Should preserve database state when AutoMigrate is disabled", func(t *testing.T) {
		ctx, testStore, cleanup := helpers.SetupStoreTestWithSharedDB(t, false)
		defer cleanup()

		// Verify database remains in pristine state - no migration artifacts
		helpers.VerifyMigrationTableExists(ctx, t, testStore.DB.Pool(), false)

		// Verify business tables were not created
		businessTables := []string{"workflow_states", "task_states", "users", "api_keys"}
		helpers.VerifyTablesDoNotExist(ctx, t, testStore.DB.Pool(), businessTables)
	})

	t.Run("Should return error when configuration is invalid", func(t *testing.T) {
		ctx := context.Background()

		// Initialize context-based config manager with invalid database settings
		mgr := config.NewManager(nil)
		_, err := mgr.Load(ctx, config.NewDefaultProvider())
		require.NoError(t, err)
		ctx = config.ContextWithManager(ctx, mgr)
		// Set invalid connection configuration
		cfg := mgr.Get()
		cfg.Database.AutoMigrate = true
		cfg.Database.ConnString = "invalid://connection/string"

		// Attempt to create store with invalid config - should fail
		testStore, err := store.SetupStoreWithConfig(ctx)

		// Verify appropriate error handling
		assert.Error(t, err, "should return error for invalid database configuration")
		assert.Nil(t, testStore, "store should be nil when configuration is invalid")
		assert.Contains(
			t,
			err.Error(),
			"parsing connection string",
			"error should indicate connection string parsing issue",
		)

		// Clean up config state
		_ = mgr.Close(ctx)
	})

	t.Run("Should handle multiple migration calls idempotently", func(t *testing.T) {
		ctx, testStore1, cleanup1 := helpers.SetupStoreTestWithSharedDB(t, true)
		defer cleanup1()

		// First migration should succeed
		expectedTables := []string{"workflow_states", "task_states", "users", "api_keys"}
		helpers.VerifyTablesExist(ctx, t, testStore1.DB.Pool(), expectedTables)

		// Second store creation with migrations should also succeed (idempotent)
		ctx2, testStore2, cleanup2 := helpers.SetupStoreTestWithSharedDB(t, true)
		defer cleanup2()

		// Tables should still exist
		helpers.VerifyTablesExist(ctx2, t, testStore2.DB.Pool(), expectedTables)
		helpers.VerifyMigrationTableExists(ctx2, t, testStore2.DB.Pool(), true)
	})

	t.Run("Should verify table schemas and indexes", func(t *testing.T) {
		ctx, testStore, cleanup := helpers.SetupStoreTestWithSharedDB(t, true)
		defer cleanup()

		pool := testStore.DB.Pool()

		// Verify workflow_states table structure
		var columnCount int
		err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM information_schema.columns 
			 WHERE table_name = 'workflow_states'`).Scan(&columnCount)
		require.NoError(t, err)
		assert.Equal(t, 8, columnCount, "workflow_states should have 8 columns")

		// Verify indexes exist
		var indexCount int
		err = pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM pg_indexes 
			 WHERE tablename = 'workflow_states'`).Scan(&indexCount)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, indexCount, 5, "workflow_states should have at least 5 indexes")

		// Verify primary key constraint
		var constraintExists bool
		err = pool.QueryRow(ctx,
			`SELECT EXISTS (
				SELECT 1 FROM information_schema.table_constraints 
				WHERE table_name = 'workflow_states' AND constraint_type = 'PRIMARY KEY'
			)`).Scan(&constraintExists)
		require.NoError(t, err)
		assert.True(t, constraintExists, "workflow_states should have a primary key constraint")
	})
}
