package helpers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/infra/repo"
	"github.com/compozy/compozy/engine/infra/sqlite"
	"github.com/compozy/compozy/pkg/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	postgresStartupTimeout = 3 * time.Minute
	sqliteMigrationTimeout = 45 * time.Second
	sqliteShutdownTimeout  = 5 * time.Second
	testDriverSQLite       = "sqlite"
	testDriverPostgres     = "postgres"
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

// GetSharedPostgresDB returns a shared PostgreSQL database for tests that require Postgres-specific
// behavior. Prefer SetupTestDatabase(t) for new tests so they run against the fast, dependency-free
// SQLite :memory: backend. GetSharedPostgresDB lazily starts a single shared PostgreSQL container
// and connection pool on first use. If initialization fails the test is failed via t.Fatalf. Per-test
// isolation is expected to be achieved by running each test inside its own transaction, so the returned
// cleanup is a no-op.
func GetSharedPostgresDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	pgContainerOnce.Do(func() {
		sharedPGContainer, sharedPGPool, pgContainerStartError = startSharedContainer(t)
	})
	if pgContainerStartError != nil {
		t.Fatalf("Failed to start shared container: %v", pgContainerStartError)
	}
	cleanup := func() {}
	return sharedPGPool, cleanup
}

// createPostgresContainer creates a PostgreSQL container with standard configuration
func createPostgresContainer(ctx context.Context) (*postgres.PostgresContainer, *pgxpool.Pool, error) {
	pgContainer, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test-db"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(postgresStartupTimeout),
				wait.ForListeningPort("5432/tcp").
					WithStartupTimeout(postgresStartupTimeout),
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

// startSharedContainer creates and starts a shared PostgreSQL test container, returns the container
// and a configured pgx connection pool, and ensures database migrations are applied.
// The testing.T parameter is unused.
// Returns an error if container creation, pool setup, or applying migrations fails.
func startSharedContainer(t *testing.T) (*postgres.PostgresContainer, *pgxpool.Pool, error) {
	pgContainer, pool, err := createPostgresContainer(t.Context())
	if err != nil {
		return nil, nil, err
	}
	if err := ensureTablesExist(pool); err != nil {
		return nil, nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	return pgContainer, pool, nil
}

// BeginTestTx starts a transaction pinned to a single connection and registers
// a rollback in t.Cleanup.  It returns both the pgx.Tx and the same cleanup
// BeginTestTx starts a transaction tied to the given testing.T and pgxpool.Pool, and registers a cleanup
// function that will rollback the transaction and release the acquired connection when the test ends.
//
// If one or more pgx.TxOptions are provided, the first is used to configure the transaction.
// The returned cleanup is also registered with t.Cleanup; it performs the rollback using a background
// context with a 5-second timeout and treats pgx.ErrTxClosed as a non-fatal condition.
func BeginTestTx(
	t *testing.T,
	pool *pgxpool.Pool,
	opts ...pgx.TxOptions,
) (pgx.Tx, func()) {
	t.Helper()
	conn, err := pool.Acquire(t.Context())
	if err != nil {
		t.Fatalf("failed to acquire connection: %v", err)
	}
	var txOpts pgx.TxOptions
	if len(opts) > 0 {
		txOpts = opts[0]
	}
	tx, err := conn.BeginTx(t.Context(), txOpts)
	if err != nil {
		conn.Release()
		t.Fatalf("failed to begin transaction: %v", err)
	}
	cleanup := func() {
		// NOTE: Roll back with a separate timeout to guarantee cleanup even if the test context is canceled.
		rollbackCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		if err := tx.Rollback(rollbackCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Logf("warning: rollback failed: %v", err)
		}
		conn.Release()
	}
	t.Cleanup(cleanup)
	return tx, cleanup
}

// GetSharedPostgresTx is a convenience wrapper that obtains the shared pool
// GetSharedPostgresTx obtains the shared test PostgreSQL pool and starts a test-scoped transaction.
// It returns the active pgx.Tx and a cleanup function. The cleanup is registered with t.Cleanup and
// will rollback the transaction and release the underlying connection when the test completes.
// Optional pgx.TxOptions may be provided to configure the transaction.
func GetSharedPostgresTx(t *testing.T, opts ...pgx.TxOptions) (pgx.Tx, func()) {
	t.Helper()
	pool, _ := GetSharedPostgresDB(t)
	return BeginTestTx(t, pool, opts...)
}

// GetSharedPostgresDBWithoutMigrations returns a shared PostgreSQL database without running migrations
// GetSharedPostgresDBWithoutMigrations returns a shared PostgreSQL connection pool backed
// by a lazily-initialized test container that does NOT run migrations.
//
// This helper is intended for tests that need to exercise migration logic itself (so the
// container is left in its pristine state). It initializes a separate shared container
// on first use; if initialization fails the test is fatally failed via t.Fatalf.
// The returned cleanup is a no-op — tests should obtain per-test isolation by using
// BeginTestTx or GetSharedPostgresTx to run each test inside a rollbackable transaction.
func GetSharedPostgresDBWithoutMigrations(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	pgContainerNoMigrationsOnce.Do(func() {
		sharedPGContainerNoMigrations, sharedPGPoolNoMigrations, pgContainerNoMigrationsStartError =
			startSharedContainerWithoutMigrations(t)
	})
	if pgContainerNoMigrationsStartError != nil {
		t.Fatalf("Failed to start shared container without migrations: %v", pgContainerNoMigrationsStartError)
	}
	cleanup := func() {}
	return sharedPGPoolNoMigrations, cleanup
}

// startSharedContainerWithoutMigrations initializes the shared PostgreSQL container without migrations
func startSharedContainerWithoutMigrations(
	t *testing.T,
) (*postgres.PostgresContainer, *pgxpool.Pool, error) {
	pgContainer, pool, err := createPostgresContainer(t.Context())
	if err != nil {
		return nil, nil, err
	}
	// NOTE: Skip migrations for testing migration logic
	return pgContainer, pool, nil
}

// CleanupSharedContainer terminates and cleans up any shared PostgreSQL test containers and connection pools.
//
// It closes and releases the shared pgx connection pools and attempts to terminate both the main
// and "no-migrations" testcontainers, using a 30s timeout for termination. The function is safe
// to call multiple times and is intended to be invoked from TestMain; it will not fatally fail
// on termination errors but will print warnings instead. Cleanup is serialized with internal mutexes.
func CleanupSharedContainer(ctx context.Context) {
	pgContainerMu.Lock()
	if sharedPGPool != nil {
		sharedPGPool.Close()
	}
	if sharedPGContainer != nil {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if err := sharedPGContainer.Terminate(ctx); err != nil {
			fmt.Printf("Warning: failed to terminate shared container: %v\n", err)
		}
		cancel()
	}
	pgContainerMu.Unlock()
	pgContainerNoMigrationsMu.Lock()
	if sharedPGPoolNoMigrations != nil {
		sharedPGPoolNoMigrations.Close()
	}
	if sharedPGContainerNoMigrations != nil {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if err := sharedPGContainerNoMigrations.Terminate(ctx); err != nil {
			fmt.Printf("Warning: failed to terminate shared no-migrations container: %v\n", err)
		}
		cancel()
	}
	pgContainerNoMigrationsMu.Unlock()
}

// ensureTablesExist runs goose migrations to create the required tables
func ensureTablesExist(db *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(db)
	defer sqlDB.Close()
	gooseDialectOnce.Do(func() {
		if err := goose.SetDialect("postgres"); err != nil {
			panic(fmt.Sprintf("failed to set goose dialect: %v", err))
		}
	})
	projectRoot, err := FindProjectRoot()
	if err != nil {
		return err
	}
	migrationDir := filepath.Join(projectRoot, "engine", "infra", "postgres", "migrations")
	if err := goose.Up(sqlDB, migrationDir); err != nil {
		return fmt.Errorf("failed to run goose migrations: %w", err)
	}
	return nil
}

// EnsureTablesExistForTest is a small wrapper to expose ensureTablesExist to tests in other packages.
func EnsureTablesExistForTest(db *pgxpool.Pool) error { return ensureTablesExist(db) }

// SetupDatabaseWithMode configures a test database according to the requested mode and returns
// the sqlx handle alongside an idempotent cleanup function. Supported modes map to backends as:
//   - memory: SQLite :memory: (fastest, fully in-memory)
//   - persistent: SQLite file stored inside t.TempDir()
//   - distributed: PostgreSQL via the shared testcontainer
//
// Example usage:
//
//	db, cleanup := helpers.SetupDatabaseWithMode(t, config.ModeMemory)
//	defer cleanup()
//	// run assertions using db
//
// Callers must defer the returned cleanup; it is also registered with t.Cleanup for safety.
func SetupDatabaseWithMode(t *testing.T, mode string) (*sqlx.DB, func()) {
	t.Helper()

	ctx := NewTestContext(t)
	cfg := config.FromContext(ctx)
	require.NotNil(t, cfg, "config manager missing from context")

	resolvedMode, ok := normalizeMode(mode)
	if !ok {
		t.Fatalf("helpers: unsupported database mode %q", mode)
	}
	cfg.Mode = resolvedMode

	switch resolvedMode {
	case config.ModeMemory:
		cfg.Database.Driver = testDriverSQLite
		cfg.Database.Path = ":memory:"
		cfg.Database.AutoMigrate = true
		cfg.Database.ConnString = ""
		return setupSQLiteDatabaseForMode(ctx, t, cfg.Database.Path, true)
	case config.ModePersistent:
		cfg.Database.Driver = testDriverSQLite
		cfg.Database.AutoMigrate = true
		persistentPath := filepath.Join(t.TempDir(), "compozy.db")
		cfg.Database.Path = persistentPath
		cfg.Database.ConnString = ""
		return setupSQLiteDatabaseForMode(ctx, t, persistentPath, false)
	case config.ModeDistributed:
		cfg.Database.Driver = testDriverPostgres
		cfg.Database.AutoMigrate = false
		cfg.Database.Path = ""
		return setupPostgresDatabaseForMode(ctx, t, cfg)
	default:
		t.Fatalf("helpers: unsupported database mode %q", mode)
		return nil, nil
	}
}

func normalizeMode(mode string) (string, bool) {
	normalized := strings.TrimSpace(strings.ToLower(mode))
	if normalized == "" {
		return config.ModeMemory, true
	}
	switch normalized {
	case config.ModeMemory:
		return config.ModeMemory, true
	case config.ModePersistent:
		return config.ModePersistent, true
	case config.ModeDistributed:
		return config.ModeDistributed, true
	default:
		return "", false
	}
}

func setupSQLiteDatabaseForMode(
	ctx context.Context,
	t *testing.T,
	path string,
	inMemory bool,
) (*sqlx.DB, func()) {
	t.Helper()

	store, err := sqlite.NewStore(ctx, &sqlite.Config{Path: path})
	require.NoError(t, err)

	migrateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sqliteMigrationTimeout)
	defer cancel()
	require.NoError(t, sqlite.ApplyMigrations(migrateCtx, path))

	db := sqlx.NewDb(store.DB(), testDriverSQLite)

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sqliteShutdownTimeout)
			defer cancel()
			if err := store.Close(shutdownCtx); err != nil && !errors.Is(err, sql.ErrConnDone) {
				t.Logf("warning: closing sqlite test database: %v", err)
			}
			if !inMemory {
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					t.Logf("warning: removing sqlite database file: %v", err)
				}
			}
		})
	}
	t.Cleanup(cleanup)

	return db, cleanup
}

func setupPostgresDatabaseForMode(
	ctx context.Context,
	t *testing.T,
	cfg *config.Config,
) (*sqlx.DB, func()) {
	t.Helper()

	pool, containerCleanup, err := SetupTestReposWithRetry(ctx, t)
	require.NoError(t, err)

	cfg.Database.ConnString = pool.Config().ConnString()

	sqlDB := stdlib.OpenDBFromPool(pool)
	db := sqlx.NewDb(sqlDB, "pgx")

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			if err := db.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
				t.Logf("warning: closing postgres test database: %v", err)
			}
			containerCleanup()
		})
	}
	t.Cleanup(cleanup)

	return db, cleanup
}

// SetupTestDatabase provisions a repo provider for tests. By default it returns a SQLite :memory:
// database with automigrations applied so tests run quickly without Docker. To exercise PostgreSQL-
// specific behavior either pass "postgres" explicitly or call SetupPostgresContainer(t).
func SetupTestDatabase(t *testing.T, driver ...string) (*repo.Provider, func()) {
	t.Helper()
	selectedDriver := "sqlite"
	if len(driver) > 0 && driver[0] != "" {
		selectedDriver = driver[0]
	}
	switch selectedDriver {
	case "postgres":
		return setupPostgresTest(t)
	case "sqlite":
		return setupSQLiteTest(t)
	default:
		t.Fatalf("unsupported driver: %s", selectedDriver)
		return nil, nil
	}
}

// SetupPostgresContainer provisions a PostgreSQL-backed repo provider using testcontainers.
// Use this helper only when tests need PostgreSQL-specific features such as pgvector or dialect-
// dependent behavior. Most tests should rely on SetupTestDatabase(t) to benefit from SQLite speed.
func SetupPostgresContainer(t *testing.T) (*repo.Provider, func()) {
	t.Helper()
	return setupPostgresTest(t)
}

func setupPostgresTest(t *testing.T) (*repo.Provider, func()) {
	t.Helper()
	pool, containerCleanup, err := SetupTestReposWithRetry(t.Context(), t)
	require.NoError(t, err)

	ctx := NewTestContext(t)
	cfg := &config.DatabaseConfig{
		Driver:      "postgres",
		ConnString:  pool.Config().ConnString(),
		AutoMigrate: false,
	}
	provider, providerCleanup, err := repo.NewProvider(ctx, cfg)
	require.NoError(t, err)
	cleanup := func() {
		providerCleanup()
		containerCleanup()
	}
	return provider, cleanup
}

func setupSQLiteTest(t *testing.T) (*repo.Provider, func()) {
	t.Helper()
	ctx := NewTestContext(t)
	cfg := &config.DatabaseConfig{
		Driver:      "sqlite",
		Path:        ":memory:",
		AutoMigrate: true,
	}
	provider, cleanup, err := repo.NewProvider(ctx, cfg)
	require.NoError(t, err)
	return provider, cleanup
}
