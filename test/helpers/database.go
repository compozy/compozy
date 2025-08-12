package helpers

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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

	// No-op cleanup; per-test isolation now achieved with transactions
	cleanup := func() {}

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

// BeginTestTx starts a transaction pinned to a single connection and registers
// a rollback in t.Cleanup.  It returns both the pgx.Tx and the same cleanup
// so callers may invoke it early if desired.
func BeginTestTx(
	ctx context.Context,
	t *testing.T,
	pool *pgxpool.Pool,
	opts ...pgx.TxOptions,
) (pgx.Tx, func()) {
	t.Helper()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("failed to acquire connection: %v", err)
	}

	var txOpts pgx.TxOptions
	if len(opts) > 0 {
		txOpts = opts[0]
	}

	tx, err := conn.BeginTx(ctx, txOpts)
	if err != nil {
		conn.Release()
		t.Fatalf("failed to begin transaction: %v", err)
	}

	cleanup := func() {
		// Use a background context with timeout for rollback to ensure it runs
		// even if the test's context is canceled
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Attempt rollback; ignore ErrTxClosed which means the test already closed it
		if err := tx.Rollback(rollbackCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Logf("warning: rollback failed: %v", err)
		}
		conn.Release()
	}

	// Ensure rollback & release even if the test panics or fails
	t.Cleanup(cleanup)

	return tx, cleanup
}

// GetSharedPostgresTx is a convenience wrapper that obtains the shared pool
// and starts a test-scoped transaction using BeginTestTx.
func GetSharedPostgresTx(
	ctx context.Context,
	t *testing.T,
	opts ...pgx.TxOptions,
) (pgx.Tx, func()) {
	t.Helper()

	pool, _ := GetSharedPostgresDB(ctx, t)
	return BeginTestTx(ctx, t, pool, opts...)
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
	// No-op cleanup; tests should call BeginTestTx / GetSharedPostgresTx for isolation
	cleanup := func() {}
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
	// Cleanup for the main shared container
	pgContainerMu.Lock()
	if sharedPGPool != nil {
		sharedPGPool.Close()
	}
	if sharedPGContainer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := sharedPGContainer.Terminate(ctx); err != nil {
			// Log but don't fail
			fmt.Printf("Warning: failed to terminate shared container: %v\n", err)
		}
		cancel()
	}
	pgContainerMu.Unlock()

	// Cleanup for the no-migrations container
	pgContainerNoMigrationsMu.Lock()
	if sharedPGPoolNoMigrations != nil {
		sharedPGPoolNoMigrations.Close()
	}
	if sharedPGContainerNoMigrations != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := sharedPGContainerNoMigrations.Terminate(ctx); err != nil {
			// Log but don't fail
			fmt.Printf("Warning: failed to terminate shared no-migrations container: %v\n", err)
		}
		cancel()
	}
	pgContainerNoMigrationsMu.Unlock()
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
