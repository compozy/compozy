package utils

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
)

// -----
// Shared Database Setup
// -----

var (
	sharedTestDB     *pgxpool.Pool
	testDBSetupOnce  sync.Once
	testDBSetupError error
)

// GetSharedTestDB returns a shared test database connection pool
// Uses dedicated test database - no cleanup needed as entire DB can be reset!
func GetSharedTestDB(t *testing.T) *pgxpool.Pool {
	testDBSetupOnce.Do(func() {
		ctx := context.Background()

		// Get test database configuration from environment or use test defaults
		dbHost := GetTestEnvOrDefault("TEST_DB_HOST", "localhost")
		dbPort := GetTestEnvOrDefault("TEST_DB_PORT", "5434") // Different port for test DB
		dbUser := GetTestEnvOrDefault("TEST_DB_USER", "postgres")
		dbPassword := GetTestEnvOrDefault("TEST_DB_PASSWORD", "postgres")
		dbName := GetTestEnvOrDefault("TEST_DB_NAME", "compozy_test") // Dedicated test database

		// Create connection string for test database
		connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&pool_max_conns=20",
			dbUser, dbPassword, dbHost, dbPort, dbName)

		// Create connection pool with optimized settings for testing
		config, err := pgxpool.ParseConfig(connStr)
		if err != nil {
			testDBSetupError = fmt.Errorf("failed to parse test database connection string: %w", err)
			return
		}

		// Optimize pool settings for tests
		config.MaxConns = 20
		config.MinConns = 2
		config.MaxConnLifetime = 1 * time.Hour
		config.MaxConnIdleTime = 30 * time.Minute

		sharedTestDB, err = pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			testDBSetupError = fmt.Errorf("failed to create test database connection pool: %w", err)
			return
		}

		// Ensure we can connect to the test database
		if err = sharedTestDB.Ping(ctx); err != nil {
			testDBSetupError = fmt.Errorf("failed to connect to test database: %w", err)
			return
		}

		// Ensure tables exist once (this is expensive, so we only do it once)
		if err = ensureTablesExist(sharedTestDB); err != nil {
			testDBSetupError = fmt.Errorf("failed to ensure tables exist in test database: %w", err)
			return
		}

		t.Logf("Shared test database connection pool initialized successfully on %s:%s/%s",
			dbHost, dbPort, dbName)
	})

	require.NoError(t, testDBSetupError, "Failed to setup shared test database. "+
		"Make sure test PostgreSQL is running via docker-compose (test-postgresql service)")

	return sharedTestDB
}

// ensureTablesExist runs goose migrations to create the required tables
func ensureTablesExist(db *pgxpool.Pool) error {
	// Convert pgxpool to standard sql.DB for goose
	sqlDB := stdlib.OpenDBFromPool(db)
	defer sqlDB.Close()

	// Set the PostgreSQL dialect for goose
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	// Find the project root using common utility
	projectRoot, err := FindProjectRoot()
	if err != nil {
		return err
	}

	migrationDir := filepath.Join(projectRoot, "engine", "infra", "store", "migrations")

	// Reset migrations to clean state - this drops all tables and resets goose tracking
	if err := goose.Reset(sqlDB, migrationDir); err != nil {
		return fmt.Errorf("failed to reset goose migrations: %w", err)
	}

	// Run migrations up to the latest version
	if err := goose.Up(sqlDB, migrationDir); err != nil {
		return fmt.Errorf("failed to run goose migrations: %w", err)
	}

	return nil
}
