package helpers

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
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

	// Track active tests and serialize truncation
	activeTestCount int64      // number of tests currently using the shared DB
	truncMu         sync.Mutex // ensures only one goroutine truncates tables
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

// createPostgresContainer creates a PostgreSQL container with standard configuration
func createPostgresContainer(ctx context.Context) (*postgres.PostgresContainer, *pgxpool.Pool, error) {
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
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse config: %w", err)
	}
	config.MaxConns = 50
	config.MinConns = 5
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create pool: %w", err)
	}
	return pgContainer, pool, nil
}

// startSharedContainer initializes the shared PostgreSQL container
func startSharedContainer(ctx context.Context, _ *testing.T) (*postgres.PostgresContainer, *pgxpool.Pool, error) {
	pgContainer, pool, err := createPostgresContainer(ctx)
	if err != nil {
		return nil, nil, err
	}
	// Run migrations once
	if err := ensureTablesExist(pool); err != nil {
		return nil, nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	return pgContainer, pool, nil
}

// createTestIsolation creates cleanup function that preserves the shared container
// We use table truncation for test isolation to ensure database consistency
func createTestIsolation(ctx context.Context, t *testing.T, pool *pgxpool.Pool) func() {
	// Increment the number of active tests using the shared DB.
	atomic.AddInt64(&activeTestCount, 1)

	// Return cleanup that will be executed when the individual test finishes.
	return func() {
		// Decrement and capture the remaining count.
		newCount := atomic.AddInt64(&activeTestCount, -1)

		// Only the last finishing test performs the truncation.
		if newCount != 0 {
			return
		}

		// Serialise truncation to avoid races with future tests that might start
		// before this cleanup executes.
		truncMu.Lock()
		defer truncMu.Unlock()

		// Truncate tables in reverse dependency order to avoid foreign key violations.
		tables := []string{
			"task_states",
			"workflow_states",
			"agent_states",
			"tool_states",
		}
		for _, table := range tables {
			query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)
			if _, err := pool.Exec(ctx, query); err != nil {
				t.Logf("Warning: failed to truncate %s: %v", table, err)
			}
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
	pgContainer, pool, err := createPostgresContainer(ctx)
	if err != nil {
		return nil, nil, err
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
