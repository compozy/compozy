package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/test/helpers"
)

const (
	expectedWorkflowColumns = 9
	minWorkflowIndexes      = 5
)

// TestMigrationsOptimized_Integration uses shared database for better performance
func TestMigrationsOptimized_Integration(t *testing.T) {
	// Note: Shared container cleanup is handled by TestMain in operations_test.go

	t.Run("Should create all required tables when AutoMigrate is enabled", func(t *testing.T) {
		ctx, pool, cleanup := helpers.SetupStoreTestWithSharedDB(t, true)
		defer cleanup()

		// Verify all expected tables were created by migrations
		expectedTables := []string{"workflow_states", "task_states", "users", "api_keys"}
		helpers.VerifyTablesExist(ctx, t, pool, expectedTables)

		// Verify migration tracking table exists
		helpers.VerifyMigrationTableExists(ctx, t, pool, true)
	})

	t.Run("Should preserve database state when AutoMigrate is disabled", func(t *testing.T) {
		ctx, pool, cleanup := helpers.SetupStoreTestWithSharedDB(t, false)
		defer cleanup()

		// Verify database remains in pristine state - no migration artifacts
		helpers.VerifyMigrationTableExists(ctx, t, pool, false)

		// Verify business tables were not created
		businessTables := []string{"workflow_states", "task_states", "users", "api_keys"}
		helpers.VerifyTablesDoNotExist(ctx, t, pool, businessTables)
	})

	t.Run("Should error on invalid DSN when applying migrations", func(t *testing.T) {
		// Test that migrations fail gracefully with an invalid connection
		invalidPool, err := pgxpool.New(
			context.Background(),
			"postgres://invalid:invalid@127.0.0.1:1/db?sslmode=disable",
		)
		require.NoError(t, err, "Pool creation should succeed with lazy connection")
		require.NotNil(t, invalidPool, "Pool should be created even with invalid DSN")
		defer invalidPool.Close()

		// Now try to run migrations, which should fail when attempting to connect
		err = helpers.EnsureTablesExistForTest(invalidPool)
		require.Error(t, err, "Should fail when attempting to run migrations on invalid connection")
		require.ErrorContains(t, err, "migrations")
	})

	t.Run("Should handle multiple migration calls idempotently", func(t *testing.T) {
		ctx, pool1, cleanup1 := helpers.SetupStoreTestWithSharedDB(t, true)
		defer cleanup1()

		// First migration should succeed
		expectedTables := []string{"workflow_states", "task_states", "users", "api_keys"}
		helpers.VerifyTablesExist(ctx, t, pool1, expectedTables)

		// Second store creation with migrations should also succeed (idempotent)
		ctx2, pool2, cleanup2 := helpers.SetupStoreTestWithSharedDB(t, true)
		defer cleanup2()

		// Tables should still exist
		helpers.VerifyTablesExist(ctx2, t, pool2, expectedTables)
		helpers.VerifyMigrationTableExists(ctx2, t, pool2, true)
	})

	t.Run("Should verify table schemas and indexes", func(t *testing.T) {
		ctx, pool, cleanup := helpers.SetupStoreTestWithSharedDB(t, true)
		defer cleanup()

		// Verify workflow_states table structure
		var columnCount int
		err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM information_schema.columns
             WHERE table_name = 'workflow_states'`).Scan(&columnCount)
		require.NoError(t, err)
		assert.Equal(
			t,
			expectedWorkflowColumns,
			columnCount,
			"workflow_states should have %d columns",
			expectedWorkflowColumns,
		)

		// Verify indexes exist
		var indexCount int
		err = pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM pg_indexes
			 WHERE tablename = 'workflow_states'`).Scan(&indexCount)
		require.NoError(t, err)
		assert.GreaterOrEqual(
			t,
			indexCount,
			minWorkflowIndexes,
			"workflow_states should have at least %d indexes",
			minWorkflowIndexes,
		)

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
