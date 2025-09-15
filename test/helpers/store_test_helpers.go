package helpers

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// SetupStoreTestWithSharedDB returns a ctx + shared pool prepared per autoMigrate flag.
func SetupStoreTestWithSharedDB(t *testing.T, autoMigrate bool) (context.Context, *pgxpool.Pool, func()) {
	ctx := context.Background()
	// Use shared container for better performance
	pool, cleanup := GetSharedPostgresDB(ctx, t)
	// Clean existing schema if we need a fresh start
	if !autoMigrate {
		cleanupExistingSchema(ctx, pool)
	} else {
		// Ensure tables exist for tests that expect migrated schema
		require.NoError(t, ensureTablesExist(pool))
	}
	return ctx, pool, func() { cleanup() }
}

// cleanupExistingSchema removes existing tables and migration state for clean testing
func cleanupExistingSchema(ctx context.Context, pool *pgxpool.Pool) {
	// Drop all business tables
	tables := []string{"workflow_states", "task_states", "users", "api_keys", "goose_db_version"}
	for _, table := range tables {
		if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS "+table+" CASCADE"); err != nil {
			// Log error but continue cleanup - table might not exist
			continue
		}
	}
}

// VerifyTablesExist checks if expected tables exist in the database
func VerifyTablesExist(ctx context.Context, t *testing.T, pool *pgxpool.Pool, expectedTables []string) {
	for _, tableName := range expectedTables {
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)",
			tableName).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "table %s should exist", tableName)
	}
}

// VerifyTablesDoNotExist checks if tables do not exist in the database
func VerifyTablesDoNotExist(ctx context.Context, t *testing.T, pool *pgxpool.Pool, tables []string) {
	for _, tableName := range tables {
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)",
			tableName).Scan(&exists)
		require.NoError(t, err)
		require.False(t, exists, "table %s should not exist", tableName)
	}
}

// VerifyMigrationTableExists checks if the goose migration tracking table exists
func VerifyMigrationTableExists(ctx context.Context, t *testing.T, pool *pgxpool.Pool, shouldExist bool) {
	var exists bool
	err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'goose_db_version')").
		Scan(&exists)
	require.NoError(t, err)
	if shouldExist {
		require.True(t, exists, "goose_db_version table should exist to track migration state")
	} else {
		require.False(t, exists, "goose_db_version table should not exist when migrations are disabled")
	}
}
