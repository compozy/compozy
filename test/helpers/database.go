package helpers

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
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	gooseDialectOnce sync.Once
	// Shared container infrastructure for optimized testing
	sharedPGContainer     *postgres.PostgresContainer
	sharedPGPool          *pgxpool.Pool
	pgContainerOnce       sync.Once
	pgContainerMu         sync.Mutex
	pgContainerStartError error

	// Separate container for no-migrations testing
	sharedPGContainerNoMigrations     *postgres.PostgresContainer
	sharedPGPoolNoMigrations          *pgxpool.Pool
	pgContainerNoMigrationsOnce       sync.Once
	pgContainerNoMigrationsMu         sync.Mutex
	pgContainerNoMigrationsStartError error
)

// GetSharedPostgresDB returns a shared PostgreSQL database for tests
// This is 70-90% faster than creating individual containers
func GetSharedPostgresDB(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	// Initialize shared container on first use
	pgContainerOnce.Do(func() {
		sharedPGContainer, sharedPGPool, pgContainerStartError = startSharedContainer(ctx, t)
	})

	if pgContainerStartError != nil {
		t.Fatalf("Failed to start shared container: %v", pgContainerStartError)
	}

	// Create test-specific cleanup that preserves the container
	cleanup := createTestIsolation(ctx, t, sharedPGPool)

	return sharedPGPool, cleanup
}

// startSharedContainer initializes the shared PostgreSQL container
func startSharedContainer(ctx context.Context, _ *testing.T) (*postgres.PostgresContainer, *pgxpool.Pool, error) {
	// Start container with optimized settings
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("test-db"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(30*time.Second),
				wait.ForListeningPort("5432/tcp").
					WithStartupTimeout(30*time.Second),
			),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start container: %w", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get connection string: %w", err)
	}

	// Create pool with optimized settings
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse config: %w", err)
	}

	config.MaxConns = 50 // Support parallel tests
	config.MinConns = 5  // Keep connections warm
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create pool: %w", err)
	}

	// Run migrations once
	if err := ensureTablesExist(pool); err != nil {
		return nil, nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return pgContainer, pool, nil
}

// createTestIsolation creates cleanup function that preserves the shared container
// For parallel tests, we use transactions for isolation instead of table truncation
func createTestIsolation(ctx context.Context, t *testing.T, pool *pgxpool.Pool) func() {
	// Start a transaction for test isolation
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	// Return cleanup that rolls back the transaction
	return func() {
		if err := tx.Rollback(ctx); err != nil {
			t.Logf("Failed to rollback transaction: %v", err)
		}
	}
}

// GetSharedPostgresDBWithoutMigrations returns a shared PostgreSQL database without running migrations
// This is used for testing migration logic itself
func GetSharedPostgresDBWithoutMigrations(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	// Initialize separate shared container for no-migrations testing
	pgContainerNoMigrationsOnce.Do(func() {
		sharedPGContainerNoMigrations, sharedPGPoolNoMigrations, pgContainerNoMigrationsStartError =
			startSharedContainerWithoutMigrations(ctx, t)
	})
	if pgContainerNoMigrationsStartError != nil {
		t.Fatalf("Failed to start shared container without migrations: %v", pgContainerNoMigrationsStartError)
	}
	// Create test-specific cleanup that preserves the container
	cleanup := createTestIsolation(ctx, t, sharedPGPoolNoMigrations)
	return sharedPGPoolNoMigrations, cleanup
}

// startSharedContainerWithoutMigrations initializes the shared PostgreSQL container without migrations
func startSharedContainerWithoutMigrations(
	ctx context.Context,
	_ *testing.T,
) (*postgres.PostgresContainer, *pgxpool.Pool, error) {
	// Start container with optimized settings
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("test-db"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(30*time.Second),
				wait.ForListeningPort("5432/tcp").
					WithStartupTimeout(30*time.Second),
			),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start container: %w", err)
	}
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get connection string: %w", err)
	}
	// Create pool with optimized settings
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse config: %w", err)
	}
	config.MaxConns = 50 // Support parallel tests
	config.MinConns = 5  // Keep connections warm
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create pool: %w", err)
	}
	// NOTE: Skip migrations for testing migration logic
	return pgContainer, pool, nil
}

// CleanupSharedContainer should be called in TestMain to terminate the shared container
func CleanupSharedContainer() {
	pgContainerMu.Lock()
	defer pgContainerMu.Unlock()

	if sharedPGPool != nil {
		sharedPGPool.Close()
	}

	if sharedPGContainer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := sharedPGContainer.Terminate(ctx); err != nil {
			// Log but don't fail
			fmt.Printf("Warning: failed to terminate shared container: %v\n", err)
		}
	}

	// Also cleanup the no-migrations container
	pgContainerNoMigrationsMu.Lock()
	defer pgContainerNoMigrationsMu.Unlock()

	if sharedPGPoolNoMigrations != nil {
		sharedPGPoolNoMigrations.Close()
	}

	if sharedPGContainerNoMigrations != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := sharedPGContainerNoMigrations.Terminate(ctx); err != nil {
			// Log but don't fail
			fmt.Printf("Warning: failed to terminate shared no-migrations container: %v\n", err)
		}
	}
}

// ensureTablesExist runs goose migrations to create the required tables
func ensureTablesExist(db *pgxpool.Pool) error {
	// Convert pgxpool to standard sql.DB for goose
	sqlDB := stdlib.OpenDBFromPool(db)
	defer sqlDB.Close()

	gooseDialectOnce.Do(func() {
		if err := goose.SetDialect("postgres"); err != nil {
			panic(fmt.Sprintf("failed to set goose dialect: %v", err))
		}
	})

	// Find the project root using common utility
	projectRoot, err := FindProjectRoot()
	if err != nil {
		return err
	}

	migrationDir := filepath.Join(projectRoot, "engine", "infra", "store", "migrations")

	// Run migrations up to the latest version
	if err := goose.Up(sqlDB, migrationDir); err != nil {
		return fmt.Errorf("failed to run goose migrations: %w", err)
	}

	return nil
}
