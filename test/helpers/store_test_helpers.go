package helpers

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/pkg/config"
)

// SetupStoreTestWithSharedDB creates a test store using the shared database
// This is much faster than creating individual containers
func SetupStoreTestWithSharedDB(t *testing.T, autoMigrate bool) (context.Context, *store.Store, func()) {
	ctx := context.Background()
	// Reset state to ensure clean test environment
	config.Close(ctx)
	config.ResetForTest()
	store.ResetMigrationsForTest()
	// Use shared container for better performance
	pool, cleanup := GetSharedPostgresDB(ctx, t)
	connStr := pool.Config().ConnString()
	// Clean existing schema if we need a fresh start
	if !autoMigrate {
		cleanupExistingSchema(ctx, pool)
	}
	// Initialize config with fresh state
	err := config.Initialize(ctx, nil, config.NewDefaultProvider())
	require.NoError(t, err)
	// Set up configuration
	cfg := config.Get()
	cfg.Database.AutoMigrate = autoMigrate
	cfg.Database.ConnString = connStr
	// Create store which will trigger migrations if enabled
	testStore, err := store.SetupStoreWithConfig(ctx)
	require.NoError(t, err)
	require.NotNil(t, testStore)
	return ctx, testStore, func() {
		// Clean up store resources but keep shared database
		testStore.DB.Close(ctx)
		config.Close(ctx)
		cleanup() // This will truncate tables for next test
	}
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
